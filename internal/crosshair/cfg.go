package crosshair

import (
	"strconv"
	"strings"
)

// blockStartMarker / blockEndMarker 是工具自管的准星区块边界。
//
// 有显式结束标记，才能在下一次写入时精确删掉旧区块再重建，
// 否则每次保存都会在文件末尾堆一截。
const (
	blockStartMarker = "// ===== 准星设置（由 CS2 配置备份工具生成"
	blockEndMarker   = "// ===== 准星设置结束 ====="
	managedPrefix    = "// [准星] "
)

// ParsedCFG 是从一份 autoexec.cfg 文本里读到的准星现状。
type ParsedCFG struct {
	// Crosshair 是解析出的准星；文件里没有的键保留 Default 的值。
	Crosshair Crosshair `json:"crosshair"`
	// FoundKeys 是文件里实际出现的准星键。
	FoundKeys []string `json:"found_keys"`
	// MissingKeys 是文件里缺失、当前用默认值顶上的键。
	MissingKeys []string `json:"missing_keys"`
	// DisplayToggle 是 crosshair 开关的原始值（如 "1" / "0" / "true"）。
	DisplayToggle string `json:"display_toggle"`
	// DisplayTogglePresent 表示文件里是否存在 crosshair 开关。
	DisplayTogglePresent bool `json:"display_toggle_present"`
}

// WriteReport 描述一次写入到底改了什么，供前端展示与日志。
type WriteReport struct {
	// ReplacedKeys 是在原位置直接改值的键。
	ReplacedKeys []string `json:"replaced_keys"`
	// AppendedKeys 是文件里原本没有、追加到区块里的键。
	AppendedKeys []string `json:"appended_keys"`
	// CommentedKeys 是被注释掉的旧键或重复行。
	CommentedKeys []string `json:"commented_keys"`
	// DisplayToggle 是保留下来的准星开关值，空串表示文件里没有这个键。
	DisplayToggle string `json:"display_toggle"`
	// PreservedToggle 表示 crosshair 开关是否被原样保留。
	PreservedToggle bool `json:"preserved_toggle"`
}

// OwnedKeys 返回工具接管的全部键名：现行指令 + 已删除旧键 + 隐藏遗留键。
func OwnedKeys() map[string]bool {
	owned := make(map[string]bool, 40)
	for _, c := range Default().Convars() {
		owned[c.Key] = true
	}
	for k := range DeletedConvars {
		owned[k] = true
	}
	for k := range LegacyHiddenConvars {
		owned[k] = true
	}
	return owned
}

// stripComment 去掉行内 `//` 注释并裁剪空白。
func stripComment(line string) string {
	if idx := strings.Index(line, "//"); idx >= 0 {
		line = line[:idx]
	}
	return strings.TrimSpace(line)
}

// splitFields 把一行 cfg 拆成「键 + 其余部分」。
func splitFields(line string) (key, rest string, ok bool) {
	fields := strings.Fields(line)
	if len(fields) == 0 {
		return "", "", false
	}
	key = CanonicalKey(fields[0])
	rest = strings.Join(fields[1:], " ")
	return key, rest, true
}

// ParseCFG 从 autoexec.cfg 文本里读出准星现状。
//
// 缺失的键用 Default() 顶上，并通过 MissingKeys 告知调用方，避免 UI 显示假数据
// 却不说清楚。
func ParseCFG(text string) ParsedCFG {
	// 切片一律初始化成空切片而不是留 nil：nil 会被 encoding/json 序列化成 null，
	// 前端拿 `null.length` 会直接抛异常，把整个弹窗的渲染搞崩。
	out := ParsedCFG{Crosshair: Default(), FoundKeys: []string{}, MissingKeys: []string{}}
	seen := make(map[string]bool)
	owned := OwnedKeys()

	for _, raw := range splitLines(text) {
		key, rest, ok := splitFields(stripComment(raw))
		if !ok {
			continue
		}
		if key == DisplayToggleKey {
			out.DisplayToggle = rest
			out.DisplayTogglePresent = true
			continue
		}
		if !owned[key] {
			continue
		}
		// 已删除 / 隐藏的旧键不参与解析，否则会用旧值覆盖新值。
		if _, gone := DeletedConvars[key]; gone {
			continue
		}
		if _, hidden := LegacyHiddenConvars[key]; hidden {
			continue
		}
		if !applyConvar(&out.Crosshair, key, rest) {
			continue
		}
		if !seen[key] {
			seen[key] = true
			out.FoundKeys = append(out.FoundKeys, key)
		}
	}

	for _, c := range out.Crosshair.Convars() {
		if !seen[c.Key] {
			out.MissingKeys = append(out.MissingKeys, c.Key)
		}
	}
	out.Crosshair.Normalize()
	return out
}

