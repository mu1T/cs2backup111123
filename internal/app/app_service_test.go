package app

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"cs2backup/internal/lan"
	"cs2backup/internal/steam"

	"cs2backup/internal/logger"

	"cs2backup/internal/testutil"
)

type mockEmitter struct {
	mu     sync.Mutex
	events []struct {
		name string
		data []interface{}
	}
}

func (m *mockEmitter) Emit(ctx context.Context, name string, data ...interface{}) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.events = append(m.events, struct {
		name string
		data []interface{}
	}{name: name, data: data})
}

func TestAppService_GetAppInfo(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	info := svc.GetAppInfo()

	if info.Name == "" || info.Version == "" {
		t.Fatalf("expected non-empty Name and Version, got %+v", info)
	}
	if info.Name != "CS2 Config Backup & Sync" {
		t.Errorf("expected app name 'CS2 Config Backup & Sync', got '%s'", info.Name)
	}
}

func TestAppService_EmitLogAndRetrieve(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	mock := &mockEmitter{}
	svc.SetEventEmitter(mock.Emit)
	svc.SetContext(context.Background())

	entry := svc.EmitLog(logger.LevelInfo, "System initialized")

	if entry.Message != "System initialized" {
		t.Fatalf("unexpected entry message: %s", entry.Message)
	}
	if entry.Level != logger.LevelInfo {
		t.Fatalf("unexpected entry level: %s", entry.Level)
	}

	// Verify buffer retrieval
	logs := svc.GetLogs()
	if len(logs) == 0 {
		t.Fatalf("expected at least one log entry in buffer")
	}

	mock.mu.Lock()
	defer mock.mu.Unlock()
	if len(mock.events) == 0 {
		t.Fatalf("expected event emitted to frontend mock")
	}
	if mock.events[0].name != "log" {
		t.Errorf("expected event name 'log', got %s", mock.events[0].name)
	}
}

func TestAppService_EmitLogWithoutEmitterDoesNotPanic(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	// No emitter or context set
	entry := svc.EmitLog(logger.LevelWarning, "Stand-alone warning")
	if entry.Level != logger.LevelWarning {
		t.Errorf("expected level warning, got %s", entry.Level)
	}
}

func TestAppService_ClearLogs(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	svc.EmitLog(logger.LevelError, "Error occurred")
	if len(svc.GetLogs()) == 0 {
		t.Fatalf("log buffer should not be empty")
	}

	svc.ClearLogs()
	if len(svc.GetLogs()) != 0 {
		t.Fatalf("expected empty log buffer after clear, got %d", len(svc.GetLogs()))
	}
}

func TestAppService_SteamDelegations(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")

	// Setup mock steam directory
	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)

	loginusersContent := `"users"
{
	"76561198083722517"
	{
		"AccountName"		"player1"
		"PersonaName"		"ProGamer"
		"mostrecent"		"1"
	}
}`
	_ = os.WriteFile(filepath.Join(steamDir, "config", "loginusers.vdf"), []byte(loginusersContent), 0644)

	appSvc := NewAppServiceWithConfigDir(configDir)
	mock := &mockEmitter{}
	appSvc.SetEventEmitter(mock.Emit)
	appSvc.SetContext(context.Background())

	// SetSteamPath
	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if sp := appSvc.GetSteamPath(); sp != steamDir {
		t.Errorf("GetSteamPath = %q, want %q", sp, steamDir)
	}

	// GetUsers
	users, err := appSvc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 user, got %d", len(users))
	}
	if users[0].AccountID != "123456789" {
		t.Errorf("unexpected account id: %s", users[0].AccountID)
	}

	// SaveUserNote
	if err := appSvc.SaveUserNote("123456789", "大号"); err != nil {
		t.Fatalf("SaveUserNote failed: %v", err)
	}

	// Verify note appears in GetUsers
	users, _ = appSvc.GetUsers()
	if users[0].Notes != "大号" {
		t.Errorf("expected notes '大号', got %q", users[0].Notes)
	}

	// Resolve user name fallback
	name, _ := appSvc.ResolveUserName("123456789")
	if name == "" {
		t.Errorf("expected non-empty resolved name")
	}
}

