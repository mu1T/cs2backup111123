package internal_test

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"cs2backup/internal/app"
	"cs2backup/internal/backup"
	"cs2backup/internal/steam"
	"cs2backup/internal/video"

	"cs2backup/internal/testutil"
)

const sampleBenchmarkVDF = `"users"
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
	"76561198000000003"
	{
		"AccountName"		"user_three"
		"PersonaName"		"SniperWolf"
		"RememberPassword"	"1"
		"MostRecent"		"0"
		"timestamp"			"1793701000"
	}
	"76561198000000004"
	{
		"AccountName"		"user_four"
		"PersonaName"		"AceClutcher"
		"RememberPassword"	"1"
		"MostRecent"		"0"
		"timestamp"			"1793690000"
	}
}
`

const sampleBenchmarkVideoCfg = `"video.cfg"
{
	"setting.csm_quality_level"		"3"
	"setting.mat_software_aa_strength"		"1"
	"setting.mat_motion_blur_enabled"		"0"
	"setting.fullscreen"		"1"
	"setting.defaultres"		"1920"
	"setting.defaultresheight"		"1080"
	"setting.aspectratiomode"		"1"
	"setting.nowindowborder"		"0"
	"setting.mat_vsync"		"0"
	"setting.mat_triplebuffered"		"0"
	"setting.mat_monitorgamma"		"2.200000"
	"setting.mat_queue_mode"		"-1"
	"setting.mat_sharpen_amount"		"1.000000"
	"setting.shadow_quality"		"1"
	"setting.model_detail"		"0"
	"setting.texture_detail"		"1"
	"setting.effect_detail"		"0"
	"setting.shader_detail"		"0"
	"setting.particle_detail"		"0"
	"setting.ambient_occlusion"		"0"
	"setting.high_dynamic_range"		"0"
	"setting.fidelityfx_super_resolution"		"0"
	"setting.nvidia_reflex"		"1"
}
`

func generateTestConfig(lines int, variant int) string {
	var sb strings.Builder
	for i := 1; i <= lines; i++ {
		switch {
		case variant == 1 && i%7 == 0:
			// Modified line
			sb.WriteString(fmt.Sprintf("bind \"key_%d\" \"action_modified_%d\"\n", i, i))
		case variant == 1 && i%11 == 0:
			// Added in variant 1
			sb.WriteString(fmt.Sprintf("setinfo \"extra_var_%d\" \"val_%d\"\n", i, i))
		default:
			sb.WriteString(fmt.Sprintf("bind \"key_%d\" \"action_default_%d\"\n", i, i))
		}
	}
	return sb.String()
}

// BenchmarkVDFTokenizer tests zero-allocation streaming tokenization of VDF files.
func BenchmarkVDFTokenizer(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(sampleBenchmarkVDF)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := strings.NewReader(sampleBenchmarkVDF)
		count, err := steam.ScanVDFTokens(r)
		if err != nil {
			b.Fatalf("ScanVDFTokens error: %v", err)
		}
		if count == 0 {
			b.Fatalf("expected non-zero token count")
		}
	}
}

// BenchmarkVDFKeyValuesParsing tests parsing speed and allocations of VDF KeyValues.
func BenchmarkVDFKeyValuesParsing(b *testing.B) {
	b.ReportAllocs()
	b.SetBytes(int64(len(sampleBenchmarkVDF)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		r := strings.NewReader(sampleBenchmarkVDF)
		users, err := steam.ParseLoginUsers(r)
		if err != nil {
			b.Fatalf("ParseLoginUsers error: %v", err)
		}
		if len(users) != 4 {
			b.Fatalf("expected 4 users, got %d", len(users))
		}
	}
}

// BenchmarkBufferPoolFileCopy tests streaming file copy reusing 64KB sync.Pool buffers.
func BenchmarkBufferPoolFileCopy(b *testing.B) {
	tempDir := b.TempDir()
	srcPath := filepath.Join(tempDir, "src_bench.bin")
	dstPath := filepath.Join(tempDir, "dst_bench.bin")

	// Create 256KB test file
	data := make([]byte, 256*1024)
	for i := range data {
		data[i] = byte(i % 256)
	}
	if err := os.WriteFile(srcPath, data, 0644); err != nil {
		b.Fatalf("failed to create src file: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		if err := backup.CopyFile(srcPath, dstPath); err != nil {
			b.Fatalf("CopyFile failed: %v", err)
		}
	}
}

// BenchmarkDiffEngineLineComparison tests line-by-line diff computation on CS2 configs.
func BenchmarkDiffEngineLineComparison(b *testing.B) {
	cfgA := generateTestConfig(300, 0)
	cfgB := generateTestConfig(300, 1)

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		diff, err := backup.GetTextDiff(cfgA, cfgB)
		if err != nil || len(diff.Lines) == 0 {
			b.Fatalf("expected diff lines, err: %v", err)
		}
	}
}

