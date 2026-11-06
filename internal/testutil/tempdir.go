// Package testutil 放测试用的小工具。
package testutil

import (
	"os"
	"strings"
	"testing"
	"time"
)

// TempDir 等价于 t.TempDir()，但清理时会重试。
//
// 为什么需要：Windows 上删掉一个刚被 rename 覆盖过的文件后，目录项会短暂处于
// 「删除挂起」状态——名字已经没了，但 rmdir 仍报 ENOTEMPTY。t.TempDir 的清理
// 是一次性的 os.RemoveAll，撞上这个窗口就报
//
//	TempDir RemoveAll cleanup: unlinkat <dir>: The directory is not empty
//
// 于是测试本身通过了却被判红。内部包里凡是会经 atomicfile 写文件的用例都会
// 撞上，而且频率不低（本机实测 internal/app 约一半）。
//
// 这个窗口只有几毫秒，重试几次就过去了。
func TempDir(t *testing.T) string {
	t.Helper()

	dir, err := os.MkdirTemp("", sanitize(t.Name()))
	if err != nil {
		t.Fatalf("创建临时目录失败: %v", err)
	}

	t.Cleanup(func() {
		if err := RemoveAll(dir); err != nil {
			t.Logf("清理临时目录 %s 失败: %v", dir, err)
		}
	})
	return dir
}

// RemoveAll 删除目录，对 Windows 的删除挂起做退避重试。
func RemoveAll(dir string) error {
	var err error
	delay := 5 * time.Millisecond
	for attempt := 0; attempt < 10; attempt++ {
		if err = os.RemoveAll(dir); err == nil {
			return nil
		}
		time.Sleep(delay)
		delay *= 2
	}
	return err
}

func sanitize(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", " ", "_")
	return replacer.Replace(name)
}
