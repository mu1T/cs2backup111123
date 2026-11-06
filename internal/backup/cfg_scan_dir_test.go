package backup

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestListGameCFGFilesWithTempDir(t *testing.T) {
	root := testutil.TempDir(t)
	cfgDir := filepath.Join(root, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0755); err != nil {
		t.Fatal(err)
	}
	for _, n := range []string{
		"autoexec.cfg", "surf.cfg",
		"pwa.cfg", "pwa_userconfig76561198083722517.cfg",
		"f5eautoexec.cfg",
		"gamemode_competitive.cfg", "boot.vcfg",
		"readme.md", // 非 cfg 扩展名，应被忽略
	} {
		if err := os.WriteFile(filepath.Join(cfgDir, n), []byte("x"), 0644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.MkdirAll(filepath.Join(cfgDir, "devonly"), 0755); err != nil {
		t.Fatal(err)
	}

	svc := &Service{}
	svc.SetSteamPath(root)

	files, err := svc.ListGameCFGFiles()
	if err != nil {
		t.Fatalf("ListGameCFGFiles: %v", err)
	}
	if len(files) != 7 {
		t.Fatalf("expected 7 files (excluding dir and .md), got %d", len(files))
	}
	if files[0].Source != CFGSourcePlayer || files[0].Name != "autoexec.cfg" {
		t.Errorf("first file should be player/autoexec.cfg, got %s/%s", files[0].Source, files[0].Name)
	}
	last := files[len(files)-1]
	if last.Source != CFGSourceValve {
		t.Errorf("last file should be valve, got %s", last.Source)
	}
}
