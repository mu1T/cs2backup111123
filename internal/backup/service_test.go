package backup

import (
	"fmt"
	"bytes"
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/steam"

	"cs2backup/internal/testutil"
)

func setupTestEnvironment(t *testing.T) (string, string, string) {
	tempDir := testutil.TempDir(t)
	steamDir := filepath.Join(tempDir, "Steam")
	backupDir := filepath.Join(tempDir, "Backups")
	configDir := filepath.Join(tempDir, "Config")

	_ = os.MkdirAll(steamDir, 0755)
	_ = os.MkdirAll(backupDir, 0755)
	_ = os.MkdirAll(configDir, 0755)

	return steamDir, backupDir, configDir
}

func TestService_BackupAndRestoreUser(t *testing.T) {
	steamDir, backupDir, configDir := setupTestEnvironment(t)

	cm, err := steam.NewCacheManager(configDir)
	if err != nil {
		t.Fatal(err)
	}

	svc, err := NewService(configDir, cm)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	// Setup source user 123456789 userdata
	srcUserDir := filepath.Join(steamDir, "userdata", "123456789", "730", "local", "cfg")
	_ = os.MkdirAll(srcUserDir, 0755)
	srcVideoData := []byte("source video config 123456789")
	_ = os.WriteFile(filepath.Join(srcUserDir, "cs2_video.txt"), srcVideoData, 0644)

	// Setup target user 666000333 userdata
	targetUserDir := filepath.Join(steamDir, "userdata", "666000333", "730", "local", "cfg")
	_ = os.MkdirAll(targetUserDir, 0755)
	origTargetData := []byte("original target video config 666000333")
	_ = os.WriteFile(filepath.Join(targetUserDir, "cs2_video.txt"), origTargetData, 0644)

	// 1. Backup User
	if err := svc.BackupUser("123456789"); err != nil {
		t.Fatalf("BackupUser failed: %v", err)
	}

	backupFile := filepath.Join(backupDir, "userdata_backup", "123456789", "730", "local", "cfg", "cs2_video.txt")
	readBackup, err := os.ReadFile(backupFile)
	if err != nil {
		t.Fatalf("failed to read backup file %s: %v", backupFile, err)
	}
	if !bytes.Equal(srcVideoData, readBackup) {
		t.Errorf("backup file content mismatch")
	}

	// 2. Restore User (123456789 -> 666000333)
	if err := svc.RestoreUser("666000333", "123456789"); err != nil {
		t.Fatalf("RestoreUser failed: %v", err)
	}

	// Check rollback snapshot
	rollbackFile := filepath.Join(backupDir, "restore_originals", "user_730", "666000333", "local", "cfg", "cs2_video.txt")
	readRollback, err := os.ReadFile(rollbackFile)
	if err != nil {
		t.Fatalf("rollback file missing: %v", err)
	}
	if !bytes.Equal(origTargetData, readRollback) {
		t.Errorf("expected rollback file to retain original target data")
	}

	// Check restored target data
	readRestored, err := os.ReadFile(filepath.Join(targetUserDir, "cs2_video.txt"))
	if err != nil {
		t.Fatalf("target file missing: %v", err)
	}
	if !bytes.Equal(srcVideoData, readRestored) {
		t.Errorf("restored target data mismatch")
	}
}

