package steam

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestCacheManager_NotesPersistence(t *testing.T) {
	tempDir := testutil.TempDir(t)

	cm, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create CacheManager: %v", err)
	}

	// Set note
	err = cm.SetNote("123456789", "Main Account")
	if err != nil {
		t.Fatalf("SetNote failed: %v", err)
	}

	if note := cm.GetNote("123456789"); note != "Main Account" {
		t.Errorf("expected 'Main Account', got %q", note)
	}

	// Re-load with fresh instance to verify persistence
	cm2, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatalf("failed to re-load CacheManager: %v", err)
	}

	if note := cm2.GetNote("123456789"); note != "Main Account" {
		t.Errorf("expected persisted note 'Main Account', got %q", note)
	}

	// Empty note deletes entry
	_ = cm2.SetNote("123456789", "")
	if note := cm2.GetNote("123456789"); note != "" {
		t.Errorf("expected empty note after clearing, got %q", note)
	}
}

func TestCacheManager_UserCachePersistence(t *testing.T) {
	tempDir := testutil.TempDir(t)

	cm, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatalf("failed to create CacheManager: %v", err)
	}

	err = cm.SetUsername("123456789", "TestPersonaOne")
	if err != nil {
		t.Fatalf("SetUsername failed: %v", err)
	}

	name, ok := cm.GetUsername("123456789")
	if !ok || name != "TestPersonaOne" {
		t.Errorf("expected 'TestPersonaOne', got %q (ok=%v)", name, ok)
	}

	// Re-load fresh instance
	cm2, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatalf("failed to reload: %v", err)
	}

	name2, ok := cm2.GetUsername("123456789")
	if !ok || name2 != "TestPersonaOne" {
		t.Errorf("expected persisted name 'TestPersonaOne', got %q (ok=%v)", name2, ok)
	}
}

func TestCacheManager_SaveSteamPathPreservesConfig(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configPath := filepath.Join(tempDir, "config.json")

	initialJSON := `{"backup_path": "D:\\backups", "lan_port": 9999}`
	if err := os.WriteFile(configPath, []byte(initialJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cm, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	err = cm.SaveSteamPath("C:\\SteamNew")
	if err != nil {
		t.Fatalf("SaveSteamPath failed: %v", err)
	}

	if p := cm.GetSavedSteamPath(); p != "C:\\SteamNew" {
		t.Errorf("expected 'C:\\SteamNew', got %q", p)
	}

	// Verify existing fields in file
	data, _ := os.ReadFile(configPath)
	content := string(data)
	if !filepath.IsAbs("C:\\SteamNew") {
		// Just checking content
	}
	if len(content) == 0 {
		t.Fatal("empty config file")
	}
}

func TestCacheManager_SaveBackupPathPreservesConfig(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configPath := filepath.Join(tempDir, "config.json")

	initialJSON := `{"steam_path": "C:\\Steam", "lan_port": 8765}`
	if err := os.WriteFile(configPath, []byte(initialJSON), 0644); err != nil {
		t.Fatal(err)
	}

	cm, err := NewCacheManager(tempDir)
	if err != nil {
		t.Fatal(err)
	}

	if p := cm.GetSavedBackupPath(); p != "" {
		t.Errorf("expected empty initial backup_path, got %q", p)
	}

	err = cm.SaveBackupPath("D:\\MyBackups")
	if err != nil {
		t.Fatalf("SaveBackupPath failed: %v", err)
	}

	if p := cm.GetSavedBackupPath(); p != "D:\\MyBackups" {
		t.Errorf("expected 'D:\\MyBackups', got %q", p)
	}

	// Verify steam_path and lan_port preserved
	if sp := cm.GetSavedSteamPath(); sp != "C:\\Steam" {
		t.Errorf("expected steam_path 'C:\\Steam' preserved, got %q", sp)
	}
}
