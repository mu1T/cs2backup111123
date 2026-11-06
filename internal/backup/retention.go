package backup

import (
	"os"
	"path/filepath"
	"sort"
)

// DefaultRetentionLimit 是每个账号（以及通用 CFG）默认保留的历史版本数。
const DefaultRetentionLimit = 10

// SelectVersionsToKeep 决定哪些历史版本留下、哪些该删。
//
// versions 必须按「新 → 旧」排好（manifest 就是这么存的，新版本插在头部）。
// 返回的 kept 与 dropped 都保持输入顺序。
//
// keep <= 0 视为「不清理」，全部保留。这是个有意的选择：上限被误设成 0 时，
// 删光所有历史是灾难性的，而「什么都不删」最多只是占点磁盘。要清空历史应该
// 走显式的删除操作，不该是设错一个数字的副作用。
//
// 这个函数不碰磁盘、不看时间戳格式——只按位置切。所以它的测试是毫秒级的，
// 而它恰好是整个保留逻辑里唯一有分支的地方。
func SelectVersionsToKeep(versions []SnapshotInfo, keep int) (kept, dropped []SnapshotInfo) {
	if keep <= 0 || len(versions) <= keep {
		return versions, nil
	}
	return versions[:keep], versions[keep:]
}

// VersionIDs 抽出 ID 列表，用于日志与提示文案。
func VersionIDs(versions []SnapshotInfo) []string {
	if len(versions) == 0 {
		return []string{}
	}
	out := make([]string, 0, len(versions))
	for _, v := range versions {
		out = append(out, v.ID)
	}
	return out
}

// selectNamesToKeep 与 SelectVersionsToKeep 同样的判定，只是输入是名字列表。
//
// names 必须按「新 → 旧」排好。通用 CFG 的历史版本没有 manifest，只有目录名，
// 所以复用同一个位置切法而不是再写一遍。
func selectNamesToKeep(names []string, keep int) (kept, dropped []string) {
	versions := make([]SnapshotInfo, len(names))
	for i, n := range names {
		versions[i] = SnapshotInfo{ID: n}
	}
	k, d := SelectVersionsToKeep(versions, keep)
	return VersionIDs(k), VersionIDs(d)
}

// pruneVersionDirs 按目录名（时间戳）倒序保留最近 keep 个，返回被删的目录名。
//
// 目录名是 `YYYYMMDD_HHMMSS` 格式，字典序即时间序，所以不用解析时间。
func pruneVersionDirs(root string, keep int) ([]string, error) {
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))

	_, dropped := selectNamesToKeep(names, keep)
	for _, name := range dropped {
		_ = os.RemoveAll(filepath.Join(root, name))
	}
	return dropped, nil
}
