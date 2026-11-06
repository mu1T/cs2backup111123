package steam

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

const sampleSwitchUsersVDF = `"users"
{
	"76561198083722517"
	{
		"AccountName"		"test_account_one"
		"PersonaName"		"TestPersonaOne"
		"RememberPassword"	"1"
		"MostRecent"		"1"
		"Timestamp"			"1793753897"
		"WantsOfflineMode"	"0"
	}
	"76561198947920049"
	{
		"AccountName"		"test_account_two"
		"PersonaName"		"TestPersonaTwo"
		"RememberPassword"	"1"
		"MostRecent"		"0"
		"Timestamp"			"1793709635"
		"WantsOfflineMode"	"0"
	}
	"76561198000000003"
	{
		"AccountName"		"user_three"
		"PersonaName"		"SniperWolf"
		"RememberPassword"	"1"
		"Timestamp"			"1793701000"
		"WantsOfflineMode"	"0"
	}
}
`

func TestUpdateLoginUsersMostRecent_Success(t *testing.T) {
	tempDir := testutil.TempDir(t)
	vdfPath := filepath.Join(tempDir, "loginusers.vdf")
	if err := os.WriteFile(vdfPath, []byte(sampleSwitchUsersVDF), 0644); err != nil {
		t.Fatalf("failed to write test vdf: %v", err)
	}

	// Switch to test_account_two (by AccountName)
	if err := UpdateLoginUsersMostRecent(vdfPath, "test_account_two"); err != nil {
		t.Fatalf("UpdateLoginUsersMostRecent failed: %v", err)
	}

	// Verify updated VDF
	users, err := ParseLoginUsersFile(vdfPath)
	if err != nil {
		t.Fatalf("ParseLoginUsersFile failed: %v", err)
	}

	// test_account_two (account ID Steam64ToAccountID(76561198947920049)) must be MostRecent
	test_account_twoAid := Steam64ToAccountID(76561198947920049)
	test_account_oneAid := Steam64ToAccountID(76561198083722517)
	userThreeAid := Steam64ToAccountID(76561198000000003)

	if u, ok := users[test_account_twoAid]; !ok || !u.MostRecent {
		t.Errorf("expected test_account_two to have MostRecent == true, got %v", u)
	}
	if u, ok := users[test_account_oneAid]; !ok || u.MostRecent {
		t.Errorf("expected test_account_one to have MostRecent == false, got %v", u)
	}
	if u, ok := users[userThreeAid]; !ok || u.MostRecent {
		t.Errorf("expected user_three to have MostRecent == false, got %v", u)
	}
}

func TestUpdateLoginUsersMostRecent_MissingKeyInsertion(t *testing.T) {
	tempDir := testutil.TempDir(t)
	vdfPath := filepath.Join(tempDir, "loginusers.vdf")
	if err := os.WriteFile(vdfPath, []byte(sampleSwitchUsersVDF), 0644); err != nil {
		t.Fatalf("failed to write test vdf: %v", err)
	}

	// user_three in sampleSwitchUsersVDF originally had NO MostRecent key.
	// Switch to user_three (by SteamID64)
	if err := UpdateLoginUsersMostRecent(vdfPath, "76561198000000003"); err != nil {
		t.Fatalf("UpdateLoginUsersMostRecent failed: %v", err)
	}

	users, err := ParseLoginUsersFile(vdfPath)
	if err != nil {
		t.Fatalf("ParseLoginUsersFile failed: %v", err)
	}

	userThreeAid := Steam64ToAccountID(76561198000000003)
	if u, ok := users[userThreeAid]; !ok || !u.MostRecent {
		t.Errorf("expected user_three to have MostRecent == true after insertion, got %v", u)
	}

	// Verify the raw file content contains "MostRecent" "1" inside user_three block
	content, err := os.ReadFile(vdfPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(content), `"MostRecent"`) {
		t.Errorf("expected file to contain MostRecent key")
	}
}

func TestUpdateLoginUsersMostRecent_AccountNotFound(t *testing.T) {
	tempDir := testutil.TempDir(t)
	vdfPath := filepath.Join(tempDir, "loginusers.vdf")
	if err := os.WriteFile(vdfPath, []byte(sampleSwitchUsersVDF), 0644); err != nil {
		t.Fatalf("failed to write test vdf: %v", err)
	}

	err := UpdateLoginUsersMostRecent(vdfPath, "non_existent_account")
	if err == nil {
		t.Fatal("expected error for non existent account, got nil")
	}
}

