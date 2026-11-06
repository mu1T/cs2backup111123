package main

import (
	"context"
	"fmt"
	"io/fs"

	"github.com/wailsapp/wails/v2/pkg/runtime"

	"cs2backup/internal/app"
	"cs2backup/internal/backup"
	"cs2backup/internal/crosshair"
	"cs2backup/internal/lan"
	"cs2backup/internal/logger"
	"cs2backup/internal/steam"
	"cs2backup/internal/video"
)

// App struct manages Wails lifecycle and frontend bindings
type App struct {
	ctx        context.Context
	appService *app.AppService
}

// safeEventEmitter emits events only when a valid Wails context is active
func safeEventEmitter(ctx context.Context, name string, data ...interface{}) {
	if ctx == nil || ctx.Value("events") == nil {
		return
	}
	runtime.EventsEmit(ctx, name, data...)
}

// NewApp creates a new App application struct
func NewApp() *App {
	return NewAppWithConfigDir("config")
}

// NewAppWithConfigDir creates an App struct using the specified configuration directory
func NewAppWithConfigDir(configDir string) *App {
	svc := app.NewAppServiceWithConfigDir(configDir)
	svc.SetEventEmitter(safeEventEmitter)
	return &App{
		appService: svc,
	}
}

// startup is called when the app starts. The context is saved
// so we can call runtime methods.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.appService.SetContext(ctx)

	a.appService.SetDirectoryPicker(func(title string) (string, error) {
		return runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
			Title: title,
		})
	})

	a.appService.SetFilePicker(func(title, filterDisplay, filterPattern string) ([]string, error) {
		return runtime.OpenMultipleFilesDialog(a.ctx, runtime.OpenDialogOptions{
			Title: title,
			Filters: []runtime.FileFilter{
				{
					DisplayName: filterDisplay,
					Pattern:     filterPattern,
				},
			},
		})
	})

	a.appService.SetSingleFilePicker(func(title, filterDisplay, filterPattern string) (string, error) {
		return runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
			Title: title,
			Filters: []runtime.FileFilter{
				{
					DisplayName: filterDisplay,
					Pattern:     filterPattern,
				},
			},
		})
	})

	info := a.appService.GetAppInfo()
	a.appService.LogSuccess(fmt.Sprintf("%s v%s 启动成功 (平台: %s/%s)", info.Name, info.Version, info.OS, info.Arch))

	// 首次启动没有保存过 Steam 路径时自动探一次，让首屏路径栏就有值，
	// 也让备份服务拿到 Steam 目录（游戏 cfg、快照、准星都靠它）。
	a.appService.EnsureSteamPath()
}

// GetAppInfo returns application metadata
func (a *App) GetAppInfo() app.AppInfo {
	return a.appService.GetAppInfo()
}

// GetLogs returns all retained log entries
func (a *App) GetLogs() []logger.LogEntry {
	return a.appService.GetLogs()
}

// ClearLogs clears system logs
func (a *App) ClearLogs() {
	a.appService.ClearLogs()
}

// EmitLog allows logging a message directly from frontend or backend
func (a *App) EmitLog(level, message string) logger.LogEntry {
	return a.appService.EmitLog(level, message)
}

// Steam API bindings

// GetSteamPath returns the currently configured Steam path
func (a *App) GetSteamPath() string {
	return a.appService.GetSteamPath()
}

// SetSteamPath validates and persists a new Steam installation path
func (a *App) SetSteamPath(path string) error {
	return a.appService.SetSteamPath(path)
}

// DetectSteamPath automatically discovers the Steam path on the host system
func (a *App) DetectSteamPath() (string, error) {
	return a.appService.DetectSteamPath()
}

// SelectSteamDirectory opens a native directory picker dialog and configures the selected Steam path
func (a *App) SelectSteamDirectory() (string, error) {
	if a.ctx == nil {
		return "", fmt.Errorf("runtime context not ready")
	}
	dir, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择 Steam 安装根目录",
	})
	if err != nil {
		return "", err
	}
	if dir == "" {
		return "", nil // user cancelled
	}
	// 只把路径交回界面，不在这里落盘：用户可能随后点「取消」，
	// 提前保存就回不去了。真正的写入由路径设置弹窗的「保存」统一完成。
	return dir, nil
}

// GetUsers returns all user cards discovered from the Steam installation
func (a *App) GetUsers() ([]steam.UserCardInfo, error) {
	return a.appService.GetUsers()
}

// SaveUserNote persists custom user notes for an account ID
func (a *App) SaveUserNote(accountID string, note string) error {
	return a.appService.SaveUserNote(accountID, note)
}