func TestService_BackupAndRestoreGameCFG(t *testing.T) {
	steamDir, backupDir, configDir := setupTestEnvironment(t)

	svc, err := NewService(configDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	// Setup CS2 game CFG directory
	cs2CfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(cs2CfgDir, 0755)

	// Default files
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "boot.vcfg"), []byte("boot"), 0644)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "gamemode_competitive.cfg"), []byte("comp"), 0644)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "server.cfg"), []byte("server"), 0644)

	// Custom files
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "autoexec.cfg"), []byte("echo autoexec loaded"), 0644)
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "practice.cfg"), []byte("sv_cheats 1"), 0644)

	// 1. Mode "custom": filters out boot.vcfg, gamemode_competitive.cfg, server.cfg
	count, err := svc.BackupGameCFG("custom", nil)
	if err != nil {
		t.Fatalf("BackupGameCFG custom failed: %v", err)
	}
	if count != 2 {
		t.Errorf("expected 2 custom files backed up, got %d", count)
	}

	if _, err := os.Stat(filepath.Join(backupDir, "cs2_cfg", "autoexec.cfg")); err != nil {
		t.Errorf("expected autoexec.cfg in backup")
	}
	if _, err := os.Stat(filepath.Join(backupDir, "cs2_cfg", "boot.vcfg")); err == nil {
		t.Errorf("boot.vcfg should have been filtered out")
	}

	// 2. Mode "all": backs up all 5 files
	countAll, err := svc.BackupGameCFG("all", nil)
	if err != nil {
		t.Fatalf("BackupGameCFG all failed: %v", err)
	}
	if countAll != 5 {
		t.Errorf("expected 5 files backed up in all mode, got %d", countAll)
	}

	// 3. Mode "selected": backs up only selected files
	selFile := filepath.Join(cs2CfgDir, "autoexec.cfg")
	countSel, err := svc.BackupGameCFG("selected", []string{selFile, "bad.exe"})
	if err != nil {
		t.Fatalf("BackupGameCFG selected failed: %v", err)
	}
	if countSel != 1 {
		t.Errorf("expected 1 selected file backed up, got %d", countSel)
	}

	// 4. Restore Game CFG
	// Modify cs2CfgDir to simulate changed/lost files
	_ = os.WriteFile(filepath.Join(cs2CfgDir, "autoexec.cfg"), []byte("corrupted"), 0644)

	restoredCount, err := svc.RestoreGameCFG()
	if err != nil {
		t.Fatalf("RestoreGameCFG failed: %v", err)
	}
	if restoredCount != 1 {
		t.Errorf("expected 1 file restored, got %d", restoredCount)
	}

	// Verify rollback point exists
	rollbackFile := filepath.Join(backupDir, "restore_originals", "cs2_cfg_global", "autoexec.cfg")
	readRollback, err := os.ReadFile(rollbackFile)
	if err != nil {
		t.Fatalf("rollback file missing: %v", err)
	}
	if string(readRollback) != "corrupted" {
		t.Errorf("expected rollback to have 'corrupted', got %q", string(readRollback))
	}

	// 3. Test HasGameCFGRollback & RollbackGameCFG
	if !svc.HasGameCFGRollback() {
		t.Errorf("expected HasGameCFGRollback to be true")
	}
	if _, err := svc.RollbackGameCFG(); err != nil {
		t.Fatalf("RollbackGameCFG failed: %v", err)
	}
	readAfterRollback, err := os.ReadFile(filepath.Join(cs2CfgDir, "autoexec.cfg"))
	if err != nil {
		t.Fatalf("failed to read live file after rollback: %v", err)
	}
	if string(readAfterRollback) != "corrupted" {
		t.Errorf("expected game cfg after rollback to have 'corrupted', got %q", string(readAfterRollback))
	}
}

func TestService_GetBackedUpAccounts(t *testing.T) {
	_, backupDir, configDir := setupTestEnvironment(t)

	svc, err := NewService(configDir, nil)
	if err != nil {
		t.Fatal(err)
	}
	_ = svc.SetBackupPath(backupDir)

	// Create test backup accounts
	_ = os.MkdirAll(filepath.Join(backupDir, "userdata_backup", "666000333", "730"), 0755)
	_ = os.MkdirAll(filepath.Join(backupDir, "userdata_backup", "123456789", "730"), 0755)
	_ = os.MkdirAll(filepath.Join(backupDir, "userdata_backup", "123", "730"), 0755)
	_ = os.MkdirAll(filepath.Join(backupDir, "userdata_backup", "remote_laptop", "730"), 0755)

	accounts, err := svc.GetBackedUpAccounts()
	if err != nil {
		t.Fatalf("GetBackedUpAccounts failed: %v", err)
	}

	expected := []string{"123", "123456789", "666000333", "remote_laptop"}
	if len(accounts) != len(expected) {
		t.Fatalf("expected %d accounts, got %d: %v", len(expected), len(accounts), accounts)
	}

	for i, exp := range expected {
		if accounts[i] != exp {
			t.Errorf("index %d: expected %s, got %s", i, exp, accounts[i])
		}
	}
}