func TestSwitchSteamAccount_Lifecycle(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	_ = os.MkdirAll(configDir, 0755)
	vdfPath := filepath.Join(configDir, "loginusers.vdf")
	if err := os.WriteFile(vdfPath, []byte(sampleSwitchUsersVDF), 0644); err != nil {
		t.Fatal(err)
	}

	fakeSteamExe := filepath.Join(tempDir, "steam.exe")
	if err := os.WriteFile(fakeSteamExe, []byte("fake-steam"), 0755); err != nil {
		t.Fatal(err)
	}

	// Save and restore hooks
	origChecker := runningChecker
	origShutdowner := steamShutdowner
	origRegistry := registrySetter
	origStarter := processStarter
	defer func() {
		runningChecker = origChecker
		steamShutdowner = origShutdowner
		registrySetter = origRegistry
		processStarter = origStarter
	}()

	runningCalls := 0
	runningChecker = func() (bool, error) {
		runningCalls++
		return true, nil // Steam is running initially
	}

	shutdownCalled := false
	steamShutdowner = func(steamPath string) error {
		shutdownCalled = true
		return nil
	}

	var registryAccount string
	registrySetter = func(accountName string) error {
		registryAccount = accountName
		return nil
	}

	starterCalled := false
	var starterPath string
	processStarter = func(path string, args ...string) error {
		starterCalled = true
		starterPath = path
		return nil
	}

	// Switch to test_account_two
	err := SwitchSteamAccount(tempDir, "test_account_two")
	if err != nil {
		t.Fatalf("SwitchSteamAccount failed: %v", err)
	}

	if !shutdownCalled {
		t.Errorf("expected steamShutdowner to be called")
	}
	if registryAccount != "test_account_two" {
		t.Errorf("expected registry set to 'test_account_two', got %q", registryAccount)
	}
	if !starterCalled || starterPath != tempDir {
		t.Errorf("expected processStarter called with %q, got %v (path: %s)", tempDir, starterCalled, starterPath)
	}

	// Verify loginusers.vdf updated
	users, err := ParseLoginUsersFile(vdfPath)
	if err != nil {
		t.Fatal(err)
	}
	test_account_twoAid := Steam64ToAccountID(76561198947920049)
	if u, ok := users[test_account_twoAid]; !ok || !u.MostRecent {
		t.Errorf("expected test_account_two to be MostRecent after switch")
	}
}

func TestSwitchSteamAccount_SteamNotRunning(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	_ = os.MkdirAll(configDir, 0755)
	vdfPath := filepath.Join(configDir, "loginusers.vdf")
	_ = os.WriteFile(vdfPath, []byte(sampleSwitchUsersVDF), 0644)
	_ = os.WriteFile(filepath.Join(tempDir, "steam.exe"), []byte("fake-steam"), 0755)

	origChecker := runningChecker
	origShutdowner := steamShutdowner
	origRegistry := registrySetter
	origStarter := processStarter
	defer func() {
		runningChecker = origChecker
		steamShutdowner = origShutdowner
		registrySetter = origRegistry
		processStarter = origStarter
	}()

	runningChecker = func() (bool, error) {
		return false, nil // Not running
	}

	shutdownCalled := false
	steamShutdowner = func(steamPath string) error {
		shutdownCalled = true
		return nil
	}

	registrySetter = func(accountName string) error {
		return nil
	}
	processStarter = func(path string, args ...string) error {
		return nil
	}

	err := SwitchSteamAccount(tempDir, "test_account_one")
	if err != nil {
		t.Fatalf("SwitchSteamAccount failed: %v", err)
	}

	if shutdownCalled {
		t.Errorf("expected steamShutdowner NOT to be called when Steam is not running")
	}
}

func TestSwitchSteamAccount_RememberPasswordDisabled(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	_ = os.MkdirAll(configDir, 0755)
	vdfPath := filepath.Join(configDir, "loginusers.vdf")
	vdfContent := `"users"
{
	"76561198083722517"
	{
		"AccountName"		"test_account_one"
		"PersonaName"		"TestPersonaOne"
		"RememberPassword"	"0"
		"MostRecent"		"1"
	}
	"76561198947920049"
	{
		"AccountName"		"test_account_two"
		"PersonaName"		"TestPersonaTwo"
		"RememberPassword"	"1"
		"MostRecent"		"0"
	}
}
`
	if err := os.WriteFile(vdfPath, []byte(vdfContent), 0644); err != nil {
		t.Fatal(err)
	}

	shutdownCalled := false
	origShutdowner := steamShutdowner
	defer func() { steamShutdowner = origShutdowner }()
	steamShutdowner = func(steamPath string) error {
		shutdownCalled = true
		return nil
	}

	err := SwitchSteamAccount(tempDir, "test_account_one")
	if err == nil {
		t.Fatal("expected error when RememberPassword is 0, got nil")
	}
	if !errors.Is(err, ErrRememberPasswordDisabled) {
		t.Fatalf("expected ErrRememberPasswordDisabled, got: %v", err)
	}
	if shutdownCalled {
		t.Errorf("expected steamShutdowner NOT to be called when RememberPassword is disabled")
	}
}

func TestSwitchSteamAccount_ValidationErrors(t *testing.T) {
	if err := SwitchSteamAccount("dummy", ""); err == nil {
		t.Error("expected error for empty accountName, got nil")
	}

	origChecker := runningChecker
	origShutdowner := steamShutdowner
	origRegistry := registrySetter
	defer func() {
		runningChecker = origChecker
		steamShutdowner = origShutdowner
		registrySetter = origRegistry
	}()

	runningChecker = func() (bool, error) { return true, nil }
	steamShutdowner = func(steamPath string) error {
		return errors.New("shutdown failed")
	}

	if err := SwitchSteamAccount("dummy", "acc"); err == nil {
		t.Error("expected shutdown error propagation, got nil")
	}

	steamShutdowner = func(steamPath string) error { return nil }
	registrySetter = func(accountName string) error {
		return errors.New("registry access denied")
	}

	if err := SwitchSteamAccount("dummy", "acc"); err == nil {
		t.Error("expected registry error propagation, got nil")
	}
}
