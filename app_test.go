package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/lan"
	"cs2backup/internal/logger"
	"cs2backup/internal/steam"

	"cs2backup/internal/testutil"
)

func TestApp_LifecycleAndBindings(t *testing.T) {
	// 启动会尝试自动探测 Steam 目录。这里钉死成「没装」，用例就不依赖宿主机了
	// ——之前就是因为隐式依赖宿主环境，CI 上红过一次。
	restoreDetect := steam.SetSteamPathDetectorForTesting(func() (string, error) {
		return "", steam.ErrSteamNotFound
	})
	defer restoreDetect()

	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	app := NewAppWithConfigDir(configDir)
	if app == nil || app.appService == nil {
		t.Fatalf("expected initialized App and AppService")
	}

	// Startup lifecycle
	ctx := context.Background()
	app.startup(ctx)

	// Check AppInfo
	info := app.GetAppInfo()
	if info.Name != "CS2 Config Backup & Sync" {
		t.Errorf("unexpected name: %s", info.Name)
	}

	// Check that startup logged a success message
	logs := app.GetLogs()
	if len(logs) == 0 {
		t.Fatalf("expected startup log entry")
	}
	if logs[0].Level != logger.LevelSuccess {
		t.Errorf("expected LevelSuccess on startup, got %s", logs[0].Level)
	}

	// Test EmitLog
	e := app.EmitLog(logger.LevelInfo, "UI Loaded")
	if e.Message != "UI Loaded" {
		t.Errorf("expected 'UI Loaded', got %s", e.Message)
	}

	// 不锁死条数：启动过程里可能补上别的日志（比如自动探测 Steam 目录）。
	// 要验的是「启动成功那条在前、EmitLog 那条在后」。
	logsAfter := app.GetLogs()
	if len(logsAfter) < 2 {
		t.Fatalf("expected at least 2 logs, got %d", len(logsAfter))
	}
	if last := logsAfter[len(logsAfter)-1]; last.Message != "UI Loaded" {
		t.Errorf("expected last log to be 'UI Loaded', got %q", last.Message)
	}

	// Test ClearLogs
	app.ClearLogs()
	if len(app.GetLogs()) != 0 {
		t.Errorf("expected 0 logs after ClearLogs")
	}
}

func TestApp_SteamBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")

	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config", "avatarcache"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "config", "avatarcache", "76561198083722517.png"), []byte("avatar"), 0644)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)

	app := NewAppWithConfigDir(configDir)
	steamSvc, err := steam.NewService(configDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	app.appService.SetSteamService(steamSvc)

	// Set Steam path
	err = app.SetSteamPath(steamDir)
	if err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if p := app.GetSteamPath(); p != steamDir {
		t.Errorf("GetSteamPath = %s, want %s", p, steamDir)
	}

	// Save user note
	err = app.SaveUserNote("123456789", "Primary")
	if err != nil {
		t.Fatalf("SaveUserNote failed: %v", err)
	}

	// Get users
	users, err := app.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) != 1 || users[0].Notes != "Primary" {
		t.Fatalf("unexpected users result: %+v", users)
	}

	// Resolve user name
	name, _ := app.ResolveUserName("123456789")
	if name == "" {
		t.Errorf("expected non-empty resolved name")
	}
}

