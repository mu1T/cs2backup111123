package crosshair

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func testCrosshair() Crosshair {
	c := Default()
	c.Style = StyleStaticCross
	c.Length = 7
	c.Thickness = 3
	c.Gap = 12
	c.Dot = true
	c.Color = RGBA{208, 77, 6, 255}
	c.OutlineMode = OutlineHalf
	c.ScreenHeight = 1024
	return c
}

func TestParseCFGReadsExistingValues(t *testing.T) {
	text := strings.Join([]string{
		"fps_max 0",
		"cl_crosshairstyle 4",
		"cl_crosshair_length 2 // 手写备注",
		"cl_crosshair_thickness 2",
		"cl_crosshair_gap -3",
		"cl_crosshaircolor_r 208",
		"cl_crosshairdot 1",
		"crosshair 0",
		"sensitivity 1.25",
	}, "\n")

	got := ParseCFG(text)

	if got.Crosshair.Style != 4 {
		t.Errorf("style = %d, 期望 4", got.Crosshair.Style)
	}
	if got.Crosshair.Length != 2 {
		t.Errorf("length = %d, 期望 2", got.Crosshair.Length)
	}
	if got.Crosshair.Gap != -3 {
		t.Errorf("gap = %d, 期望 -3", got.Crosshair.Gap)
	}
	if got.Crosshair.Color.R != 208 {
		t.Errorf("color.r = %d, 期望 208", got.Crosshair.Color.R)
	}
	if !got.Crosshair.Dot {
		t.Error("dot 应为 true")
	}
	if !got.DisplayTogglePresent || got.DisplayToggle != "0" {
		t.Errorf("crosshair 开关 = %q (present=%v), 期望 \"0\"", got.DisplayToggle, got.DisplayTogglePresent)
	}
	// 文件里没写的键要进 MissingKeys，UI 才知道哪些是默认值。
	if len(got.MissingKeys) == 0 {
		t.Error("MissingKeys 不应为空")
	}
	for _, k := range got.MissingKeys {
		if k == "cl_crosshairstyle" {
			t.Error("已存在的键不应出现在 MissingKeys 里")
		}
	}
}

func TestParseCFGIgnoresDeletedConvars(t *testing.T) {
	// 旧键与只差下划线的隐藏键都不应污染解析结果。
	text := "cl_crosshair_gap 12\ncl_crosshairgap 999\ncl_crosshairsize 9.9\ncl_crosshairthickness 5\n"
	got := ParseCFG(text)
	if got.Crosshair.Gap != 12 {
		t.Errorf("gap = %d, 期望 12（旧键 cl_crosshairgap 应被忽略）", got.Crosshair.Gap)
	}
	if got.Crosshair.Thickness != Default().Thickness {
		t.Errorf("thickness = %d, 期望保持默认 %d", got.Crosshair.Thickness, Default().Thickness)
	}
}

func TestApplyToCFGReplacesInPlace(t *testing.T) {
	text := "fps_max 0\ncl_crosshairstyle 1\nsensitivity 1.25\n"
	out, report := ApplyToCFG(text, testCrosshair(), "CSxxx")

	if !strings.Contains(out, "cl_crosshairstyle 4") {
		t.Errorf("cl_crosshairstyle 未被替换，输出:\n%s", out)
	}
	if strings.Contains(out, "cl_crosshairstyle 1") {
		t.Errorf("旧值残留，输出:\n%s", out)
	}
	// 原位替换：fps_max 仍在第一行，sensitivity 仍在 crosshairstyle 之后。
	lines := strings.Split(out, "\n")
	if lines[0] != "fps_max 0" {
		t.Errorf("第一行被破坏: %q", lines[0])
	}
	if report.PreservedToggle {
		t.Error("文件里没有 crosshair 开关时不应报告 PreservedToggle")
	}
}

