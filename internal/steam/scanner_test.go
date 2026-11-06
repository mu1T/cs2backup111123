package steam

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestScanUserdataAccountIDs(t *testing.T) {
	tempDir := testutil.TempDir(t)
	userdataDir := filepath.Join(tempDir, "userdata")
	if err := os.Mkdir(userdataDir, 0755); err != nil {
		t.Fatal(err)
	}

	// Create test directories
	testFolders := []string{
		"666000333",
		"123456789",
		"123",
		"0",           // Anonymous folder, should be skipped
		"not_numeric", // Non numeric, should be skipped
		"temp.txt",    // File, should be skipped
	}

	for _, name := range testFolders {
		path := filepath.Join(userdataDir, name)
		if name == "temp.txt" {
			_ = os.WriteFile(path, []byte("file"), 0644)
		} else {
			_ = os.Mkdir(path, 0755)
		}
	}

	ids, err := ScanUserdataAccountIDs(tempDir)
	if err != nil {
		t.Fatalf("ScanUserdataAccountIDs failed: %v", err)
	}

	expected := []uint32{123, 123456789, 666000333}
	if len(ids) != len(expected) {
		t.Fatalf("expected %d IDs, got %d: %v", len(expected), len(ids), ids)
	}

	for i := range expected {
		if ids[i] != expected[i] {
			t.Errorf("at index %d: expected %d, got %d", i, expected[i], ids[i])
		}
	}
}

func TestScanUserdataAccountIDs_MissingUserdata(t *testing.T) {
	tempDir := testutil.TempDir(t)
	_, err := ScanUserdataAccountIDs(tempDir)
	if err == nil {
		t.Fatal("expected error when userdata does not exist")
	}
}
