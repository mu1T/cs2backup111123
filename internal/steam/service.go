package steam

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"sync"
	"time"
)

// UserCardInfo encapsulates complete user information required for frontend cards.
type UserCardInfo struct {
	AccountID        string `json:"account_id"`
	SteamID64        string `json:"steam64"`
	Username         string `json:"username"`
	AccountName      string `json:"account_name"`
	RememberPassword bool   `json:"remember_password"`
	IsRecent         bool   `json:"is_recent"`
	AvatarURL        string `json:"avatar_url"`
	Notes            string `json:"notes"`
}

// Service coordinates Steam detection, account scanning, VDF parsing, and cache management.
type Service struct {
	mu        sync.RWMutex
	steamPath string
	cacheMgr  *CacheManager
	avatarMgr *AvatarManager
}

// NewService creates a Steam service configured with the given config directory.
func NewService(configDir string, client HTTPDoer) (*Service, error) {
	cm, err := NewCacheManager(configDir)
	if err != nil {
		return nil, fmt.Errorf("failed to init CacheManager: %w", err)
	}

	am, err := NewAvatarManager(configDir, cm, client)
	if err != nil {
		return nil, fmt.Errorf("failed to init AvatarManager: %w", err)
	}

	svc := &Service{
		cacheMgr:  cm,
		avatarMgr: am,
	}

	// Try loading saved path
	saved := cm.GetSavedSteamPath()
	if saved != "" && ValidateSteamPath(saved) == nil {
		svc.steamPath = saved
		am.SetSteamPath(saved)
	}

	return svc, nil
}

// WaitBackground 阻塞直到异步头像抓取结束。
// 抓取失败会异步写 config/.avatar_failed_cache，删除配置目录前需要先等它写完。
func (s *Service) WaitBackground() {
	if s.avatarMgr != nil {
		s.avatarMgr.WaitBackground()
	}
}

// SetUserUpdatedHook 注册「某个账号的昵称 / 头像已更新」的回调。
//
// 后台抓取是异步的，不通知前端的话用户得重开面板才看得到新名字。
func (s *Service) SetUserUpdatedHook(fn func()) {
	s.mu.RLock()
	am := s.avatarMgr
	s.mu.RUnlock()
	if am != nil {
		am.SetUpdateNotifier(fn)
	}
}

// GetSteamPath returns the currently configured Steam installation path.
func (s *Service) GetSteamPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.steamPath
}

// SetSteamPath validates and saves the Steam installation path.
func (s *Service) SetSteamPath(path string) error {
	cleanPath := filepath.Clean(path)
	if err := ValidateSteamPath(cleanPath); err != nil {
		return err
	}

	s.mu.Lock()
	s.steamPath = cleanPath
	s.avatarMgr.SetSteamPath(cleanPath)
	s.mu.Unlock()

	return s.cacheMgr.SaveSteamPath(cleanPath)
}

// DetectSteamPath automatically finds the Steam directory, validates it, and saves it.
func (s *Service) DetectSteamPath() (string, error) {
	detected, err := detectSteamPathFn()
	if err != nil {
		return "", err
	}

	if err := s.SetSteamPath(detected); err != nil {
		return "", err
	}

	return detected, nil
}