func TestAppService_BackupDelegations(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")
	backupDir := filepath.Join(tempDir, "backup")

	// Setup directories and dummy files
	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)
	_ = os.WriteFile(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg", "cs2_video.txt"), []byte("video data"), 0644)

	cs2CfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(cs2CfgDir, 0755)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "autoexec.cfg"), []byte("echo loaded"), 0644)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "boot.vcfg"), []byte("boot"), 0644)

	appSvc := NewAppServiceWithConfigDir(configDir)
	mock := &mockEmitter{}
	appSvc.SetEventEmitter(mock.Emit)
	appSvc.SetContext(context.Background())

	_ = appSvc.SetSteamPath(steamDir)

	// Set backup path
	if err := appSvc.SetBackupPath(backupDir); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}
	if bp := appSvc.GetBackupPath(); bp != backupDir {
		t.Errorf("GetBackupPath = %q, want %q", bp, backupDir)
	}

	// Test DirectoryPicker
	appSvc.SetDirectoryPicker(func(title string) (string, error) {
		return filepath.Join(tempDir, "new_backup"), nil
	})
	pickedDir, err := appSvc.SelectBackupDirectory()
	if err != nil {
		t.Fatalf("SelectBackupDirectory failed: %v", err)
	}
	if pickedDir != filepath.Join(tempDir, "new_backup") {
		t.Errorf("unexpected picked dir: %s", pickedDir)
	}
	// Restore backupDir for subsequent tests
	_ = appSvc.SetBackupPath(backupDir)

	// Test FilePicker
	appSvc.SetFilePicker(func(title, filterDisplay, filterPattern string) ([]string, error) {
		return []string{filepath.Join(cs2CfgDir, "autoexec.cfg")}, nil
	})
	pickedFiles, err := appSvc.SelectCFGFiles()
	if err != nil {
		t.Fatalf("SelectCFGFiles failed: %v", err)
	}
	if len(pickedFiles) != 1 || pickedFiles[0] != filepath.Join(cs2CfgDir, "autoexec.cfg") {
		t.Errorf("unexpected picked files: %v", pickedFiles)
	}

	// Test GetCS2CFGPath
	cfgPath, err := appSvc.GetCS2CFGPath()
	if err != nil {
		t.Fatalf("GetCS2CFGPath failed: %v", err)
	}
	if cfgPath != cs2CfgDir {
		t.Errorf("GetCS2CFGPath = %q, want %q", cfgPath, cs2CfgDir)
	}

	// Test BackupUser
	if err := appSvc.BackupUser("123456789"); err != nil {
		t.Fatalf("BackupUser failed: %v", err)
	}

	// Test GetBackedUpAccounts
	backedUp, err := appSvc.GetBackedUpAccounts()
	if err != nil {
		t.Fatalf("GetBackedUpAccounts failed: %v", err)
	}
	if len(backedUp) != 1 || backedUp[0] != "123456789" {
		t.Errorf("expected ['123456789'], got %v", backedUp)
	}

	// Test RestoreUser
	// Setup target user 123
	targetUserDir := filepath.Join(steamDir, "userdata", "123", "730", "local", "cfg")
	_ = os.MkdirAll(targetUserDir, 0755)
	_ = os.WriteFile(filepath.Join(targetUserDir, "cs2_video.txt"), []byte("target video"), 0644)

	if err := appSvc.RestoreUser("123", "123456789"); err != nil {
		t.Fatalf("RestoreUser failed: %v", err)
	}

	// Test BackupGameCFG (mode custom)
	count, err := appSvc.BackupGameCFG("custom", nil)
	if err != nil {
		t.Fatalf("BackupGameCFG failed: %v", err)
	}
	if count != 1 {
		t.Errorf("expected 1 custom file, got %d", count)
	}

	// Test RestoreGameCFG
	restored, err := appSvc.RestoreGameCFG()
	if err != nil {
		t.Fatalf("RestoreGameCFG failed: %v", err)
	}
	if restored != 1 {
		t.Errorf("expected 1 restored file, got %d", restored)
	}

	// Test Rollback capabilities
	if !appSvc.HasUserRollback("123") {
		t.Errorf("expected HasUserRollback('123') to be true")
	}
	if err := appSvc.RollbackUserRestore("123"); err != nil {
		t.Fatalf("RollbackUserRestore failed: %v", err)
	}
	rbContent, _ := os.ReadFile(filepath.Join(targetUserDir, "cs2_video.txt"))
	if string(rbContent) != "target video" {
		t.Errorf("expected target video after rollback, got %q", string(rbContent))
	}

	if !appSvc.HasGameCFGRollback() {
		t.Errorf("expected HasGameCFGRollback() to be true")
	}
	if rbCount, err := appSvc.RollbackGameCFG(); err != nil || rbCount == 0 {
		t.Fatalf("RollbackGameCFG failed: count=%d, err=%v", rbCount, err)
	}

	// Verify logs were emitted for all operations
	logs := appSvc.GetLogs()
	if len(logs) < 5 {
		t.Errorf("expected at least 5 logs, got %d", len(logs))
	}
}