/**
 * 备份不再覆盖：连续备份两次得到两个时间戳不同的历史版本，镜像指向最新那个。
 *
 * 以前 BackupUser 是先 RemoveAll 再 CopyDir，第二次备份会把第一次的内容抹掉。
 */
func TestBackupUser_CreatesTimestampedVersions(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	const accountID = "123456789"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	videoFile := filepath.Join(cfgDir, "cs2_video.txt")

	if err := os.WriteFile(videoFile, []byte("第一版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.BackupUser(accountID); err != nil {
		t.Fatalf("第一次 BackupUser 失败: %v", err)
	}

	if err := os.WriteFile(videoFile, []byte("第二版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.BackupUser(accountID); err != nil {
		t.Fatalf("第二次 BackupUser 失败: %v", err)
	}

	// 1. 两个历史版本，各自内容都在
	versions := listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))
	if len(versions) != 2 {
		t.Fatalf("期望 2 个历史版本，实际 %d 个: %v", len(versions), versions)
	}
	contents := map[string]string{}
	for _, v := range versions {
		data, err := os.ReadFile(filepath.Join(backupDir, "snapshots", accountID, v, "local", "cfg", "cs2_video.txt"))
		if err != nil {
			t.Fatalf("读版本 %s 失败: %v", v, err)
		}
		contents[v] = string(data)
	}
	got := map[string]bool{}
	for _, c := range contents {
		got[c] = true
	}
	if !got["第一版"] || !got["第二版"] {
		t.Errorf("两个版本的内容应分别是「第一版」「第二版」，实际 %v", contents)
	}

	// 2. 镜像指向最新那一版
	mirror, err := os.ReadFile(filepath.Join(backupDir, "userdata_backup", accountID, "730", "local", "cfg", "cs2_video.txt"))
	if err != nil {
		t.Fatalf("读镜像失败: %v", err)
	}
	if string(mirror) != "第二版" {
		t.Errorf("镜像应指向最新版本「第二版」，实际 %q", string(mirror))
	}
}

// listVersionDirs 列出某个账号快照目录下的版本子目录（跳过 manifest.json）。
func listVersionDirs(t *testing.T, dir string) []string {
	t.Helper()
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读快照目录失败: %v", err)
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	return out
}

/**
 * 备份到第 11 次之后，只剩最近 10 个版本——上限默认 10，超出自动裁剪。
 */