func TestApplyToCFGKeepsDisplayToggleUntouched(t *testing.T) {
	for _, toggle := range []string{"crosshair 0", "crosshair 1", "crosshair false", "crosshair true"} {
		text := "cl_crosshairstyle 1\n" + toggle + "\n"
		out, report := ApplyToCFG(text, testCrosshair(), "")

		if !strings.Contains(out, toggle) {
			t.Errorf("%q 未原样保留，输出:\n%s", toggle, out)
		}
		if !report.PreservedToggle {
			t.Errorf("%q: PreservedToggle 应为 true", toggle)
		}
		// 工具不得自己生成 crosshair 行。
		count := 0
		for _, line := range strings.Split(out, "\n") {
			key, _, ok := splitFields(stripComment(line))
			if ok && key == DisplayToggleKey {
				count++
			}
		}
		if count != 1 {
			t.Errorf("%q: crosshair 行出现 %d 次，期望 1 次", toggle, count)
		}
	}
}

func TestApplyToCFGNeverInventsDisplayToggle(t *testing.T) {
	out, _ := ApplyToCFG("fps_max 0\n", testCrosshair(), "")
	for _, line := range strings.Split(out, "\n") {
		key, _, ok := splitFields(stripComment(line))
		if ok && key == DisplayToggleKey {
			t.Fatalf("文件里原本没有 crosshair 开关，工具不应写入:\n%s", out)
		}
	}
}

// sizeConvarKeys 是会被 cl_crosshair_screen_height 记录影响的尺寸类指令。
// 官方对 screen_height 的描述是「changes whenever a size setting is updated」，
// 所以它必须排在所有尺寸类指令之后。
var sizeConvarKeys = map[string]bool{
	"cl_crosshair_length":    true,
	"cl_crosshair_thickness": true,
	"cl_crosshair_gap":       true,
}

func TestApplyToCFGScreenHeightComesLast(t *testing.T) {
	// 用户把 screen_height 写在尺寸类指令之前，工具必须把它挪到最后。
	text := "cl_crosshair_screen_height 1024\ncl_crosshair_length 2\ncl_crosshair_thickness 2\ncl_crosshair_gap 3\n"
	out, _ := ApplyToCFG(text, testCrosshair(), "")

	lines := strings.Split(out, "\n")
	lastScreenHeight := -1
	for i, line := range lines {
		key, _, ok := splitFields(stripComment(line))
		if !ok {
			continue
		}
		if key == "cl_crosshair_screen_height" {
			lastScreenHeight = i
		}
		if sizeConvarKeys[key] && lastScreenHeight >= 0 {
			t.Fatalf("%s 出现在 cl_crosshair_screen_height 之后，顺序错误:\n%s", key, out)
		}
	}
	if lastScreenHeight < 0 {
		t.Fatalf("没有写出 cl_crosshair_screen_height:\n%s", out)
	}
	// 生效的那一行必须是用户设置的值。
	if got := lines[lastScreenHeight]; !strings.Contains(got, "1024") {
		t.Errorf("生效的 screen_height 行 = %q, 期望含 1024", got)
	}
}

func TestApplyToCFGCommentsDeletedConvars(t *testing.T) {
	text := "cl_crosshairgap 2\ncl_crosshaircolor 5\ncl_crosshairsize 3.9\n"
	out, report := ApplyToCFG(text, testCrosshair(), "")

	for _, gone := range []string{"cl_crosshairgap 2", "cl_crosshaircolor 5", "cl_crosshairsize 3.9"} {
		// 原文必须只以注释形式存在，不能还是生效行。
		for _, line := range strings.Split(out, "\n") {
			if strings.TrimSpace(line) == gone {
				t.Errorf("旧键 %q 仍是生效行:\n%s", gone, out)
			}
		}
	}
	if !strings.Contains(out, "已废弃（CS2 2026-09 删除）") {
		t.Errorf("缺少废弃说明注释:\n%s", out)
	}
	if len(report.CommentedKeys) != 3 {
		t.Errorf("CommentedKeys = %v, 期望 3 项", report.CommentedKeys)
	}
}

