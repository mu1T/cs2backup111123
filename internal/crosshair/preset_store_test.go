package crosshair

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestStore(t *testing.T) *PresetStore {
	t.Helper()
	store, err := NewPresetStore(tempDir(t))
	if err != nil {
		t.Fatalf("构造预设存储失败: %v", err)
	}
	return store
}

func TestBuiltinPresetsAreProPlayers(t *testing.T) {
	list := BuiltinPresets()
	if len(list) != 16 {
		t.Fatalf("内置预设 %d 个, 期望 16 个", len(list))
	}

	want := []string{
		"ZywOo", "donk", "m0NESY", "kscerato", "sh1ro", "ropz", "molodoy", "spinx",
		"makazze", "NiKo", "kyousuke", "b1t", "torzsi", "910", "flameZ", "YEKINDAR",
	}

	seen := make(map[string]bool, len(list))
	for i, p := range list {
		if p.Name != want[i] {
			t.Errorf("第 %d 个预设 = %q, 期望 %q", i, p.Name, want[i])
		}
		if !p.Builtin {
			t.Errorf("%s: Builtin 应为 true", p.Name)
		}
		if p.ID == "" || p.Description == "" {
			t.Errorf("%s: ID / Description 不能为空", p.Name)
		}
		if seen[p.ID] {
			t.Errorf("预设 ID 重复: %s", p.ID)
		}
		seen[p.ID] = true
	}
}

func TestBuiltinPresetsMatchResearch(t *testing.T) {
	// 数值来自 docs/research/cs2-pro-crosshairs.md §2 的「新版现状」。
	cases := []struct {
		id                     string
		length, thickness, gap int
		height                 int
		color                  RGBA
	}{
		{"pro-zywoo", 2, 2, 2, 768, RGBA{0, 233, 255, 200}},
		{"pro-donk", 2, 2, 1, 960, RGBA{37, 255, 0, 255}},
		{"pro-monesy", 3, 1, 2, 960, RGBA{255, 255, 255, 255}},
		{"pro-kscerato", 1, 2, 1, 864, RGBA{0, 255, 226, 255}},
		{"pro-sh1ro", 2, 1, 1, 960, RGBA{0, 255, 0, 200}},
		{"pro-ropz", 4, 1, 2, 1080, RGBA{0, 255, 0, 255}},
		{"pro-molodoy", 2, 2, 2, 768, RGBA{16, 255, 255, 200}},
		{"pro-spinx", 2, 2, 1, 960, RGBA{0, 255, 0, 255}},
		{"pro-makazze", 2, 2, 1, 960, RGBA{0, 255, 255, 255}},
		{"pro-niko", 2, 2, 2, 960, RGBA{0, 255, 145, 255}},
		{"pro-kyousuke", 2, 2, 1, 960, RGBA{0, 255, 0, 200}},
		{"pro-b1t", 2, 2, 1, 960, RGBA{0, 255, 0, 200}},
		{"pro-torzsi", 2, 2, 1, 960, RGBA{0, 255, 0, 255}},
		{"pro-910", 2, 2, 1, 960, RGBA{255, 255, 255, 200}},
		{"pro-flamez", 3, 1, 2, 960, RGBA{0, 229, 255, 255}},
		{"pro-yekindar", 2, 2, 1, 1050, RGBA{38, 241, 27, 255}},
	}
	for _, tc := range cases {
		p, ok := FindBuiltinPreset(tc.id)
		if !ok {
			t.Errorf("找不到预设 %s", tc.id)
			continue
		}
		c := p.Crosshair
		if c.Length != tc.length || c.Thickness != tc.thickness || c.Gap != tc.gap {
			t.Errorf("%s: 尺寸 %d/%d/%d, 期望 %d/%d/%d",
				tc.id, c.Length, c.Thickness, c.Gap, tc.length, tc.thickness, tc.gap)
		}
		if c.ScreenHeight != tc.height {
			t.Errorf("%s: 参考高 %d, 期望 %d", tc.id, c.ScreenHeight, tc.height)
		}
		if c.Color != tc.color {
			t.Errorf("%s: 颜色 %+v, 期望 %+v", tc.id, c.Color, tc.color)
		}
		// 十六人里只有 flameZ 是射击反馈样式且开中心点，其余都是静态十字、无中心点、无 T 型。
		if tc.id == "pro-flamez" {
			if c.Style != StyleStaticCrossFeedback || !c.Dot {
				t.Errorf("%s: 应为 style 5 + 中心点，实际 style=%d dot=%v", tc.id, c.Style, c.Dot)
			}
		} else {
			if c.Style != StyleStaticCross {
				t.Errorf("%s: style %d, 期望 %d", tc.id, c.Style, StyleStaticCross)
			}
			if c.Dot {
				t.Errorf("%s: dot 应为关闭", tc.id)
			}
		}
		if c.TStyle {
			t.Errorf("%s: T 型应为关闭", tc.id)
		}
	}
}

func TestPresetStoreListsBuiltinsByDefault(t *testing.T) {
	store := newTestStore(t)
	got := store.List()
	if len(got) != 16 {
		t.Fatalf("默认应列出 16 个内置预设, 实际 %d", len(got))
	}
	if hidden := store.HiddenBuiltins(); len(hidden) != 0 {
		t.Errorf("默认不应有隐藏项, 实际 %d", len(hidden))
	}
}

