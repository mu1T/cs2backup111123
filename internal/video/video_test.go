package video

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

const sampleVideoCfg = `"video.cfg"
{
	"setting.defaultres"		"1920"
	"setting.defaultresheight"		"1080"
	"setting.refreshrate_numerator"		"144"
	"setting.refreshrate_denominator"		"1"
	"setting.fullscreen"		"1"
	"setting.coop_fullscreen"		"0"
	"setting.nowindowborder"		"0"
	"setting.mat_vsync"		"0"
	"setting.fullscreen_min_on_focus_loss"		"1"
	"setting.high_dpi"		"0"
	"setting.aspectratiomode"		"1"
	"setting.monitor_index"		"0"
	"setting.cpu_level"		"0"
	"setting.gpu_level"		"0"
	"setting.gpu_mem_level"		"0"
	"setting.msaa_samples"		"4"
	"setting.r_csgo_cmaa_enable"		"0"
	"setting.videocfg_fsr_detail"		"0"
	"setting.videocfg_dynamic_shadows"		"1"
	"setting.videocfg_ao_detail"		"0"
	"setting.videocfg_shadow_quality"		"1"
	"setting.videocfg_particle_detail"		"0"
	"setting.videocfg_texture_detail"		"0"
	"setting.shaderquality"		"0"
	"setting.videocfg_hdr_detail"		"3"
	"setting.r_texturefilteringquality"		"1"
	"setting.r_low_latency"		"1"
}
`

func TestLoadConfigFile_Success(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cfgPath := filepath.Join(tempDir, "cs2_video.txt")

	if err := os.WriteFile(cfgPath, []byte(sampleVideoCfg), 0666); err != nil {
		t.Fatalf("Failed to write sample config: %v", err)
	}

	cf, err := LoadConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfigFile failed: %v", err)
	}

	if cf.Settings["setting.defaultres"] != "1920" {
		t.Errorf("Expected defaultres 1920, got %s", cf.Settings["setting.defaultres"])
	}
	if cf.Settings["setting.defaultresheight"] != "1080" {
		t.Errorf("Expected defaultresheight 1080, got %s", cf.Settings["setting.defaultresheight"])
	}
	if cf.Settings["setting.msaa_samples"] != "4" {
		t.Errorf("Expected msaa_samples 4, got %s", cf.Settings["setting.msaa_samples"])
	}
	if cf.ReadOnly {
		t.Errorf("Expected ReadOnly to be false initially")
	}
}

func TestSaveConfigFile_UpdateAndReadOnlyLock(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cfgPath := filepath.Join(tempDir, "cs2_video.txt")

	if err := os.WriteFile(cfgPath, []byte(sampleVideoCfg), 0666); err != nil {
		t.Fatalf("Failed to write sample config: %v", err)
	}

	// Update resolution and apply read-only lock
	updates := map[string]string{
		"setting.defaultres":       "1280",
		"setting.defaultresheight": "960",
		"setting.msaa_samples":     "2",
		"setting.custom_extra_key": "999", // newly added key
	}

	err := SaveConfigFile(cfgPath, updates, true)
	if err != nil {
		t.Fatalf("SaveConfigFile failed: %v", err)
	}

	// Verify read-only lock is active
	isRO, err := IsReadOnly(cfgPath)
	if err != nil {
		t.Fatalf("IsReadOnly check failed: %v", err)
	}
	if !isRO {
		t.Errorf("Expected file to be read-only")
	}

	// Verify file content
	cf, err := LoadConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfigFile after save failed: %v", err)
	}
	if cf.Settings["setting.defaultres"] != "1280" {
		t.Errorf("Expected updated defaultres 1280, got %s", cf.Settings["setting.defaultres"])
	}
	if cf.Settings["setting.defaultresheight"] != "960" {
		t.Errorf("Expected updated defaultresheight 960, got %s", cf.Settings["setting.defaultresheight"])
	}
	if cf.Settings["setting.msaa_samples"] != "2" {
		t.Errorf("Expected updated msaa_samples 2, got %s", cf.Settings["setting.msaa_samples"])
	}
	if cf.Settings["setting.custom_extra_key"] != "999" {
		t.Errorf("Expected added custom_extra_key 999, got %s", cf.Settings["setting.custom_extra_key"])
	}
	if !cf.ReadOnly {
		t.Errorf("Expected cf.ReadOnly to be true")
	}

	// Verify we can save again to an already read-only file and unlock it
	updates2 := map[string]string{
		"setting.defaultres": "1440",
	}
	err = SaveConfigFile(cfgPath, updates2, false)
	if err != nil {
		t.Fatalf("SaveConfigFile overwriting read-only file failed: %v", err)
	}

	isRO2, _ := IsReadOnly(cfgPath)
	if isRO2 {
		t.Errorf("Expected file to no longer be read-only")
	}

	cf2, _ := LoadConfigFile(cfgPath)
	if cf2.Settings["setting.defaultres"] != "1440" {
		t.Errorf("Expected defaultres 1440, got %s", cf2.Settings["setting.defaultres"])
	}
	if cf2.Settings["setting.defaultresheight"] != "960" {
		t.Errorf("Expected defaultresheight to remain 960, got %s", cf2.Settings["setting.defaultresheight"])
	}
}

func TestSaveConfigFile_NewFileCreation(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cfgPath := filepath.Join(tempDir, "subdir", "cs2_video.txt")

	settings := map[string]string{
		"setting.defaultres":       "2560",
		"setting.defaultresheight": "1440",
	}

	err := SaveConfigFile(cfgPath, settings, false)
	if err != nil {
		t.Fatalf("SaveConfigFile to new file failed: %v", err)
	}

	cf, err := LoadConfigFile(cfgPath)
	if err != nil {
		t.Fatalf("LoadConfigFile failed on newly created file: %v", err)
	}

	if cf.Settings["setting.defaultres"] != "2560" || cf.Settings["setting.defaultresheight"] != "1440" {
		t.Errorf("Incorrect settings in newly created file: %+v", cf.Settings)
	}

	data, _ := os.ReadFile(cfgPath)
	content := string(data)
	if !strings.Contains(content, "\"video.cfg\"") || !strings.Contains(content, "}") {
		t.Errorf("Expected valid VDF skeleton, got:\n%s", content)
	}
}

func TestGetAccountVideoPath(t *testing.T) {
	steamPath := `C:\Steam`
	accountID := "12345678"
	expected := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_video.txt")
	got := GetAccountVideoPath(steamPath, accountID)
	if got != expected {
		t.Errorf("Expected %s, got %s", expected, got)
	}
}
