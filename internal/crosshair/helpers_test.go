package crosshair

import (
	"testing"

	"cs2backup/internal/testutil"
)

// tempDir 是本包的临时目录工厂。
//
// 清理必须带重试：Windows 上删掉刚被 rename 覆盖过的文件后，目录项会短暂处于
// 「删除挂起」状态，一次性的 os.RemoveAll 会拿到 ENOTEMPTY，表现为随机红的
// flake。testutil.TempDir 就是那个带重试的版本。
func tempDir(t *testing.T) string {
	t.Helper()
	return testutil.TempDir(t)
}
