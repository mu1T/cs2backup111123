package steam

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	// AvatarFailCooldown is the cooldown duration before retrying a failed avatar fetch.
	AvatarFailCooldown = 10 * time.Minute
	// MaxAvatarWorkers limits concurrent avatar fetch routines.
	MaxAvatarWorkers = 4
	// DefaultAvatarTimeout is the HTTP timeout for avatar/profile queries.
	DefaultAvatarTimeout = 4 * time.Second
	// MaxFailCacheEntries bounds the in-memory failure cache to prevent unbounded growth.
	MaxFailCacheEntries = 200
)

var ioCopyBufPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 32*1024) // 32KB buffer per ADR 0002
		return &buf
	},
}

// HTTPDoer interface allows mocking HTTP calls in unit tests.
type HTTPDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// SteamProfileXML represents the response structure of Steam community profile XML.
type SteamProfileXML struct {
	XMLName      xml.Name `xml:"profile"`
	SteamID      string   `xml:"steamID"`
	AvatarIcon   string   `xml:"avatarIcon"`
	AvatarMedium string   `xml:"avatarMedium"`
	AvatarFull   string   `xml:"avatarFull"`
}

// AvatarManager handles disk caching, failure cooldown, and bounded worker fetching.
type AvatarManager struct {
	mu           sync.RWMutex
	configDir    string
	avatarDir    string
	failFilePath string
	failCache    map[string]time.Time
	httpClient   HTTPDoer
	workerSem    chan struct{}
	cacheMgr     *CacheManager
	steamPath    string
	// onUpdated 在昵称 / 头像有更新后调用，用来通知前端刷新卡片。
	onUpdated func()
	// 待抓取队列。原来「槽位满了就直接丢弃」，账号一多就有一大半永远排不上，
	// 昵称自然补不齐；改成排队后每个账号在一轮里都能轮到。
	pending    []uint32
	pendingSet map[uint32]bool
	draining   bool
	// bg 追踪 EnqueueFetch 派出去的抓取协程。抓取失败时会异步写 .avatar_failed_cache，
	// 如果不等待，调用方（尤其是测试的临时目录清理）可能在写入前就删目录。
	bg sync.WaitGroup
}

// NewAvatarManager creates and initializes an AvatarManager.
func NewAvatarManager(configDir string, cacheMgr *CacheManager, client HTTPDoer) (*AvatarManager, error) {
	if configDir == "" {
		configDir = "config"
	}
	avatarDir := filepath.Join(configDir, "avatarcache")
	if err := os.MkdirAll(avatarDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create avatar dir: %w", err)
	}

	if client == nil {
		client = &http.Client{
			Timeout: DefaultAvatarTimeout,
		}
	}

	am := &AvatarManager{
		configDir:    configDir,
		avatarDir:    avatarDir,
		failFilePath: filepath.Join(configDir, ".avatar_failed_cache"),
		failCache:    make(map[string]time.Time),
		httpClient:   client,
		workerSem:    make(chan struct{}, MaxAvatarWorkers),
		pendingSet:   make(map[uint32]bool),
		cacheMgr:     cacheMgr,
	}

	am.loadFailCache()
	return am, nil
}

// SetUpdateNotifier 注册「昵称或头像已更新」的回调。
//
// 后台抓到的昵称如果只写进缓存不通知前端，用户得手动重开面板才看得到，等于白抓。
func (am *AvatarManager) SetUpdateNotifier(fn func()) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.onUpdated = fn
}

func (am *AvatarManager) notifyUpdated() {
	am.mu.RLock()
	fn := am.onUpdated
	am.mu.RUnlock()
	if fn != nil {
		fn()
	}
}

// SetSteamPath updates the local Steam installation path used for scanning local Steam avatar cache.
func (am *AvatarManager) SetSteamPath(path string) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.steamPath = path
}

func (am *AvatarManager) loadFailCache() {
	am.mu.Lock()
	defer am.mu.Unlock()

	data, err := os.ReadFile(am.failFilePath)
	if err != nil {
		return
	}

	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}
		parts := strings.Split(line, "|")
		if len(parts) == 2 {
			aid := parts[0]
			sec, err := strconv.ParseFloat(parts[1], 64)
			if err == nil {
				failTime := time.Unix(int64(sec), int64((sec-float64(int64(sec)))*1e9))
				if time.Since(failTime) < AvatarFailCooldown && len(am.failCache) < MaxFailCacheEntries {
					am.failCache[aid] = failTime
				}
			}
		}
	}
}

func (am *AvatarManager) saveFailCacheLocked() {
	var sb strings.Builder
	now := time.Now()
	for aid, t := range am.failCache {
		if now.Sub(t) < AvatarFailCooldown {
			fmt.Fprintf(&sb, "%s|%.4f\n", aid, float64(t.UnixNano())/1e9)
		}
	}
	_ = os.WriteFile(am.failFilePath, []byte(sb.String()), 0644)
}