// GetRemoteBackups returns labels for downloaded remote backup archives (not Steam accounts)
func (a *App) GetRemoteBackups() []steam.UserCardInfo {
	return a.appService.GetRemoteBackups()
}

// ResolveUserName returns the username for an account ID
func (a *App) ResolveUserName(accountID string) (string, error) {
	return a.appService.ResolveUserName(accountID)
}

// Backup API bindings

// GetBackupPath returns the active backup repository path
func (a *App) GetBackupPath() string {
	return a.appService.GetBackupPath()
}

// SetBackupPath validates and saves the new backup repository path
func (a *App) SetBackupPath(path string) error {
	return a.appService.SetBackupPath(path)
}

// SelectBackupDirectory prompts the user for a backup directory using a native directory picker
func (a *App) SelectBackupDirectory() (string, error) {
	return a.appService.SelectBackupDirectory()
}

// OpenPathInExplorer opens a filesystem path in the host file manager
func (a *App) OpenPathInExplorer(targetPath string) error {
	return a.appService.OpenPathInExplorer(targetPath)
}

// GetCS2CFGPath locates the CS2 game cfg directory
func (a *App) GetCS2CFGPath() (string, error) {
	return a.appService.GetCS2CFGPath()
}

// GetCS2Path returns the manually configured CS2 cfg directory, or "" when auto-detecting
func (a *App) GetCS2Path() string {
	return a.appService.GetCS2Path()
}

// SetCS2Path overrides the CS2 cfg directory; pass an empty string to restore auto-detection
func (a *App) SetCS2Path(path string) error {
	return a.appService.SetCS2Path(path)
}

// SelectCS2Directory opens a native directory picker and saves the normalized CS2 cfg path
func (a *App) SelectCS2Directory() (string, error) {
	return a.appService.SelectCS2Directory()
}

// BackupUser backups userdata/<accountID>/730
func (a *App) BackupUser(accountID string) error {
	return a.appService.BackupUser(accountID)
}

// RestoreUser restores userdata/<sourceAccountID>/730 into target userdata/<targetAccountID>/730
func (a *App) RestoreUser(targetAccountID, sourceAccountID string) error {
	return a.appService.RestoreUser(targetAccountID, sourceAccountID)
}

// ListUserCFGFiles scans userdata/<accountID>/730/local/cfg for the config editor
func (a *App) ListUserCFGFiles(accountID string) ([]backup.GameCFGFile, error) {
	return a.appService.ListUserCFGFiles(accountID)
}

// ListGameCFGFiles scans game/csgo/cfg and returns files with origin classification
func (a *App) ListGameCFGFiles() ([]backup.GameCFGFile, error) {
	return a.appService.ListGameCFGFiles()
}

// BackupGameCFG backs up CS2 game configs with mode "all", "custom", or "selected"
func (a *App) BackupGameCFG(mode string, selectedFiles []string) (int, error) {
	return a.appService.BackupGameCFG(mode, selectedFiles)
}

// RestoreGameCFG restores CS2 game configs into game/csgo/cfg
func (a *App) RestoreGameCFG() (int, error) {
	return a.appService.RestoreGameCFG()
}

// HasUserRollback checks if an automatic pre-restore rollback snapshot exists for the target account
func (a *App) HasUserRollback(accountID string) bool {
	return a.appService.HasUserRollback(accountID)
}

// RollbackUserRestore restores the pre-restore rollback snapshot back to the account's userdata/730 directory
func (a *App) RollbackUserRestore(accountID string) error {
	return a.appService.RollbackUserRestore(accountID)
}

// HasGameCFGRollback checks if an automatic pre-restore rollback snapshot exists for the global game CFG
func (a *App) HasGameCFGRollback() bool {
	return a.appService.HasGameCFGRollback()
}

// RollbackGameCFG restores the pre-restore rollback snapshot back to the CS2 game cfg directory
func (a *App) RollbackGameCFG() (int, error) {
	return a.appService.RollbackGameCFG()
}

// SelectCFGFiles opens native file dialog allowing user to choose .cfg/.vcfg/.txt files
func (a *App) SelectCFGFiles() ([]string, error) {
	return a.appService.SelectCFGFiles()
}

// GetBackedUpAccounts returns a list of account IDs that have existing backups
func (a *App) GetBackedUpAccounts() ([]string, error) {
	return a.appService.GetBackedUpAccounts()
}

// ReadConfigFile securely reads configuration file text for an account or game_cfg
func (a *App) ReadConfigFile(accountID, relPath string) (string, error) {
	return a.appService.ReadConfigFile(accountID, relPath)
}

