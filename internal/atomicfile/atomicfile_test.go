package atomicfile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

func TestWrite_CreatesMissingDirectory(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "a", "b", "c", "autoexec.cfg")
	res, err := Write(path, []byte("cl_crosshairgap 0"), Options{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if !res.Created {
		t.Errorf("期望 Created=true，实际 false")
	}
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读回失败: %v", err)
	}
	if string(got) != "cl_crosshairgap 0" {
		t.Errorf("内容不符: %q", string(got))
	}
}

func TestWrite_ReplacesExistingContentWithoutLeavingTempFiles(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "autoexec.cfg")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Write(path, []byte("new"), Options{})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if res.Created {
		t.Errorf("文件已存在时不该报 Created")
	}

	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("内容未替换: %q", string(got))
	}

	entries, _ := os.ReadDir(dir)
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), ".tmp-") {
			t.Errorf("残留临时文件: %s", e.Name())
		}
	}
}

func TestWrite_UnlocksReadOnlyTarget(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "autoexec.cfg")
	if err := os.WriteFile(path, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}

	if _, err := Write(path, []byte("new"), Options{}); err != nil {
		t.Fatalf("只读目标应能写入，实际失败: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("内容未替换: %q", string(got))
	}
}

func TestWriteInPlace_OverwritesInPlaceAndUnlocks(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "cs2_video.txt")
	if err := os.WriteFile(path, []byte("old"), 0o444); err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}

	if _, err := WriteInPlace(path, []byte("new"), Options{}); err != nil {
		t.Fatalf("WriteInPlace failed: %v", err)
	}
	got, _ := os.ReadFile(path)
	if string(got) != "new" {
		t.Errorf("内容未替换: %q", string(got))
	}

	after, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	// 原地覆盖必须保留同一个文件：删掉重建会让监视该文件的进程丢句柄。
	if !os.SameFile(before, after) {
		t.Errorf("WriteInPlace 换掉了文件本体，应该原地覆盖")
	}
}

func TestWrite_BackupKeepsOriginalContent(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "autoexec.cfg")
	if err := os.WriteFile(path, []byte("old"), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Write(path, []byte("new"), Options{Backup: true})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if res.BackupPath == "" {
		t.Fatalf("期望生成备份，实际没有")
	}
	if !strings.Contains(res.BackupPath, ".bak.") {
		t.Errorf("备份文件名不合约定: %s", res.BackupPath)
	}
	got, err := os.ReadFile(res.BackupPath)
	if err != nil {
		t.Fatalf("读备份失败: %v", err)
	}
	if string(got) != "old" {
		t.Errorf("备份内容应是旧值，实际 %q", string(got))
	}
	live, _ := os.ReadFile(path)
	if string(live) != "new" {
		t.Errorf("正本应是新值，实际 %q", string(live))
	}
}

func TestWrite_BackupSkippedWhenTargetMissing(t *testing.T) {
	path := filepath.Join(testutil.TempDir(t), "new.cfg")
	res, err := Write(path, []byte("x"), Options{Backup: true})
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if res.BackupPath != "" {
		t.Errorf("目标不存在时不该备份，实际 %s", res.BackupPath)
	}
}

// 同一秒内连续备份不能互相覆盖：用户可能连点两次保存。
func TestWrite_BackupDoesNotOverwritePreviousBackup(t *testing.T) {
	dir := testutil.TempDir(t)
	path := filepath.Join(dir, "autoexec.cfg")
	if err := os.WriteFile(path, []byte("v1"), 0o644); err != nil {
		t.Fatal(err)
	}

	first, err := Write(path, []byte("v2"), Options{Backup: true})
	if err != nil {
		t.Fatalf("第一次 Write failed: %v", err)
	}
	second, err := Write(path, []byte("v3"), Options{Backup: true})
	if err != nil {
		t.Fatalf("第二次 Write failed: %v", err)
	}

	if first.BackupPath == second.BackupPath {
		t.Fatalf("两次备份写到了同一个文件: %s", first.BackupPath)
	}
	if got, _ := os.ReadFile(first.BackupPath); string(got) != "v1" {
		t.Errorf("第一份备份内容应是 v1，实际 %q", string(got))
	}
	if got, _ := os.ReadFile(second.BackupPath); string(got) != "v2" {
		t.Errorf("第二份备份内容应是 v2，实际 %q", string(got))
	}
}