// applyConvar 把一个 convar 的原始值写进 Crosshair，返回是否识别成功。
func applyConvar(c *Crosshair, key, raw string) bool {
	switch key {
	case "cl_crosshairstyle":
		c.Style = parseIntOr(raw, c.Style)
	case "cl_crosshair_length":
		c.Length = parseIntOr(raw, c.Length)
	case "cl_crosshair_thickness":
		c.Thickness = parseIntOr(raw, c.Thickness)
	case "cl_crosshair_gap":
		c.Gap = parseIntOr(raw, c.Gap)
	case "cl_crosshairdot":
		c.Dot = parseBoolOr(raw, c.Dot)
	case "cl_crosshair_t":
		c.TStyle = parseBoolOr(raw, c.TStyle)
	case "cl_crosshair_recoil":
		c.FollowRecoil = parseBoolOr(raw, c.FollowRecoil)
	case "cl_crosshaircolor_r":
		c.Color.R = parseIntOr(raw, c.Color.R)
	case "cl_crosshaircolor_g":
		c.Color.G = parseIntOr(raw, c.Color.G)
	case "cl_crosshaircolor_b":
		c.Color.B = parseIntOr(raw, c.Color.B)
	case "cl_crosshaircolor_a":
		c.Color.A = parseIntOr(raw, c.Color.A)
	case "cl_crosshair_drawoutline":
		c.OutlineMode = parseIntOr(raw, c.OutlineMode)
	case "cl_crosshairoutline_r":
		c.OutlineColor.R = parseIntOr(raw, c.OutlineColor.R)
	case "cl_crosshairoutline_g":
		c.OutlineColor.G = parseIntOr(raw, c.OutlineColor.G)
	case "cl_crosshairoutline_b":
		c.OutlineColor.B = parseIntOr(raw, c.OutlineColor.B)
	case "cl_crosshairoutline_a":
		c.OutlineColor.A = parseIntOr(raw, c.OutlineColor.A)
	case "cl_crosshair_dynamic_spread_limit":
		c.SpreadLimit = parseIntOr(raw, c.SpreadLimit)
	case "cl_crosshair_dynamic_splitdist":
		c.SplitDist = parseIntOr(raw, c.SplitDist)
	case "cl_crosshair_dynamic_splitalpha_innermod":
		c.InnerAlpha = parseFloatOr(raw, c.InnerAlpha)
	case "cl_crosshair_dynamic_splitalpha_outermod":
		c.OuterAlpha = parseFloatOr(raw, c.OuterAlpha)
	case "cl_crosshair_dynamic_maxdist_splitratio":
		c.SplitRatio = parseFloatOr(raw, c.SplitRatio)
	case "cl_crosshair_screen_height":
		c.ScreenHeight = parseIntOr(raw, c.ScreenHeight)
	case "cl_ironsight_dot_scale":
		c.ScopeDotScale = parseFloatOr(raw, c.ScopeDotScale)
	case "cl_ironsight_usecrosshaircolor":
		c.ScopeDotUseColor = parseBoolOr(raw, c.ScopeDotUseColor)
	default:
		return false
	}
	return true
}

