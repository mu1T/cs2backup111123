package backup

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

func setupTestSteamUserdata(t *testing.T, steamPath, accountID string) {
	t.Helper()
	cfgDir := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("failed to create fake steam userdata: %v", err)
	}

	videoFile := filepath.Join(cfgDir, "cs2_video.txt")
	keysFile := filepath.Join(cfgDir, "cs2_user_keys.vcfg")

	if err := os.WriteFile(videoFile, []byte(`"VideoConfig" { "setting.csm_quality_level" "3" }`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
	if err := os.WriteFile(keysFile, []byte(`"bindings" { "w" "+forward" }`), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}
}

func TestCreateSnapshot_Basic(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "12345"

	setupTestSteamUserdata(t, steamPath, accountID)

	snap, err := CreateSnapshot(steamPath, backupPath, accountID, "初始全量备份")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	if snap.AccountID != accountID {
		t.Errorf("expected AccountID %s, got %s", accountID, snap.AccountID)
	}
	if snap.TotalFiles != 2 {
		t.Errorf("expected 2 files, got %d", snap.TotalFiles)
	}
	if snap.TotalBytes <= 0 {
		t.Errorf("expected positive total bytes, got %d", snap.TotalBytes)
	}
	if len(snap.FileHashes) != 2 {
		t.Errorf("expected 2 file hashes, got %d", len(snap.FileHashes))
	}
	if snap.Note != "初始全量备份" {
		t.Errorf("expected note '初始全量备份', got '%s'", snap.Note)
	}

	// Verify manifest on disk
	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		t.Fatalf("failed to load manifest: %v", err)
	}
	if len(m.Snapshots) != 1 {
		t.Fatalf("expected 1 snapshot in manifest, got %d", len(m.Snapshots))
	}
	if m.Snapshots[0].ID != snap.ID {
		t.Errorf("expected snapshot ID %s, got %s", snap.ID, m.Snapshots[0].ID)
	}

	// Verify snapshot file contents
	snapVideo := filepath.Join(backupPath, "snapshots", accountID, snap.ID, "local", "cfg", "cs2_video.txt")
	content, err := os.ReadFile(snapVideo)
	if err != nil {
		t.Fatalf("failed to read snapshot video file: %v", err)
	}
	if string(content) != `"VideoConfig" { "setting.csm_quality_level" "3" }` {
		t.Errorf("unexpected content: %s", string(content))
	}
}

func TestCreateSnapshot_Deduplication(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "12345"

	setupTestSteamUserdata(t, steamPath, accountID)

	// First snapshot
	snap1, err := CreateSnapshot(steamPath, backupPath, accountID, "版本 1")
	if err != nil {
		t.Fatalf("first snapshot failed: %v", err)
	}

	// Modify one file in Steam, keep the other file identical
	keysFile := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_user_keys.vcfg")
	if err := os.WriteFile(keysFile, []byte(`"bindings" { "w" "+forward" "s" "+back" }`), 0644); err != nil {
		t.Fatalf("failed to modify keys file: %v", err)
	}

	// Second snapshot
	snap2, err := CreateSnapshot(steamPath, backupPath, accountID, "版本 2")
	if err != nil {
		t.Fatalf("second snapshot failed: %v", err)
	}

	if snap1.ID == snap2.ID {
		t.Errorf("snapshots should have unique IDs")
	}

	// Video file was identical: verify hashes match
	videoRel := "local/cfg/cs2_video.txt"
	if snap1.FileHashes[videoRel] != snap2.FileHashes[videoRel] {
		t.Errorf("video file hash should match across snapshots")
	}

	// Keys file was changed: verify hashes differ
	keysRel := "local/cfg/cs2_user_keys.vcfg"
	if snap1.FileHashes[keysRel] == snap2.FileHashes[keysRel] {
		t.Errorf("keys file hash should differ across snapshots")
	}

	// Check if hardlink deduplication succeeded (when supported on OS volume)
	video1 := filepath.Join(backupPath, "snapshots", accountID, snap1.ID, "local", "cfg", "cs2_video.txt")
	video2 := filepath.Join(backupPath, "snapshots", accountID, snap2.ID, "local", "cfg", "cs2_video.txt")
	fi1, err1 := os.Stat(video1)
	fi2, err2 := os.Stat(video2)
	if err1 == nil && err2 == nil {
		if os.SameFile(fi1, fi2) {
			t.Logf("Verified NTFS hardlink deduplication active: %s <-> %s", video1, video2)
		} else {
			t.Logf("Fallback CopyFile active (different volume or permissions): contents identical")
		}
	}
}