func TestApplyToCFGIsIdempotent(t *testing.T) {
	text := "fps_max 0\ncl_crosshairstyle 1\ncrosshair 1\n"
	ch := testCrosshair()

	first, _ := ApplyToCFG(text, ch, "CSxxx")
	second, _ := ApplyToCFG(first, ch, "CSxxx")

	if first != second {
		t.Errorf("重复写入不稳定：\n第一次:\n%s\n第二次:\n%s", first, second)
	}
	// 区块只能有一份。
	if n := strings.Count(second, blockStartMarker); n != 1 {
		t.Errorf("区块出现 %d 次，期望 1 次:\n%s", n, second)
	}
	if n := strings.Count(second, blockEndMarker); n != 1 {
		t.Errorf("结束标记出现 %d 次，期望 1 次:\n%s", n, second)
	}
}

func TestApplyToCFGUpdatesBlockOnSecondWrite(t *testing.T) {
	text := "fps_max 0\n"
	first, _ := ApplyToCFG(text, testCrosshair(), "CSaaa")

	changed := testCrosshair()
	changed.Length = 20
	second, _ := ApplyToCFG(first, changed, "CSbbb")

	if !strings.Contains(second, "cl_crosshair_length 20") {
		t.Errorf("第二次写入未更新长度:\n%s", second)
	}
	if strings.Contains(second, "CSaaa") {
		t.Errorf("旧分享码残留:\n%s", second)
	}
	if !strings.Contains(second, "CSbbb") {
		t.Errorf("新分享码缺失:\n%s", second)
	}
}

func TestApplyToCFGPreservesCRLFAndBOM(t *testing.T) {
	text := "\ufefffps_max 0\r\ncl_crosshairstyle 1\r\n"
	out, _ := ApplyToCFG(text, testCrosshair(), "")

	if !strings.HasPrefix(out, "\ufeff") {
		t.Error("BOM 丢失")
	}
	if !strings.Contains(out, "\r\n") {
		t.Error("CRLF 换行风格丢失")
	}
	if strings.Contains(strings.ReplaceAll(out, "\r\n", ""), "\n") {
		t.Error("混入了裸 LF")
	}
}

func TestApplyToCFGKeepsTrailingNewlineAbsent(t *testing.T) {
	out, _ := ApplyToCFG("fps_max 0", testCrosshair(), "")
	if strings.HasSuffix(out, "\n") {
		t.Error("原文件没有结尾换行，不应凭空添加")
	}
}

func TestApplyToCFGHandlesEmptyFile(t *testing.T) {
	out, report := ApplyToCFG("", testCrosshair(), "")
	if strings.HasPrefix(out, "\n") {
		t.Errorf("空文件不应以空行开头:\n%q", out)
	}
	if len(report.AppendedKeys) == 0 {
		t.Error("空文件应把全部键追加进区块")
	}
	if !strings.Contains(out, "cl_crosshairstyle 4") {
		t.Errorf("输出缺少样式:\n%s", out)
	}
}

func TestApplyToCFGCommentsDuplicateLines(t *testing.T) {
	text := "cl_crosshair_length 2\ncl_crosshair_length 9\n"
	out, report := ApplyToCFG(text, testCrosshair(), "")

	active := 0
	for _, line := range strings.Split(out, "\n") {
		key, _, ok := splitFields(stripComment(line))
		if ok && key == "cl_crosshair_length" {
			active++
		}
	}
	if active != 1 {
		t.Errorf("生效的 cl_crosshair_length 有 %d 行，期望 1 行:\n%s", active, out)
	}
	found := false
	for _, k := range report.CommentedKeys {
		if k == "cl_crosshair_length" {
			found = true
		}
	}
	if !found {
		t.Errorf("重复行未记入 CommentedKeys: %v", report.CommentedKeys)
	}
}

