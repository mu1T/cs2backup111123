package app

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

// TestDirectoryPickersDoNotPersist 是本轮评审的回归护栏。
//
// 三个「浏览」按钮原来点完就立刻把路径存盘了，用户随后点「取消」也回不去。
// 正确契约是：浏览只把路径交回界面，落盘由「保存」统一完成。
func TestDirectoryPickersDoNotPersist(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	defer svc.WaitBackground()

	pickedBackup := filepath.Join(tempDir, "picked_backup")
	cs2Root := filepath.Join(tempDir, "cs2")
	cs2Cfg := filepath.Join(cs2Root, "game", "csgo", "cfg")
	if err := os.MkdirAll(cs2Cfg, 0o755); err != nil {
		t.Fatal(err)
	}

	svc.SetDirectoryPicker(func(title string) (string, error) {
		switch {
		case strings.Contains(title, "备份"):
			return pickedBackup, nil
		case strings.Contains(title, "CS2"):
			return cs2Root, nil
		default:
			return filepath.Join(tempDir, "steam"), nil
		}
	})

	backupBefore := svc.GetBackupPath()

	if _, err := svc.SelectBackupDirectory(); err != nil {
		t.Fatalf("选择备份目录失败: %v", err)
	}
	if got := svc.GetBackupPath(); got != backupBefore {
		t.Errorf("浏览备份目录不该立即落盘：%q → %q", backupBefore, got)
	}

	if _, err := svc.SelectCS2Directory(); err != nil {
		t.Fatalf("选择 CS2 目录失败: %v", err)
	}
	if got := svc.GetCS2Path(); got != "" {
		t.Errorf("浏览 CS2 目录不该立即落盘，实际 %q", got)
	}
}

// TestSelectCS2DirectoryNormalizes 浏览时仍要归一化并校验，
// 这样选错层级能当场报错，而不是等保存时才发现。
func TestSelectCS2DirectoryNormalizes(t *testing.T) {
	tempDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(tempDir)
	defer svc.WaitBackground()

	cs2Root := filepath.Join(tempDir, "cs2")
	cs2Cfg := filepath.Join(cs2Root, "game", "csgo", "cfg")
	if err := os.MkdirAll(cs2Cfg, 0o755); err != nil {
		t.Fatal(err)
	}
	svc.SetDirectoryPicker(func(string) (string, error) { return cs2Root, nil })

	got, err := svc.SelectCS2Directory()
	if err != nil {
		t.Fatalf("选择失败: %v", err)
	}
	if got != cs2Cfg {
		t.Errorf("应返回归一化后的 cfg 目录 %q，实际 %q", cs2Cfg, got)
	}

	// 选一个无关目录要当场报错
	bad := testutil.TempDir(t)
	svc.SetDirectoryPicker(func(string) (string, error) { return bad, nil })
	if _, err := svc.SelectCS2Directory(); err == nil {
		t.Error("无关目录应当报错")
	}
}