func TestPresetStoreHideAndRestore(t *testing.T) {
	store := newTestStore(t)

	if err := store.SetHidden("pro-zywoo", true); err != nil {
		t.Fatalf("隐藏失败: %v", err)
	}
	if got := store.List(); len(got) != 15 {
		t.Errorf("隐藏后应剩 15 个, 实际 %d", len(got))
	}
	for _, p := range store.List() {
		if p.ID == "pro-zywoo" {
			t.Error("被隐藏的预设不应出现在列表里")
		}
	}
	if hidden := store.HiddenBuiltins(); len(hidden) != 1 || hidden[0].ID != "pro-zywoo" {
		t.Errorf("隐藏列表不正确: %+v", hidden)
	}

	// 重复隐藏应幂等。
	if err := store.SetHidden("pro-zywoo", true); err != nil {
		t.Fatalf("重复隐藏失败: %v", err)
	}
	if got := store.HiddenBuiltins(); len(got) != 1 {
		t.Errorf("重复隐藏产生了 %d 条记录", len(got))
	}

	if err := store.SetHidden("pro-zywoo", false); err != nil {
		t.Fatalf("恢复失败: %v", err)
	}
	if got := store.List(); len(got) != 16 {
		t.Errorf("恢复后应回到 16 个, 实际 %d", len(got))
	}
}

func TestPresetStoreRejectsUnknownBuiltin(t *testing.T) {
	store := newTestStore(t)
	if err := store.SetHidden("pro-nobody", true); err == nil {
		t.Error("对不存在的内置预设设置隐藏应报错")
	}
}

func TestPresetStoreUserPresetLifecycle(t *testing.T) {
	dir := tempDir(t)
	store, err := NewPresetStore(dir)
	if err != nil {
		t.Fatal(err)
	}

	ch := Default()
	ch.Length = 9
	ch.Gap = -2

	created, err := store.AddUser("  我的准星  ", ch)
	if err != nil {
		t.Fatalf("新增用户预设失败: %v", err)
	}
	if created.Name != "我的准星" {
		t.Errorf("名称未裁剪空白: %q", created.Name)
	}
	if created.ID == "" {
		t.Error("ID 不能为空")
	}

	list := store.List()
	if len(list) != 17 {
		t.Fatalf("内置 16 + 用户 1 = 17, 实际 %d", len(list))
	}
	last := list[len(list)-1]
	if last.Builtin {
		t.Error("用户预设 Builtin 应为 false")
	}
	if last.Crosshair.Length != 9 || last.Crosshair.Gap != -2 {
		t.Errorf("用户预设数值未保存: %+v", last.Crosshair)
	}

	// 重新打开应还在（落盘生效）。
	reopened, err := NewPresetStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := reopened.List(); len(got) != 17 {
		t.Errorf("重开后应仍有 17 个, 实际 %d", len(got))
	}

	if err := reopened.DeleteUser(created.ID); err != nil {
		t.Fatalf("删除用户预设失败: %v", err)
	}
	if got := reopened.List(); len(got) != 16 {
		t.Errorf("删除后应剩 16 个, 实际 %d", len(got))
	}
	if err := reopened.DeleteUser(created.ID); err == nil {
		t.Error("删除不存在的预设应报错")
	}
}

func TestPresetStoreRejectsEmptyName(t *testing.T) {
	store := newTestStore(t)
	if _, err := store.AddUser("   ", Default()); err == nil {
		t.Error("空名称应报错")
	}
}

func TestPresetStoreTruncatesLongName(t *testing.T) {
	store := newTestStore(t)
	long := "一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十一二三四五六七八九十"
	got, err := store.AddUser(long, Default())
	if err != nil {
		t.Fatalf("新增失败: %v", err)
	}
	if n := len([]rune(got.Name)); n > 40 {
		t.Errorf("名称长度 %d, 应截到 40 字以内", n)
	}
}

func TestPresetStorePersistsAcrossHiddenAndUser(t *testing.T) {
	dir := tempDir(t)
	store, err := NewPresetStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetHidden("pro-donk", true); err != nil {
		t.Fatal(err)
	}
	if _, err := store.AddUser("测试", Default()); err != nil {
		t.Fatal(err)
	}

	raw, err := os.ReadFile(filepath.Join(dir, "crosshair_presets.json"))
	if err != nil {
		t.Fatalf("预设文件未生成: %v", err)
	}
	if len(raw) == 0 {
		t.Fatal("预设文件为空")
	}

	reopened, err := NewPresetStore(dir)
	if err != nil {
		t.Fatal(err)
	}
	// 16 内置 - 1 隐藏 + 1 用户 = 16
	if got := reopened.List(); len(got) != 16 {
		t.Errorf("重开后应为 16 个, 实际 %d", len(got))
	}
	if got := reopened.HiddenBuiltins(); len(got) != 1 || got[0].ID != "pro-donk" {
		t.Errorf("隐藏状态未持久化: %+v", got)
	}
}

func TestPresetStoreSurvivesCorruptFile(t *testing.T) {
	dir := tempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "crosshair_presets.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, err := NewPresetStore(dir)
	if err != nil {
		t.Fatalf("文件损坏不应导致构造失败: %v", err)
	}
	if got := store.List(); len(got) != 16 {
		t.Errorf("文件损坏时应退回内置预设, 实际 %d 个", len(got))
	}
}