// BenchmarkSnapshotSHA256Calculation tests streaming SHA-256 hash computation.
func BenchmarkSnapshotSHA256Calculation(b *testing.B) {
	tempDir := b.TempDir()
	filePath := filepath.Join(tempDir, "test_sha.dat")
	data := make([]byte, 128*1024) // 128KB
	for i := range data {
		data[i] = byte(i % 251)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		b.Fatalf("failed to write test file: %v", err)
	}

	b.ReportAllocs()
	b.SetBytes(int64(len(data)))
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		hash, sz, err := backup.ComputeFileSHA256(filePath)
		if err != nil {
			b.Fatalf("ComputeFileSHA256 failed: %v", err)
		}
		if hash == "" || sz != int64(len(data)) {
			b.Fatalf("invalid hash calculation")
		}
	}
}

// BenchmarkLaunchOptionsParsingAndEscaping tests launch option sanitization, escaping, and parsing.
func BenchmarkLaunchOptionsParsingAndEscaping(b *testing.B) {
	tempDir := b.TempDir()
	steamDir := filepath.Join(tempDir, "Steam")
	accID := "12345"
	userCfgDir := filepath.Join(steamDir, "userdata", accID, "config")
	_ = os.MkdirAll(userCfgDir, 0755)
	vdfPath := filepath.Join(userCfgDir, "localconfig.vdf")
	initialVDF := `"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"apps"
				{
					"730"
					{
						"LaunchOptions"		"-novid -freq 240 +fps_max 0"
					}
				}
			}
		}
	}
}
`
	if err := os.WriteFile(vdfPath, []byte(initialVDF), 0644); err != nil {
		b.Fatalf("failed to create localconfig.vdf: %v", err)
	}

	mgr := steam.NewLaunchOptionsManager(steamDir)
	rawOptions := "  -novid   -tickrate 128   +exec \"autoexec.cfg\"   "
	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		sanitized := steam.SanitizeLaunchOptions(rawOptions)
		escaped := steam.EscapeVDFString(sanitized)
		_ = steam.UnescapeVDFString(escaped)

		if err := mgr.SetLaunchOptions(accID, "-novid -tickrate 128"); err != nil {
			b.Fatalf("SetLaunchOptions failed: %v", err)
		}
		opts, err := mgr.GetLaunchOptions(accID)
		if err != nil || opts == "" {
			b.Fatalf("GetLaunchOptions failed: %v", err)
		}
	}
}

// BenchmarkVideoSettingsParsingAndSerialization tests loading and saving cs2_video.txt.
func BenchmarkVideoSettingsParsingAndSerialization(b *testing.B) {
	tempDir := b.TempDir()
	videoPath := filepath.Join(tempDir, "cs2_video.txt")
	if err := os.WriteFile(videoPath, []byte(sampleBenchmarkVideoCfg), 0644); err != nil {
		b.Fatalf("failed to write test video config: %v", err)
	}

	updated := map[string]string{
		"setting.defaultres":       "2560",
		"setting.defaultresheight": "1440",
		"setting.shadow_quality":   "2",
		"setting.nvidia_reflex":    "2",
	}

	b.ReportAllocs()
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		cf, err := video.LoadConfigFile(videoPath)
		if err != nil {
			b.Fatalf("LoadConfigFile failed: %v", err)
		}
		if len(cf.Settings) == 0 {
			b.Fatalf("empty settings parsed")
		}

		if err := video.SaveConfigFile(videoPath, updated, false); err != nil {
			b.Fatalf("SaveConfigFile failed: %v", err)
		}
	}
}