func TestRenderCFG(t *testing.T) {
	out := RenderCFG(testCrosshair(), "CSabc", "2026-11-06")

	if !strings.Contains(out, "cl_crosshairstyle 4") {
		t.Errorf("缺少样式:\n%s", out)
	}
	if strings.Contains(out, "\ncrosshair ") {
		t.Errorf("导出的 crosshair.cfg 不应含显示开关:\n%s", out)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	last := lines[len(lines)-1]
	if !strings.HasPrefix(last, "cl_crosshair_screen_height ") {
		t.Errorf("最后一行应为 screen_height，实际 %q", last)
	}
	if !strings.Contains(out, "CSabc") {
		t.Error("缺少分享码注释")
	}
}

func TestRenderConsoleIsSingleLine(t *testing.T) {
	out := RenderConsole(testCrosshair())
	if strings.Contains(out, "\n") {
		t.Error("控制台指令不应含换行")
	}
	if !strings.Contains(out, "cl_crosshairstyle 4") {
		t.Error("缺少样式")
	}
	if !strings.HasSuffix(out, "cl_crosshair_screen_height 1024") {
		t.Errorf("screen_height 应在最后，实际结尾: %q", out[max(0, len(out)-60):])
	}
}

func TestConvarsExcludeDisplayToggle(t *testing.T) {
	for _, c := range Default().Convars() {
		if c.Key == DisplayToggleKey {
			t.Fatal("Convars 不应包含 crosshair 显示开关")
		}
	}
}

func TestConvarsScreenHeightLast(t *testing.T) {
	list := Default().Convars()
	if list[len(list)-1].Key != "cl_crosshair_screen_height" {
		t.Errorf("最后一项 = %s, 期望 cl_crosshair_screen_height", list[len(list)-1].Key)
	}
}

func TestServiceSaveCreatesFileAndBacksUp(t *testing.T) {
	dir := tempDir(t)
	svc := NewService(dir)

	// 第一次保存：文件不存在 → 新建，不产生备份。
	res, err := svc.Save(testCrosshair(), SaveOptions{Backup: true})
	if err != nil {
		t.Fatalf("首次保存失败: %v", err)
	}
	if !res.Created {
		t.Error("首次保存应标记 Created")
	}
	if res.BackupPath != "" {
		t.Errorf("首次保存不应产生备份，实际 %s", res.BackupPath)
	}
	if res.ShareCode == "" {
		t.Error("应返回分享码")
	}

	// 第二次保存：文件已存在 → 产生备份。
	res2, err := svc.Save(testCrosshair(), SaveOptions{Backup: true})
	if err != nil {
		t.Fatalf("二次保存失败: %v", err)
	}
	if res2.Created {
		t.Error("二次保存不应标记 Created")
	}
	if res2.BackupPath == "" {
		t.Fatal("二次保存应产生备份")
	}
	if _, statErr := os.Stat(res2.BackupPath); statErr != nil {
		t.Errorf("备份文件不存在: %v", statErr)
	}
}

func TestServiceLoadReportsMissingFile(t *testing.T) {
	svc := NewService(tempDir(t))
	st, err := svc.Load()
	if err != nil {
		t.Fatalf("文件不存在不应报错: %v", err)
	}
	if st.Exists {
		t.Error("Exists 应为 false")
	}
	if len(st.Parsed.MissingKeys) == 0 {
		t.Error("全部键都应标记为缺失")
	}
	if st.ShareCode == "" {
		t.Error("应给出默认设置的分享码")
	}
}

func TestServiceRoundTrip(t *testing.T) {
	dir := tempDir(t)
	svc := NewService(dir)

	want := testCrosshair()
	if _, err := svc.Save(want, SaveOptions{}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}
	st, err := svc.Load()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	got := st.Parsed.Crosshair

	if got.Style != want.Style || got.Length != want.Length || got.Thickness != want.Thickness {
		t.Errorf("尺寸字段不一致: got %d/%d/%d, want %d/%d/%d",
			got.Style, got.Length, got.Thickness, want.Style, want.Length, want.Thickness)
	}
	if got.Gap != want.Gap {
		t.Errorf("gap = %d, 期望 %d", got.Gap, want.Gap)
	}
	if got.Color != want.Color {
		t.Errorf("color = %+v, 期望 %+v", got.Color, want.Color)
	}
	if got.OutlineMode != want.OutlineMode {
		t.Errorf("outline_mode = %d, 期望 %d", got.OutlineMode, want.OutlineMode)
	}
	if got.ScreenHeight != want.ScreenHeight {
		t.Errorf("screen_height = %d, 期望 %d", got.ScreenHeight, want.ScreenHeight)
	}
	if !got.Dot {
		t.Error("dot 丢失")
	}
}

func TestServicePreservesUserContent(t *testing.T) {
	dir := tempDir(t)
	svc := NewService(dir)

	original := "fps_max 0\nbind mwheeldown +jump\nsensitivity 1.25\nexec practice.cfg\n"
	if err := os.WriteFile(svc.AutoexecPath(), []byte(original), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := svc.Save(testCrosshair(), SaveOptions{}); err != nil {
		t.Fatalf("保存失败: %v", err)
	}

	data, err := os.ReadFile(svc.AutoexecPath())
	if err != nil {
		t.Fatal(err)
	}
	out := string(data)
	for _, keep := range []string{"fps_max 0", "bind mwheeldown +jump", "sensitivity 1.25", "exec practice.cfg"} {
		if !strings.Contains(out, keep) {
			t.Errorf("用户内容被破坏，缺少 %q:\n%s", keep, out)
		}
	}
}

func TestServiceExportCFG(t *testing.T) {
	dir := tempDir(t)
	svc := NewService(dir)

	path, err := svc.ExportCFG(testCrosshair(), "2026-11-06")
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if filepath.Base(path) != ExportFileName {
		t.Errorf("文件名 = %s, 期望 %s", filepath.Base(path), ExportFileName)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "cl_crosshairstyle 4") {
		t.Errorf("导出内容不正确:\n%s", data)
	}
	// 导出不应顺手创建 autoexec.cfg。
	if _, statErr := os.Stat(svc.AutoexecPath()); !os.IsNotExist(statErr) {
		t.Error("导出 crosshair.cfg 不应创建 autoexec.cfg")
	}
}

func TestServiceRequiresCFGDir(t *testing.T) {
	svc := NewService("  ")
	if _, err := svc.Load(); err == nil {
		t.Error("cfg 目录为空时 Load 应报错")
	}
	if _, err := svc.Save(testCrosshair(), SaveOptions{}); err == nil {
		t.Error("cfg 目录为空时 Save 应报错")
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// TestJSONNeverEmitsNullArrays 是本轮 bug 的回归护栏。
//
// Go 的 nil 切片会被 encoding/json 写成 null，前端按数组用就会抛
// "Cannot read properties of null (reading 'length')"，把整个弹窗渲染搞崩。
// 所以这两个结构体里所有对外暴露的切片都必须是空切片。
func TestJSONNeverEmitsNullArrays(t *testing.T) {
	assertNoNullArrays := func(t *testing.T, label string, v any) {
		t.Helper()
		raw, err := json.Marshal(v)
		if err != nil {
			t.Fatalf("%s: 序列化失败: %v", label, err)
		}
		if strings.Contains(string(raw), ":null") {
			t.Errorf("%s 序列化出了 null：%s", label, raw)
		}
	}

	// 全部键都存在（用户保存过准星后的常态）：missing_keys 会为空
	full := RenderCFG(testCrosshair(), "", "")
	parsed := ParseCFG(full)
	if len(parsed.MissingKeys) != 0 {
		t.Fatalf("前提不成立：应当没有缺失键，实际 %v", parsed.MissingKeys)
	}
	assertNoNullArrays(t, "ParsedCFG（全键存在）", parsed)
	assertNoNullArrays(t, "ParsedCFG（空文件）", ParseCFG(""))

	// 写入时没有任何替换 / 注释：两张清单会为空
	_, report := ApplyToCFG("fps_max 0\n", Default(), "")
	if len(report.ReplacedKeys) != 0 || len(report.CommentedKeys) != 0 {
		t.Fatalf("前提不成立：不该有替换或注释")
	}
	assertNoNullArrays(t, "WriteReport", report)

	// 命令预览
	assertNoNullArrays(t, "CommandPreview", CommandPreview{
		Lines:   Default().Convars(),
		Console: RenderConsole(Default()),
		Share:   Encode(Default()),
	})
}

// TestRenderCFGContainsEveryConvar 防的是「导出文件缺键」这类静默缺陷：
// 少写一条不会报错，但用户 exec 之后准星就是不对。
func TestRenderCFGContainsEveryConvar(t *testing.T) {
	ch := testCrosshair()
	out := RenderCFG(ch, "", "")

	for _, c := range ch.Convars() {
		if !strings.Contains(out, c.Key+" "+c.Value) {
			t.Errorf("导出的 crosshair.cfg 缺少 %s %s", c.Key, c.Value)
		}
	}
	if strings.Contains(out, "\ncrosshair ") {
		t.Error("导出的 crosshair.cfg 不应含显示开关")
	}
	// screen_height 必须最后
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if !strings.HasPrefix(lines[len(lines)-1], "cl_crosshair_screen_height ") {
		t.Errorf("最后一行应为 screen_height，实际 %q", lines[len(lines)-1])
	}
}

// TestServiceLoadMissingFileHasNoNullArrays 覆盖「autoexec.cfg 还不存在」这条路径：
// 那条分支是自己拼 ParsedCFG 的，很容易漏掉切片初始化。
func TestServiceLoadMissingFileHasNoNullArrays(t *testing.T) {
	svc := NewService(tempDir(t))
	st, err := svc.Load()
	if err != nil {
		t.Fatalf("读取失败: %v", err)
	}
	if st.Exists {
		t.Fatal("前提不成立：文件不该存在")
	}
	raw, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}
	if strings.Contains(string(raw), ":null") {
		t.Errorf("文件不存在路径序列化出了 null：%s", raw)
	}
}

// TestServiceSaveTwiceIsByteIdentical 是「写两次结果一致」的真回归护栏。
//
// 原来区块标题里塞了 time.Now()，跨分钟保存文本就会变——直接调 ApplyToCFG 的
// 老测试传固定字符串，压根盖不到这条路径，必须走 Service.Save 才测得出来。
func TestServiceSaveTwiceIsByteIdentical(t *testing.T) {
	dir := tempDir(t)
	svc := NewService(dir)

	if _, err := svc.Save(testCrosshair(), SaveOptions{Backup: true}); err != nil {
		t.Fatalf("第一次保存失败: %v", err)
	}
	first, err := os.ReadFile(svc.AutoexecPath())
	if err != nil {
		t.Fatal(err)
	}

	// 跨过分钟边界：旧实现会把新的时间戳写进标题
	time.Sleep(1100 * time.Millisecond)

	if _, err := svc.Save(testCrosshair(), SaveOptions{Backup: true}); err != nil {
		t.Fatalf("第二次保存失败: %v", err)
	}
	second, err := os.ReadFile(svc.AutoexecPath())
	if err != nil {
		t.Fatal(err)
	}

	if string(first) != string(second) {
		t.Errorf("两次保存结果不一致：\n第一次:\n%s\n第二次:\n%s", first, second)
	}
}

// TestApplyToCFGDoesNotReportScreenHeightAsAppendedEveryTime 防的是
// 「每次保存都报新增 1 项」——screen_height 永远由区块末尾产出，
// 只有在原文件里本来没有时才该算新增。
func TestApplyToCFGDoesNotReportScreenHeightAsAppendedEveryTime(t *testing.T) {
	ch := testCrosshair()

	// 第一次：文件里没有 → 算新增
	first, firstReport := ApplyToCFG("fps_max 0\n", ch, "")
	if !contains(firstReport.AppendedKeys, "cl_crosshair_screen_height") {
		t.Errorf("首次写入应把 screen_height 记为新增，实际 %v", firstReport.AppendedKeys)
	}

	// 第二次：文件里已经有了（第一次写的）→ 不该再算新增
	_, secondReport := ApplyToCFG(first, ch, "")
	if contains(secondReport.AppendedKeys, "cl_crosshair_screen_height") {
		t.Errorf("再次写入不该把 screen_height 记为新增，实际 %v", secondReport.AppendedKeys)
	}
	if len(secondReport.AppendedKeys) != 0 || len(secondReport.ReplacedKeys) != 0 {
		t.Errorf("同一份设置再写一次应当没有任何改动，实际 追加=%v 替换=%v",
			secondReport.AppendedKeys, secondReport.ReplacedKeys)
	}
}

func contains(list []string, want string) bool {
	for _, v := range list {
		if v == want {
			return true
		}
	}
	return false
}
