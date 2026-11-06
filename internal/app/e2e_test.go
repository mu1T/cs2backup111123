package app_test

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/app"
	"cs2backup/internal/backup"
	"cs2backup/internal/steam"
	"cs2backup/internal/video"

	"cs2backup/internal/testutil"
)

const e2eLoginUsersVDF = `"users"
{
	"76561198083722517"
	{
		"AccountName"		"progamer"
		"PersonaName"		"ProGamer_CS"
		"RememberPassword"	"1"
		"MostRecent"		"1"
		"timestamp"			"1793800000"
	}
	"76561198737266172"
	{
		"AccountName"		"casual"
		"PersonaName"		"CasualPlayer"
		"RememberPassword"	"1"
		"MostRecent"		"0"
		"timestamp"			"1793700000"
	}
}
`

const e2eInitialVideoCfg = `"video.cfg"
{
	"setting.csm_quality_level"		"2"
	"setting.mat_software_aa_strength"		"1"
	"setting.mat_motion_blur_enabled"		"0"
	"setting.fullscreen"		"1"
	"setting.defaultres"		"1920"
	"setting.defaultresheight"		"1080"
	"setting.aspectratiomode"		"1"
	"setting.nowindowborder"		"0"
	"setting.mat_vsync"		"0"
	"setting.shadow_quality"		"1"
	"setting.nvidia_reflex"		"1"
}
`

const e2eTourneyPresetCfg = `"video.cfg"
{
	"setting.csm_quality_level"		"3"
	"setting.mat_software_aa_strength"		"0"
	"setting.mat_motion_blur_enabled"		"0"
	"setting.fullscreen"		"1"
	"setting.defaultres"		"1280"
	"setting.defaultresheight"		"960"
	"setting.aspectratiomode"		"0"
	"setting.nowindowborder"		"0"
	"setting.mat_vsync"		"0"
	"setting.shadow_quality"		"0"
	"setting.nvidia_reflex"		"2"
}
`