// CreateConfigFile 新建一个空的 cfg 文件（只能新建，同名已存在会报错）。
// accountID 传 "game_cfg" 落在 game/csgo/cfg，否则落在该账号的 userdata cfg 目录。
func (a *App) CreateConfigFile(accountID, name string) (string, error) {
	return a.appService.CreateConfigFile(accountID, name)
}

// SaveConfigFileText securely saves configuration file text for an account or game_cfg
func (a *App) SaveConfigFileText(accountID, relPath, content string) error {
	return a.appService.SaveConfigFileText(accountID, relPath, content)
}

// Snapshot API bindings

// GetSnapshots lists all snapshots for an account
func (a *App) GetSnapshots(accountID string) ([]backup.SnapshotInfo, error) {
	return a.appService.GetSnapshots(accountID)
}

// CreateSnapshot creates an immutable snapshot of userdata/<accountID>/730 with deduplication
func (a *App) CreateSnapshot(accountID, note string) (*backup.SnapshotInfo, error) {
	return a.appService.CreateSnapshot(accountID, note)
}

// RestoreSnapshot restores the specified snapshot into userdata/<accountID>/730
func (a *App) RestoreSnapshot(accountID, snapshotID string) error {
	return a.appService.RestoreSnapshot(accountID, snapshotID)
}

// DeleteSnapshot deletes the specified snapshot
func (a *App) DeleteSnapshot(accountID, snapshotID string) error {
	return a.appService.DeleteSnapshot(accountID, snapshotID)
}

// CompareSnapshot compares a snapshot against live Steam userdata
func (a *App) CompareSnapshot(accountID, snapshotID string) ([]backup.FileDiffItem, error) {
	return a.appService.CompareSnapshot(accountID, snapshotID)
}

// GetTextDiff compares two strings line-by-line
func (a *App) GetTextDiff(textA, textB string) (*backup.DiffResult, error) {
	return a.appService.GetTextDiff(textA, textB)
}

// Process Supervisor and Launch Options bindings

// IsSteamRunning checks whether steam.exe or steamwebhelper.exe is running
func (a *App) IsSteamRunning() (bool, error) {
	return a.appService.IsSteamRunning()
}

// ShutdownSteam gracefully exits Steam with a 5-second wait timeout before fallback termination
func (a *App) ShutdownSteam() error {
	return a.appService.ShutdownSteam()
}

// StartSteam launches Steam in detached background mode without blocking.
// If launchGame is provided and true, it launches CS2 directly via -applaunch 730.
func (a *App) StartSteam(launchGame bool) error {
	return a.appService.StartSteam(launchGame)
}

// StartCS2Game launches CS2 directly via steam.exe -applaunch 730.
func (a *App) StartCS2Game() error {
	return a.appService.StartCS2Game()
}

// SwitchSteamAccount switches the active Steam account to accountName
func (a *App) SwitchSteamAccount(accountName string) error {
	return a.appService.SwitchSteamAccount(accountName)
}

// GetLaunchOptions retrieves CS2 launch options for an account
func (a *App) GetLaunchOptions(accountID string) (string, error) {
	return a.appService.GetLaunchOptions(accountID)
}

// SetLaunchOptions updates CS2 launch options for an account
func (a *App) SetLaunchOptions(accountID, options string) error {
	return a.appService.SetLaunchOptions(accountID, options)
}

// Video Settings and Presets bindings

// GetDisplayModes enumerates display modes supported by the primary monitor
func (a *App) GetDisplayModes() ([]video.DisplayMode, error) {
	return a.appService.GetDisplayModes()
}

// GetVideoSettings reads and parses cs2_video.txt for an account
func (a *App) GetVideoSettings(accountID string) (*video.VideoConfigData, error) {
	return a.appService.GetVideoSettings(accountID)
}

// SaveVideoSettings saves video settings to cs2_video.txt with read-only lock
func (a *App) SaveVideoSettings(accountID string, settings map[string]string, readOnly bool) error {
	return a.appService.SaveVideoSettings(accountID, settings, readOnly)
}

// SelectVideoConfigFile opens native file dialog for selecting a .txt video config
func (a *App) SelectVideoConfigFile() (string, error) {
	return a.appService.SelectVideoConfigFile()
}

// ImportVideoPreset imports an external .txt file as a new preset
func (a *App) ImportVideoPreset(filePath, name string, setAsDefault bool) (*video.VideoPreset, error) {
	return a.appService.ImportVideoPreset(filePath, name, setAsDefault)
}