func TestApp_BackupBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")
	backupDir := filepath.Join(tempDir, "backup")

	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config", "avatarcache"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "config", "avatarcache", "76561198083722517.png"), []byte("avatar"), 0644)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)
	_ = os.WriteFile(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg", "cs2_video.txt"), []byte("video data"), 0644)

	cs2CfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(cs2CfgDir, 0755)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "autoexec.cfg"), []byte("echo loaded"), 0644)

	app := NewAppWithConfigDir(configDir)
	_ = app.SetSteamPath(steamDir)

	if err := app.SetBackupPath(backupDir); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}
	if bp := app.GetBackupPath(); bp != backupDir {
		t.Errorf("GetBackupPath = %q, want %q", bp, backupDir)
	}

	// Backup user
	if err := app.BackupUser("123456789"); err != nil {
		t.Fatalf("BackupUser failed: %v", err)
	}

	// Get backed up accounts
	accounts, err := app.GetBackedUpAccounts()
	if err != nil {
		t.Fatalf("GetBackedUpAccounts failed: %v", err)
	}
	if len(accounts) != 1 || accounts[0] != "123456789" {
		t.Errorf("expected ['123456789'], got %v", accounts)
	}

	// Backup game cfg
	count, err := app.BackupGameCFG("all", nil)
	if err != nil {
		t.Fatalf("BackupGameCFG failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 file backed up, got %d", count)
	}

	// Restore game cfg
	restored, err := app.RestoreGameCFG()
	if err != nil {
		t.Fatalf("RestoreGameCFG failed: %v", err)
	}
	if restored != 1 {
		t.Errorf("expected 1 restored file, got %d", restored)
	}

	// Restore user (creating rollback snapshot)
	if err := app.RestoreUser("123456789", "123456789"); err != nil {
		t.Fatalf("RestoreUser failed: %v", err)
	}

	// Test Rollback bindings
	if !app.HasUserRollback("123456789") {
		t.Errorf("expected app.HasUserRollback('123456789') to be true")
	}
	if err := app.RollbackUserRestore("123456789"); err != nil {
		t.Fatalf("app.RollbackUserRestore failed: %v", err)
	}
	if !app.HasGameCFGRollback() {
		t.Errorf("expected app.HasGameCFGRollback() to be true")
	}
	if rbCount, err := app.RollbackGameCFG(); err != nil || rbCount != 1 {
		t.Fatalf("app.RollbackGameCFG failed: count=%d, err=%v", rbCount, err)
	}
}

func TestApp_ProcessAndLaunchOptionsBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")
	accountID := "123456789"

	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", accountID, "config"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)

	app := NewAppWithConfigDir(configDir)
	_ = app.SetSteamPath(steamDir)

	// IsSteamRunning
	_, err := app.IsSteamRunning()
	if err != nil {
		t.Fatalf("IsSteamRunning failed: %v", err)
	}

	// Set & Get LaunchOptions
	opts := "-novid -freq 240 +fps_max 0"
	if err := app.SetLaunchOptions(accountID, opts); err != nil {
		t.Fatalf("SetLaunchOptions failed: %v", err)
	}

	retrieved, err := app.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("GetLaunchOptions failed: %v", err)
	}
	if retrieved != opts {
		t.Errorf("expected %q, got %q", opts, retrieved)
	}

	// Test StartSteam(true) & StartCS2Game
	var capturedArgs []string
	restore := steam.SetProcessStarterForTesting(func(p string, args ...string) error {
		capturedArgs = args
		return nil
	})
	defer restore()

	if err := app.StartCS2Game(); err != nil {
		t.Fatalf("StartCS2Game failed: %v", err)
	}
	if len(capturedArgs) < 2 || capturedArgs[0] != "-applaunch" || capturedArgs[1] != "730" {
		t.Fatalf("expected CS2 applaunch args, got %v", capturedArgs)
	}
}

