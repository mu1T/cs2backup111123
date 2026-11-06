package lan

import (
	"archive/zip"
	"bytes"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

func TestClient_Authenticate(t *testing.T) {
	srv := NewServer()
	srv.SetPort(0) // Random available port
	srv.SetPIN("888999")
	srv.SetPINEnabled(true)

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start LAN server: %v", err)
	}
	defer srv.Stop()

	status := srv.GetStatus()
	client := NewClient()

	// 1. Wrong PIN fails
	_, err := client.Authenticate("127.0.0.1", status.Port, "000000")
	if err == nil {
		t.Fatalf("expected authentication error with wrong PIN, got nil")
	}

	// 2. Correct PIN succeeds
	token, err := client.Authenticate("127.0.0.1", status.Port, "888999")
	if err != nil {
		t.Fatalf("Authenticate failed with correct PIN: %v", err)
	}
	if token == "" {
		t.Fatalf("expected non-empty token")
	}
}

func TestClient_GetRemoteBackups(t *testing.T) {
	tmpDir := testutil.TempDir(t)

	// Prepare mock userdata & cfg
	udDir := filepath.Join(tmpDir, "userdata", "123456789", "730", "local", "cfg")
	cfgDir := filepath.Join(tmpDir, "game", "csgo", "cfg")
	_ = os.MkdirAll(udDir, 0755)
	_ = os.MkdirAll(cfgDir, 0755)
	_ = os.WriteFile(filepath.Join(udDir, "cs2_user_keys.vcfg"), []byte("bind w +forward"), 0644)
	_ = os.WriteFile(filepath.Join(cfgDir, "autoexec.cfg"), []byte("echo autoexec"), 0644)

	// Create a test LAN package
	pkgPath := filepath.Join(tmpDir, "test_pkg.zip")
	_, err := CreateLANPackage(PackageOptions{
		AccountID:       "123456789",
		Name:            "Pro_Config",
		IncludeUserdata: true,
		UserdataDir:     filepath.Join(tmpDir, "userdata", "123456789", "730"),
		IncludeCfg:      true,
		CfgDir:          cfgDir,
		OutputPath:      pkgPath,
	})
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}

	srv := NewServer()
	srv.SetPort(0)
	srv.SetPIN("123456")
	srv.SetPINEnabled(true)
	srv.AddZIPBackup(pkgPath)

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	status := srv.GetStatus()
	client := NewClient()

	// 1. Without PIN when PIN enabled -> fails
	_, err = client.GetRemoteBackups("127.0.0.1", status.Port, "")
	if err == nil {
		t.Fatalf("expected error without PIN, got nil")
	}

	// 2. With PIN -> returns backups
	backups, err := client.GetRemoteBackups("127.0.0.1", status.Port, "123456")
	if err != nil {
		t.Fatalf("GetRemoteBackups failed: %v", err)
	}
	if len(backups) != 1 {
		t.Fatalf("expected 1 backup, got %d", len(backups))
	}
	if backups[0].BackupName != "Pro_Config" {
		t.Errorf("expected backup name 'Pro_Config', got '%s'", backups[0].BackupName)
	}
	if !backups[0].ContainsCfg || !backups[0].ContainsUserdata {
		t.Errorf("expected backup to contain cfg and userdata: %+v", backups[0])
	}
}