func TestAppService_SnapshotAndDiffIntegration(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "Steam")
	backupDir := filepath.Join(tempDir, "Backup")

	_ = os.MkdirAll(configDir, 0755)
	_ = os.MkdirAll(backupDir, 0755)

	accountID := "123456789"
	user730 := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	_ = os.MkdirAll(user730, 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("dummy"), 0644)
	_ = os.WriteFile(filepath.Join(user730, "cs2_video.txt"), []byte("setting.csm_quality_level 3\n"), 0644)

	appSvc := NewAppServiceWithConfigDir(configDir)
	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if err := appSvc.SetBackupPath(backupDir); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}

	// Test CreateSnapshot
	snap, err := appSvc.CreateSnapshot(accountID, "集成测试快照")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}
	if snap == nil || snap.AccountID != accountID {
		t.Fatalf("unexpected snapshot: %+v", snap)
	}

	// Test GetSnapshots
	snaps, err := appSvc.GetSnapshots(accountID)
	if err != nil {
		t.Fatalf("GetSnapshots failed: %v", err)
	}
	if len(snaps) != 1 || snaps[0].ID != snap.ID {
		t.Fatalf("expected 1 snapshot with ID %s, got %v", snap.ID, snaps)
	}

	// Test CompareSnapshot
	diffItems, err := appSvc.CompareSnapshot(accountID, snap.ID)
	if err != nil {
		t.Fatalf("CompareSnapshot failed: %v", err)
	}
	if len(diffItems) != 1 || diffItems[0].Status != "identical" {
		t.Fatalf("expected 1 identical item, got %+v", diffItems)
	}

	// Test GetTextDiff
	diffRes, err := appSvc.GetTextDiff("a\nb\n", "a\nc\n")
	if err != nil {
		t.Fatalf("GetTextDiff failed: %v", err)
	}
	if diffRes.ModCount != 1 {
		t.Errorf("expected 1 mod line, got %d", diffRes.ModCount)
	}

	// Test RestoreSnapshot
	_ = os.WriteFile(filepath.Join(user730, "cs2_video.txt"), []byte("corrupted"), 0644)
	if err := appSvc.RestoreSnapshot(accountID, snap.ID); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}
	restored, _ := os.ReadFile(filepath.Join(user730, "cs2_video.txt"))
	if string(restored) != "setting.csm_quality_level 3\n" {
		t.Errorf("restored content mismatch: %s", string(restored))
	}

	// Test DeleteSnapshot
	if err := appSvc.DeleteSnapshot(accountID, snap.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}
	snapsAfterDelete, err := appSvc.GetSnapshots(accountID)
	if err != nil || len(snapsAfterDelete) != 0 {
		t.Errorf("expected 0 snapshots after delete, got %d", len(snapsAfterDelete))
	}
}

func TestAppService_SteamProcessAndLaunchOptionsBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	// Set a valid mock Steam directory
	steamDir := filepath.Join(tempDir, "Steam")
	accountID := "55667788"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	_ = os.WriteFile(filepath.Join(steamDir, "steam.exe"), []byte("mock"), 0755)

	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}

	// 1. Test IsSteamRunning
	_, err := appSvc.IsSteamRunning()
	if err != nil {
		t.Fatalf("IsSteamRunning failed: %v", err)
	}

	// 2. Test GetLaunchOptions (initially empty)
	opts, err := appSvc.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("GetLaunchOptions failed: %v", err)
	}
	if opts != "" {
		t.Errorf("expected empty launch options, got %q", opts)
	}

	// 3. Test SetLaunchOptions
	newOpts := `-novid -freq 240 +exec "auto.cfg"`
	if err := appSvc.SetLaunchOptions(accountID, newOpts); err != nil {
		t.Fatalf("SetLaunchOptions failed: %v", err)
	}

	// 4. Test GetLaunchOptions after set
	retrieved, err := appSvc.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("GetLaunchOptions after set failed: %v", err)
	}
	if retrieved != newOpts {
		t.Errorf("expected %q, got %q", newOpts, retrieved)
	}
}