func TestApp_VideoBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")
	accountID := "12345678"

	videoDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	_ = os.MkdirAll(videoDir, 0755)
	sampleContent := "\"video.cfg\"\n{\n\t\"setting.defaultres\"\t\t\"1920\"\n\t\"setting.defaultresheight\"\t\t\"1080\"\n\t\"setting.fullscreen\"\t\t\"1\"\n\t\"setting.msaa_samples\"\t\t\"4\"\n}\n"
	_ = os.WriteFile(filepath.Join(videoDir, "cs2_video.txt"), []byte(sampleContent), 0666)

	app := NewAppWithConfigDir(configDir)
	_ = app.SetSteamPath(steamDir)

	// GetVideoSettings
	cfg, err := app.GetVideoSettings(accountID)
	if err != nil {
		t.Fatalf("GetVideoSettings failed: %v", err)
	}
	if cfg.Settings["setting.defaultres"] != "1920" {
		t.Errorf("expected defaultres 1920, got %s", cfg.Settings["setting.defaultres"])
	}

	// SaveVideoSettings with readOnly
	err = app.SaveVideoSettings(accountID, map[string]string{"setting.defaultres": "1440"}, true)
	if err != nil {
		t.Fatalf("SaveVideoSettings failed: %v", err)
	}

	// SaveAsVideoPreset
	p, err := app.SaveAsVideoPreset(accountID, "测试预设", true)
	if err != nil {
		t.Fatalf("SaveAsVideoPreset failed: %v", err)
	}

	// GetVideoPresets
	list, err := app.GetVideoPresets()
	if err != nil || len(list) != 1 {
		t.Fatalf("expected 1 preset, got %d", len(list))
	}

	// SetDefaultVideoPreset
	if err := app.SetDefaultVideoPreset(p.ID); err != nil {
		t.Fatalf("SetDefaultVideoPreset failed: %v", err)
	}

	// ApplyDefaultVideoPreset
	targetAccount := "87654321"
	if err := app.ApplyDefaultVideoPreset(targetAccount, false); err != nil {
		t.Fatalf("ApplyDefaultVideoPreset failed: %v", err)
	}

	// DeleteVideoPreset
	if err := app.DeleteVideoPreset(p.ID); err != nil {
		t.Fatalf("DeleteVideoPreset failed: %v", err)
	}
}

func TestApp_LANBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	app := NewAppWithConfigDir(configDir)
	app.SetBackupPath(filepath.Join(tempDir, "backup"))

	// 1. StartLANServer
	if err := app.StartLANServer(0, "TestAppServer"); err != nil {
		t.Fatalf("StartLANServer failed: %v", err)
	}
	defer app.StopLANServer()

	// 2. GetLANServerStatus
	status, err := app.GetLANServerStatus()
	if err != nil {
		t.Fatalf("GetLANServerStatus failed: %v", err)
	}
	if !status.Running {
		t.Errorf("expected Running true")
	}

	// 3. ToggleLANPin
	if err := app.ToggleLANPin(true); err != nil {
		t.Fatalf("ToggleLANPin failed: %v", err)
	}

	// 4. RegenerateLANPin
	pin, err := app.RegenerateLANPin()
	if err != nil || len(pin) != 6 {
		t.Fatalf("RegenerateLANPin failed: pin=%s, err=%v", pin, err)
	}

	// 5. CreateLANPackage
	pkgPath, err := app.CreateLANPackage("123456789", "TestAppPackage", false, false)
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}

	// 6. GetLANPackages
	pkgs, err := app.GetLANPackages()
	if err != nil || len(pkgs) != 1 {
		t.Fatalf("GetLANPackages failed: pkgs=%+v, err=%v", pkgs, err)
	}

	// 7. SetLANSharedBackups
	if err := app.SetLANSharedBackups([]string{"123456789"}, []string{pkgPath}); err != nil {
		t.Fatalf("SetLANSharedBackups failed: %v", err)
	}

	// 8. StopLANServer
	if err := app.StopLANServer(); err != nil {
		t.Fatalf("StopLANServer failed: %v", err)
	}
}

func TestApp_DiscoveryAndRemoteBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	backupDir := filepath.Join(tempDir, "backup")

	app := NewAppWithConfigDir(configDir)
	_ = app.SetBackupPath(backupDir)

	// Configure discovery with custom test port to avoid conflict
	disc := lan.NewDiscoveryService(8765, "AppTestPC", "2.0.0")
	disc.SetUDPPort(19877)
	app.appService.SetDiscoveryService(disc)

	// Test Discovery bindings
	if err := app.StartDiscovery(); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}
	defer app.StopDiscovery()

	if err := app.AddManualDevice("10.0.0.99", 8765); err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	devices, err := app.GetDiscoveredDevices()
	if err != nil || len(devices) == 0 {
		t.Fatalf("GetDiscoveredDevices failed: %+v, %v", devices, err)
	}

	// Test StopDiscovery
	if err := app.StopDiscovery(); err != nil {
		t.Fatalf("StopDiscovery failed: %v", err)
	}
}