// SaveAsVideoPreset saves the account's live video config as a new preset
func (a *App) SaveAsVideoPreset(accountID, name string, setAsDefault bool) (*video.VideoPreset, error) {
	return a.appService.SaveAsVideoPreset(accountID, name, setAsDefault)
}

// GetVideoPresets returns all saved video presets
func (a *App) GetVideoPresets() ([]video.VideoPreset, error) {
	return a.appService.GetVideoPresets()
}

// SetDefaultVideoPreset designates a preset as global default
func (a *App) SetDefaultVideoPreset(presetID string) error {
	return a.appService.SetDefaultVideoPreset(presetID)
}

// DeleteVideoPreset deletes a preset
func (a *App) DeleteVideoPreset(presetID string) error {
	return a.appService.DeleteVideoPreset(presetID)
}

// ApplyVideoPreset applies a preset to the target account with optional read-only lock
func (a *App) ApplyVideoPreset(accountID, presetID string, setReadOnly bool) error {
	return a.appService.ApplyVideoPreset(accountID, presetID, setReadOnly)
}

// ApplyDefaultVideoPreset applies the global default preset to the target account
func (a *App) ApplyDefaultVideoPreset(accountID string, setReadOnly bool) error {
	return a.appService.ApplyDefaultVideoPreset(accountID, setReadOnly)
}

// Crosshair Settings bindings

// GetCrosshairState reads the crosshair settings currently present in autoexec.cfg
func (a *App) GetCrosshairState() (*crosshair.State, error) {
	return a.appService.GetCrosshairState()
}

// SaveCrosshair writes the crosshair into autoexec.cfg, replacing existing keys in place
func (a *App) SaveCrosshair(ch crosshair.Crosshair) (*crosshair.SaveResult, error) {
	return a.appService.SaveCrosshair(ch)
}

// DecodeCrosshairCode parses a share code, including legacy CSGO- format.
// accountID lets legacy codes be converted using that account's CS2 resolution.
func (a *App) DecodeCrosshairCode(accountID, code string) (*crosshair.Crosshair, error) {
	return a.appService.DecodeCrosshairCode(accountID, code)
}

// EncodeCrosshairCode encodes the crosshair into the current CS2 share code format
func (a *App) EncodeCrosshairCode(ch crosshair.Crosshair) (string, error) {
	return a.appService.EncodeCrosshairCode(ch)
}

// ExportCrosshairCFG writes a standalone crosshair.cfg next to autoexec.cfg
func (a *App) ExportCrosshairCFG(ch crosshair.Crosshair) (string, error) {
	return a.appService.ExportCrosshairCFG(ch)
}

// GetCrosshairConsole returns a single-line console command string
func (a *App) GetCrosshairConsole(ch crosshair.Crosshair) string {
	return a.appService.GetCrosshairConsole(ch)
}

// GetCrosshairPresets returns built-in pro presets plus user-saved presets
func (a *App) GetCrosshairPresets() []crosshair.Preset {
	return a.appService.GetCrosshairPresets()
}

// GetHiddenCrosshairPresets returns built-in presets the user has hidden
func (a *App) GetHiddenCrosshairPresets() []crosshair.Preset {
	return a.appService.GetHiddenCrosshairPresets()
}

// SaveCrosshairPreset saves the given crosshair as a user preset
func (a *App) SaveCrosshairPreset(name string, ch crosshair.Crosshair) (*crosshair.Preset, error) {
	return a.appService.SaveCrosshairPreset(name, ch)
}

// DeleteCrosshairPreset deletes a user preset
func (a *App) DeleteCrosshairPreset(id string) error {
	return a.appService.DeleteCrosshairPreset(id)
}

// SetCrosshairPresetHidden hides or restores a built-in preset
func (a *App) SetCrosshairPresetHidden(id string, hidden bool) error {
	return a.appService.SetCrosshairPresetHidden(id, hidden)
}

// GetCrosshairCommands returns the convar lines and one-line console command for a crosshair
func (a *App) GetCrosshairCommands(ch crosshair.Crosshair) crosshair.CommandPreview {
	return a.appService.GetCrosshairCommands(ch)
}

// SetAssets sets embedded frontend SPA assets on the LAN server
func (a *App) SetAssets(assets fs.FS) {
	a.appService.SetAssets(assets)
}

// SetLANBindAddress 指定局域网共享只在哪张网卡的地址上提供服务，空表示全部网卡。
func (a *App) SetLANBindAddress(addr string) error {
	return a.appService.SetLANBindAddress(addr)
}