// ApplyToCFG 把准星写进 autoexec.cfg 文本，返回新文本与改动报告。
//
// 规则（对应用户需求）：
//   - 已存在的键在原位置改值，不追加重复行；
//   - 缺失的键追加到文件末尾的自管区块里；
//   - cl_crosshair_screen_height 永远排在区块最后——写尺寸类指令会把它重置为当前分辨率；
//   - crosshair 显示开关原样保留，工具不写、不改、不删；
//   - 已从游戏删除的旧键与只差一个下划线的隐藏旧键会被注释掉并注明替代键，
//     否则用户会「看起来设置了却没变化」。
func ApplyToCFG(text string, ch Crosshair, shareCode string) (string, WriteReport) {
	ch.Normalize()

	lines, eol, trailingNewline, bom := splitLinesWithMeta(text)

	targets := ch.Convars()
	targetValues := make(map[string]string, len(targets))
	targetOrder := make([]string, 0, len(targets))
	for _, c := range targets {
		targetValues[c.Key] = c.Value
		targetOrder = append(targetOrder, c.Key)
	}

	// 先记下「原文件里每个准星键的生效值」，再删自管区块。
	// 区块里的键也算「原本就有」——否则每次保存都会把整块键重新报成「新增」，
	// 用户看到的改动明细就永远是一长串。
	before := make(map[string]string, len(targets))
	for _, raw := range lines {
		key, rest, ok := splitFields(stripComment(raw))
		if !ok {
			continue
		}
		if _, isTarget := targetValues[key]; !isTarget {
			continue
		}
		if _, seen := before[key]; seen {
			continue
		}
		before[key] = rest
	}

	lines = removeManagedBlock(lines)

	// 同上：三张清单都必须是空切片，不能是 nil。
	report := WriteReport{ReplacedKeys: []string{}, AppendedKeys: []string{}, CommentedKeys: []string{}}
	written := make(map[string]bool, len(targets))
	out := make([]string, 0, len(lines)+len(targets)+4)

	for _, raw := range lines {
		key, rest, ok := splitFields(stripComment(raw))
		if !ok {
			out = append(out, raw)
			continue
		}

		// 准星显示开关：原样保留，一个字节都不动。
		if key == DisplayToggleKey {
			report.DisplayToggle = rest
			report.PreservedToggle = true
			out = append(out, raw)
			continue
		}

		if replacement, gone := DeletedConvars[key]; gone {
			out = append(out, managedPrefix+"已废弃（CS2 2026-09 删除），改用 "+replacement+"："+strings.TrimSpace(raw))
			report.CommentedKeys = append(report.CommentedKeys, key)
			continue
		}

		if replacement, hidden := LegacyHiddenConvars[key]; hidden {
			out = append(out, managedPrefix+"旧键已不再描述外观，改用 "+replacement+"："+strings.TrimSpace(raw))
			report.CommentedKeys = append(report.CommentedKeys, key)
			continue
		}

		value, isTarget := targetValues[key]
		if !isTarget {
			out = append(out, raw)
			continue
		}

		// screen_height 一律移到区块末尾，原位那行注释掉。
		if key == "cl_crosshair_screen_height" {
			out = append(out, managedPrefix+"已移至区块末尾（写尺寸类指令会重置该值）："+strings.TrimSpace(raw))
			report.CommentedKeys = append(report.CommentedKeys, key)
			continue
		}

		if written[key] {
			out = append(out, managedPrefix+"重复行已注释："+strings.TrimSpace(raw))
			report.CommentedKeys = append(report.CommentedKeys, key)
			continue
		}

		out = append(out, key+" "+value)
		written[key] = true
	}

	// 追加自管区块：先放缺失的键，最后放 screen_height。
	block := make([]string, 0, len(targets)+4)
	// 区块标题刻意不带时间戳：这个函数每次点「写入」都会跑一遍，
	// 标题里塞时间会让同一份设置每写一次就变一次文本，违背「写两次结果一致」。
	// 时间信息在备份文件名里已经有了。
	block = append(block, blockStartMarker+"）=====")
	if strings.TrimSpace(shareCode) != "" {
		block = append(block, "// 分享码："+strings.TrimSpace(shareCode))
	}
	for _, key := range targetOrder {
		if key == "cl_crosshair_screen_height" {
			continue
		}
		if written[key] {
			continue
		}
		block = append(block, key+" "+targetValues[key])
	}
	block = append(block, "// 尺寸类指令会把基准高度重置为当前分辨率，故本行必须最后")
	block = append(block, "cl_crosshair_screen_height "+targetValues["cl_crosshair_screen_height"])
	block = append(block, blockEndMarker)

	// 改动明细按「原文件里的值 → 目标值」算，而不是按「我们改了几行文本」算。
	// 这样同一份设置重复保存时明细是空的，用户才能一眼看出这次到底改了什么。
	for _, key := range targetOrder {
		want := targetValues[key]
		if got, ok := before[key]; ok {
			if got != want {
				report.ReplacedKeys = append(report.ReplacedKeys, key)
			}
			continue
		}
		report.AppendedKeys = append(report.AppendedKeys, key)
	}

	// 区块前留一个空行，避免和最后一行粘连。
	if len(out) > 0 && strings.TrimSpace(out[len(out)-1]) != "" {
		out = append(out, "")
	}
	out = append(out, block...)

	return joinLinesWithMeta(out, eol, trailingNewline, bom), report
}

