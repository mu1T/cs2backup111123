package steam

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

const sampleLocalConfigVDF = `"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"Apps"
				{
					"570"
					{
						"LaunchOptions"		"-novid"
						"LastPlayed"		"1600000000"
					}
					"730"
					{
						"LastPlayed"		"1700000000"
						"LaunchOptions"		"-novid -tickrate 128 +fps_max 0"
						"BadgeData"		"1"
					}
				}
			}
		}
	}
}
`

const sampleWithoutLaunchOptionsVDF = `"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"Apps"
				{
					"730"
					{
						"LastPlayed"		"1700000000"
						"BadgeData"		"1"
					}
				}
			}
		}
	}
}
`

const sampleWithout730VDF = `"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"Apps"
				{
					"570"
					{
						"LaunchOptions"		"-novid"
					}
				}
			}
		}
	}
}
`

func TestParseLaunchOptions_Success(t *testing.T) {
	opts, err := ParseLaunchOptions(strings.NewReader(sampleLocalConfigVDF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := "-novid -tickrate 128 +fps_max 0"
	if opts != expected {
		t.Errorf("expected %q, got %q", expected, opts)
	}
}

func TestParseLaunchOptions_Missing(t *testing.T) {
	opts, err := ParseLaunchOptions(strings.NewReader(sampleWithoutLaunchOptionsVDF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts != "" {
		t.Errorf("expected empty launch options, got %q", opts)
	}
}

func TestParseLaunchOptions_Missing730(t *testing.T) {
	opts, err := ParseLaunchOptions(strings.NewReader(sampleWithout730VDF))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if opts != "" {
		t.Errorf("expected empty launch options, got %q", opts)
	}
}

func TestParseLaunchOptions_Unescaping(t *testing.T) {
	vdf := `"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"Apps"
				{
					"730"
					{
						"LaunchOptions"		"-novid +exec \"autoexec.cfg\" -path \"C:\\CS2\\cfg\""
					}
				}
			}
		}
	}
}
`
	opts, err := ParseLaunchOptions(strings.NewReader(vdf))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	expected := `-novid +exec "autoexec.cfg" -path "C:\CS2\cfg"`
	if opts != expected {
		t.Errorf("expected %q, got %q", expected, opts)
	}
}

func TestUpdateLaunchOptions_ReplaceExisting(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	accountID := "12345678"
	cfgDir := filepath.Join(tmpDir, "userdata", accountID, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(cfgDir, "localconfig.vdf")
	if err := os.WriteFile(filePath, []byte(sampleLocalConfigVDF), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewLaunchOptionsManager(tmpDir)

	// Read initial
	initial, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("failed to get launch options: %v", err)
	}
	if initial != "-novid -tickrate 128 +fps_max 0" {
		t.Errorf("unexpected initial: %s", initial)
	}

	// Update with special characters
	newOpts := `-allow_third_party_software +exec "auto.cfg" -dir "D:\CS2"`
	if err := mgr.SetLaunchOptions(accountID, newOpts); err != nil {
		t.Fatalf("failed to set launch options: %v", err)
	}

	// Read back
	updated, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("failed to get updated launch options: %v", err)
	}
	if updated != newOpts {
		t.Errorf("expected %q, got %q", newOpts, updated)
	}

	// Verify AppID 570 was NOT touched and still has -novid
	rawBytes, err := os.ReadFile(filePath)
	if err != nil {
		t.Fatal(err)
	}
	rawStr := string(rawBytes)
	if !strings.Contains(rawStr, `"570"`) || !strings.Contains(rawStr, `"BadgeData"`) {
		t.Errorf("other VDF contents were corrupted! Content:\n%s", rawStr)
	}
	if !strings.Contains(rawStr, `\"auto.cfg\"`) {
		t.Errorf("quotes were not properly escaped in raw file! Content:\n%s", rawStr)
	}
	if !strings.Contains(rawStr, `D:\\CS2`) {
		t.Errorf("backslashes were not properly escaped in raw file! Content:\n%s", rawStr)
	}
}

func TestUpdateLaunchOptions_InsertIntoExisting730(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	accountID := "87654321"
	cfgDir := filepath.Join(tmpDir, "userdata", accountID, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(cfgDir, "localconfig.vdf")
	if err := os.WriteFile(filePath, []byte(sampleWithoutLaunchOptionsVDF), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewLaunchOptionsManager(tmpDir)

	newOpts := "-freq 240 +fps_max 400"
	if err := mgr.SetLaunchOptions(accountID, newOpts); err != nil {
		t.Fatalf("failed to set launch options: %v", err)
	}

	updated, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("failed to get updated launch options: %v", err)
	}
	if updated != newOpts {
		t.Errorf("expected %q, got %q", newOpts, updated)
	}
}

func TestUpdateLaunchOptions_Insert730IntoApps(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	accountID := "11223344"
	cfgDir := filepath.Join(tmpDir, "userdata", accountID, "config")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}

	filePath := filepath.Join(cfgDir, "localconfig.vdf")
	if err := os.WriteFile(filePath, []byte(sampleWithout730VDF), 0644); err != nil {
		t.Fatal(err)
	}

	mgr := NewLaunchOptionsManager(tmpDir)

	newOpts := "-threads 8 -high"
	if err := mgr.SetLaunchOptions(accountID, newOpts); err != nil {
		t.Fatalf("failed to set launch options: %v", err)
	}

	updated, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("failed to get updated launch options: %v", err)
	}
	if updated != newOpts {
		t.Errorf("expected %q, got %q", newOpts, updated)
	}
}

func TestUpdateLaunchOptions_CreateFileSkeletonWhenMissing(t *testing.T) {
	tmpDir := testutil.TempDir(t)
	accountID := "99887766"

	mgr := NewLaunchOptionsManager(tmpDir)

	// Initial read should return empty
	initial, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("unexpected error for missing file: %v", err)
	}
	if initial != "" {
		t.Errorf("expected empty string for non-existent file, got %q", initial)
	}

	// Write launch options
	newOpts := "-novid -freq 144"
	if err := mgr.SetLaunchOptions(accountID, newOpts); err != nil {
		t.Fatalf("failed to set launch options: %v", err)
	}

	// Verify read
	updated, err := mgr.GetLaunchOptions(accountID)
	if err != nil {
		t.Fatalf("failed to read created launch options: %v", err)
	}
	if updated != newOpts {
		t.Errorf("expected %q, got %q", newOpts, updated)
	}
}

func TestSanitizeLaunchOptions(t *testing.T) {
	input := "  -novid\r\n-freq 240\t+exec \"test\"\n  "
	sanitized := SanitizeLaunchOptions(input)
	expected := `-novid -freq 240 +exec "test"`
	if sanitized != expected {
		t.Errorf("expected %q, got %q", expected, sanitized)
	}
}

func TestEscapeAndUnescapeVDFString(t *testing.T) {
	original := `-novid +exec "cfg\auto.cfg" -path "D:\Game\\CS2"`
	escaped := EscapeVDFString(original)
	expectedEscaped := `-novid +exec \"cfg\\auto.cfg\" -path \"D:\\Game\\\\CS2\"`
	if escaped != expectedEscaped {
		t.Errorf("expected escaped %q, got %q", expectedEscaped, escaped)
	}

	unescaped := UnescapeVDFString(escaped)
	if unescaped != original {
		t.Errorf("expected unescaped %q, got %q", original, unescaped)
	}
}

func BenchmarkParseLaunchOptions(b *testing.B) {
	data := []byte(sampleLocalConfigVDF)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		r := strings.NewReader(sampleLocalConfigVDF)
		_, _ = ParseLaunchOptions(r)
	}
	_ = data
}

func BenchmarkTokenizeWithOffsets(b *testing.B) {
	data := []byte(sampleLocalConfigVDF)
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = tokenizeWithOffsets(data)
	}
}

func BenchmarkSanitizeLaunchOptions(b *testing.B) {
	s := `  -novid -tickrate 128   +exec "auto.cfg" -freq 240  `
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = SanitizeLaunchOptions(s)
	}
}

func BenchmarkEscapeVDFString(b *testing.B) {
	s := `-novid +exec "cfg\auto.cfg" -path "D:\Game\\CS2"`
	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = EscapeVDFString(s)
	}
}

