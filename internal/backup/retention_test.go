package backup

import (
	"fmt"
	"testing"
)

// v 造一个只带 ID 的版本，保留逻辑只关心顺序，不关心内容。
func v(id string) SnapshotInfo { return SnapshotInfo{ID: id} }

// vn 造 n 个版本，按「新 → 旧」排：v5, v4, v3, v2, v1。
func vn(n int) []SnapshotInfo {
	out := make([]SnapshotInfo, 0, n)
	for i := n; i >= 1; i-- {
		out = append(out, v(fmt.Sprintf("v%d", i)))
	}
	return out
}

func ids(versions []SnapshotInfo) []string { return VersionIDs(versions) }

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func TestSelectVersionsToKeep(t *testing.T) {
	tests := []struct {
		name        string
		count       int
		keep        int
		wantKept    []string
		wantDropped []string
	}{
		{
			name:        "0 个版本：什么都不删",
			count:       0,
			keep:        10,
			wantKept:    []string{},
			wantDropped: nil,
		},
		{
			name:        "1 个版本，上限 10：留着",
			count:       1,
			keep:        10,
			wantKept:    []string{"v1"},
			wantDropped: nil,
		},
		{
			name:        "恰好等于上限：一个都不删",
			count:       10,
			keep:        10,
			wantKept:    []string{"v10", "v9", "v8", "v7", "v6", "v5", "v4", "v3", "v2", "v1"},
			wantDropped: nil,
		},
		{
			name:        "上限 + 1：删最旧的 1 个",
			count:       11,
			keep:        10,
			wantKept:    []string{"v11", "v10", "v9", "v8", "v7", "v6", "v5", "v4", "v3", "v2"},
			wantDropped: []string{"v1"},
		},
		{
			name:        "远超上限：只留最近 10 个",
			count:       15,
			keep:        10,
			wantKept:    []string{"v15", "v14", "v13", "v12", "v11", "v10", "v9", "v8", "v7", "v6"},
			wantDropped: []string{"v5", "v4", "v3", "v2", "v1"},
		},
		{
			name:        "上限为 0：不清理，全部保留",
			count:       5,
			keep:        0,
			wantKept:    []string{"v5", "v4", "v3", "v2", "v1"},
			wantDropped: nil,
		},
		{
			name:        "上限为负：同样不清理",
			count:       3,
			keep:        -1,
			wantKept:    []string{"v3", "v2", "v1"},
			wantDropped: nil,
		},
		{
			name:        "上限为 1：只留最新一个",
			count:       4,
			keep:        1,
			wantKept:    []string{"v4"},
			wantDropped: []string{"v3", "v2", "v1"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kept, dropped := SelectVersionsToKeep(vn(tt.count), tt.keep)

			if got := ids(kept); !equal(got, tt.wantKept) {
				t.Errorf("kept = %v, want %v", got, tt.wantKept)
			}
			if got := ids(dropped); !equal(got, tt.wantDropped) {
				t.Errorf("dropped = %v, want %v", got, tt.wantDropped)
			}

			// 不变量：留下 + 丢掉 = 原来的全部，不重不漏
			if len(kept)+len(dropped) != tt.count {
				t.Errorf("kept(%d) + dropped(%d) != 原始数量 %d", len(kept), len(dropped), tt.count)
			}
		})
	}
}

/**
 * 上限为 0 时「不清理」是刻意的：上限被误设成 0 时删光所有历史是灾难性的，
 * 而什么都不删最多只是占点磁盘。要清空历史应该走显式删除。
 */
func TestSelectVersionsToKeep_ZeroMeansKeepAll(t *testing.T) {
	kept, dropped := SelectVersionsToKeep(vn(20), 0)
	if len(kept) != 20 || len(dropped) != 0 {
		t.Errorf("上限 0 应全部保留，实际 kept=%d dropped=%d", len(kept), len(dropped))
	}
}

/** VersionIDs 对空输入返回空切片而不是 nil——前端拿到 null 会崩。 */
func TestVersionIDs_EmptyReturnsEmptySlice(t *testing.T) {
	if got := VersionIDs(nil); got == nil {
		t.Error("空输入应返回空切片，不能是 nil")
	}
}