func TestAppService_VideoIntegration(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	steamDir := filepath.Join(tempDir, "Steam")
	accountID := "11223344"
	videoCfgDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(videoCfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	videoCfgFile := filepath.Join(videoCfgDir, "cs2_video.txt")
	sampleContent := "\"video.cfg\"\n{\n\t\"setting.defaultres\"\t\t\"1920\"\n\t\"setting.defaultresheight\"\t\t\"1080\"\n\t\"setting.fullscreen\"\t\t\"1\"\n\t\"setting.msaa_samples\"\t\t\"4\"\n}\n"
	if err := os.WriteFile(videoCfgFile, []byte(sampleContent), 0666); err != nil {
		t.Fatal(err)
	}

	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatal(err)
	}

	// 1. GetVideoSettings
	cfgData, err := appSvc.GetVideoSettings(accountID)
	if err != nil {
		t.Fatalf("GetVideoSettings failed: %v", err)
	}
	if cfgData.Settings["setting.defaultres"] != "1920" {
		t.Errorf("expected defaultres 1920, got %s", cfgData.Settings["setting.defaultres"])
	}
	if cfgData.ReadOnly {
		t.Errorf("expected ReadOnly false initially")
	}

	// 2. SaveVideoSettings with readOnly lock
	updatedSettings := map[string]string{
		"setting.defaultres":       "1280",
		"setting.defaultresheight": "960",
		"setting.msaa_samples":     "2",
	}
	if err := appSvc.SaveVideoSettings(accountID, updatedSettings, true); err != nil {
		t.Fatalf("SaveVideoSettings failed: %v", err)
	}
	cfgDataAfterSave, err := appSvc.GetVideoSettings(accountID)
	if err != nil {
		t.Fatalf("GetVideoSettings after save failed: %v", err)
	}
	if cfgDataAfterSave.Settings["setting.defaultres"] != "1280" {
		t.Errorf("expected defaultres 1280, got %s", cfgDataAfterSave.Settings["setting.defaultres"])
	}
	if !cfgDataAfterSave.ReadOnly {
		t.Errorf("expected ReadOnly true after save with readOnly=true")
	}

	// 3. SelectVideoConfigFile mock
	appSvc.SetSingleFilePicker(func(title, filterDisplay, filterPattern string) (string, error) {
		return "C:\\mock\\video.txt", nil
	})
	pickedFile, err := appSvc.SelectVideoConfigFile()
	if err != nil || pickedFile != "C:\\mock\\video.txt" {
		t.Errorf("unexpected SelectVideoConfigFile result: %s, %v", pickedFile, err)
	}

	// 4. SaveAsVideoPreset
	preset1, err := appSvc.SaveAsVideoPreset(accountID, "大号4:3拉伸", true)
	if err != nil {
		t.Fatalf("SaveAsVideoPreset failed: %v", err)
	}
	if preset1.Name != "大号4:3拉伸" || !preset1.IsDefault {
		t.Errorf("unexpected preset1: %+v", preset1)
	}

	// 5. ImportVideoPreset
	extFile := filepath.Join(tempDir, "ext_video.txt")
	_ = os.WriteFile(extFile, []byte(sampleContent), 0666)
	preset2, err := appSvc.ImportVideoPreset(extFile, "外部导入画质", false)
	if err != nil {
		t.Fatalf("ImportVideoPreset failed: %v", err)
	}
	if preset2.Name != "外部导入画质" || preset2.IsDefault {
		t.Errorf("unexpected preset2: %+v", preset2)
	}

	// 6. GetVideoPresets
	presets, err := appSvc.GetVideoPresets()
	if err != nil || len(presets) != 2 {
		t.Fatalf("expected 2 presets, got %d, err: %v", len(presets), err)
	}

	// 7. SetDefaultVideoPreset
	if err := appSvc.SetDefaultVideoPreset(preset2.ID); err != nil {
		t.Fatalf("SetDefaultVideoPreset failed: %v", err)
	}

	// 8. ApplyVideoPreset to target account
	targetAccountID := "99887766"
	if err := appSvc.ApplyVideoPreset(targetAccountID, preset1.ID, false); err != nil {
		t.Fatalf("ApplyVideoPreset failed: %v", err)
	}
	targetCfg, err := appSvc.GetVideoSettings(targetAccountID)
	if err != nil {
		t.Fatalf("GetVideoSettings on target failed: %v", err)
	}
	if targetCfg.Settings["setting.defaultres"] != "1280" {
		t.Errorf("expected applied defaultres 1280, got %s", targetCfg.Settings["setting.defaultres"])
	}

	// 9. ApplyDefaultVideoPreset
	if err := appSvc.ApplyDefaultVideoPreset(targetAccountID, true); err != nil {
		t.Fatalf("ApplyDefaultVideoPreset failed: %v", err)
	}
	targetCfg2, _ := appSvc.GetVideoSettings(targetAccountID)
	if targetCfg2.Settings["setting.defaultres"] != "1920" {
		t.Errorf("expected default preset defaultres 1920, got %s", targetCfg2.Settings["setting.defaultres"])
	}
	if !targetCfg2.ReadOnly {
		t.Errorf("expected targetCfg2 to be ReadOnly")
	}

	// 10. DeleteVideoPreset
	if err := appSvc.DeleteVideoPreset(preset2.ID); err != nil {
		t.Fatalf("DeleteVideoPreset failed: %v", err)
	}
	presetsAfterDel, _ := appSvc.GetVideoPresets()
	if len(presetsAfterDel) != 1 {
		t.Errorf("expected 1 preset remaining, got %d", len(presetsAfterDel))
	}
}