func TestBackupUser_PrunesToRetentionLimit(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	const accountID = "555"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	videoFile := filepath.Join(cfgDir, "cs2_video.txt")

	var lastDropped []string
	for i := 0; i < 12; i++ {
		// 每次都改内容，确保是 12 个不同的版本而不是去重成 1 个
		if err := os.WriteFile(videoFile, []byte(fmt.Sprintf("v%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		dropped, err := svc.BackupUserWithReport(accountID)
		if err != nil {
			t.Fatalf("第 %d 次备份失败: %v", i+1, err)
		}
		lastDropped = dropped
	}

	versions := listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))
	if len(versions) != DefaultRetentionLimit {
		t.Fatalf("期望只剩 %d 个版本，实际 %d 个", DefaultRetentionLimit, len(versions))
	}

	// 裁剪是逐次收敛的：第 11 次备份之后就一直稳定在 10 个，
	// 所以第 12 次只裁掉 1 个（进来一个、出去一个），不是「12 - 10 = 2」。
	if len(lastDropped) != 1 {
		t.Errorf("第 12 次备份应报告裁掉 1 个版本，实际 %d 个: %v", len(lastDropped), lastDropped)
	}

	// 被裁掉的版本目录真的不在了
	for _, id := range lastDropped {
		if _, err := os.Stat(filepath.Join(backupDir, "snapshots", accountID, id)); !os.IsNotExist(err) {
			t.Errorf("被裁掉的版本 %s 目录还在", id)
		}
	}

	// 最新版本仍在，且镜像指向它
	if err := os.WriteFile(videoFile, []byte("latest"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupUserWithReport(accountID); err != nil {
		t.Fatal(err)
	}
	mirror, err := os.ReadFile(filepath.Join(backupDir, "userdata_backup", accountID, "730", "local", "cfg", "cs2_video.txt"))
	if err != nil {
		t.Fatalf("读镜像失败: %v", err)
	}
	if string(mirror) != "latest" {
		t.Errorf("镜像应指向最新版本，实际 %q", string(mirror))
	}
}

/**
 * 上限设成 0 表示不清理——误设不该导致历史被删光。
 */
func TestBackupUser_ZeroRetentionKeepsAll(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)
	svc.SetRetentionLimit(0)

	const accountID = "777"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(cfgDir, "a.cfg")
	for i := 0; i < 12; i++ {
		if err := os.WriteFile(f, []byte(fmt.Sprintf("v%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.BackupUserWithReport(accountID); err != nil {
			t.Fatalf("第 %d 次备份失败: %v", i+1, err)
		}
	}

	if got := len(listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))); got != 12 {
		t.Errorf("上限 0 应保留全部 12 个版本，实际 %d 个", got)
	}
}

/**
 * 还原到指定历史版本：文件回到那一版、还原前自动留底、镜像跟着刷到那一版。
 */
func TestRestoreVersion_RestoresAndRefreshesMirror(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	const accountID = "888"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	videoFile := filepath.Join(cfgDir, "cs2_video.txt")

	// 第一版
	if err := os.WriteFile(videoFile, []byte("第一版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupUserWithReport(accountID); err != nil {
		t.Fatal(err)
	}
	firstVersions := listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))
	if len(firstVersions) != 1 {
		t.Fatalf("期望 1 个版本，实际 %d 个", len(firstVersions))
	}
	firstID := firstVersions[0]

	// 第二版
	if err := os.WriteFile(videoFile, []byte("第二版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupUserWithReport(accountID); err != nil {
		t.Fatal(err)
	}

	// 还原到第一版
	if _, err := svc.RestoreVersion(accountID, firstID); err != nil {
		t.Fatalf("RestoreVersion 失败: %v", err)
	}

	// 1. 本地文件回到第一版
	got, err := os.ReadFile(videoFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "第一版" {
		t.Errorf("还原后本地文件应为「第一版」，实际 %q", string(got))
	}

	// 2. 镜像刷到被还原的版本（局域网共享打的包要跟着变）
	mirror, err := os.ReadFile(filepath.Join(backupDir, "userdata_backup", accountID, "730", "local", "cfg", "cs2_video.txt"))
	if err != nil {
		t.Fatal(err)
	}
	if string(mirror) != "第一版" {
		t.Errorf("镜像应指向被还原的版本「第一版」，实际 %q", string(mirror))
	}

	// 3. 还原前自动留了底：版本数从 2 变成 3
	versions := listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))
	if len(versions) != 3 {
		t.Errorf("还原应自动留底，期望 3 个版本，实际 %d 个: %v", len(versions), versions)
	}
}

/** 还原一个不存在的版本时报错，且不破坏现有数据。 */
func TestRestoreVersion_MissingVersionFails(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)

	const accountID = "999"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	orig := []byte("原始")
	if err := os.WriteFile(filepath.Join(cfgDir, "a.cfg"), orig, 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RestoreVersion(accountID, "不存在的版本"); err == nil {
		t.Fatal("还原不存在的版本应该报错")
	}

	// 本地文件没被动过
	got, _ := os.ReadFile(filepath.Join(cfgDir, "a.cfg"))
	if !bytes.Equal(got, orig) {
		t.Errorf("还原失败不该改动本地文件，实际 %q", string(got))
	}
}

/**
 * PreviewPrune 只算不删：列表里有会删哪些，磁盘上一个都没动。
 */
func TestPreviewPrune_DoesNotDelete(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)
	// 把上限调小，制造「存量超限」的局面
	svc.SetRetentionLimit(0) // 先不清理，堆够 5 个

	const accountID = "321"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(cfgDir, "a.cfg")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(f, []byte(fmt.Sprintf("v%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.BackupUserWithReport(accountID); err != nil {
			t.Fatal(err)
		}
	}

	svc.SetRetentionLimit(2)
	before := len(listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID)))

	doomed, err := svc.PreviewPrune(accountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(doomed) != 3 {
		t.Errorf("上限 2、存量 5，应预览到 3 个将被删除，实际 %d 个: %v", len(doomed), doomed)
	}

	after := len(listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID)))
	if after != before {
		t.Errorf("预览不该删任何东西：预览前 %d 个，预览后 %d 个", before, after)
	}
}

