package lan

import (
	"archive/zip"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

func TestCreateAndValidateLANPackage(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// Setup fake userdata (730)
	userdataDir := filepath.Join(tempDir, "userdata_source", "730", "local", "cfg")
	if err := os.MkdirAll(userdataDir, 0755); err != nil {
		t.Fatalf("failed to create userdataDir: %v", err)
	}
	videoFile := filepath.Join(userdataDir, "cs2_video.txt")
	if err := os.WriteFile(videoFile, []byte(`"VideoConfig" { "setting.csm" "3" }`), 0644); err != nil {
		t.Fatalf("failed to write videoFile: %v", err)
	}

	// Setup fake game cfg
	cfgDir := filepath.Join(tempDir, "game_cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatalf("failed to create cfgDir: %v", err)
	}
	autoexecFile := filepath.Join(cfgDir, "autoexec.cfg")
	if err := os.WriteFile(autoexecFile, []byte(`echo "CS2 Pro Config Loaded"`), 0644); err != nil {
		t.Fatalf("failed to write autoexec: %v", err)
	}

	outZipPath := filepath.Join(tempDir, "output", "test_package.zip")

	opts := PackageOptions{
		AccountID:       "123456789",
		Name:            "ZywOo_Test",
		IncludeUserdata: true,
		UserdataDir:     filepath.Join(tempDir, "userdata_source", "730"),
		IncludeCfg:      true,
		CfgDir:          cfgDir,
		OutputPath:      outZipPath,
	}

	resultZip, err := CreateLANPackage(opts)
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}
	if resultZip != outZipPath {
		t.Errorf("resultZip = %s; want %s", resultZip, outZipPath)
	}

	// Validate the created package
	valResult, err := ValidatePackage(outZipPath)
	if err != nil {
		t.Fatalf("ValidatePackage failed: %v", err)
	}
	if !valResult.AllValid {
		t.Errorf("ValidatePackage returned AllValid=false, details: %+v", valResult)
	}
	if valResult.Metadata.Name != "ZywOo_Test" {
		t.Errorf("Metadata.Name = %s; want ZywOo_Test", valResult.Metadata.Name)
	}
	if !valResult.Metadata.ContainsCfg || !valResult.Metadata.ContainsUserdata {
		t.Errorf("Metadata flags invalid: cfg=%v, userdata=%v", valResult.Metadata.ContainsCfg, valResult.Metadata.ContainsUserdata)
	}
	if !valResult.HasRestoreBat {
		t.Errorf("Package should contain restore.bat")
	}

	// Verify restore.bat content
	r, err := zip.OpenReader(outZipPath)
	if err != nil {
		t.Fatalf("open zip failed: %v", err)
	}
	defer r.Close()

	foundBat := false
	found730 := false
	foundCfg := false
	for _, f := range r.File {
		if strings.HasSuffix(f.Name, "restore.bat") {
			foundBat = true
		}
		if strings.HasPrefix(f.Name, "730/") {
			found730 = true
		}
		if strings.HasPrefix(f.Name, "cfg/") {
			foundCfg = true
		}
	}
	if !foundBat || !found730 || !foundCfg {
		t.Errorf("zip contents missing components: bat=%v, 730=%v, cfg=%v", foundBat, found730, foundCfg)
	}
}

func TestValidatePackageZipSlip(t *testing.T) {
	tempDir := testutil.TempDir(t)
	maliciousZipPath := filepath.Join(tempDir, "evil.zip")

	// Create a zip with path traversal attempt
	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)

	// Safe files
	w, _ := zw.Create("metadata.json")
	w.Write([]byte(`{"name":"Evil","files":[]}`))
	w, _ = zw.Create("restore.bat")
	w.Write([]byte(`@echo off`))

	// Malicious file
	w, _ = zw.Create("../../../windows/system32/cmd.exe")
	w.Write([]byte("malicious"))

	zw.Close()

	if err := os.WriteFile(maliciousZipPath, buf.Bytes(), 0644); err != nil {
		t.Fatalf("failed to write evil zip: %v", err)
	}

	_, err := ValidatePackage(maliciousZipPath)
	if err == nil {
		t.Fatalf("expected Zip-Slip error, got nil")
	}
	if !strings.Contains(err.Error(), "traversal") && !strings.Contains(err.Error(), "zip-slip") {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestValidatePackageTampered(t *testing.T) {
	tempDir := testutil.TempDir(t)

	userdataDir := filepath.Join(tempDir, "730", "local", "cfg")
	os.MkdirAll(userdataDir, 0755)
	os.WriteFile(filepath.Join(userdataDir, "cs2_video.txt"), []byte("original"), 0644)

	zipPath := filepath.Join(tempDir, "tamper.zip")
	_, err := CreateLANPackage(PackageOptions{
		AccountID:       "123",
		Name:            "TamperTest",
		IncludeUserdata: true,
		UserdataDir:     filepath.Join(tempDir, "730"),
		OutputPath:      zipPath,
	})
	if err != nil {
		t.Fatalf("CreateLANPackage failed: %v", err)
	}

	// Create tampered zip by rewriting cs2_video.txt with different content
	tamperedZipPath := filepath.Join(tempDir, "tampered_result.zip")
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		t.Fatalf("open original zip failed: %v", err)
	}
	defer r.Close()

	buf := new(bytes.Buffer)
	zw := zip.NewWriter(buf)
	for _, f := range r.File {
		w, _ := zw.Create(f.Name)
		if strings.HasSuffix(f.Name, "cs2_video.txt") {
			w.Write([]byte("TAMPERED_CONTENT_DIFF"))
		} else {
			rc, _ := f.Open()
			bufData := make([]byte, f.UncompressedSize64)
			rc.Read(bufData)
			rc.Close()
			w.Write(bufData)
		}
	}
	zw.Close()
	os.WriteFile(tamperedZipPath, buf.Bytes(), 0644)

	valResult, err := ValidatePackage(tamperedZipPath)
	if err != nil {
		t.Fatalf("ValidatePackage returned unexpected error: %v", err)
	}
	if valResult.AllValid {
		t.Errorf("expected AllValid to be false for tampered zip")
	}
}

func TestGenerateRestoreBat_ActiveCopy(t *testing.T) {
	bat := GenerateRestoreBat(true, true, "123456789")
	if !strings.Contains(bat, "xcopy") {
		t.Errorf("restore.bat should use xcopy for active file copying")
	}
	if !strings.Contains(bat, "robocopy") {
		t.Errorf("restore.bat should have robocopy fallback")
	}
	if !strings.Contains(bat, "ACCOUNT_ID=123456789") {
		t.Errorf("restore.bat should set ACCOUNT_ID when provided")
	}
	if !strings.Contains(bat, "DEST_730") {
		t.Errorf("restore.bat should define DEST_730 destination")
	}
}