// GetUsers returns all user accounts found in userdata/, enriched with VDF metadata, cached notes, and avatars.
// The active/most recent account is placed at index 0.
func (s *Service) GetUsers() ([]UserCardInfo, error) {
	s.mu.RLock()
	path := s.steamPath
	s.mu.RUnlock()

	if path == "" {
		// Attempt auto-detection if not yet configured
		detected, err := s.DetectSteamPath()
		if err != nil {
			return nil, fmt.Errorf("steam path not configured: %w", err)
		}
		path = detected
	}

	// Scan userdata for numeric account IDs
	accountIDs, err := ScanUserdataAccountIDs(path)
	if err != nil {
		return nil, err
	}

	// Parse loginusers.vdf
	vdfCandidates := []string{
		filepath.Join(path, "config", "loginusers.vdf"),
		filepath.Join(path, "userdata", "0", "config", "loginusers.vdf"),
	}

	loginUsers := make(map[uint32]*LoginUserInfo)
	for _, vdfPath := range vdfCandidates {
		if _, err := os.Stat(vdfPath); err == nil {
			if users, err := ParseLoginUsersFile(vdfPath); err == nil {
				loginUsers = users
				break
			}
		}
	}

	// Build user cards
	cards := make([]UserCardInfo, 0, len(accountIDs))
	var activeCard *UserCardInfo
	var otherCards []UserCardInfo

	for _, aid := range accountIDs {
		aidStr := strconv.FormatUint(uint64(aid), 10)
		s64 := AccountIDToSteam64(aid)
		s64Str := strconv.FormatUint(s64, 10)

		username := ""
		accountName := ""
		rememberPassword := false
		isRecent := false

		entry, hasEntry := s.cacheMgr.GetUsernameEntry(aidStr)

		// 1. 社区接口给的昵称 —— 服务端真值，优先级最高。
		//    本地文件只是「上次这台机器用该账号时的快照」，用户在别处改名后本地还是旧的，
		//    所以只要手上有接口给的版本，就以它为准。
		if hasEntry && entry.Source == UsernameSourceXML && entry.Username != "" {
			username = entry.Username
		}

		// 2. loginusers.vdf（本机登录过的账号，Steam 随登录更新）
		if lu, ok := loginUsers[aid]; ok {
			if username == "" && lu.PersonaName != "" {
				username = lu.PersonaName
				s.cacheLocalUsername(aidStr, lu.PersonaName, entry, hasEntry)
			}
			accountName = lu.AccountName
			rememberPassword = lu.RememberPassword
			isRecent = lu.MostRecent
		}

		// 3. userdata/<id>/config/localconfig.vdf —— 覆盖面比 loginusers 广，离线可用
		if username == "" {
			if local := ReadLocalPersonaName(path, aidStr); local != "" {
				username = local
				s.cacheLocalUsername(aidStr, local, entry, hasEntry)
			}
		}

		// 4. 其它来源的缓存兜底
		if username == "" && hasEntry && entry.Username != "" {
			username = entry.Username
		}

		// 5. 都没有就只能显示 32 位账号 ID，并标记出来去查一次接口
		hasRealName := username != ""
		if !hasRealName {
			username = aidStr
		}

		notes := s.cacheMgr.GetNote(aidStr)

		avatarURI, found := s.avatarMgr.GetLocalAvatarDataURI(s64)
		// 头像和昵称原本是绑在一起抓的：只要头像命中缓存就不再请求。
		// 结果是「头像有了、名字还缺」的账号永远补不上名字，所以这里拆开判断。
		if !found || needsUsernameRefresh(entry, hasEntry) {
			s.avatarMgr.EnqueueFetch(aid)
		}

		card := UserCardInfo{
			AccountID:        aidStr,
			SteamID64:        s64Str,
			Username:         username,
			AccountName:      accountName,
			RememberPassword: rememberPassword,
			IsRecent:         isRecent,
			AvatarURL:        avatarURI,
			Notes:            notes,
		}

		if isRecent && activeCard == nil {
			activeCard = &card
		} else {
			otherCards = append(otherCards, card)
		}
	}

	// Sort other accounts numerically ascending by AccountID
	sort.Slice(otherCards, func(i, j int) bool {
		ai, _ := strconv.ParseUint(otherCards[i].AccountID, 10, 32)
		aj, _ := strconv.ParseUint(otherCards[j].AccountID, 10, 32)
		return ai < aj
	})

	// Active user card is placed first
	if activeCard != nil {
		cards = append(cards, *activeCard)
	}
	cards = append(cards, otherCards...)

	return cards, nil
}

// SaveUserNote persists custom user notes for an account ID.
func (s *Service) SaveUserNote(accountID string, note string) error {
	return s.cacheMgr.SetNote(accountID, note)
}

// ResolveUserName resolves the human-readable username for an account ID from cache, VDF, or community.
func (s *Service) ResolveUserName(accountID string) (string, error) {
	if cached, ok := s.cacheMgr.GetUsername(accountID); ok && cached != "" {
		return cached, nil
	}

	aid, err := strconv.ParseUint(accountID, 10, 32)
	if err != nil {
		return accountID, fmt.Errorf("invalid account ID: %w", err)
	}

	// Try synchronous fetch
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	name, _, err := s.avatarMgr.FetchProfileAndAvatarSync(ctx, uint32(aid))
	if err == nil && name != "" {
		return name, nil
	}

	return accountID, nil
}

