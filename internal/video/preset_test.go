package video

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

func TestPresetManager_Lifecycle(t *testing.T) {
	tempConfigDir := testutil.TempDir(t)
	pm, err := NewPresetManager(tempConfigDir)
	if err != nil {
		t.Fatalf("NewPresetManager failed: %v", err)
	}

	// 1. Initial presets list should be empty
	presets, err := pm.GetPresets()
	if err != nil {
		t.Fatalf("GetPresets failed: %v", err)
	}
	if len(presets) != 0 {
		t.Errorf("Expected 0 initial presets, got %d", len(presets))
	}

	// 2. Prepare sample source .txt file
	srcFile := filepath.Join(tempConfigDir, "test_preset.txt")
	if err := os.WriteFile(srcFile, []byte(sampleVideoCfg), 0666); err != nil {
		t.Fatalf("Failed to write test preset file: %v", err)
	}

	// 3. Import preset and mark as default
	preset1, err := pm.ImportPreset(srcFile, "竞技优化预设 (4:3 拉伸)", true)
	if err != nil {
		t.Fatalf("ImportPreset failed: %v", err)
	}
	if preset1.Name != "竞技优化预设 (4:3 拉伸)" {
		t.Errorf("Unexpected preset name: %s", preset1.Name)
	}
	if !preset1.IsDefault {
		t.Errorf("Expected preset1 to be default")
	}
	if preset1.Settings["setting.defaultres"] != "1920" {
		t.Errorf("Expected defaultres 1920, got %s", preset1.Settings["setting.defaultres"])
	}

	// 4. Import second preset without marking default
	preset2, err := pm.ImportPreset(srcFile, "2K画质全开", false)
	if err != nil {
		t.Fatalf("ImportPreset 2 failed: %v", err)
	}
	if preset2.IsDefault {
		t.Errorf("Expected preset2 not to be default")
	}

	// Verify presets list
	list, err := pm.GetPresets()
	if err != nil {
		t.Fatalf("GetPresets failed: %v", err)
	}
	if len(list) != 2 {
		t.Fatalf("Expected 2 presets, got %d", len(list))
	}

	// 5. Switch default preset
	if err := pm.SetDefaultPreset(preset2.ID); err != nil {
		t.Fatalf("SetDefaultPreset failed: %v", err)
	}

	p1, _ := pm.GetPreset(preset1.ID)
	p2, _ := pm.GetPreset(preset2.ID)
	if p1.IsDefault {
		t.Errorf("Expected p1 to no longer be default")
	}
	if !p2.IsDefault {
		t.Errorf("Expected p2 to be default")
	}

	// 6. Test SaveAccountAsPreset
	steamDir := testutil.TempDir(t)
	accountID := "87654321"
	accountVideoDir := filepath.Join(steamDir, "userdata", accountID, "730", "local", "cfg")
	if err := os.MkdirAll(accountVideoDir, 0755); err != nil {
		t.Fatalf("Failed to create account video dir: %v", err)
	}
	accountVideoFile := filepath.Join(accountVideoDir, "cs2_video.txt")
	if err := os.WriteFile(accountVideoFile, []byte(sampleVideoCfg), 0666); err != nil {
		t.Fatalf("Failed to write account video file: %v", err)
	}

	preset3, err := pm.SaveAccountAsPreset(steamDir, accountID, "我的主力大号预设", false)
	if err != nil {
		t.Fatalf("SaveAccountAsPreset failed: %v", err)
	}
	if preset3.Name != "我的主力大号预设" {
		t.Errorf("Unexpected preset3 name: %s", preset3.Name)
	}

	// 7. Test ApplyPreset with read-only lock
	targetAccountID := "99999999"
	err = pm.ApplyPreset(steamDir, targetAccountID, preset1.ID, true)
	if err != nil {
		t.Fatalf("ApplyPreset failed: %v", err)
	}

	targetVideoPath := GetAccountVideoPath(steamDir, targetAccountID)
	targetCF, err := LoadConfigFile(targetVideoPath)
	if err != nil {
		t.Fatalf("Failed to load target video config after ApplyPreset: %v", err)
	}
	if targetCF.Settings["setting.defaultres"] != "1920" {
		t.Errorf("Expected defaultres 1920, got %s", targetCF.Settings["setting.defaultres"])
	}
	if !targetCF.ReadOnly {
		t.Errorf("Expected target video config to be read-only")
	}

	// 8. Test ApplyDefaultPreset
	err = pm.ApplyDefaultPreset(steamDir, targetAccountID, false)
	if err != nil {
		t.Fatalf("ApplyDefaultPreset failed: %v", err)
	}
	targetCF2, _ := LoadConfigFile(targetVideoPath)
	if targetCF2.ReadOnly {
		t.Errorf("Expected target video config to have read-only disabled")
	}

	// 9. Test DeletePreset
	if err := pm.DeletePreset(preset2.ID); err != nil {
		t.Fatalf("DeletePreset failed: %v", err)
	}
	remaining, _ := pm.GetPresets()
	if len(remaining) != 2 {
		t.Errorf("Expected 2 remaining presets, got %d", len(remaining))
	}
}