func (am *AvatarManager) isFailedRecently(accountID string) bool {
	am.mu.RLock()
	defer am.mu.RUnlock()

	failTime, ok := am.failCache[accountID]
	if !ok {
		return false
	}
	return time.Since(failTime) < AvatarFailCooldown
}

func (am *AvatarManager) recordFailure(accountID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	now := time.Now()
	// Bound failCache: evict expired entries first
	if len(am.failCache) >= MaxFailCacheEntries {
		for aid, t := range am.failCache {
			if now.Sub(t) >= AvatarFailCooldown {
				delete(am.failCache, aid)
			}
		}
		// If still at capacity, evict oldest entry
		for len(am.failCache) >= MaxFailCacheEntries {
			var oldestAID string
			var oldestTime time.Time
			first := true
			for aid, t := range am.failCache {
				if first || t.Before(oldestTime) {
					oldestTime = t
					oldestAID = aid
					first = false
				}
			}
			if oldestAID != "" {
				delete(am.failCache, oldestAID)
			} else {
				break
			}
		}
	}

	am.failCache[accountID] = now
	am.saveFailCacheLocked()
}

func (am *AvatarManager) clearFailure(accountID string) {
	am.mu.Lock()
	defer am.mu.Unlock()

	delete(am.failCache, accountID)
	am.saveFailCacheLocked()
}

// GetLocalAvatarDataURI checks for a cached avatar on disk and returns a data:image/png;base64,... URI.
// If not found in config/avatarcache, it checks Steam's own avatar cache and copies it if present.
func (am *AvatarManager) GetLocalAvatarDataURI(steam64 uint64) (string, bool) {
	steam64Str := strconv.FormatUint(steam64, 10)
	cachedPath := filepath.Join(am.avatarDir, steam64Str+".png")

	// 1. Check local config/avatarcache
	if data, err := os.ReadFile(cachedPath); err == nil && len(data) > 0 {
		return "data:image/png;base64," + base64.StdEncoding.EncodeToString(data), true
	}

	// 2. Check Steam's local cache
	am.mu.RLock()
	steamPath := am.steamPath
	am.mu.RUnlock()

	if steamPath != "" {
		steamAvatarDir := filepath.Join(steamPath, "config", "avatarcache")
		// Check .png or .jpg
		for _, ext := range []string{".png", ".jpg"} {
			srcFile := filepath.Join(steamAvatarDir, steam64Str+ext)
			if srcData, err := os.ReadFile(srcFile); err == nil && len(srcData) > 0 {
				_ = os.WriteFile(cachedPath, srcData, 0644)
				mime := "image/png"
				if ext == ".jpg" {
					mime = "image/jpeg"
				}
				return fmt.Sprintf("data:%s;base64,%s", mime, base64.StdEncoding.EncodeToString(srcData)), true
			}
		}
	}

	return "", false
}

// EnqueueFetch triggers an asynchronous fetch of profile XML and avatar via the worker pool.
func (am *AvatarManager) EnqueueFetch(accountID uint32) {
	if am.isFailedRecently(strconv.FormatUint(uint64(accountID), 10)) {
		return
	}

	am.mu.Lock()
	if am.pendingSet == nil {
		am.pendingSet = make(map[uint32]bool)
	}
	if am.pendingSet[accountID] {
		am.mu.Unlock()
		return
	}
	am.pendingSet[accountID] = true
	am.pending = append(am.pending, accountID)
	start := !am.draining
	if start {
		am.draining = true
		am.bg.Add(1)
	}
	am.mu.Unlock()

	if start {
		go am.runFetchQueue()
	}
}

// runFetchQueue 逐个消化待抓取账号。
//
// 只用一个协程：既不会像「每个账号一个 goroutine」那样在账号多时把槽位抢光、
// 后面的直接被丢弃，也能把请求速率压在比较客气的水平。
func (am *AvatarManager) runFetchQueue() {
	defer am.bg.Done()

	for {
		am.mu.Lock()
		if len(am.pending) == 0 {
			// 与 EnqueueFetch 的判断在同一把锁里，不会漏掉刚入队的账号
			am.draining = false
			am.mu.Unlock()
			return
		}
		aid := am.pending[0]
		am.pending = am.pending[1:]
		delete(am.pendingSet, aid)
		am.mu.Unlock()

		// 排队期间可能已被别的路径标记失败（例如手动解析昵称时失败）
		if am.isFailedRecently(strconv.FormatUint(uint64(aid), 10)) {
			continue
		}

		am.workerSem <- struct{}{}
		_, _, _ = am.fetchProfileAndAvatar(context.Background(), aid)
		<-am.workerSem
	}
}