func TestGetSnapshots_LegacyMigration(t *testing.T) {
	tmp := testutil.TempDir(t)
	backupPath := filepath.Join(tmp, "Backup")
	configDir := filepath.Join(tmp, "config")
	accountID := "99999"

	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatal(err)
	}
	notes := map[string]string{
		accountID: "我的经典老账号",
	}
	notesData, _ := json.Marshal(notes)
	if err := os.WriteFile(filepath.Join(configDir, "user_notes.json"), notesData, 0644); err != nil {
		t.Fatal(err)
	}

	// Setup legacy backup structure
	legacyDir := filepath.Join(backupPath, "userdata_backup", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(legacyDir, 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(legacyDir, "cs2_video.txt"), []byte("legacy video"), 0644); err != nil {
		t.Fatal(err)
	}

	// Call GetSnapshotsWithConfig - should auto-migrate v0-legacy
	snaps, err := GetSnapshotsWithConfig(backupPath, accountID, configDir)
	if err != nil {
		t.Fatalf("GetSnapshotsWithConfig failed: %v", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("expected 1 migrated snapshot, got %d", len(snaps))
	}
	if snaps[0].ID != "v0-legacy" {
		t.Errorf("expected ID 'v0-legacy', got %s", snaps[0].ID)
	}
	if snaps[0].Note != "我的经典老账号" {
		t.Errorf("expected preserved note '我的经典老账号', got '%s'", snaps[0].Note)
	}
	if snaps[0].TotalFiles != 1 {
		t.Errorf("expected 1 file, got %d", snaps[0].TotalFiles)
	}

	// Verify manifest exists on disk
	m, err := loadManifest(backupPath, accountID)
	if err != nil || len(m.Snapshots) != 1 {
		t.Fatalf("manifest should exist with 1 snapshot: %v", err)
	}

	// Second query should read from manifest directly
	snaps2, err := GetSnapshotsWithConfig(backupPath, accountID, configDir)
	if err != nil || len(snaps2) != 1 {
		t.Fatalf("second call failed: %v", err)
	}
}

func TestRestoreSnapshot(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "55555"

	setupTestSteamUserdata(t, steamPath, accountID)

	snap, err := CreateSnapshot(steamPath, backupPath, accountID, "待恢复存档")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Mutate live file
	videoFile := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_video.txt")
	if err := os.WriteFile(videoFile, []byte("tampered content"), 0644); err != nil {
		t.Fatalf("failed to mutate file: %v", err)
	}

	// Restore snapshot
	if err := RestoreSnapshot(steamPath, backupPath, accountID, snap.ID); err != nil {
		t.Fatalf("RestoreSnapshot failed: %v", err)
	}

	// Verify live file was restored
	restored, err := os.ReadFile(videoFile)
	if err != nil {
		t.Fatalf("failed to read restored file: %v", err)
	}
	if string(restored) != `"VideoConfig" { "setting.csm_quality_level" "3" }` {
		t.Errorf("restored content mismatch: %s", string(restored))
	}

	// Verify rollback directory was created with the tampered content
	rollbackFile := filepath.Join(backupPath, "restore_originals", "user_730", accountID, "local", "cfg", "cs2_video.txt")
	rbContent, err := os.ReadFile(rollbackFile)
	if err != nil {
		t.Fatalf("rollback file not found: %v", err)
	}
	if string(rbContent) != "tampered content" {
		t.Errorf("rollback content mismatch: %s", string(rbContent))
	}
}

func TestDeleteSnapshot(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "77777"

	setupTestSteamUserdata(t, steamPath, accountID)

	snap1, err := CreateSnapshot(steamPath, backupPath, accountID, "快照 1")
	if err != nil {
		t.Fatalf("snapshot 1 failed: %v", err)
	}

	snap2, err := CreateSnapshot(steamPath, backupPath, accountID, "快照 2")
	if err != nil {
		t.Fatalf("snapshot 2 failed: %v", err)
	}

	// Delete snap1
	if err := DeleteSnapshot(backupPath, accountID, snap1.ID); err != nil {
		t.Fatalf("DeleteSnapshot failed: %v", err)
	}

	// Verify snap1 folder is gone
	snap1Dir := filepath.Join(backupPath, "snapshots", accountID, snap1.ID)
	if _, err := os.Stat(snap1Dir); !os.IsNotExist(err) {
		t.Errorf("expected snap1 folder to be deleted")
	}

	// Verify manifest only has snap2
	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		t.Fatalf("loadManifest failed: %v", err)
	}
	if len(m.Snapshots) != 1 || m.Snapshots[0].ID != snap2.ID {
		t.Errorf("expected manifest to retain only snap2, got %v", m.Snapshots)
	}
}