// TestMemoryProfile_MultiAccountAndSnapshotLoad verifies that heap allocation remains <= 30MB
// under full multi-account backup and snapshot workloads per ADR 0002.
func TestMemoryProfile_MultiAccountAndSnapshotLoad(t *testing.T) {
	tempDir := testutil.TempDir(t)
	steamPath := filepath.Join(tempDir, "Steam")
	backupPath := filepath.Join(tempDir, "Backup")
	configPath := filepath.Join(tempDir, "config")

	if err := os.MkdirAll(filepath.Join(steamPath, "config"), 0755); err != nil {
		t.Fatalf("failed to make steam config dir: %v", err)
	}
	if err := os.MkdirAll(backupPath, 0755); err != nil {
		t.Fatalf("failed to make backup dir: %v", err)
	}
	if err := os.MkdirAll(configPath, 0755); err != nil {
		t.Fatalf("failed to make app config dir: %v", err)
	}

	// 1. Setup 8 simulated Steam accounts
	numAccounts := 8
	var loginUsersSB strings.Builder
	loginUsersSB.WriteString("\"users\"\n{\n")

	accountIDs := make([]string, numAccounts)
	for i := 0; i < numAccounts; i++ {
		accID := fmt.Sprintf("%d", 100000+i)
		accountIDs[i] = accID
		s64 := steam.AccountIDToSteam64(uint32(100000 + i))
		mostRecent := "0"
		if i == 0 {
			mostRecent = "1"
		}
		loginUsersSB.WriteString(fmt.Sprintf(`	"%d"
	{
		"AccountName"		"player_%d"
		"PersonaName"		"Persona_%d"
		"RememberPassword"	"1"
		"MostRecent"		"%s"
		"timestamp"			"%d"
	}
`, s64, i, i, mostRecent, 1700000000+i*100))

		// Create user CS2 folder structure
		userCS2Dir := filepath.Join(steamPath, "userdata", accID, "730", "local", "cfg")
		if err := os.MkdirAll(userCS2Dir, 0755); err != nil {
			t.Fatalf("failed to create userdata dir for %s: %v", accID, err)
		}
		// Write video config
		if err := os.WriteFile(filepath.Join(userCS2Dir, "cs2_video.txt"), []byte(sampleBenchmarkVideoCfg), 0644); err != nil {
			t.Fatalf("failed to write video cfg: %v", err)
		}
		// Write convars and keys
		if err := os.WriteFile(filepath.Join(userCS2Dir, "cs2_user_keys.vcfg"), []byte(generateTestConfig(100, 0)), 0644); err != nil {
			t.Fatalf("failed to write keys: %v", err)
		}
		if err := os.WriteFile(filepath.Join(userCS2Dir, "cs2_user_convars.vcfg"), []byte(generateTestConfig(100, 0)), 0644); err != nil {
			t.Fatalf("failed to write convars: %v", err)
		}
		// Write localconfig.vdf
		userCfgDir := filepath.Join(steamPath, "userdata", accID, "config")
		if err := os.MkdirAll(userCfgDir, 0755); err != nil {
			t.Fatalf("failed to create user cfg dir: %v", err)
		}
		localVDF := fmt.Sprintf(`"UserLocalConfigStore"
{
	"Software"
	{
		"Valve"
		{
			"Steam"
			{
				"apps"
				{
					"730"
					{
						"LaunchOptions"		"-novid -tickrate 128 +rate %d"
					}
				}
			}
		}
	}
}
`, 786432+i*1000)
		if err := os.WriteFile(filepath.Join(userCfgDir, "localconfig.vdf"), []byte(localVDF), 0644); err != nil {
			t.Fatalf("failed to write localconfig.vdf: %v", err)
		}
	}
	loginUsersSB.WriteString("}\n")
	if err := os.WriteFile(filepath.Join(steamPath, "config", "loginusers.vdf"), []byte(loginUsersSB.String()), 0644); err != nil {
		t.Fatalf("failed to write loginusers.vdf: %v", err)
	}

	// 2. Initialize AppService
	appSvc := app.NewAppServiceWithConfigDir(configPath)
	if err := appSvc.SetSteamPath(steamPath); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if err := appSvc.SetBackupPath(backupPath); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}

	// 3. Execute heavy multi-account workload
	users, err := appSvc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) != numAccounts {
		t.Fatalf("expected %d accounts, got %d", numAccounts, len(users))
	}

	// Preset for video tests
	presetFile := filepath.Join(tempDir, "preset.txt")
	if err := os.WriteFile(presetFile, []byte(sampleBenchmarkVideoCfg), 0644); err != nil {
		t.Fatalf("failed to write preset file: %v", err)
	}
	_, err = appSvc.ImportVideoPreset(presetFile, "TournamentPreset", true)
	if err != nil {
		t.Fatalf("ImportVideoPreset failed: %v", err)
	}

	// Loop over all accounts: backup, multiple snapshots, diff, preset apply
	for _, accID := range accountIDs {
		if err := appSvc.BackupUser(accID); err != nil {
			t.Fatalf("BackupUser %s failed: %v", accID, err)
		}

		// Create 3 snapshots per account with modifications to test deduplication
		for sIdx := 1; sIdx <= 3; sIdx++ {
			snap, err := appSvc.CreateSnapshot(accID, fmt.Sprintf("Snapshot %d for account %s", sIdx, accID))
			if err != nil {
				t.Fatalf("CreateSnapshot %d for %s failed: %v", sIdx, accID, err)
			}

			// Modify live file to trigger diff
			userCfg := filepath.Join(steamPath, "userdata", accID, "730", "local", "cfg", "cs2_user_keys.vcfg")
			_ = os.WriteFile(userCfg, []byte(generateTestConfig(100+sIdx, 1)), 0644)

			diffItems, err := appSvc.CompareSnapshot(accID, snap.ID)
			if err != nil {
				t.Fatalf("CompareSnapshot failed: %v", err)
			}
			if len(diffItems) == 0 {
				t.Fatalf("expected modified diff items")
			}
		}

		// Apply video preset with read-only lock
		if err := appSvc.ApplyDefaultVideoPreset(accID, true); err != nil {
			t.Fatalf("ApplyDefaultVideoPreset %s failed: %v", accID, err)
		}

		// Update launch options
		if err := appSvc.SetLaunchOptions(accID, "-novid -tickrate 128 -refresh 240 +fps_max 0"); err != nil {
			t.Fatalf("SetLaunchOptions %s failed: %v", accID, err)
		}

		// Create LAN package
		_, err = appSvc.CreateLANPackage(accID, fmt.Sprintf("Pack_%s", accID), false, true)
		if err != nil {
			t.Fatalf("CreateLANPackage %s failed: %v", accID, err)
		}
	}

	// 4. Force GC and evaluate memory footprint
	runtime.GC()
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	heapAllocMB := float64(m.Alloc) / (1024 * 1024)
	heapInUseMB := float64(m.HeapInuse) / (1024 * 1024)
	totalAllocMB := float64(m.TotalAlloc) / (1024 * 1024)

	t.Logf("=== Memory Profile Under Multi-Account Load ===")
	t.Logf("Heap Alloc in use:   %.2f MB (threshold: <= 30.00 MB)", heapAllocMB)
	t.Logf("Heap in use (pages): %.2f MB", heapInUseMB)
	t.Logf("Total Allocations:   %.2f MB", totalAllocMB)
	t.Logf("System Memory:       %.2f MB", float64(m.Sys)/(1024*1024))
	t.Logf("GC Cycles:           %d", m.NumGC)

	const maxAllowedHeapAllocBytes = 30 * 1024 * 1024 // 30MB limit
	if m.Alloc > maxAllowedHeapAllocBytes {
		t.Fatalf("Heap allocation exceeded limit: got %.2f MB, allowed <= 30 MB", heapAllocMB)
	}
}