func TestClient_DownloadRemoteBackup(t *testing.T) {
	tmpDir := testutil.TempDir(t)

	// Prepare mock source files
	udDir := filepath.Join(tmpDir, "userdata", "12345", "730", "local", "cfg")
	cfgDir := filepath.Join(tmpDir, "game", "csgo", "cfg")
	_ = os.MkdirAll(udDir, 0755)
	_ = os.MkdirAll(cfgDir, 0755)
	_ = os.WriteFile(filepath.Join(udDir, "cs2_user_keys.vcfg"), []byte("bind mouse1 +attack"), 0644)
	_ = os.WriteFile(filepath.Join(cfgDir, "nadethrow.cfg"), []byte("echo nadethrow"), 0644)

	// Create test package
	pkgPath := filepath.Join(tmpDir, "source_pkg.zip")
	_, err := CreateLANPackage(PackageOptions{
		AccountID:       "12345",
		Name:            "S1mple_CS2",
		IncludeUserdata: true,
		UserdataDir:     filepath.Join(tmpDir, "userdata", "12345", "730"),
		IncludeCfg:      true,
		CfgDir:          cfgDir,
		OutputPath:      pkgPath,
	})
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}

	srv := NewServer()
	srv.SetPort(0)
	srv.SetPIN("556677")
	srv.SetPINEnabled(true)
	srv.AddZIPBackup(pkgPath)

	if err := srv.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer srv.Stop()

	status := srv.GetStatus()
	client := NewClient()

	backups, err := client.GetRemoteBackups("127.0.0.1", status.Port, "556677")
	if err != nil || len(backups) == 0 {
		t.Fatalf("failed to get backup items: %v", err)
	}
	backupID := backups[0].ID

	// Target local backup repository
	backupRepo := filepath.Join(tmpDir, "local_backup_repo")

	// Download and unpack
	res, err := client.DownloadRemoteBackup("127.0.0.1", status.Port, "556677", backupID, backupRepo)
	if err != nil {
		t.Fatalf("DownloadRemoteBackup failed: %v", err)
	}

	if !strings.HasPrefix(res.AccountID, "remote_") {
		t.Errorf("expected account ID starting with remote_, got '%s'", res.AccountID)
	}
	if res.TotalFiles <= 0 {
		t.Errorf("expected TotalFiles > 0, got %d", res.TotalFiles)
	}
	if res.TotalBytes <= 0 {
		t.Errorf("expected TotalBytes > 0, got %d", res.TotalBytes)
	}

	// Verify unpacked directory structure:
	// <backupRepo>/userdata_backup/remote_<timestamp>/730/local/cfg/cs2_user_keys.vcfg
	unpackedKey := filepath.Join(res.DestDir, "730", "local", "cfg", "cs2_user_keys.vcfg")
	if data, err := os.ReadFile(unpackedKey); err != nil || string(data) != "bind mouse1 +attack" {
		t.Errorf("unpacked 730 file invalid or missing: %v", err)
	}

	// <backupRepo>/userdata_backup/remote_<timestamp>/cfg/nadethrow.cfg
	unpackedCfg := filepath.Join(res.DestDir, "cfg", "nadethrow.cfg")
	if data, err := os.ReadFile(unpackedCfg); err != nil || string(data) != "echo nadethrow" {
		t.Errorf("unpacked cfg file invalid or missing: %v", err)
	}

	// Verify remote_info.json
	remoteInfoPath := filepath.Join(res.DestDir, "remote_info.json")
	infoData, err := os.ReadFile(remoteInfoPath)
	if err != nil {
		t.Fatalf("remote_info.json not found: %v", err)
	}
	if !strings.Contains(string(infoData), `"username": "来自其它设备的备份"`) {
		t.Errorf("remote_info.json username mismatch: %s", string(infoData))
	}
	if !strings.Contains(string(infoData), `"type": "remote"`) {
		t.Errorf("remote_info.json type mismatch: %s", string(infoData))
	}
}

func TestClient_DownloadRemoteBackup_ZipSlipDefense(t *testing.T) {
	// Craft a malicious ZIP containing a path traversal entry
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../../../evil.txt")
	if err != nil {
		t.Fatalf("failed to create zip entry: %v", err)
	}
	_, _ = w.Write([]byte("malicious payload"))
	_ = zw.Close()

	// Mock server serving malicious zip
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/zip")
		_, _ = w.Write(buf.Bytes())
	}))
	defer ts.Close()

	parts := strings.Split(strings.TrimPrefix(ts.URL, "http://"), ":")
	ip := parts[0]
	var port int
	fmt.Sscanf(parts[1], "%d", &port)

	tmpDir := testutil.TempDir(t)
	client := NewClient()

	_, err = client.DownloadRemoteBackup(ip, port, "", "evil_backup", tmpDir)
	if err == nil {
		t.Fatalf("expected Zip-Slip defense error, got nil")
	}

	// Ensure no evil.txt was extracted
	if _, err := os.Stat(filepath.Join(tmpDir, "evil.txt")); !os.IsNotExist(err) {
		t.Errorf("evil.txt was written! Zip-Slip defense failed")
	}
}
