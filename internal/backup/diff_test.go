package backup

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestComputeTextDiff_Identical(t *testing.T) {
	lines := []string{"line1", "line2", "line3"}
	res := ComputeTextDiff(lines, lines)

	if res.AddCount != 0 || res.DelCount != 0 || res.ModCount != 0 {
		t.Errorf("expected 0 differences for identical files, got add=%d del=%d mod=%d",
			res.AddCount, res.DelCount, res.ModCount)
	}
	if len(res.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(res.Lines))
	}
	for i, l := range res.Lines {
		if l.Type != DiffSame || l.OldLineNum != i+1 || l.NewLineNum != i+1 {
			t.Errorf("line %d mismatch: %+v", i, l)
		}
	}
}

func TestComputeTextDiff_Mod(t *testing.T) {
	a := []string{`bind "e" "+use"`, `volume "0.5"`, `fps_max "300"`}
	b := []string{`bind "e" "+use"`, `volume "0.8"`, `fps_max "300"`}

	res := ComputeTextDiff(a, b)

	if res.ModCount != 1 || res.AddCount != 0 || res.DelCount != 0 {
		t.Errorf("expected 1 mod, got add=%d del=%d mod=%d", res.AddCount, res.DelCount, res.ModCount)
	}
	if len(res.Lines) != 3 {
		t.Fatalf("expected 3 lines, got %d", len(res.Lines))
	}

	modLine := res.Lines[1]
	if modLine.Type != DiffMod {
		t.Errorf("expected line 2 to be mod, got %s", modLine.Type)
	}
	if modLine.Content != `volume "0.8"` || modLine.OldContent != `volume "0.5"` {
		t.Errorf("unexpected mod contents: content='%s' oldContent='%s'", modLine.Content, modLine.OldContent)
	}
	if modLine.OldLineNum != 2 || modLine.NewLineNum != 2 {
		t.Errorf("unexpected line nums: old=%d new=%d", modLine.OldLineNum, modLine.NewLineNum)
	}
}

func TestComputeTextDiff_AddAndDel(t *testing.T) {
	a := []string{"keep1", "delete_me", "keep2"}
	b := []string{"keep1", "keep2", "added_line"}

	res := ComputeTextDiff(a, b)

	if res.DelCount != 1 || res.AddCount != 1 {
		t.Errorf("expected 1 del and 1 add, got del=%d add=%d", res.DelCount, res.AddCount)
	}

	foundDel := false
	foundAdd := false
	for _, l := range res.Lines {
		if l.Type == DiffDel && l.Content == "delete_me" && l.OldLineNum == 2 {
			foundDel = true
		}
		if l.Type == DiffAdd && l.Content == "added_line" && l.NewLineNum == 3 {
			foundAdd = true
		}
	}

	if !foundDel {
		t.Errorf("expected to find delete_me deleted")
	}
	if !foundAdd {
		t.Errorf("expected to find added_line added")
	}
}

func TestGetTextDiff_CRLF_Normalization(t *testing.T) {
	crlfText := "setting1\r\nsetting2\r\nsetting3\r\n"
	lfText := "setting1\nsetting2\nsetting3\n"

	res, err := GetTextDiff(crlfText, lfText)
	if err != nil {
		t.Fatalf("GetTextDiff failed: %v", err)
	}

	if res.AddCount != 0 || res.DelCount != 0 || res.ModCount != 0 {
		t.Errorf("crlf and lf should be treated as identical content, got add=%d del=%d mod=%d",
			res.AddCount, res.DelCount, res.ModCount)
	}
}

func TestCompareConfigFiles(t *testing.T) {
	tmp := testutil.TempDir(t)
	pathA := filepath.Join(tmp, "a.cfg")
	pathB := filepath.Join(tmp, "b.cfg")

	if err := os.WriteFile(pathA, []byte("sensitivity 1.5\nzoom_sensitivity_ratio 0.8\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pathB, []byte("sensitivity 2.0\nzoom_sensitivity_ratio 0.8\n"), 0644); err != nil {
		t.Fatal(err)
	}

	res, err := CompareConfigFiles(pathA, pathB)
	if err != nil {
		t.Fatalf("CompareConfigFiles failed: %v", err)
	}

	if res.ModCount != 1 {
		t.Errorf("expected 1 mod, got %d", res.ModCount)
	}

	// Compare with empty pathA (new file)
	resNew, err := CompareConfigFiles("", pathB)
	if err != nil {
		t.Fatalf("CompareConfigFiles with empty pathA failed: %v", err)
	}
	if resNew.AddCount != 2 {
		t.Errorf("expected 2 additions, got %d", resNew.AddCount)
	}
}

func TestCompareSnapshotWithLive(t *testing.T) {
	tmp := testutil.TempDir(t)
	steamPath := filepath.Join(tmp, "Steam")
	backupPath := filepath.Join(tmp, "Backup")
	accountID := "44444"

	setupTestSteamUserdata(t, steamPath, accountID)

	snap, err := CreateSnapshot(steamPath, backupPath, accountID, "基准对比快照")
	if err != nil {
		t.Fatalf("CreateSnapshot failed: %v", err)
	}

	// Mutate live userdata:
	// 1. Modify cs2_video.txt
	videoFile := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_video.txt")
	if err := os.WriteFile(videoFile, []byte(`"VideoConfig" { "setting.csm_quality_level" "1" }`), 0644); err != nil {
		t.Fatal(err)
	}
	// 2. Add cs2_user_convars.vcfg
	convarsFile := filepath.Join(steamPath, "userdata", accountID, "730", "local", "cfg", "cs2_user_convars.vcfg")
	if err := os.WriteFile(convarsFile, []byte(`"convars" { "volume" "0.7" }`), 0644); err != nil {
		t.Fatal(err)
	}

	diffItems, err := CompareSnapshotWithLive(steamPath, backupPath, accountID, snap.ID)
	if err != nil {
		t.Fatalf("CompareSnapshotWithLive failed: %v", err)
	}

	statusMap := make(map[string]string)
	for _, item := range diffItems {
		statusMap[item.RelPath] = item.Status
	}

	if statusMap["local/cfg/cs2_video.txt"] != "modified" {
		t.Errorf("expected video file to be modified, got %s", statusMap["local/cfg/cs2_video.txt"])
	}
	if statusMap["local/cfg/cs2_user_keys.vcfg"] != "identical" {
		t.Errorf("expected keys file to be identical, got %s", statusMap["local/cfg/cs2_user_keys.vcfg"])
	}
	if statusMap["local/cfg/cs2_user_convars.vcfg"] != "added" {
		t.Errorf("expected convars file to be added, got %s", statusMap["local/cfg/cs2_user_convars.vcfg"])
	}
}
