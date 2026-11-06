package steam

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"cs2backup/internal/atomicfile"
)

// CachedUser represents an entry in config/user_cache.json.
type CachedUser struct {
	Username  string `json:"username"`
	UpdatedAt string `json:"updated_at"`
	// Source 记录这个昵称是从哪来的，决定它在展示优先级里的位置：
	//   "xml"  —— Steam 社区接口给的，服务端真值，最优先；
	//   "none" —— 查过接口但对方没设置社区资料，短期内不必再查；
	//   其他   —— 本地来源或历史数据。
	Source string `json:"source,omitempty"`
}

// 昵称来源标记。
const (
	UsernameSourceXML  = "xml"
	UsernameSourceNone = "none"
)

// ConfigData represents data saved in config/config.json.
type ConfigData struct {
	SteamPath string                 `json:"steam_path"`
	Extra     map[string]interface{} `json:"-"`
}

// CacheManager manages config/user_cache.json, config/user_notes.json, and config/config.json.
type CacheManager struct {
	mu            sync.RWMutex
	configDir     string
	userCache     map[string]CachedUser
	userNotes     map[string]string
	userCachePath string
	userNotesPath string
	configPath    string
}

// NewCacheManager loads or creates cache files in the given config directory.
func NewCacheManager(configDir string) (*CacheManager, error) {
	if configDir == "" {
		configDir = "config"
	}
	if err := os.MkdirAll(configDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create config dir %s: %w", configDir, err)
	}

	cm := &CacheManager{
		configDir:     configDir,
		userCache:     make(map[string]CachedUser),
		userNotes:     make(map[string]string),
		userCachePath: filepath.Join(configDir, "user_cache.json"),
		userNotesPath: filepath.Join(configDir, "user_notes.json"),
		configPath:    filepath.Join(configDir, "config.json"),
	}

	_ = cm.loadUserCache()
	_ = cm.loadUserNotes()

	return cm, nil
}

func (cm *CacheManager) loadUserCache() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.userCachePath)
	if err != nil {
		return err
	}

	var m map[string]CachedUser
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}

	// 顺手清掉「昵称 == 账号 ID」的历史脏数据（旧版本留下的），
	// 让它们在下次扫描时走本地 localconfig 或网络重新取一次。
	dirty := false
	for id, u := range m {
		if strings.TrimSpace(u.Username) == id {
			delete(m, id)
			dirty = true
		}
	}
	cm.userCache = m
	if dirty {
		_ = cm.saveUserCacheLocked()
	}
	return nil
}

func (cm *CacheManager) saveUserCacheLocked() error {
	data, err := json.MarshalIndent(cm.userCache, "", "  ")
	if err != nil {
		return err
	}
	_, werr := atomicfile.Write(cm.userCachePath, data, atomicfile.Options{Mode: 0o644})
	return werr
}

func (cm *CacheManager) loadUserNotes() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.userNotesPath)
	if err != nil {
		return err
	}

	var m map[string]string
	if err := json.Unmarshal(data, &m); err != nil {
		return err
	}
	cm.userNotes = m
	return nil
}

func (cm *CacheManager) saveUserNotesLocked() error {
	data, err := json.MarshalIndent(cm.userNotes, "", "  ")
	if err != nil {
		return err
	}
	_, werr := atomicfile.Write(cm.userNotesPath, data, atomicfile.Options{Mode: 0o644})
	return werr
}

// GetUsername retrieves the cached persona name for an Account ID.
func (cm *CacheManager) GetUsername(accountID string) (string, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	u, ok := cm.userCache[accountID]
	if !ok || u.Username == "" {
		return "", false
	}
	// 历史版本曾把账号 ID 当成昵称写进缓存，这种值等于没有名字，
	// 不能让调用方拿它去显示（否则界面上就是一串数字）。
	if strings.TrimSpace(u.Username) == accountID {
		return "", false
	}
	return u.Username, true
}

// SetUsername saves or updates the cached persona name for an Account ID.
func (cm *CacheManager) SetUsername(accountID, username string) error {
	return cm.SetUsernameFrom(accountID, username, "")
}