func TestAppService_LANServerBindings(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)
	appSvc.SetBackupPath(filepath.Join(tempDir, "backup"))

	// 1. StartLANServer with dynamic port (port 0)
	if err := appSvc.StartLANServer(0, "TestLANHost"); err != nil {
		t.Fatalf("StartLANServer failed: %v", err)
	}
	defer appSvc.StopLANServer()

	// 2. GetLANServerStatus
	status, err := appSvc.GetLANServerStatus()
	if err != nil {
		t.Fatalf("GetLANServerStatus failed: %v", err)
	}
	if !status.Running {
		t.Errorf("expected server to be running")
	}
	if status.ShareName != "TestLANHost" {
		t.Errorf("expected ShareName TestLANHost, got %s", status.ShareName)
	}
	if status.Port <= 0 {
		t.Errorf("expected bound Port > 0, got %d", status.Port)
	}

	// 3. ToggleLANPin
	if err := appSvc.ToggleLANPin(true); err != nil {
		t.Fatalf("ToggleLANPin failed: %v", err)
	}
	status2, _ := appSvc.GetLANServerStatus()
	if !status2.PINEnabled {
		t.Errorf("expected PINEnabled true")
	}

	// 4. RegenerateLANPin
	oldPIN := status2.PIN
	newPIN, err := appSvc.RegenerateLANPin()
	if err != nil {
		t.Fatalf("RegenerateLANPin failed: %v", err)
	}
	if len(newPIN) != 6 {
		t.Errorf("expected 6-digit PIN, got %s", newPIN)
	}
	_ = oldPIN

	// 5. Setup fake local userdata backup
	userDir := filepath.Join(tempDir, "backup", "userdata_backup", "123456789", "730", "local", "cfg")
	if err := os.MkdirAll(userDir, 0755); err != nil {
		t.Fatalf("mkdir userdata failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "cs2_video.txt"), []byte("video_data"), 0644); err != nil {
		t.Fatalf("write cs2_video.txt failed: %v", err)
	}

	// Setup fake cs2_cfg
	cfgDir := filepath.Join(tempDir, "backup", "cs2_cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("mkdir cs2_cfg failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "autoexec.cfg"), []byte("bind c +jump"), 0644); err != nil {
		t.Fatalf("write autoexec.cfg failed: %v", err)
	}

	// 6. CreateLANPackage
	pkgPath, err := appSvc.CreateLANPackage("123456789", "ZywOo_TestPkg", true, true)
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}
	if _, err := os.Stat(pkgPath); os.IsNotExist(err) {
		t.Fatalf("package file not created at %s", pkgPath)
	}

	// 7. GetLANPackages
	pkgs, err := appSvc.GetLANPackages()
	if err != nil {
		t.Fatalf("GetLANPackages failed: %v", err)
	}
	if len(pkgs) != 1 {
		t.Errorf("expected 1 package, got %d", len(pkgs))
	}

	// 8. SetLANSharedBackups
	if err := appSvc.SetLANSharedBackups([]string{"123456789"}, []string{pkgPath}); err != nil {
		t.Fatalf("SetLANSharedBackups failed: %v", err)
	}
	status3, _ := appSvc.GetLANServerStatus()
	if status3.SharedCount != 2 {
		t.Errorf("expected SharedCount 2, got %d", status3.SharedCount)
	}

	// 9. StopLANServer
	if err := appSvc.StopLANServer(); err != nil {
		t.Fatalf("StopLANServer failed: %v", err)
	}
	status4, _ := appSvc.GetLANServerStatus()
	if status4.Running {
		t.Errorf("expected server to be stopped")
	}
}