// IsSteamRunning checks if steam.exe or steamwebhelper.exe is currently running.
func (s *Service) IsSteamRunning() (bool, error) {
	return IsSteamRunning()
}

// ShutdownSteam requests Steam to gracefully exit (-shutdown), waiting up to 5s before fallback termination.
func (s *Service) ShutdownSteam() error {
	path := s.GetSteamPath()
	if path == "" {
		detected, err := s.DetectSteamPath()
		if err == nil {
			path = detected
		}
	}
	return ShutdownSteam(path)
}

// StartSteam launches steam.exe in detached background mode.
// If launchGame is true, CS2 is launched directly via -applaunch 730.
func (s *Service) StartSteam(launchGame ...bool) error {
	path := s.GetSteamPath()
	if path == "" {
		detected, err := s.DetectSteamPath()
		if err == nil {
			path = detected
		}
	}
	return StartSteam(path, launchGame...)
}

// StartCS2Game launches CS2 directly via steam.exe -applaunch 730.
func (s *Service) StartCS2Game() error {
	return s.StartSteam(true)
}

// GetLaunchOptions retrieves the CS2 LaunchOptions for the specified account ID.
func (s *Service) GetLaunchOptions(accountID string) (string, error) {
	path := s.GetSteamPath()
	if path == "" {
		detected, err := s.DetectSteamPath()
		if err == nil {
			path = detected
		}
	}
	if path == "" {
		return "", ErrSteamNotFound
	}
	mgr := NewLaunchOptionsManager(path)
	return mgr.GetLaunchOptions(accountID)
}

// SetLaunchOptions updates the CS2 LaunchOptions for the specified account ID.
func (s *Service) SetLaunchOptions(accountID, options string) error {
	path := s.GetSteamPath()
	if path == "" {
		detected, err := s.DetectSteamPath()
		if err == nil {
			path = detected
		}
	}
	if path == "" {
		return ErrSteamNotFound
	}
	mgr := NewLaunchOptionsManager(path)
	return mgr.SetLaunchOptions(accountID, options)
}

// SwitchSteamAccount switches the active Steam account to the specified accountName.
func (s *Service) SwitchSteamAccount(accountName string) error {
	path := s.GetSteamPath()
	if path == "" {
		detected, err := s.DetectSteamPath()
		if err == nil {
			path = detected
		}
	}
	if path == "" {
		return ErrSteamNotFound
	}
	return SwitchSteamAccount(path, accountName)
}

// usernameRefreshWindow 是「多久再去社区接口核对一次昵称」。
//
// 昵称是低频变化的数据，没必要每次启动都全量请求；一天一次足够跟上改名。
const usernameRefreshWindow = 24 * time.Hour

// needsUsernameRefresh 判断要不要为该账号去社区接口核对昵称。
//
//   - 手上没有接口给的版本 → 要查；
//   - 查过但对方没设置社区资料（source=none）→ 隔一天再试，别每次启动都问；
//   - 有接口版本 → 超过一天再核对。
func needsUsernameRefresh(entry CachedUser, ok bool) bool {
	if !ok {
		return true
	}
	if entry.UpdatedAt == "" {
		return true
	}
	// 时间戳是本地时区写进去的，必须按本地时区解析；用 time.Parse 会当 UTC，
	// 在 UTC+8 上等于把时间往前推了 8 小时，24 小时的窗口会失真。
	updated, err := time.ParseInLocation("2006-01-02T15:04:05.000000", entry.UpdatedAt, time.Local)
	if err != nil {
		return true
	}
	if entry.Source != UsernameSourceXML && entry.Source != UsernameSourceNone {
		return true
	}
	return time.Since(updated) >= usernameRefreshWindow
}

// cacheLocalUsername 把本地来源的昵称存一份缓存。
//
// 缓存里已经是接口版本（source=xml）时不能覆盖：接口是服务端真值，
// 本地只是这台机器上次用该账号时的快照。
func (s *Service) cacheLocalUsername(accountID, name string, entry CachedUser, hasEntry bool) {
	if hasEntry && entry.Source == UsernameSourceXML {
		return
	}
	_ = s.cacheMgr.SetUsername(accountID, name)
}
