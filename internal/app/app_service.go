package app

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"cs2backup/internal/backup"
	"cs2backup/internal/crosshair"
	"cs2backup/internal/lan"
	"cs2backup/internal/logger"
	"cs2backup/internal/steam"
	"cs2backup/internal/video"
)

// AppInfo encapsulates application metadata exposed to frontend and logs.
type AppInfo struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	Description string `json:"description"`
	OS          string `json:"os"`
	Arch        string `json:"arch"`
}

// EventEmitterFunc defines the signature for emitting events to frontend.
type EventEmitterFunc func(ctx context.Context, name string, data ...interface{})

// DirectoryPickerFunc defines the signature for prompting the user to select a directory.
type DirectoryPickerFunc func(title string) (string, error)

// FilePickerFunc defines the signature for prompting the user to select files.
type FilePickerFunc func(title string, filterDisplay, filterPattern string) ([]string, error)

// SingleFilePickerFunc defines the signature for prompting the user to select a single file.
type SingleFilePickerFunc func(title, filterDisplay, filterPattern string) (string, error)

// PathOpenerFunc defines the signature for opening paths in the OS file manager.
type PathOpenerFunc func(targetPath string) error

var defaultPathOpener = func(targetPath string) error {
	cleanPath := filepath.Clean(targetPath)
	if runtime.GOOS == "windows" {
		return exec.Command("explorer.exe", cleanPath).Start()
	} else if runtime.GOOS == "darwin" {
		return exec.Command("open", cleanPath).Start()
	}
	return exec.Command("xdg-open", cleanPath).Start()
}

// AppService provides core app metadata, logging, Steam coordination, and backup/restore services.
type AppService struct {
	mu               sync.RWMutex
	ctx              context.Context
	logBuffer        *logger.LogBuffer
	emitter          EventEmitterFunc
	dirPicker        DirectoryPickerFunc
	filePicker       FilePickerFunc
	info             AppInfo
	steamService     *steam.Service
	backupService    *backup.Service
	singlePicker     SingleFilePickerFunc
	videoManager     *video.PresetManager
	crosshairPresets *crosshair.PresetStore
	lanServer        *lan.Server
	discoveryService *lan.DiscoveryService
	lanClient        *lan.Client
	pathOpener       PathOpenerFunc
}

// NewAppService initializes a new AppService with default metadata, log buffer, Steam service, and Backup service.
func NewAppService() *AppService {
	return NewAppServiceWithConfigDir("config")
}

// NewAppServiceWithConfigDir initializes a new AppService with specified config directory.
func NewAppServiceWithConfigDir(configDir string) *AppService {
	cm, _ := steam.NewCacheManager(configDir)
	steamSvc, _ := steam.NewService(configDir, nil)
	backupSvc, _ := backup.NewService(configDir, cm)
	if steamSvc != nil && backupSvc != nil {
		backupSvc.SetSteamPath(steamSvc.GetSteamPath())
	}
	videoMgr, _ := video.NewPresetManager(configDir)
	crosshairPresets, _ := crosshair.NewPresetStore(configDir)
	lanSrv := lan.NewServer()
	discSvc := lan.NewDiscoveryService(lanSrv.GetPort(), lanSrv.GetShareName(), "2.0.0")
	lanCli := lan.NewClient()

	return &AppService{
		logBuffer:        logger.NewLogBuffer(1000),
		steamService:     steamSvc,
		backupService:    backupSvc,
		videoManager:     videoMgr,
		crosshairPresets: crosshairPresets,
		lanServer:        lanSrv,
		discoveryService: discSvc,
		lanClient:        lanCli,
		pathOpener:       defaultPathOpener,
		info: AppInfo{
			Name:        "CS2 Config Backup & Sync",
			Version:     "2.0.0",
			Description: "CS2 个人配置备份与局域网同步工具 (Golang + Vue 3)",
			OS:          runtime.GOOS,
			Arch:        runtime.GOARCH,
		},
	}
}

// SetContext assigns the active runtime context (e.g. from Wails OnStartup).
func (s *AppService) SetContext(ctx context.Context) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ctx = ctx
}

// SetEventEmitter configures the function used to emit events to frontend.
func (s *AppService) SetEventEmitter(emitter EventEmitterFunc) {
	s.mu.Lock()
	s.emitter = emitter
	steamSvc := s.steamService
	s.mu.Unlock()

	// 后台补到昵称 / 头像后推一个事件，前端据此只刷新卡片上的身份信息，
	// 不用整页重载，也不用等用户重开面板。
	if steamSvc != nil {
		steamSvc.SetUserUpdatedHook(func() {
			s.mu.RLock()
			ctx := s.ctx
			s.mu.RUnlock()
			emitter(ctx, "steam:users-updated")
		})
	}
}

// SetDirectoryPicker configures the function used to prompt user for a directory.
func (s *AppService) SetDirectoryPicker(picker DirectoryPickerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.dirPicker = picker
}

// SetFilePicker configures the function used to prompt user for multiple files.

// SetSingleFilePicker configures the function used to prompt user for a single file.
func (s *AppService) SetSingleFilePicker(picker SingleFilePickerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.singlePicker = picker
}

