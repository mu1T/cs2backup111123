package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestDetectCS2CFGPath_LibraryFoldersVDF(t *testing.T) {
	tempDir := testutil.TempDir(t)

	steamDir := filepath.Join(tempDir, "Steam")
	secondaryLib := filepath.Join(tempDir, "Games", "SteamLibrary")

	_ = os.MkdirAll(filepath.Join(steamDir, "steamapps"), 0755)
	_ = os.MkdirAll(filepath.Join(secondaryLib, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg"), 0755)

	// Write libraryfolders.vdf pointing to secondaryLib
	// Note: in VDF backslashes are escaped
	escapedLib := secondaryLib
	vdfContent := fmt.Sprintf(`"libraryfolders"
{
	"0"
	{
		"path" "%s"
	}
	"1"
	{
		"path" "%s"
		"apps"
		{
			"730" "35948384210"
		}
	}
}`, filepath.ToSlash(steamDir), filepath.ToSlash(escapedLib))

	_ = os.WriteFile(filepath.Join(steamDir, "steamapps", "libraryfolders.vdf"), []byte(vdfContent), 0644)

	detected, err := DetectCS2CFGPath(steamDir)
	if err != nil {
		t.Fatalf("DetectCS2CFGPath failed: %v", err)
	}

	expected := filepath.Join(secondaryLib, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	if filepath.Clean(detected) != filepath.Clean(expected) {
		t.Errorf("expected %s, got %s", expected, detected)
	}
}

func TestDetectCS2CFGPath_FallbackToSteamCommon(t *testing.T) {
	tempDir := testutil.TempDir(t)

	steamDir := filepath.Join(tempDir, "Steam")
	cs2CfgDir := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(cs2CfgDir, 0755)

	detected, err := DetectCS2CFGPath(steamDir)
	if err != nil {
		t.Fatalf("DetectCS2CFGPath fallback failed: %v", err)
	}

	if filepath.Clean(detected) != filepath.Clean(cs2CfgDir) {
		t.Errorf("expected %s, got %s", cs2CfgDir, detected)
	}
}

func TestDetectCS2CFGPath_NotFound(t *testing.T) {
	tempDir := testutil.TempDir(t)
	steamDir := filepath.Join(tempDir, "SteamEmpty")
	_ = os.MkdirAll(steamDir, 0755)

	_, err := DetectCS2CFGPath(steamDir)
	if err == nil {
		t.Errorf("expected error when CS2 cfg is not found, got nil")
	}
}