/** ApplyPrune 真的收敛到上限，并返回被删的 ID。 */
func TestApplyPrune_DeletesToLimit(t *testing.T) {
	steamDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(steamDir)
	_ = svc.SetBackupPath(backupDir)
	svc.SetRetentionLimit(0)

	const accountID = "654"
	cfgDir := filepath.Join(steamDir, "userdata", accountID, "730")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}
	f := filepath.Join(cfgDir, "a.cfg")
	for i := 0; i < 5; i++ {
		if err := os.WriteFile(f, []byte(fmt.Sprintf("v%d", i)), 0o644); err != nil {
			t.Fatal(err)
		}
		if _, err := svc.BackupUserWithReport(accountID); err != nil {
			t.Fatal(err)
		}
	}

	svc.SetRetentionLimit(2)
	dropped, err := svc.ApplyPrune(accountID)
	if err != nil {
		t.Fatal(err)
	}
	if len(dropped) != 3 {
		t.Errorf("应删掉 3 个，实际 %d 个: %v", len(dropped), dropped)
	}

	remaining := listVersionDirs(t, filepath.Join(backupDir, "snapshots", accountID))
	if len(remaining) != 2 {
		t.Errorf("收敛后应剩 2 个，实际 %d 个: %v", len(remaining), remaining)
	}
	for _, id := range dropped {
		if _, err := os.Stat(filepath.Join(backupDir, "snapshots", accountID, id)); !os.IsNotExist(err) {
			t.Errorf("被删的版本 %s 目录还在", id)
		}
	}
}

/**
 * 通用 CFG 也版本化：两次备份得到两个版本，镜像指向最新，超出上限自动裁剪。
 */