func TestSnapshotManifest_CanonicalAndFallback(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "88888"

	setupTestSteamUserdata(t, steamPath, accountID)

	// Create snapshot: verify created file is snapshot_manifest.json
	snap, err := CreateSnapshot(steamPath, backupPath, accountID, "测试清单命名")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	canonicalPath := filepath.Join(backupPath, "snapshots", accountID, "snapshot_manifest.json")
	if _, err := os.Stat(canonicalPath); err != nil {
		t.Fatalf("expected canonical snapshot_manifest.json at %s, err: %v", canonicalPath, err)
	}

	// Test fallback reading: rename to manifest.json and verify loadManifest reads it
	legacyPath := filepath.Join(backupPath, "snapshots", accountID, "manifest.json")
	if err := os.Rename(canonicalPath, legacyPath); err != nil {
		t.Fatalf("failed to rename to legacy manifest: %v", err)
	}

	snaps, err := GetSnapshots(backupPath, accountID)
	if err != nil {
		t.Fatalf("GetSnapshots fallback failed: %v", err)
	}
	if len(snaps) != 1 || snaps[0].ID != snap.ID {
		t.Fatalf("expected fallback reading to find snapshot %s, got: %+v", snap.ID, snaps)
	}
}

func TestSnapshot_LegacyCS2CFGRecognition(t *testing.T) {
	tmp := testutil.TempDir(t)
	backupPath := filepath.Join(tmp, "Backup")
	configDir := filepath.Join(tmp, "config")

	// Create legacy cs2_cfg backup directory
	legacyCS2Cfg := filepath.Join(backupPath, "cs2_cfg")
	if err := os.MkdirAll(legacyCS2Cfg, 0755); err != nil {
		t.Fatalf("failed to create legacy cs2_cfg: %v", err)
	}
	autoexecFile := filepath.Join(legacyCS2Cfg, "autoexec.cfg")
	_ = os.WriteFile(autoexecFile, []byte("echo autoexec_legacy"), 0644)

	snaps, err := GetSnapshotsWithConfig(backupPath, "cs2_cfg", configDir)
	if err != nil {
		t.Fatalf("GetSnapshotsWithConfig failed for legacy cs2_cfg: %v", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("expected 1 legacy snapshot for cs2_cfg, got %d", len(snaps))
	}
	if snaps[0].ID != "v0-legacy" {
		t.Errorf("expected snapshot ID 'v0-legacy', got %s", snaps[0].ID)
	}
	if !strings.Contains(snaps[0].Note, "历史游戏全局 CFG 备份") {
		t.Errorf("expected legacy note containing CFG backup, got %s", snaps[0].Note)
	}
}
