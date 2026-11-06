package backup

import (
	"bytes"
	"crypto/rand"
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/testutil"
)

func TestCopyFile(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// 1. Small file (< 64KB)
	smallData := []byte("hello world CS2 backup test")
	srcSmall := filepath.Join(tempDir, "small.txt")
	dstSmall := filepath.Join(tempDir, "dst_small.txt")
	if err := os.WriteFile(srcSmall, smallData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(srcSmall, dstSmall); err != nil {
		t.Fatalf("CopyFile small failed: %v", err)
	}
	readSmall, err := os.ReadFile(dstSmall)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(smallData, readSmall) {
		t.Errorf("content mismatch in small file copy")
	}

	// 2. Large file (> 128KB) to exercise sync.Pool buffer looping
	largeData := make([]byte, 200*1024) // 200KB
	_, _ = rand.Read(largeData)

	srcLarge := filepath.Join(tempDir, "sub", "large.bin")
	dstLarge := filepath.Join(tempDir, "target_sub", "large.bin")
	if err := os.MkdirAll(filepath.Dir(srcLarge), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(srcLarge, largeData, 0644); err != nil {
		t.Fatal(err)
	}

	if err := CopyFile(srcLarge, dstLarge); err != nil {
		t.Fatalf("CopyFile large failed: %v", err)
	}
	readLarge, err := os.ReadFile(dstLarge)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(largeData, readLarge) {
		t.Errorf("content mismatch in large file copy")
	}
}

func TestCopyDir(t *testing.T) {
	tempDir := testutil.TempDir(t)

	srcDir := filepath.Join(tempDir, "src_tree")
	dstDir := filepath.Join(tempDir, "dst_tree")

	// Create nested directory structure
	_ = os.MkdirAll(filepath.Join(srcDir, "local", "cfg"), 0755)
	_ = os.WriteFile(filepath.Join(srcDir, "local", "cfg", "cs2_video.txt"), []byte("video settings"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "local", "cfg", "cs2_user_keys.vcfg"), []byte("key binds"), 0644)
	_ = os.WriteFile(filepath.Join(srcDir, "root.txt"), []byte("root file"), 0644)

	if err := CopyDir(srcDir, dstDir); err != nil {
		t.Fatalf("CopyDir failed: %v", err)
	}

	// Verify all files exist in dst
	for _, rel := range []string{
		filepath.Join("local", "cfg", "cs2_video.txt"),
		filepath.Join("local", "cfg", "cs2_user_keys.vcfg"),
		"root.txt",
	} {
		dstFile := filepath.Join(dstDir, rel)
		if _, err := os.Stat(dstFile); err != nil {
			t.Errorf("expected copied file to exist: %s (%v)", dstFile, err)
		}
	}
}
