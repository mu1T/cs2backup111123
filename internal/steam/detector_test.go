package steam

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestValidateSteamPath(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// Empty directory should fail validation
	err := ValidateSteamPath(tempDir)
	if err == nil {
		t.Fatal("expected error for empty directory")
	}

	// Adding dummy steam.exe should make it valid
	steamExe := filepath.Join(tempDir, "steam.exe")
	if err := os.WriteFile(steamExe, []byte("dummy"), 0644); err != nil {
		t.Fatal(err)
	}

	err = ValidateSteamPath(tempDir)
	if err != nil {
		t.Fatalf("expected valid with steam.exe, got: %v", err)
	}

	// Remove steam.exe, add userdata directory
	_ = os.Remove(steamExe)
	userdataDir := filepath.Join(tempDir, "userdata")
	if err := os.Mkdir(userdataDir, 0755); err != nil {
		t.Fatal(err)
	}

	err = ValidateSteamPath(tempDir)
	if err != nil {
		t.Fatalf("expected valid with userdata dir, got: %v", err)
	}

	// Non-existent path
	err = ValidateSteamPath(filepath.Join(tempDir, "does-not-exist"))
	if err == nil {
		t.Fatal("expected error for non-existent path")
	}

	// Empty string
	err = ValidateSteamPath("")
	if err == nil {
		t.Fatal("expected error for empty string path")
	}
}

func TestDetectSteamPath(t *testing.T) {
	// On this development machine (or Windows machine with Steam installed), DetectSteamPath should either find Steam or return ErrSteamNotFound
	path, err := DetectSteamPath()
	if err == nil {
		if path == "" {
			t.Fatal("expected non-empty path if err == nil")
		}
		if err := ValidateSteamPath(path); err != nil {
			t.Fatalf("detected path %q failed validation: %v", path, err)
		}
		t.Logf("Detected Steam path on host: %s", path)
	} else {
		if err != ErrSteamNotFound {
			t.Fatalf("unexpected error: %v", err)
		}
	}
}