func TestPresetManager_ApplyDefaultPreset_NoDefaultError(t *testing.T) {
	tempConfigDir := testutil.TempDir(t)
	pm, err := NewPresetManager(tempConfigDir)
	if err != nil {
		t.Fatalf("NewPresetManager failed: %v", err)
	}

	err = pm.ApplyDefaultPreset(testutil.TempDir(t), "12345", false)
	if err == nil {
		t.Fatalf("Expected error when applying default preset with no default set, got nil")
	}
}

// 预设 ID 曾用 time.Now().UnixNano() 拼，唯一性依赖时钟精度。Windows 上
// 这个时钟的步进远大于 1ns，两次导入落在同一个 tick 就会撞 ID：两个预设
// 指向同一个文件（后写的覆盖前一个，删一个等于删两个），且 IsDefault 会被
// 算成 true——CI 上就是这样红的。
//
// 把时钟冻结之后，「同一个 tick」不再是概率事件而是必然事件：只要 ID 还从
// 时钟取，这个测试就会稳定复现。修好之后它断言的是真正的不变量——
// N 次导入必须得到 N 个不同 ID 与 N 个不同文件，与时间无关。
func TestPresetManager_ImportNeverReusesID(t *testing.T) {
	tempConfigDir := testutil.TempDir(t)
	pm, err := NewPresetManager(tempConfigDir)
	if err != nil {
		t.Fatalf("NewPresetManager failed: %v", err)
	}

	frozen := time.Date(2026, 11, 6, 14, 33, 38, 0, time.Local)
	pm.now = func() time.Time { return frozen }

	srcFile := filepath.Join(tempConfigDir, "src.txt")
	if err := os.WriteFile(srcFile, []byte(sampleVideoCfg), 0666); err != nil {
		t.Fatalf("Failed to write source file: %v", err)
	}

	const n = 50
	seenID := make(map[string]int, n)
	seenPath := make(map[string]int, n)
	for i := 0; i < n; i++ {
		wantDefault := i == 0
		preset, err := pm.ImportPreset(srcFile, fmt.Sprintf("预设 %d", i), wantDefault)
		if err != nil {
			t.Fatalf("第 %d 次 ImportPreset 失败: %v", i, err)
		}
		if prev, dup := seenID[preset.ID]; dup {
			t.Fatalf("第 %d 次导入复用了第 %d 次的预设 ID %q：两个预设会指向同一个文件", i, prev, preset.ID)
		}
		if prev, dup := seenPath[preset.FilePath]; dup {
			t.Fatalf("第 %d 次导入复用了第 %d 次的预设文件 %q", i, prev, preset.FilePath)
		}
		seenID[preset.ID] = i
		seenPath[preset.FilePath] = i

		// 只有第一个被标记为默认；撞 ID 时这里会红。
		if preset.IsDefault != wantDefault {
			t.Errorf("第 %d 次导入 IsDefault=%v，期望 %v", i, preset.IsDefault, wantDefault)
		}
		if _, statErr := os.Stat(preset.FilePath); statErr != nil {
			t.Errorf("第 %d 次导入的预设文件不存在: %v", i, statErr)
		}
	}

	list, err := pm.GetPresets()
	if err != nil {
		t.Fatalf("GetPresets failed: %v", err)
	}
	if len(list) != n {
		t.Errorf("期望 %d 个预设，实际 %d 个", n, len(list))
	}
	defaults := 0
	for _, p := range list {
		if p.IsDefault {
			defaults++
		}
	}
	if defaults != 1 {
		t.Errorf("期望恰好 1 个默认预设，实际 %d 个", defaults)
	}
}