// TestEndToEnd_CompleteLifecycle executes a thorough, unified regression across all subsystems
// validating the primary seam AppService per Ticket 09.
func TestEndToEnd_CompleteLifecycle(t *testing.T) {
	tempDir := testutil.TempDir(t)
	steamPath := filepath.Join(tempDir, "Steam")
	backupPath := filepath.Join(tempDir, "Backup")
	configPath := filepath.Join(tempDir, "config")
	presetFilePath := filepath.Join(tempDir, "major_pro_preset.txt")

	accA := "123456789" // 76561198083722517
	accB := "777000444" // 76561198737266172

	// Setup Steam directory structure
	steamConfigDir := filepath.Join(steamPath, "config")
	if err := os.MkdirAll(steamConfigDir, 0755); err != nil {
		t.Fatalf("failed to create steam config dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(steamConfigDir, "loginusers.vdf"), []byte(e2eLoginUsersVDF), 0644); err != nil {
		t.Fatalf("failed to write loginusers.vdf: %v", err)
	}

	// Setup Account A files
	accACfgDir := filepath.Join(steamPath, "userdata", accA, "730", "local", "cfg")
	accAUserConfigDir := filepath.Join(steamPath, "userdata", accA, "config")
	if err := os.MkdirAll(accACfgDir, 0755); err != nil {
		t.Fatalf("failed to create accA cfg dir: %v", err)
	}
	if err := os.MkdirAll(accAUserConfigDir, 0755); err != nil {
		t.Fatalf("failed to create accA user config dir: %v", err)
	}

	if err := os.WriteFile(filepath.Join(accACfgDir, "cs2_video.txt"), []byte(e2eInitialVideoCfg), 0644); err != nil {
		t.Fatalf("failed to write accA video: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accACfgDir, "cs2_user_keys.vcfg"), []byte("bind \"MOUSE1\" \"+attack\"\nbind \"SPACE\" \"+jump\"\n"), 0644); err != nil {
		t.Fatalf("failed to write accA keys: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accACfgDir, "cs2_user_convars.vcfg"), []byte("sensitivity \"1.25\"\nzoom_sensitivity_ratio \"1.0\"\n"), 0644); err != nil {
		t.Fatalf("failed to write accA convars: %v", err)
	}
	initialLocalVDF := `"UserLocalConfigStore"
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
						"LaunchOptions"		"-novid -freq 240"
					}
				}
			}
		}
	}
}
`
	if err := os.WriteFile(filepath.Join(accAUserConfigDir, "localconfig.vdf"), []byte(initialLocalVDF), 0644); err != nil {
		t.Fatalf("failed to write accA localconfig.vdf: %v", err)
	}

	// Setup Account B files (target for cross-account restore)
	accBCfgDir := filepath.Join(steamPath, "userdata", accB, "730", "local", "cfg")
	if err := os.MkdirAll(accBCfgDir, 0755); err != nil {
		t.Fatalf("failed to create accB cfg dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accBCfgDir, "cs2_video.txt"), []byte(e2eInitialVideoCfg), 0644); err != nil {
		t.Fatalf("failed to write accB video: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accBCfgDir, "cs2_user_keys.vcfg"), []byte("bind \"MOUSE1\" \"+attack2\"\n"), 0644); err != nil {
		t.Fatalf("failed to write accB keys: %v", err)
	}
	if err := os.WriteFile(filepath.Join(accBCfgDir, "cs2_user_convars.vcfg"), []byte("sensitivity \"4.00\"\n"), 0644); err != nil {
		t.Fatalf("failed to write accB convars: %v", err)
	}

	// Write tournament preset template
	if err := os.WriteFile(presetFilePath, []byte(e2eTourneyPresetCfg), 0644); err != nil {
		t.Fatalf("failed to write preset template: %v", err)
	}

	// Initialize AppService primary seam
	appSvc := app.NewAppServiceWithConfigDir(configPath)
	if err := appSvc.SetSteamPath(steamPath); err != nil {
		t.Fatalf("SetSteamPath failed: %v", err)
	}
	if err := appSvc.SetBackupPath(backupPath); err != nil {
		t.Fatalf("SetBackupPath failed: %v", err)
	}

	t.Log(">>> Phase 1: Steam detection & active user resolution (MostRecent=1)")
	users, err := appSvc.GetUsers()
	if err != nil {
		t.Fatalf("GetUsers failed: %v", err)
	}
	if len(users) < 2 {
		t.Fatalf("expected at least 2 users, got %d", len(users))
	}
	var cardA, cardB *steam.UserCardInfo
	for i := range users {
		if users[i].AccountID == accA {
			cardA = &users[i]
		}
		if users[i].AccountID == accB {
			cardB = &users[i]
		}
	}
	if cardA == nil || cardB == nil {
		t.Fatalf("failed to find cards for account A or B")
	}
	if !cardA.IsRecent {
		t.Errorf("expected Account A (MostRecent=1) to have IsRecent=true, got false")
	}
	if cardB.IsRecent {
		t.Errorf("expected Account B (MostRecent=0) to have IsRecent=false, got true")
	}
	if cardA.Username != "ProGamer_CS" {
		t.Errorf("expected persona 'ProGamer_CS', got '%s'", cardA.Username)
	}

	// User note persistence
	if err := appSvc.SaveUserNote(accA, "Tournament Main ID"); err != nil {
		t.Fatalf("SaveUserNote failed: %v", err)
	}
	usersUpdated, _ := appSvc.GetUsers()
	for _, u := range usersUpdated {
		if u.AccountID == accA && u.Notes != "Tournament Main ID" {
			t.Errorf("expected updated note 'Tournament Main ID', got '%s'", u.Notes)
		}
	}

	t.Log(">>> Phase 2: Userdata backup -> Snapshot creation -> SHA-256 deduplication")
	if err := appSvc.BackupUser(accA); err != nil {
		t.Fatalf("BackupUser failed: %v", err)
	}
	backedAccounts, err := appSvc.GetBackedUpAccounts()
	if err != nil {
		t.Fatalf("GetBackedUpAccounts failed: %v", err)
	}
	foundBacked := false
	for _, id := range backedAccounts {
		if id == accA {
			foundBacked = true
			break
		}
	}
	if !foundBacked {
		t.Fatalf("expected account %s in backed up accounts list", accA)
	}

	// Create Snapshot 1
	snap1, err := appSvc.CreateSnapshot(accA, "Initial Golden Baseline")
	if err != nil {
		t.Fatalf("CreateSnapshot 1 failed: %v", err)
	}
	if snap1.ID == "" || snap1.TotalFiles == 0 {
		t.Fatalf("invalid snapshot 1: %+v", snap1)
	}

	// Create Snapshot 2 without file changes -> SHA-256 deduplication check
	snap2, err := appSvc.CreateSnapshot(accA, "Identical Snapshot For Dedup")
	if err != nil {
		t.Fatalf("CreateSnapshot 2 failed: %v", err)
	}
	if len(snap1.FileHashes) != len(snap2.FileHashes) {
		t.Errorf("expected identical file count between snapshots: %d vs %d", len(snap1.FileHashes), len(snap2.FileHashes))
	}
	for relPath, hash1 := range snap1.FileHashes {
		hash2, ok := snap2.FileHashes[relPath]
		if !ok || hash1 != hash2 {
			t.Errorf("deduplication mismatch on file %s: %s != %s", relPath, hash1, hash2)
		}
	}

	t.Log(">>> Phase 3: Diff calculation between live config and snapshot")
	// Modify live convars file
	accAConvarsPath := filepath.Join(accACfgDir, "cs2_user_convars.vcfg")
	modifiedConvars := "sensitivity \"2.50\"\nzoom_sensitivity_ratio \"1.0\"\nvolume \"0.75\"\n"
	if err := os.WriteFile(accAConvarsPath, []byte(modifiedConvars), 0644); err != nil {
		t.Fatalf("failed to modify live convars: %v", err)
	}

	diffItems, err := appSvc.CompareSnapshot(accA, snap1.ID)
	if err != nil {
		t.Fatalf("CompareSnapshot failed: %v", err)
	}
	var convarsDiff *backup.FileDiffItem
	for i := range diffItems {
		if strings.HasSuffix(filepath.ToSlash(diffItems[i].RelPath), "cs2_user_convars.vcfg") {
			convarsDiff = &diffItems[i]
			break
		}
	}
	if convarsDiff == nil {
		t.Fatalf("expected diff item for cs2_user_convars.vcfg, got %+v", diffItems)
	}
	if convarsDiff.Status != "modified" {
		t.Errorf("expected status 'modified', got '%s'", convarsDiff.Status)
	}
	if convarsDiff.Diff == nil || (convarsDiff.Diff.ModCount == 0 && convarsDiff.Diff.AddCount == 0) {
		t.Errorf("expected modified/added lines in diff, got %+v", convarsDiff.Diff)
	}

	// Standalone GetTextDiff verification
	textDiff, err := appSvc.GetTextDiff("bind \"MOUSE1\" \"+attack\"", "bind \"MOUSE1\" \"+attack2\"\nbind \"F1\" \"autobuy\"")
	if err != nil {
		t.Fatalf("GetTextDiff failed: %v", err)
	}
	if len(textDiff.Lines) == 0 {
		t.Errorf("expected non-empty diff lines")
	}

	t.Log(">>> Phase 4: Steam running detection & LaunchOptions update")
	isRunning, err := appSvc.IsSteamRunning()
	if err != nil {
		t.Fatalf("IsSteamRunning failed: %v", err)
	}
	t.Logf("IsSteamRunning returned: %v", isRunning)

	initialOpts, err := appSvc.GetLaunchOptions(accA)
	if err != nil {
		t.Fatalf("GetLaunchOptions failed: %v", err)
	}
	if initialOpts != "-novid -freq 240" {
		t.Errorf("expected '-novid -freq 240', got '%s'", initialOpts)
	}

	updatedOpts := "-novid -freq 360 -tickrate 128 +rate 786432 +exec autoexec.cfg"
	if err := appSvc.SetLaunchOptions(accA, updatedOpts); err != nil {
		t.Fatalf("SetLaunchOptions failed: %v", err)
	}
	verifiedOpts, err := appSvc.GetLaunchOptions(accA)
	if err != nil {
		t.Fatalf("GetLaunchOptions after update failed: %v", err)
	}
	if verifiedOpts != updatedOpts {
		t.Errorf("expected '%s', got '%s'", updatedOpts, verifiedOpts)
	}

	t.Log(">>> Phase 5: Video config loading, preset import, default preset application with read-only lock")
	vidSettings, err := appSvc.GetVideoSettings(accA)
	if err != nil {
		t.Fatalf("GetVideoSettings failed: %v", err)
	}
	if vidSettings.Settings["setting.defaultres"] != "1920" {
		t.Errorf("expected initial 1920 res, got '%s'", vidSettings.Settings["setting.defaultres"])
	}

	// Import preset
	preset, err := appSvc.ImportVideoPreset(presetFilePath, "Major Pro Video", true)
	if err != nil {
		t.Fatalf("ImportVideoPreset failed: %v", err)
	}
	if !preset.IsDefault {
		t.Errorf("expected preset to be default")
	}

	// Apply default preset with read-only attribute lock
	if err := appSvc.ApplyDefaultVideoPreset(accA, true); err != nil {
		t.Fatalf("ApplyDefaultVideoPreset failed: %v", err)
	}

	// Verify new settings and read-only lock
	vidAfter, err := appSvc.GetVideoSettings(accA)
	if err != nil {
		t.Fatalf("GetVideoSettings after preset apply failed: %v", err)
	}
	if vidAfter.Settings["setting.defaultres"] != "1280" {
		t.Errorf("expected preset resolution 1280, got '%s'", vidAfter.Settings["setting.defaultres"])
	}
	if !vidAfter.ReadOnly {
		t.Errorf("expected video config to have read-only file attribute locked, got false")
	}

	// Test unlocking read-only for next steps
	if err := video.SetReadOnly(filepath.Join(accACfgDir, "cs2_video.txt"), false); err != nil {
		t.Fatalf("failed to unlock read-only: %v", err)
	}

	t.Log(">>> Phase 6: LAN HTTP server launch, PIN auth, ZIP package download with restore.bat")
	// Create LAN package
	zipPath, err := appSvc.CreateLANPackage(accA, "TournamentPackage", false, true)
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}
	if _, err := os.Stat(zipPath); err != nil {
		t.Fatalf("LAN package ZIP not found at %s: %v", zipPath, err)
	}

	// Verify ZIP package contents contains restore.bat and metadata.json
	zipReader, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("failed to read created ZIP package: %v", err)
	}
	var hasRestoreBat, hasMetadata bool
	for _, f := range zipReader.File {
		if strings.EqualFold(f.Name, "restore.bat") {
			hasRestoreBat = true
		}
		if strings.EqualFold(f.Name, "metadata.json") {
			hasMetadata = true
		}
	}
	zipReader.Close()
	if !hasRestoreBat {
		t.Errorf("LAN package ZIP is missing standalone restore.bat")
	}
	if !hasMetadata {
		t.Errorf("LAN package ZIP is missing metadata.json")
	}

	// Start LAN server on dynamic port
	if err := appSvc.StartLANServer(0, "Tournament-Host"); err != nil {
		t.Fatalf("StartLANServer failed: %v", err)
	}
	defer appSvc.StopLANServer()

	if err := appSvc.ToggleLANPin(true); err != nil {
		t.Fatalf("ToggleLANPin failed: %v", err)
	}
	pin, err := appSvc.RegenerateLANPin()
	if err != nil {
		t.Fatalf("RegenerateLANPin failed: %v", err)
	}
	if pin == "" {
		t.Fatalf("expected non-empty PIN")
	}

	if err := appSvc.SetLANSharedBackups([]string{accA}, []string{zipPath}); err != nil {
		t.Fatalf("SetLANSharedBackups failed: %v", err)
	}

	lanStatus, err := appSvc.GetLANServerStatus()
	if err != nil {
		t.Fatalf("GetLANServerStatus failed: %v", err)
	}
	if !lanStatus.Running || lanStatus.Port <= 0 {
		t.Fatalf("expected LAN server running on positive port, got %+v", lanStatus)
	}

	t.Log(">>> Phase 7: UDP peer discovery and remote backup download")
	if err := appSvc.StartDiscovery(); err != nil {
		t.Fatalf("StartDiscovery failed: %v", err)
	}
	defer appSvc.StopDiscovery()

	if err := appSvc.AddManualDevice("127.0.0.1", lanStatus.Port); err != nil {
		t.Fatalf("AddManualDevice failed: %v", err)
	}

	// Connect with PIN
	remoteItems, err := appSvc.ConnectRemoteDevice("127.0.0.1", lanStatus.Port, pin)
	if err != nil {
		t.Fatalf("ConnectRemoteDevice failed: %v", err)
	}
	if len(remoteItems) == 0 {
		t.Fatalf("expected at least 1 remote backup item, got 0")
	}
	t.Logf("Found %d remote backups from Tournament-Host", len(remoteItems))

	// Download remote backup
	dlResult, err := appSvc.DownloadRemoteBackup("127.0.0.1", lanStatus.Port, pin, remoteItems[0].ID)
	if err != nil {
		t.Fatalf("DownloadRemoteBackup failed: %v", err)
	}
	if dlResult == nil || dlResult.AccountID == "" {
		t.Fatalf("invalid downloaded backup result: %+v", dlResult)
	}
	t.Logf("Successfully downloaded remote backup: AccountID=%s, Destination=%s", dlResult.AccountID, dlResult.DestDir)

	t.Log(">>> Phase 8: User restore with pre-restore rollback")
	// Account B initially had bind "MOUSE1" "+attack2"
	accBKeyFile := filepath.Join(accBCfgDir, "cs2_user_keys.vcfg")
	accBKeyBytes, err := os.ReadFile(accBKeyFile)
	if err != nil || !strings.Contains(string(accBKeyBytes), "+attack2") {
		t.Fatalf("expected Account B to have '+attack2', got %s", string(accBKeyBytes))
	}

	// Restore Account A's backup onto Account B
	if err := appSvc.RestoreUser(accB, accA); err != nil {
		t.Fatalf("RestoreUser failed: %v", err)
	}

	// Check that automatic rollback copy was created
	rollbackDir := filepath.Join(backupPath, "restore_originals", "user_730", accB)
	if fi, err := os.Stat(rollbackDir); err != nil || !fi.IsDir() {
		t.Fatalf("expected automatic pre-restore rollback directory at %s, err: %v", rollbackDir, err)
	}

	// Verify rollback directory contains Account B's original '+attack2'
	origBKeyFile := filepath.Join(rollbackDir, "local", "cfg", "cs2_user_keys.vcfg")
	origBKeyBytes, err := os.ReadFile(origBKeyFile)
	if err != nil {
		t.Fatalf("failed to read rollback original key file: %v", err)
	}
	if !strings.Contains(string(origBKeyBytes), "+attack2") {
		t.Errorf("rollback data did not preserve original '+attack2': %s", string(origBKeyBytes))
	}

	// Verify Account B's live configuration now has Account A's '+attack'
	restoredBKeyBytes, err := os.ReadFile(accBKeyFile)
	if err != nil {
		t.Fatalf("failed to read restored key file: %v", err)
	}
	if !strings.Contains(string(restoredBKeyBytes), "+attack") {
		t.Errorf("expected restored config to contain '+attack', got %s", string(restoredBKeyBytes))
	}

	t.Log(">>> ALL 8 LIFECYCLE PHASES OF APPSERVICE E2E COMPLETED AND VERIFIED 100% PASS <<<")
}