func TestAppService_DiscoveryAndRemoteSync(t *testing.T) {
	tempDir := testutil.TempDir(t)
	backupDir := filepath.Join(tempDir, "backup")

	appSvc := NewAppServiceWithConfigDir(tempDir)
	_ = appSvc.SetBackupPath(backupDir)

	// 必须显式指定 Steam 目录：不指定的话 GetUsers 会去自动探测本机 Steam，
	// 而 CI 的 windows-latest 是干净虚拟机（没装 Steam），探测会失败并把测试打红。
	// 本地能过只是因为本机装了 Steam——这种隐式依赖不能留。
	steamDir := filepath.Join(tempDir, "Steam")
	_ = os.MkdirAll(filepath.Join(steamDir, "userdata"), 0o755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0o755)
	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}

	// 异步头像抓取会往 configDir 写 .avatar_failed_cache，不等它结束会和 TempDir 清理撞车
	defer appSvc.WaitBackground()

	// Configure discovery service with custom UDP port for isolated testing
	disc := lan.NewDiscoveryService(8765, "TestDevice", "2.0.0")
	disc.SetUDPPort(19876)
	appSvc.SetDiscoveryService(disc)

	// 1. Discovery lifecycle & manual device
	if err := appSvc.StartDiscovery(); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}
	defer appSvc.StopDiscovery()

	if err := appSvc.AddManualDevice("192.168.1.150", 8765); err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	devices, err := appSvc.GetDiscoveredDevices()
	if err != nil {
		t.Fatalf("GetDiscoveredDevices failed: %v", err)
	}
	if len(devices) == 0 {
		t.Fatalf("expected at least 1 discovered/manual device, got 0")
	}
	if devices[0].IP != "192.168.1.150" {
		t.Errorf("expected device IP 192.168.1.150, got %s", devices[0].IP)
	}

	// 2. Setup mock remote LAN server serving a backup
	remoteDir := filepath.Join(tempDir, "remote_source")
	udDir := filepath.Join(remoteDir, "userdata", "999", "730", "local", "cfg")
	cfgDir := filepath.Join(remoteDir, "game", "csgo", "cfg")
	_ = os.MkdirAll(udDir, 0755)
	_ = os.MkdirAll(cfgDir, 0755)
	_ = os.WriteFile(filepath.Join(udDir, "cs2_user_keys.vcfg"), []byte("bind f +lookatweapon"), 0644)
	_ = os.WriteFile(filepath.Join(cfgDir, "practice.cfg"), []byte("sv_cheats 1"), 0644)

	pkgPath := filepath.Join(remoteDir, "test_pkg.zip")
	_, err = lan.CreateLANPackage(lan.PackageOptions{
		AccountID:       "999",
		Name:            "Pro_Sniper_Config",
		IncludeUserdata: true,
		UserdataDir:     filepath.Join(remoteDir, "userdata", "999", "730"),
		IncludeCfg:      true,
		CfgDir:          cfgDir,
		OutputPath:      pkgPath,
	})
	if err != nil {
		t.Fatalf("failed to create test package: %v", err)
	}

	remoteServer := lan.NewServer()
	remoteServer.SetPort(0)
	remoteServer.SetPIN("998877")
	remoteServer.SetPINEnabled(true)
	remoteServer.AddZIPBackup(pkgPath)

	if err := remoteServer.Start(); err != nil {
		t.Fatalf("failed to start remote server: %v", err)
	}
	defer remoteServer.Stop()

	remoteStatus := remoteServer.GetStatus()

	// 3. ConnectRemoteDevice
	backups, err := appSvc.ConnectRemoteDevice("127.0.0.1", remoteStatus.Port, "998877")
	if err != nil {
		t.Fatalf("ConnectRemoteDevice failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 remote backup, got %d", len(backups))
	}
	backupID := backups[0].ID

	// 4. DownloadRemoteBackup
	res, err := appSvc.DownloadRemoteBackup("127.0.0.1", remoteStatus.Port, "998877", backupID)
	if err != nil {
		t.Fatalf("DownloadRemoteBackup failed: %v", err)
	}
	if !strings.HasPrefix(res.AccountID, "remote_") {
		t.Errorf("expected remote_ accountID, got %s", res.AccountID)
	}

	// 5. Seamless integration checks:
	// a. GetBackedUpAccounts contains res.AccountID
	backedAccounts, err := appSvc.GetBackedUpAccounts()
	if err != nil {
		t.Fatalf("GetBackedUpAccounts failed: %v", err)
	}
	foundBacked := false
	for _, acc := range backedAccounts {
		if acc == res.AccountID {
			foundBacked = true
			break
		}
	}
	if !foundBacked {
		t.Errorf("res.AccountID %s not found in GetBackedUpAccounts: %v", res.AccountID, backedAccounts)
	}

	// b. 远程备份不是 Steam 账号：不得出现在 GetUsers，只在 GetRemoteBackups 里提供标签
	if got := appSvc.GetSteamPath(); got != steamDir {
		t.Fatalf("Steam 路径应是 mock 目录 %q，实际 %q（否则会去探测本机 Steam）", steamDir, got)
	}
	userCards, err := appSvc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	for _, c := range userCards {
		if c.AccountID == res.AccountID {
			t.Fatalf("remote backup %s must not appear in GetUsers", res.AccountID)
		}
	}

	var remoteCard *steam.UserCardInfo
	for _, c := range appSvc.GetRemoteBackups() {
		if c.AccountID == res.AccountID {
			tmp := c
			remoteCard = &tmp
			break
		}
	}
	if remoteCard == nil {
		t.Fatalf("remote backup %s not found in GetRemoteBackups", res.AccountID)
	}
	if !strings.Contains(remoteCard.Username, "来自其它设备的备份") {
		t.Errorf("expected username to contain '来自其它设备的备份', got %s", remoteCard.Username)
	}

	// c. GetSnapshots for remoteAccount auto-detects and migrates v0-legacy snapshot
	snaps, err := appSvc.GetSnapshots(res.AccountID)
	if err != nil {
		t.Fatalf("GetSnapshots failed for remote account: %v", err)
	}
	if len(snaps) == 0 {
		t.Errorf("expected at least 1 snapshot for remote account, got 0")
	}

	_ = appSvc.StopDiscovery()
	_ = remoteServer.Stop()
	time.Sleep(100 * time.Millisecond)
}