// SetPathOpener configures the function used to open paths in external explorer.
func (s *AppService) SetPathOpener(opener PathOpenerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pathOpener = opener
}

// SetVideoManager allows overriding the Video preset manager instance (e.g. in tests).
func (s *AppService) SetVideoManager(mgr *video.PresetManager) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.videoManager = mgr
}

func (s *AppService) SetFilePicker(picker FilePickerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.filePicker = picker
}

// SetSteamService allows overriding the Steam service instance (e.g. in tests).
func (s *AppService) SetSteamService(svc *steam.Service) {
	s.mu.Lock()
	s.steamService = svc
	if s.backupService != nil && svc != nil {
		s.backupService.SetSteamPath(svc.GetSteamPath())
	}
	s.mu.Unlock()
}

// SetBackupService allows overriding the Backup service instance (e.g. in tests).
func (s *AppService) SetBackupService(svc *backup.Service) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupService = svc
}

// SetLANServer allows overriding or accessing the LAN server instance (e.g. in tests).
func (s *AppService) SetLANServer(srv *lan.Server) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lanServer = srv
}

// SetDiscoveryService allows overriding the discovery service instance (e.g. in tests).
func (s *AppService) SetDiscoveryService(disc *lan.DiscoveryService) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.discoveryService = disc
}

// SetLANClient allows overriding the LAN client instance (e.g. in tests).
func (s *AppService) SetLANClient(cli *lan.Client) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.lanClient = cli
}

// SetAssets sets embedded frontend SPA assets on the LAN server.
func (s *AppService) SetAssets(assets fs.FS) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.lanServer != nil {
		s.lanServer.SetAssets(assets)
	}
}

// GetAppInfo returns the application metadata.
func (s *AppService) GetAppInfo() AppInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.info
}

// GetLogs returns all retained log entries from the buffer.
func (s *AppService) GetLogs() []logger.LogEntry {
	return s.logBuffer.GetAll()
}

// ClearLogs clears the in-memory log buffer.
func (s *AppService) ClearLogs() {
	s.logBuffer.Clear()
}

// EmitLog appends a log entry to the buffer and emits a "log" event if emitter & context exist.
func (s *AppService) EmitLog(level, message string) logger.LogEntry {
	entry := s.logBuffer.Add(level, message)

	s.mu.RLock()
	emitter := s.emitter
	ctx := s.ctx
	s.mu.RUnlock()

	if emitter != nil && ctx != nil {
		emitter(ctx, "log", entry)
	}

	return entry
}

// Convenience helpers
func (s *AppService) LogInfo(msg string) logger.LogEntry {
	return s.EmitLog(logger.LevelInfo, msg)
}

func (s *AppService) LogSuccess(msg string) logger.LogEntry {
	return s.EmitLog(logger.LevelSuccess, msg)
}

func (s *AppService) LogWarning(msg string) logger.LogEntry {
	return s.EmitLog(logger.LevelWarning, msg)
}

func (s *AppService) LogError(msg string) logger.LogEntry {
	return s.EmitLog(logger.LevelError, msg)
}

// Steam Service Delegations

// GetSteamPath returns the currently configured Steam path.
func (s *AppService) GetSteamPath() string {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return ""
	}
	return svc.GetSteamPath()
}

// SetSteamPath validates and persists a new Steam installation path.
func (s *AppService) SetSteamPath(path string) error {
	s.mu.RLock()
	svc := s.steamService
	bkSvc := s.backupService
	s.mu.RUnlock()

	if svc == nil {
		return fmt.Errorf("steam service not initialized")
	}
	if err := svc.SetSteamPath(path); err != nil {
		s.LogError(fmt.Sprintf("设置 Steam 路径失败: %v", err))
		return err
	}

	if bkSvc != nil {
		bkSvc.SetSteamPath(path)
	}

	s.LogSuccess(fmt.Sprintf("已成功更新 Steam 路径: %s", path))
	return nil
}

// DetectSteamPath tries to automatically find and configure the Steam directory.
func (s *AppService) DetectSteamPath() (string, error) {
	s.mu.RLock()
	svc := s.steamService
	bkSvc := s.backupService
	s.mu.RUnlock()

	if svc == nil {
		return "", fmt.Errorf("steam service not initialized")
	}
	path, err := svc.DetectSteamPath()
	if err != nil {
		s.LogError(fmt.Sprintf("自动侦测 Steam 目录失败: %v", err))
		return "", err
	}

	if bkSvc != nil {
		bkSvc.SetSteamPath(path)
	}

	s.LogSuccess(fmt.Sprintf("成功检测到本地 Steam 目录: %s", path))
	return path, nil
}