// WaitBackground 阻塞直到已派出的头像抓取全部结束。
// 抓取失败会写 .avatar_failed_cache，调用方在删除配置目录（如测试的 TempDir）之前应当先等它。
// 只应在不再派发新抓取之后调用。
func (am *AvatarManager) WaitBackground() {
	am.bg.Wait()
}

// FetchProfileAndAvatarSync fetches user info and avatar synchronously with timeout.
func (am *AvatarManager) FetchProfileAndAvatarSync(ctx context.Context, accountID uint32) (string, string, error) {
	accountIDStr := strconv.FormatUint(uint64(accountID), 10)
	if am.isFailedRecently(accountIDStr) {
		return "", "", fmt.Errorf("in failure cooldown")
	}

	// Acquire worker slot
	select {
	case am.workerSem <- struct{}{}:
		defer func() { <-am.workerSem }()
	case <-ctx.Done():
		return "", "", ctx.Err()
	}

	return am.fetchProfileAndAvatar(ctx, accountID)
}

func (am *AvatarManager) fetchProfileAndAvatar(ctx context.Context, accountID uint32) (string, string, error) {
	accountIDStr := strconv.FormatUint(uint64(accountID), 10)
	steam64 := AccountIDToSteam64(accountID)
	steam64Str := strconv.FormatUint(steam64, 10)

	xmlURL := fmt.Sprintf("https://steamcommunity.com/profiles/%s/?xml=1", steam64Str)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, xmlURL, nil)
	if err != nil {
		am.recordFailure(accountIDStr)
		return "", "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) CS2Backup/2.0")

	resp, err := am.httpClient.Do(req)
	if err != nil {
		am.recordFailure(accountIDStr)
		return "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		am.recordFailure(accountIDStr)
		return "", "", fmt.Errorf("bad status: %d", resp.StatusCode)
	}

	xmlBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		am.recordFailure(accountIDStr)
		return "", "", err
	}

	var profile SteamProfileXML
	if err := xml.Unmarshal(xmlBytes, &profile); err != nil {
		am.recordFailure(accountIDStr)
		return "", "", fmt.Errorf("XML parse failed: %w", err)
	}

	// 注意：社区 XML 里昵称放在 <steamID> 标签里（<steamID64> 才是数字 ID），
	// 所以这里读的是 SteamID 字段，而不是某个叫 PersonaName 的标签。
	username := strings.TrimSpace(profile.SteamID)
	changed := false
	if am.cacheMgr != nil {
		if username != "" {
			prev, _ := am.cacheMgr.GetUsername(accountIDStr)
			prevEntry, _ := am.cacheMgr.GetUsernameEntry(accountIDStr)
			// 名字没变但来源还不是 xml 时也要回写：否则「本地来源」的标记会一直留着，
			// 下次启动又被判定成需要刷新，等于每次开都去问一遍。
			if prev != username || prevEntry.Source != UsernameSourceXML {
				_ = am.cacheMgr.SetUsernameFrom(accountIDStr, username, UsernameSourceXML)
			}
			if prev != username {
				changed = true
			}
		} else {
			// 对方没设置社区资料，接口里连 <steamID> 都没有。
			// 记一个 none 标记，免得每次启动都来问；同时进失败冷却，本会话内不再重试。
			_ = am.cacheMgr.SetUsernameFrom(accountIDStr, "", UsernameSourceNone)
			am.recordFailure(accountIDStr)
		}
	}

	avatarURL := profile.AvatarFull
	if avatarURL == "" {
		avatarURL = profile.AvatarMedium
	}
	if avatarURL == "" {
		avatarURL = profile.AvatarIcon
	}

	var dataURI string
	if avatarURL != "" {
		imgReq, err := http.NewRequestWithContext(ctx, http.MethodGet, avatarURL, nil)
		if err == nil {
			imgResp, err := am.httpClient.Do(imgReq)
			if err == nil && imgResp.StatusCode == http.StatusOK {
				defer imgResp.Body.Close()

				bufPtr := ioCopyBufPool.Get().(*[]byte)
				buf := *bufPtr
				defer ioCopyBufPool.Put(bufPtr)

				var imgBuf bytes.Buffer
				_, _ = io.CopyBuffer(&imgBuf, imgResp.Body, buf)
				imgBytes := imgBuf.Bytes()

				if len(imgBytes) > 0 {
					destPath := filepath.Join(am.avatarDir, steam64Str+".png")
					_ = os.WriteFile(destPath, imgBytes, 0644)
					dataURI = "data:image/png;base64," + base64.StdEncoding.EncodeToString(imgBytes)
				}
			}
		}
	}

	am.clearFailure(accountIDStr)
	if changed {
		am.notifyUpdated()
	}
	return username, dataURI, nil
}