// SetLANRateLimit 设置局域网共享的出站限速（bytes/sec），0 表示不限。
func (a *App) SetLANRateLimit(bytesPerSec int) error {
	return a.appService.SetLANRateLimit(bytesPerSec)
}

// RestoreVersion 把账号还原到指定历史版本，还原前会自动留底。
func (a *App) RestoreVersion(accountID, versionID string) error {
	return a.appService.RestoreVersion(accountID, versionID)
}

// ListGameCFGVersions 返回通用 CFG 的历史版本，最新的在前。
func (a *App) ListGameCFGVersions() ([]string, error) {
	return a.appService.ListGameCFGVersions()
}

// RestoreGameCFGVersion 把通用 CFG 还原到指定历史版本。
func (a *App) RestoreGameCFGVersion(versionID string) (int, error) {
	return a.appService.RestoreGameCFGVersion(versionID)
}

// PreviewBackupPrune 返回收敛时会删掉哪些历史版本，不删任何东西。
func (a *App) PreviewBackupPrune(accountID string) ([]string, error) {
	return a.appService.PreviewBackupPrune(accountID)
}

// ApplyBackupPrune 按当前上限收敛历史版本。
func (a *App) ApplyBackupPrune(accountID string) ([]string, error) {
	return a.appService.ApplyBackupPrune(accountID)
}

// GetBackupRetention 返回每个账号保留的历史版本数。
func (a *App) GetBackupRetention() int {
	return a.appService.GetBackupRetention()
}

// SetBackupRetention 设置每个账号保留的历史版本数，0 表示不清理。
func (a *App) SetBackupRetention(n int) error {
	return a.appService.SetBackupRetention(n)
}

// LAN Sharing bindings

// StartLANServer starts the LAN sharing server on specified port and name
func (a *App) StartLANServer(port int, name string) error {
	return a.appService.StartLANServer(port, name)
}

// StopLANServer terminates the LAN sharing server
func (a *App) StopLANServer() error {
	return a.appService.StopLANServer()
}

// GetLANServerStatus returns the active status of the LAN sharing server
func (a *App) GetLANServerStatus() (*lan.LANServerStatus, error) {
	return a.appService.GetLANServerStatus()
}

// ToggleLANPin enables or disables PIN authentication for LAN clients
func (a *App) ToggleLANPin(enabled bool) error {
	return a.appService.ToggleLANPin(enabled)
}

// RegenerateLANPin creates a new 6-digit PIN code
func (a *App) RegenerateLANPin() (string, error) {
	return a.appService.RegenerateLANPin()
}

// SetLANSharedBackups configures which local accounts and ZIP packages are shared
func (a *App) SetLANSharedBackups(accountIDs []string, zipPaths []string) error {
	return a.appService.SetLANSharedBackups(accountIDs, zipPaths)
}

// CreateLANPackage builds a standard ZIP package and registers it into LAN sharing
func (a *App) CreateLANPackage(accountID, name string, includeCfg, includeUserdata bool) (string, error) {
	return a.appService.CreateLANPackage(accountID, name, includeCfg, includeUserdata)
}

// GetLANPackages lists all ZIP backup packages in the backup repository
func (a *App) GetLANPackages() ([]lan.PackageInfo, error) {
	return a.appService.GetLANPackages()
}

// StartDiscovery launches UDP broadcast and peer discovery on local network
func (a *App) StartDiscovery() error {
	return a.appService.StartDiscovery()
}

// StopDiscovery stops UDP peer discovery
func (a *App) StopDiscovery() error {
	return a.appService.StopDiscovery()
}

// GetDiscoveredDevices returns all discovered local and manual devices
func (a *App) GetDiscoveredDevices() ([]lan.DiscoveredDevice, error) {
	return a.appService.GetDiscoveredDevices()
}

// AddManualDevice adds a target device by IP and port manually
func (a *App) AddManualDevice(ip string, port int) error {
	return a.appService.AddManualDevice(ip, port)
}

// ConnectRemoteDevice connects to a remote device and retrieves its shared backups
func (a *App) ConnectRemoteDevice(ip string, port int, pin string) ([]lan.RemoteBackupItem, error) {
	return a.appService.ConnectRemoteDevice(ip, port, pin)
}

// DownloadRemoteBackup downloads a remote backup and unpacks it into the local repository
func (a *App) DownloadRemoteBackup(ip string, port int, pin, backupID string) (*lan.DownloadedBackupResult, error) {
	return a.appService.DownloadRemoteBackup(ip, port, pin, backupID)
}