// EnsureSteamPath 首次启动时自动探测 Steam 目录，并让两个服务都用上。
//
// 以前只有 GetUsers() 会在取账号时顺手探一次，而且探完不通知备份服务。
// 结果是首屏路径栏空着、游戏 cfg 目录 / 快照 / 准星编辑器的 cfg 目录全都拿不到，
// 用户得手点一次「自动检测」才正常。
//
// 探测失败（没装 Steam、装在非常规位置）不阻断启动，留给用户在路径设置里手选。
func (s *AppService) EnsureSteamPath() {
	s.mu.RLock()
	svc := s.steamService
	bkSvc := s.backupService
	s.mu.RUnlock()

	if svc == nil || svc.GetSteamPath() != "" {
		return
	}

	path, err := svc.DetectSteamPath()
	if err != nil {
		s.LogInfo("未自动检测到 Steam 目录，可在「路径设置」里手动指定")
		return
	}

	// 备份服务只在自己构造和 AppService.SetSteamPath 时同步 Steam 目录，
	// 探测这条路必须显式告诉它，否则它会一直以为没配。
	if bkSvc != nil {
		bkSvc.SetSteamPath(path)
	}
	s.LogSuccess(fmt.Sprintf("已自动检测到本地 Steam 目录: %s", path))
}

// WaitBackground 阻塞直到后台任务（目前是异步头像抓取）结束。
// 抓取失败会异步写 config/.avatar_failed_cache，删除配置目录前需要先等它写完。
func (s *AppService) WaitBackground() {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc != nil {
		svc.WaitBackground()
	}
}

// Backup Service Delegations
// GetBackupPath returns the current backup directory path.
func (s *AppService) GetBackupPath() string {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return ""
	}
	return svc.GetBackupPath()
}

// SetBackupPath validates and saves the new backup path.
func (s *AppService) SetBackupPath(path string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}
	if err := svc.SetBackupPath(path); err != nil {
		s.LogError(fmt.Sprintf("设置备份路径失败: %v", err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已成功更新备份仓库路径: %s", path))
	return nil
}

// SelectBackupDirectory prompts the user for a backup directory and saves it.
func (s *AppService) SelectBackupDirectory() (string, error) {
	s.mu.RLock()
	picker := s.dirPicker
	s.mu.RUnlock()

	if picker == nil {
		return "", fmt.Errorf("directory picker not configured")
	}

	dir, err := picker("选择备份存储目录")
	if err != nil {
		s.LogError(fmt.Sprintf("选择备份目录异常: %v", err))
		return "", err
	}
	if dir == "" {
		return "", nil // user cancelled
	}
	// 只挑不存，避免「浏览完再点取消」时路径已经被改掉。
	return dir, nil
}

// GetCS2Path returns the manually configured CS2 cfg directory, or "" when auto-detecting.
func (s *AppService) GetCS2Path() string {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return ""
	}
	return svc.GetCS2Path()
}

// SetCS2Path 手动指定 CS2 配置目录；传空串恢复自动探测。
func (s *AppService) SetCS2Path(path string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}
	if err := svc.SetCS2Path(path); err != nil {
		s.LogError(fmt.Sprintf("设置 CS2 目录失败: %v", err))
		return err
	}
	if strings.TrimSpace(path) == "" {
		s.LogSuccess("已恢复自动探测 CS2 配置目录")
	} else {
		s.LogSuccess(fmt.Sprintf("已更新 CS2 配置目录: %s", svc.GetCS2Path()))
	}
	return nil
}

// SelectCS2Directory 打开目录选择器，把用户选中的目录归一成 game/csgo/cfg 并保存。
func (s *AppService) SelectCS2Directory() (string, error) {
	s.mu.RLock()
	picker := s.dirPicker
	s.mu.RUnlock()

	if picker == nil {
		return "", fmt.Errorf("directory picker not configured")
	}

	dir, err := picker("选择 CS2 安装目录（或 game/csgo/cfg 目录）")
	if err != nil {
		s.LogError(fmt.Sprintf("选择 CS2 目录异常: %v", err))
		return "", err
	}
	if dir == "" {
		return "", nil // user cancelled
	}
	// 只挑不存：归一化后返回给界面显示，落盘留到「保存」。
	// 归一化顺便做一次校验，选错层级能当场报错。
	normalized, err := backup.NormalizeCS2CFGPath(dir)
	if err != nil {
		return "", err
	}
	return normalized, nil
}

// OpenPathInExplorer opens the specified path in Windows Explorer (or OS file manager).
func (s *AppService) OpenPathInExplorer(targetPath string) error {
	s.mu.RLock()
	opener := s.pathOpener
	s.mu.RUnlock()
	if opener == nil {
		opener = defaultPathOpener
	}

	trimmed := strings.TrimSpace(targetPath)
	if trimmed == "" {
		return fmt.Errorf("路径不能为空")
	}
	clean := filepath.Clean(trimmed)
	if _, err := os.Stat(clean); err != nil {
		s.LogWarning(fmt.Sprintf("尝试打开不存在的路径: %s", clean))
		return fmt.Errorf("路径不存在: %s", clean)
	}

	if err := opener(clean); err != nil {
		s.LogError(fmt.Sprintf("在资源管理器中打开路径失败 [%s]: %v", clean, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已在资源管理器中打开路径: %s", clean))
	return nil
}

// GetCS2CFGPath locates the CS2 game cfg directory.
func (s *AppService) GetCS2CFGPath() (string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return "", fmt.Errorf("backup service not initialized")
	}
	return svc.GetCS2CFGPath()
}