func TestAppService_StartCS2Game(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	steamPath := filepath.Join(tempDir, "Steam")
	_ = os.MkdirAll(steamPath, 0755)
	steamExe := filepath.Join(steamPath, "steam.exe")
	_ = os.WriteFile(steamExe, []byte("fake"), 0755)
	_ = appSvc.SetSteamPath(steamPath)

	var capturedArgs []string
	restore := steam.SetProcessStarterForTesting(func(p string, args ...string) error {
		capturedArgs = args
		return nil
	})
	defer restore()

	if err := appSvc.StartCS2Game(); err != nil {
		t.Fatalf("StartCS2Game failed: %v", err)
	}

	if len(capturedArgs) < 2 || capturedArgs[0] != "-applaunch" || capturedArgs[1] != "730" {
		t.Fatalf("expected CS2 applaunch args, got %v", capturedArgs)
	}
}
func TestAppService_SwitchSteamAccount(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	steamPath := filepath.Join(tempDir, "Steam")
	_ = os.MkdirAll(filepath.Join(steamPath, "config"), 0755)
	steamExe := filepath.Join(steamPath, "steam.exe")
	_ = os.WriteFile(steamExe, []byte("fake"), 0755)

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
	_ = os.WriteFile(filepath.Join(steamPath, "config", "loginusers.vdf"), []byte(vdfContent), 0644)
	_ = appSvc.SetSteamPath(steamPath)

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

	err := appSvc.SwitchSteamAccount("test_account_two")
	if err != nil {
		t.Fatalf("SwitchSteamAccount failed: %v", err)
	}

	// Verify logs
	logs := appSvc.GetLogs()
	found := false
	for _, l := range logs {
		if strings.Contains(l.Message, "成功切换 Steam 账号至 [test_account_two]") {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected success log entry for account switch")
	}
}

func TestAppService_ReadAndSaveConfigFile(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	steamPath := filepath.Join(tempDir, "Steam")
	userCfgDir := filepath.Join(steamPath, "userdata", "123456", "730", "local", "cfg")
	_ = os.MkdirAll(userCfgDir, 0755)
	gameCfgDir := filepath.Join(steamPath, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(gameCfgDir, 0755)

	_ = appSvc.SetSteamPath(steamPath)

	// 1. User config write and read
	userContent := "bind \"mouse1\" \"+attack\"\n"
	if err := appSvc.SaveConfigFileText("123456", "binds.cfg", userContent); err != nil {
		t.Fatalf("SaveConfigFileText user failed: %v", err)
	}

	readUser, err := appSvc.ReadConfigFile("123456", "binds.cfg")
	if err != nil {
		t.Fatalf("ReadConfigFile user failed: %v", err)
	}
	if readUser != userContent {
		t.Errorf("user config mismatch: got %q, want %q", readUser, userContent)
	}

	// 2. Game cfg write and read
	gameContent := "echo \"Autoexec Executed\"\n"
	if err := appSvc.SaveConfigFileText("game_cfg", "autoexec.cfg", gameContent); err != nil {
		t.Fatalf("SaveConfigFileText game_cfg failed: %v", err)
	}

	readGame, err := appSvc.ReadConfigFile("game_cfg", "autoexec.cfg")
	if err != nil {
		t.Fatalf("ReadConfigFile game_cfg failed: %v", err)
	}
	if readGame != gameContent {
		t.Errorf("game cfg mismatch: got %q, want %q", readGame, gameContent)
	}

	// 3. Security rejection
	if _, err := appSvc.ReadConfigFile("123456", "../outside.txt"); err == nil {
		t.Errorf("expected error for path traversal, got nil")
	}
}

func TestAppService_OpenPathInExplorer(t *testing.T) {
	tempDir := testutil.TempDir(t)
	appSvc := NewAppServiceWithConfigDir(tempDir)

	var openedPath string
	appSvc.SetPathOpener(func(p string) error {
		openedPath = p
		return nil
	})

	// 1. Valid path
	existingDir := filepath.Join(tempDir, "existing")
	if err := os.MkdirAll(existingDir, 0755); err != nil {
		t.Fatalf("failed to create existing dir: %v", err)
	}

	if err := appSvc.OpenPathInExplorer(existingDir); err != nil {
		t.Fatalf("expected OpenPathInExplorer to succeed, got %v", err)
	}
	if openedPath != filepath.Clean(existingDir) {
		t.Errorf("expected openedPath=%s, got %s", filepath.Clean(existingDir), openedPath)
	}

	// 2. Empty path
	if err := appSvc.OpenPathInExplorer(""); err == nil {
		t.Errorf("expected error for empty path, got nil")
	}

	// 3. Non-existent path
	nonExistent := filepath.Join(tempDir, "does_not_exist_12345")
	if err := appSvc.OpenPathInExplorer(nonExistent); err == nil {
		t.Errorf("expected error for non-existent path, got nil")
	}
}

// 远程备份只是备份仓库里的存档，不能出现在 Steam 账号列表里（否则会出现可点「登录」的假账号）。
func TestAppService_GetUsersExcludesRemoteBackups(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")
	backupDir := filepath.Join(tempDir, "backup")

	_ = os.MkdirAll(filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)
	loginusers := `"users"
{
	"76561198083722517"
	{
		"AccountName"		"zywoo"
		"PersonaName"		"ZywOo_CS2"
		"RememberPassword"	"1"
		"MostRecent"		"1"
	}
}`
	_ = os.WriteFile(filepath.Join(steamDir, "config", "loginusers.vdf"), []byte(loginusers), 0644)

	// 备份仓库里放一个远程下载产生的存档
	remoteID := "remote_20260101_120000"
	remoteDir := filepath.Join(backupDir, "userdata_backup", remoteID)
	_ = os.MkdirAll(filepath.Join(remoteDir, "730", "local", "cfg"), 0755)
	_ = os.WriteFile(filepath.Join(remoteDir, "remote_info.json"), []byte(`{"account_id":"`+remoteID+`","username":"远程主机备份","source_name":"ZywOo-Laptop","download_time":"2026-01-01 12:00:00"}`), 0644)
	_ = os.MkdirAll(filepath.Join(backupDir, "userdata_backup", "123456789", "730"), 0755)

	appSvc := NewAppServiceWithConfigDir(configDir)
	if err := appSvc.SetSteamPath(steamDir); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if err := appSvc.SetBackupPath(backupDir); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}

	users, err := appSvc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	for _, u := range users {
		if strings.HasPrefix(u.AccountID, "remote_") {
			t.Fatalf("remote backup %s must not appear in Steam user list", u.AccountID)
		}
	}
	if len(users) != 1 {
		t.Fatalf("expected 1 steam user, got %d", len(users))
	}

	// 但远程备份仍要能被「还原来源」识别
	remotes := appSvc.GetRemoteBackups()
	if len(remotes) != 1 || remotes[0].AccountID != remoteID {
		t.Fatalf("expected 1 remote backup label, got %+v", remotes)
	}
	if !strings.Contains(remotes[0].Username, "远程主机备份") {
		t.Errorf("unexpected remote label: %s", remotes[0].Username)
	}
}