// SetUsernameFrom 同 SetUsername，但记下来源（xml / none / 空）。
func (cm *CacheManager) SetUsernameFrom(accountID, username, source string) error {
	username = strings.TrimSpace(username)
	if accountID == "" {
		return nil
	}
	// 账号 ID 不是昵称。写进来只会让界面显示一串数字，且挡住后续的真实名字。
	if username == accountID {
		return nil
	}
	// 空昵称只在「明确查过、对方没设置」时才落盘，用来记一个「别再查了」的时间戳。
	if username == "" && source != UsernameSourceNone {
		return nil
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.userCache[accountID] = CachedUser{
		Username:  username,
		UpdatedAt: time.Now().Format("2006-01-02T15:04:05.000000"),
		Source:    source,
	}
	return cm.saveUserCacheLocked()
}

// GetUsernameEntry 返回完整缓存条目（带来源与时间戳）。
//
// 与 GetUsername 一样，把「昵称等于账号 ID」的历史脏数据视为不存在。
func (cm *CacheManager) GetUsernameEntry(accountID string) (CachedUser, bool) {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	u, ok := cm.userCache[accountID]
	if !ok {
		return CachedUser{}, false
	}
	if strings.TrimSpace(u.Username) == accountID {
		return CachedUser{}, false
	}
	return u, true
}

// GetNote retrieves the user note for an Account ID.
func (cm *CacheManager) GetNote(accountID string) string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	return cm.userNotes[accountID]
}

// SetNote saves or updates the user note for an Account ID.
func (cm *CacheManager) SetNote(accountID, note string) error {
	if accountID == "" {
		return fmt.Errorf("accountID is empty")
	}

	cm.mu.Lock()
	defer cm.mu.Unlock()

	if note == "" {
		delete(cm.userNotes, accountID)
	} else {
		cm.userNotes[accountID] = note
	}

	return cm.saveUserNotesLocked()
}

// GetSavedSteamPath reads steam_path from config/config.json.
func (cm *CacheManager) GetSavedSteamPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}

	if sp, ok := raw["steam_path"].(string); ok {
		return sp
	}
	return ""
}

// SaveSteamPath writes steam_path to config/config.json, preserving all other existing fields.
func (cm *CacheManager) SaveSteamPath(steamPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	raw := make(map[string]interface{})
	if data, err := os.ReadFile(cm.configPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	raw["steam_path"] = steamPath
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	_, werr := atomicfile.Write(cm.configPath, data, atomicfile.Options{Mode: 0o644})
	return werr
}

// GetSavedCS2Path reads cs2_path from config/config.json.
//
// 这是用户手动指定的 CS2 配置目录（game/csgo/cfg）。留空表示交给自动探测，
// 适用于一台机器上装了不止一份 CS2、或游戏装在非 Steam 默认库的情况。
func (cm *CacheManager) GetSavedCS2Path() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}

	if p, ok := raw["cs2_path"].(string); ok {
		return p
	}
	return ""
}

// SaveCS2Path writes cs2_path to config/config.json, preserving all other existing fields.
// 传空串等于恢复自动探测。
func (cm *CacheManager) SaveCS2Path(cs2Path string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	raw := make(map[string]interface{})
	if data, err := os.ReadFile(cm.configPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	raw["cs2_path"] = cs2Path
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	_, werr := atomicfile.Write(cm.configPath, data, atomicfile.Options{Mode: 0o644})
	return werr
}

// GetSavedBackupPath reads backup_path from config/config.json.
func (cm *CacheManager) GetSavedBackupPath() string {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return ""
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return ""
	}

	if bp, ok := raw["backup_path"].(string); ok {
		return bp
	}
	return ""
}

// SaveBackupPath writes backup_path to config/config.json, preserving all other existing fields.
// GetBackupRetention 读取「每个账号保留多少个历史版本」。没配过就返回 0，
// 由调用方决定默认值（备份包用 10）。
func (cm *CacheManager) GetBackupRetention() int {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return 0
	}
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		return 0
	}
	switch v := raw["backup_retention"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

// SaveBackupRetention 持久化历史版本保留上限。
func (cm *CacheManager) SaveBackupRetention(n int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	raw := make(map[string]interface{})
	if data, err := os.ReadFile(cm.configPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	raw["backup_retention"] = n
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	_, werr := atomicfile.Write(cm.configPath, data, atomicfile.Options{Mode: 0o644})
	return werr
}

func (cm *CacheManager) SaveBackupPath(backupPath string) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	raw := make(map[string]interface{})
	if data, err := os.ReadFile(cm.configPath); err == nil {
		_ = json.Unmarshal(data, &raw)
	}

	raw["backup_path"] = backupPath
	data, err := json.MarshalIndent(raw, "", "  ")
	if err != nil {
		return err
	}

	_, werr := atomicfile.Write(cm.configPath, data, atomicfile.Options{Mode: 0o644})
	return werr
}