func TestApp_SwitchSteamAccountAndConfigTextEditing(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "Steam")
	backupDir := filepath.Join(tempDir, "Backup")

	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg"), 0755)
	gameCfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(gameCfgDir, 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0755)

	vdfContent := `"users"
{
	"76561198083722517"
	{
		"AccountName"	"test_account_one"
		"PersonaName"	"TestPersonaOne"
		"RememberPassword"	"1"
		"MostRecent"	"1"
	}
	"76561198947920049"
	{
		"AccountName"	"test_account_two"
		"PersonaName"	"TestPersonaTwo"
		"RememberPassword"	"1"
		"MostRecent"	"0"
	}
}
`
	_ = os.WriteFile(filepath.Join(steamPathOr(steamDir, "config", "loginusers.vdf")), []byte(vdfContent), 0644)

	app := NewAppWithConfigDir(configDir)
	_ = app.SetSteamPath(steamDir)
	_ = app.SetBackupPath(backupDir)

	restoreStarter := steam.SetProcessStarterForTesting(func(p string, args ...string) error {
		return nil
	})
	defer restoreStarter()

	restoreRegistry := steam.SetRegistrySetterForTesting(func(accountName string) error {
		return nil
	})
	defer restoreRegistry()

	restoreShutdown := steam.SetSteamShutdownerForTesting(func(p string) error {
		return nil
	})
	defer restoreShutdown()

	// 1. Test SwitchSteamAccount
	if err := app.SwitchSteamAccount("test_account_two"); err != nil {
		t.Fatalf("SwitchSteamAccount failed: %v", err)
	}

	// 2. Test SaveConfigFileText & ReadConfigFile for user config
	userTxt := "bind \"x\" \"slot10\"\n"
	if err := app.SaveConfigFileText("123456789", "molotov.cfg", userTxt); err != nil {
		t.Fatalf("SaveConfigFileText failed: %v", err)
	}

	readUser, err := app.ReadConfigFile("123456789", "molotov.cfg")
	if err != nil {
		t.Fatalf("ReadConfigFile failed: %v", err)
	}
	if readUser != userTxt {
		t.Errorf("expected %q, got %q", userTxt, readUser)
	}

	// 3. Test SaveConfigFileText & ReadConfigFile for game_cfg
	gameTxt := "echo \"Loaded via App Binding\"\n"
	if err := app.SaveConfigFileText("game_cfg", "game_app.cfg", gameTxt); err != nil {
		t.Fatalf("SaveConfigFileText game_cfg failed: %v", err)
	}

	readGame, err := app.ReadConfigFile("game_cfg", "game_app.cfg")
	if err != nil {
		t.Fatalf("ReadConfigFile game_cfg failed: %v", err)
	}
	if readGame != gameTxt {
		t.Errorf("expected %q, got %q", gameTxt, readGame)
	}

	// 4. Test security check
	if _, err := app.ReadConfigFile("123456789", "../hacked.cfg"); err == nil {
		t.Errorf("expected error for path traversal, got nil")
	}
}

func steamPathOr(parts ...string) string {
	return filepath.Join(parts...)
}

func TestApp_OpenPathInExplorer(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	app := NewAppWithConfigDir(configDir)

	var openedPath string
	app.appService.SetPathOpener(func(p string) error {
		openedPath = p
		return nil
	})

	existingDir := filepath.Join(tempDir, "opened_dir")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("mkdir failed: %v", err)
	}

	if err := app.OpenPathInExplorer(existingDir); err != nil {
		t.Fatalf("expected OpenPathInExplorer to succeed, got %v", err)
	}
	if openedPath != filepath.Clean(existingDir) {
		t.Errorf("expected openedPath=%s, got %s", filepath.Clean(existingDir), openedPath)
	}
}