// removeManagedBlock 删掉上一次写入留下的自管区块，保证重复保存不会堆叠。
func removeManagedBlock(lines []string) []string {
	start, end := -1, -1
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if start < 0 && strings.HasPrefix(trimmed, blockStartMarker) {
			start = i
			continue
		}
		if start >= 0 && trimmed == blockEndMarker {
			end = i
			break
		}
	}
	if start < 0 || end < 0 {
		return lines
	}
	// 顺带吃掉区块前那个由我们添加的空行。
	if start > 0 && strings.TrimSpace(lines[start-1]) == "" {
		start--
	}
	out := make([]string, 0, len(lines)-(end-start+1))
	out = append(out, lines[:start]...)
	out = append(out, lines[end+1:]...)
	return out
}

// RenderCFG 生成独立的 crosshair.cfg 内容（可直接 exec）。
//
// 不含 crosshair 显示开关：那是用户的全局开关，单独导出时不应改写它。
func RenderCFG(ch Crosshair, shareCode, generatedAt string) string {
	ch.Normalize()
	lines := make([]string, 0, 32)
	header := "// CS2 准星设置"
	if strings.TrimSpace(generatedAt) != "" {
		header += "（生成于 " + strings.TrimSpace(generatedAt) + "）"
	}
	lines = append(lines, header)
	lines = append(lines, "// 用法：放进 game/csgo/cfg/ 后执行 exec crosshair.cfg")
	if strings.TrimSpace(shareCode) != "" {
		lines = append(lines, "// 分享码："+strings.TrimSpace(shareCode))
	}
	// 除 screen_height 外全都要写；screen_height 单独放到最后。
	// （这里曾经误把 sizeConvars 也跳过，导致导出的 cfg 缺长度/粗细/间距。）
	for _, c := range ch.Convars() {
		if c.Key == "cl_crosshair_screen_height" {
			continue
		}
		lines = append(lines, c.Line())
	}
	lines = append(lines, "// 尺寸类指令会把基准高度重置为当前分辨率，故本行必须最后")
	lines = append(lines, "cl_crosshair_screen_height "+itoa(ch.ScreenHeight))
	return strings.Join(lines, "\n") + "\n"
}

// RenderConsole 生成单行控制台指令（`;` 分隔），方便用户直接粘进控制台。
func RenderConsole(ch Crosshair) string {
	ch.Normalize()
	parts := make([]string, 0, 26)
	for _, c := range ch.Convars() {
		parts = append(parts, c.Line())
	}
	return strings.Join(parts, "; ")
}

// splitLinesWithMeta 拆行并记录换行风格、结尾换行与 BOM。
//
// 原样保留这三点，避免工具把用户的 CRLF 文件改写成 LF、或给文件加上 BOM
// 导致首行指令解析失败。
func splitLinesWithMeta(text string) (lines []string, eol string, trailingNewline, bom bool) {
	eol = "\n"
	if strings.Contains(text, "\r\n") {
		eol = "\r\n"
	}
	bom = strings.HasPrefix(text, "\ufeff")
	if bom {
		text = strings.TrimPrefix(text, "\ufeff")
	}
	norm := strings.ReplaceAll(text, "\r\n", "\n")
	norm = strings.ReplaceAll(norm, "\r", "\n")
	if norm == "" {
		return nil, eol, false, bom
	}
	trailingNewline = strings.HasSuffix(norm, "\n")
	if trailingNewline {
		norm = norm[:len(norm)-1]
	}
	return strings.Split(norm, "\n"), eol, trailingNewline, bom
}

// joinLinesWithMeta 是 splitLinesWithMeta 的逆操作。
func joinLinesWithMeta(lines []string, eol string, trailingNewline, bom bool) string {
	out := strings.Join(lines, eol)
	if trailingNewline && out != "" {
		out += eol
	}
	if bom {
		out = "\ufeff" + out
	}
	return out
}

// splitLines 只取行内容，供解析使用。
func splitLines(text string) []string {
	lines, _, _, _ := splitLinesWithMeta(text)
	return lines
}

func parseIntOr(raw string, fallback int) int {
	if v, err := strconv.Atoi(strings.TrimSpace(raw)); err == nil {
		return v
	}
	// 允许 `4.0` 这种写法：控制台接受，但 cfg 里偶尔会被人手写成小数。
	if f, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		return roundToInt(f)
	}
	return fallback
}

func parseFloatOr(raw string, fallback float64) float64 {
	if v, err := strconv.ParseFloat(strings.TrimSpace(raw), 64); err == nil {
		return v
	}
	return fallback
}

func parseBoolOr(raw string, fallback bool) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true", "yes", "on":
		return true
	case "0", "false", "no", "off":
		return false
	}
	return fallback
}
