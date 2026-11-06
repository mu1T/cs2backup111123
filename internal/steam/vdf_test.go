package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

const sampleLoginUsersVDF = `"users"
{
	"76561198083722517"
	{
		"AccountName"		"test_account_one"
		"PersonaName"		"TestPersonaOne"
		"RememberPassword"	"1"
		"MostRecent"		"1"
		"timestamp"			"1793753897"
	}
	"76561198947920049"
	{
		"AccountName"		"test_account_two"
		"PersonaName"		"TestPersonaTwo"
		"RememberPassword"	"1"
		"MostRecent"		"0"
		"timestamp"			"1793709635"
	}
}
`

func TestParseLoginUsers(t *testing.T) {
	users, err := ParseLoginUsers(strings.NewReader(sampleLoginUsersVDF))
	if err != nil {
		t.Fatalf("unexpected error parsing VDF: %v", err)
	}

	if len(users) != 2 {
		t.Fatalf("expected 2 users, got %d", len(users))
	}

	user1, ok := users[123456789]
	if !ok {
		t.Fatalf("expected account 123456789 in result")
	}
	if user1.PersonaName != "TestPersonaOne" {
		t.Errorf("expected PersonaName 'TestPersonaOne', got %q", user1.PersonaName)
	}
	if user1.AccountName != "test_account_one" {
		t.Errorf("expected AccountName 'test_account_one', got %q", user1.AccountName)
	}
	if !user1.RememberPassword {
		t.Errorf("expected user1 RememberPassword to be true")
	}
	if !user1.MostRecent {
		t.Errorf("expected MostRecent to be true")
	}
	if user1.Timestamp != 1793753897 {
		t.Errorf("expected timestamp 1793753897, got %d", user1.Timestamp)
	}

	user2, ok := users[Steam64ToAccountID(76561198947920049)]
	if !ok {
		t.Fatalf("expected second user in result")
	}
	if user2.MostRecent {
		t.Errorf("expected second user MostRecent to be false")
	}
}

func TestParseLoginUsersFile_WithTempDir(t *testing.T) {
	tempDir := testutil.TempDir(t)
	filePath := filepath.Join(tempDir, "loginusers.vdf")

	if err := os.WriteFile(filePath, []byte(sampleLoginUsersVDF), 0644); err != nil {
		t.Fatalf("failed to write fixture file: %v", err)
	}

	users, err := ParseLoginUsersFile(filePath)
	if err != nil {
		t.Fatalf("ParseLoginUsersFile failed: %v", err)
	}
	if len(users) != 2 {
		t.Errorf("expected 2 users, got %d", len(users))
	}
}

func TestParseLoginUsers_FallbackToLatestTimestamp(t *testing.T) {
	// Neither user has MostRecent explicitly set
	vdfData := `"users"
{
	"76561198083722517"
	{
		"AccountName"	"older"
		"PersonaName"	"older_user"
		"timestamp"		"1000"
	}
	"76561198947920049"
	{
		"AccountName"	"newer"
		"PersonaName"	"newer_user"
		"timestamp"		"2000"
	}
}
`
	users, err := ParseLoginUsers(strings.NewReader(vdfData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	newerAccount := Steam64ToAccountID(76561198947920049)
	if !users[newerAccount].MostRecent {
		t.Errorf("expected user with newer timestamp (2000) to be flagged as MostRecent")
	}
	if users[123456789].MostRecent {
		t.Errorf("expected older user MostRecent to be false")
	}
}

func TestParseLoginUsers_EscapedStringsAndComments(t *testing.T) {
	vdfData := `// Top level comment
"users"
{
	// User comment
	"76561198083722517"
	{
		"AccountName"	"name\"with\"quotes"
		"PersonaName"	"special\\slash"
		"MostRecent"	"1"
		"timestamp"		"123456"
	}
}
`
	users, err := ParseLoginUsers(strings.NewReader(vdfData))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u := users[123456789]
	if u == nil {
		t.Fatal("user not found")
	}
	if u.AccountName != `name"with"quotes` {
		t.Errorf("expected escaped quotes, got %q", u.AccountName)
	}
	if u.PersonaName != `special\slash` {
		t.Errorf("expected escaped backslash, got %q", u.PersonaName)
	}
}

func TestParseLibraryFolders_ModernFormat(t *testing.T) {
	vdfContent := `"libraryfolders"
{
	"0"
	{
		"path"		"C:\\Steam"
		"label"		""
		"contentid"		"123"
		"apps"
		{
			"228980"		"182740920"
			"730"		"35948384210"
		}
	}
	"1"
	{
		"path"		"D:\\SteamLibrary"
		"apps"
		{
			"570"		"99999"
		}
	}
}`

	folders, err := ParseLibraryFolders(strings.NewReader(vdfContent))
	if err != nil {
		t.Fatalf("ParseLibraryFolders failed: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 folders, got %d", len(folders))
	}

	if folders[0].Path != filepath.Clean("C:\\Steam") {
		t.Errorf("expected C:\\Steam, got %q", folders[0].Path)
	}
	if _, ok := folders[0].Apps["730"]; !ok {
		t.Errorf("expected app 730 in folder 0")
	}

	if folders[1].Path != filepath.Clean("D:\\SteamLibrary") {
		t.Errorf("expected D:\\SteamLibrary, got %q", folders[1].Path)
	}
	if _, ok := folders[1].Apps["570"]; !ok {
		t.Errorf("expected app 570 in folder 1")
	}
}

func TestParseLibraryFolders_LegacyFormat(t *testing.T) {
	vdfContent := `"LibraryFolders"
{
	"TimeNextStatsReport"		"123456"
	"ContentStatsID"		"123456"
	"1"		"D:\\Games\\SteamLibrary"
	"2"		"E:\\SteamLibrary"
}`

	folders, err := ParseLibraryFolders(strings.NewReader(vdfContent))
	if err != nil {
		t.Fatalf("ParseLibraryFolders failed: %v", err)
	}

	if len(folders) != 2 {
		t.Fatalf("expected 2 legacy folders, got %d", len(folders))
	}
	if folders[0].Path != filepath.Clean("D:\\Games\\SteamLibrary") {
		t.Errorf("expected D:\\Games\\SteamLibrary, got %q", folders[0].Path)
	}
}
