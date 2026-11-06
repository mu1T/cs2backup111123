package steam

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestService_EndToEnd(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")

	// Setup fake Steam directory structure
	userdataDir := filepath.Join(steamDir, "userdata")
	_ = os.MkdirAll(filepath.Join(userdataDir, "123456789"), 0755)
	_ = os.MkdirAll(filepath.Join(userdataDir, "666000333"), 0755)
	_ = os.MkdirAll(filepath.Join(userdataDir, "123"), 0755)
	_ = os.MkdirAll(filepath.Join(steamDir, "config"), 0755)

	// Write loginusers.vdf with 123456789 as active (MostRecent = 1)
	vdfContent := `"users"
{
	"76561198083722517"
	{
		"AccountName"	"test_account_one"
		"PersonaName"	"TestPersonaOne"
		"RememberPassword"	"1"
		"MostRecent"	"1"
		"timestamp"		"1793753897"
	}
	"76561198626266061"
	{
		"AccountName"	"second"
		"PersonaName"	"donk"
		"RememberPassword"	"1"
		"MostRecent"	"0"
		"timestamp"		"1793448793"
	}
}
`
	_ = os.WriteFile(filepath.Join(steamDir, "config", "loginusers.vdf"), []byte(vdfContent), 0644)

	// Add an avatar file in steam avatar cache for 123456789
	steamAvatarDir := filepath.Join(steamDir, "config", "avatarcache")
	_ = os.MkdirAll(steamAvatarDir, 0755)
	_ = os.WriteFile(filepath.Join(steamAvatarDir, "76561198083722517.png"), []byte("avatar-bytes"), 0644)

	// Init service with mock client to keep test hermetic and offline-safe
	svc, err := NewService(configDir, &mockErrorClient{})
	if err != nil {
		t.Fatalf("NewService failed: %v", err)
	}
	// GetUsers 会派发异步头像抓取，失败时写 config/.avatar_failed_cache。
	// 不等它结束，t.TempDir 清理会和写入撞车（Windows 上表现为「目录非空」）。
	defer svc.WaitBackground()

	// Set Steam path
	err = svc.SetSteamPath(steamDir)
	if err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}

	if p := svc.GetSteamPath(); p != steamDir {
		t.Errorf("GetSteamPath = %q, want %q", p, steamDir)
	}

	// Save user note
	err = svc.SaveUserNote("123456789", "Main Account Note")
	if err != nil {
		t.Fatalf("SaveUserNote failed: %v", err)
	}

	// Get users
	users, err := svc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}

	if len(users) != 3 {
		t.Fatalf("expected 3 users, got %d", len(users))
	}

	// Active user (123456789) MUST be first at index 0
	if users[0].AccountID != "123456789" {
		t.Errorf("expected first user to be active account 123456789, got %s", users[0].AccountID)
	}
	if !users[0].RememberPassword {
		t.Errorf("expected active account to have RememberPassword = true")
	}
	if !users[0].IsRecent {
		t.Errorf("expected active account to have IsRecent = true")
	}
	if users[0].Username != "TestPersonaOne" {
		t.Errorf("expected username 'TestPersonaOne', got %q", users[0].Username)
	}
	if users[0].Notes != "Main Account Note" {
		t.Errorf("expected note 'Main Account Note', got %q", users[0].Notes)
	}
	if users[0].AvatarURL == "" {
		t.Errorf("expected non-empty avatar URL/DataURI for 123456789")
	}

	// Other accounts must be sorted numerically (123 then 666000333)
	if users[1].AccountID != "123" {
		t.Errorf("expected users[1] to be 123, got %s", users[1].AccountID)
	}
	if !users[2].RememberPassword {
		t.Errorf("expected users[2] to have RememberPassword = true")
	}
	if users[2].AccountID != "666000333" {
		t.Errorf("expected users[2] to be 666000333, got %s", users[2].AccountID)
	}

	// Test ResolveUserName
	name, err := svc.ResolveUserName("123456789")
	if err != nil || name != "TestPersonaOne" {
		t.Errorf("ResolveUserName(123456789) = %q, err=%v", name, err)
	}

	// Uncached unknown user falls back to account ID
	name, _ = svc.ResolveUserName("123")
	if name != "123" {
		t.Errorf("expected fallback '123', got %q", name)
	}
}