func TestBackupGameCFG_VersionedAndPruned(t *testing.T) {
	cfgDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(testutil.TempDir(t))
	_ = svc.SetBackupPath(backupDir)
	// SetCS2Path 会校验「这像不像 cfg 目录」，所以先放一个文件进去
	cfgFile := filepath.Join(cfgDir, "autoexec.cfg")
	if err := os.WriteFile(cfgFile, []byte("placeholder"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCS2Path(cfgDir); err != nil {
		t.Fatalf("SetCS2Path failed: %v", err)
	}
	versionsRoot := filepath.Join(backupDir, "cs2_cfg_versions")

	// 第一版
	if err := os.WriteFile(cfgFile, []byte("第一版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.BackupGameCFG("all", nil); err != nil || n != 1 {
		t.Fatalf("第一次备份失败: n=%d err=%v", n, err)
	}

	// 第二版
	if err := os.WriteFile(cfgFile, []byte("第二版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if n, err := svc.BackupGameCFG("all", nil); err != nil || n != 1 {
		t.Fatalf("第二次备份失败: n=%d err=%v", n, err)
	}

	// 1. 两个历史版本，内容分别是两版
	versions := listVersionDirs(t, versionsRoot)
	if len(versions) != 2 {
		t.Fatalf("期望 2 个通用 CFG 版本，实际 %d 个: %v", len(versions), versions)
	}
	seen := map[string]bool{}
	for _, v := range versions {
		data, err := os.ReadFile(filepath.Join(versionsRoot, v, "autoexec.cfg"))
		if err != nil {
			t.Fatalf("读版本 %s 失败: %v", v, err)
		}
		seen[string(data)] = true
	}
	if !seen["第一版"] || !seen["第二版"] {
		t.Errorf("两个版本内容应分别是两版，实际 %v", seen)
	}

	// 2. 镜像指向最新
	mirror, err := os.ReadFile(filepath.Join(backupDir, "cs2_cfg", "autoexec.cfg"))
	if err != nil {
		t.Fatalf("读镜像失败: %v", err)
	}
	if string(mirror) != "第二版" {
		t.Errorf("镜像应指向最新版本，实际 %q", string(mirror))
	}

	// 3. 上限设成 1，再备份一次 → 只剩 1 个版本
	svc.SetRetentionLimit(1)
	if err := os.WriteFile(cfgFile, []byte("第三版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupGameCFG("all", nil); err != nil {
		t.Fatal(err)
	}
	if got := len(listVersionDirs(t, versionsRoot)); got != 1 {
		t.Errorf("上限 1 时应只剩 1 个版本，实际 %d 个", got)
	}
}

/** 只备份玩家自定义时，Valve 官方文件不进版本目录。 */
func TestBackupGameCFG_CustomSkipsDefaults(t *testing.T) {
	cfgDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(testutil.TempDir(t))
	_ = svc.SetBackupPath(backupDir)
	if err := os.WriteFile(filepath.Join(cfgDir, "autoexec.cfg"), []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCS2Path(cfgDir); err != nil {
		t.Fatalf("SetCS2Path failed: %v", err)
	}
	if err := os.WriteFile(filepath.Join(cfgDir, "gamemode_competitive.cfg"), []byte("valve"), 0o644); err != nil {
		t.Fatal(err)
	}

	n, err := svc.BackupGameCFG("custom", nil)
	if err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("只备份自定义时应只拷 1 个文件，实际 %d 个", n)
	}
}

/** 通用 CFG 的版本列表按时间倒序；按版本还原能回到那一版。 */
func TestGameCFGVersions_ListAndRestore(t *testing.T) {
	cfgDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(testutil.TempDir(t))
	_ = svc.SetBackupPath(backupDir)

	cfgFile := filepath.Join(cfgDir, "autoexec.cfg")
	if err := os.WriteFile(cfgFile, []byte("第一版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCS2Path(cfgDir); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupGameCFG("all", nil); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cfgFile, []byte("第二版"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.BackupGameCFG("all", nil); err != nil {
		t.Fatal(err)
	}

	versions, err := svc.ListGameCFGVersions()
	if err != nil {
		t.Fatal(err)
	}
	if len(versions) != 2 {
		t.Fatalf("期望 2 个版本，实际 %d 个: %v", len(versions), versions)
	}
	// 倒序：第一个是最新的
	if versions[0] < versions[1] {
		t.Errorf("版本应按时间倒序，实际 %v", versions)
	}

	// 还原到较旧的那一版
	if _, err := svc.RestoreGameCFGVersion(versions[1]); err != nil {
		t.Fatalf("按版本还原失败: %v", err)
	}
	got, err := os.ReadFile(cfgFile)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "第一版" {
		t.Errorf("应还原到「第一版」，实际 %q", string(got))
	}
}

/** 还原不存在的版本时报错，不改动本地文件。 */
func TestRestoreGameCFGVersion_MissingFails(t *testing.T) {
	cfgDir := testutil.TempDir(t)
	backupDir := testutil.TempDir(t)

	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatal(err)
	}
	svc.SetSteamPath(testutil.TempDir(t))
	_ = svc.SetBackupPath(backupDir)

	cfgFile := filepath.Join(cfgDir, "autoexec.cfg")
	orig := []byte("原始")
	if err := os.WriteFile(cfgFile, orig, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCS2Path(cfgDir); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.RestoreGameCFGVersion("不存在的版本"); err == nil {
		t.Fatal("还原不存在的版本应该报错")
	}
	got, _ := os.ReadFile(cfgFile)
	if !bytes.Equal(got, orig) {
		t.Errorf("失败不该改动本地文件，实际 %q", string(got))
	}
}
