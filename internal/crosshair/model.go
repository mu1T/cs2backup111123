// Package crosshair 实现 CS2 准星设置的数据模型、分享码编解码与 cfg 读写。
//
// 口径：全部按 2026-09-23 准星系统重构后的「整数像素制」表达，只产出现行 convar。
// 重构前那套按 480 像素高屏幕取比例的旧键（cl_crosshairgap / cl_crosshaircolor /
// cl_crosshairusealpha 等）已从游戏删除，写进 cfg 会报 Unknown command，因此不再输出。
//
// 详见 docs/research/cs2-crosshair.md。
package crosshair

import (
	"math"
	"strconv"
	"strings"
)

// Format 标识一份准星数据来自哪种分享码容器。
//
// legacy 容器（CSGO-xxxxx，25 字符 / 18 字节）已被游戏拒收，仅用于导入兼容；
// 导出永远走 cs2 容器（CS + 44 字符 / 32 字节）。
type Format string

const (
	// FormatCS2V1 是当前唯一可用的分享码格式（CS2 1.41.8.8 起）。
	FormatCS2V1 Format = "cs2-v1"
	// FormatLegacyV1 是 CS:GO 时代到 CS2 1.41.8.1 的旧格式，尺寸为 480 像素高的比例。
	FormatLegacyV1 Format = "legacy-v1"
	// FormatLegacyV3 是 2026-09-23 的过渡格式，尺寸已改为像素。
	FormatLegacyV3 Format = "legacy-v3"
	// FormatLegacyV4 是 2026-09-24 的过渡格式，新增 Static Square 与三态描边。
	FormatLegacyV4 Format = "legacy-v4"
)

// RGBA 是 0-255 的四通道颜色。
type RGBA struct {
	R int `json:"r"`
	G int `json:"g"`
	B int `json:"b"`
	A int `json:"a"`
}

// 描边三态，对应 cl_crosshair_drawoutline 的 0/1/2。
const (
	OutlineNone = 0 // 无描边
	OutlineFull = 1 // 四周完整描边
	OutlineHalf = 2 // 只描上方与左侧
)

// 样式取值范围，对应 cl_crosshairstyle 的 0-9。
const (
	StyleDynamicCross        = 0 // 动态十字
	StyleDynamicCircle       = 1 // 动态圆形
	StyleDynamicCrossClassic = 2 // 动态十字（经典）
	StyleStaticCircle        = 3 // 静态圆形
	StyleStaticCross         = 4 // 静态十字
	StyleStaticCrossFeedback = 5 // 静态十字（射击反馈）
	StyleDotOnly             = 6 // 仅中心点
	StyleDynamicQuad         = 7 // 动态四分
	StyleStaticSquare        = 8 // 静态方形
	StyleStaticQuad          = 9 // 静态四分

	StyleMin = StyleDynamicCross
	StyleMax = StyleStaticQuad
)

// 尺寸类 convar 的合法区间，取自官方 convar 转储（convars.txt）。
const (
	GapMin          = -3840
	GapMax          = 3840
	LengthMin       = 0
	LengthMax       = 255
	ThicknessMin    = 0
	ThicknessMax    = 32
	SpreadLimitMin  = 0
	SpreadLimitMax  = 255
	SplitDistMin    = 0
	SplitDistMax    = 127
	ScreenHeightMin = 240
	ScreenHeightMax = 65535
	ScopeDotMin     = 0.1
	ScopeDotMax     = 2.0
)

// Crosshair 是一份与分享码字段一一对应的准星设置。
//
// 内存表示统一用「整数像素 + 基准屏幕高度」，legacy-v1 解码时做一次近似换算，
// 上层只认这一种模型。
type Crosshair struct {
	Format Format `json:"format"`

	Style        int  `json:"style"`
	FollowRecoil bool `json:"follow_recoil"`
	Dot          bool `json:"dot"`
	TStyle       bool `json:"t_style"`
	OutlineMode  int  `json:"outline_mode"`

	Color        RGBA `json:"color"`
	OutlineColor RGBA `json:"outline_color"`

	Gap         int `json:"gap"`
	Length      int `json:"length"`
	Thickness   int `json:"thickness"`
	SpreadLimit int `json:"spread_limit"`

	SplitDist  int     `json:"split_dist"`
	InnerAlpha float64 `json:"inner_alpha"`
	OuterAlpha float64 `json:"outer_alpha"`
	SplitRatio float64 `json:"split_ratio"`

	ScreenHeight     int     `json:"screen_height"`
	ScopeDotScale    float64 `json:"scope_dot_scale"`
	ScopeDotUseColor bool    `json:"scope_dot_use_color"`
}

// Default 返回游戏默认准星（cl_crosshairstyle 7 为 CS2 重构后的默认值）。
func Default() Crosshair {
	return Crosshair{
		Format:        FormatCS2V1,
		Style:         StyleDynamicQuad,
		FollowRecoil:  true,
		Dot:           false,
		TStyle:        false,
		OutlineMode:   OutlineFull,
		Color:         RGBA{R: 0, G: 255, B: 0, A: 255},
		OutlineColor:  RGBA{R: 0, G: 0, B: 0, A: 255},
		Gap:           4,
		Length:        8,
		Thickness:     2,
		SpreadLimit:   255,
		SplitDist:     3,
		InnerAlpha:    0,
		OuterAlpha:    1,
		SplitRatio:    1,
		ScreenHeight:  1080,
		ScopeDotScale: 1,
	}
}

// Normalize 把各字段夹到官方 convar 声明的合法区间内。
//
// 解码分享码时客户端也会做同样的夹取，所以这里镜像一遍，保证「码里越界 →
// 写进 cfg 的值」与游戏内一致。
func (c *Crosshair) Normalize() {
	if c.Format == "" {
		c.Format = FormatCS2V1
	}
	c.Style = clampInt(c.Style, StyleMin, StyleMax)
	c.OutlineMode = clampInt(c.OutlineMode, OutlineNone, OutlineHalf)
	c.Gap = clampInt(c.Gap, GapMin, GapMax)
	c.Length = clampInt(c.Length, LengthMin, LengthMax)
	c.Thickness = clampInt(c.Thickness, ThicknessMin, ThicknessMax)
	c.SpreadLimit = clampInt(c.SpreadLimit, SpreadLimitMin, SpreadLimitMax)
	c.SplitDist = clampInt(c.SplitDist, SplitDistMin, SplitDistMax)
	c.ScreenHeight = clampInt(c.ScreenHeight, ScreenHeightMin, ScreenHeightMax)
	c.InnerAlpha = clampFloat(c.InnerAlpha, 0, 1)
	c.OuterAlpha = clampFloat(c.OuterAlpha, 0.3, 1)
	c.SplitRatio = clampFloat(c.SplitRatio, 0, 1)
	c.ScopeDotScale = clampFloat(c.ScopeDotScale, ScopeDotMin, ScopeDotMax)
	c.Color = c.Color.normalized()
	c.OutlineColor = c.OutlineColor.normalized()
}

func (c RGBA) normalized() RGBA {
	return RGBA{
		R: clampInt(c.R, 0, 255),
		G: clampInt(c.G, 0, 255),
		B: clampInt(c.B, 0, 255),
		A: clampInt(c.A, 0, 255),
	}
}

// Convar 是「键 + 值」的一行准星指令。
type Convar struct {
	Key   string `json:"key"`
	Value string `json:"value"`
}

// CommandPreview 是编辑器里「看得见的键」：逐条指令 + 单行控制台串 + 分享码。
type CommandPreview struct {
	Lines   []Convar `json:"lines"`
	Console string   `json:"console"`
	Share   string   `json:"share"`
}

// Line 渲染成 cfg 中的一行，例如 `cl_crosshair_gap 4`。
func (c Convar) Line() string {
	return c.Key + " " + c.Value
}

// Convars 按写入 cfg 的顺序返回全部准星指令。
//
// 刻意不包含 `crosshair`（显示总开关）：那是用户的全局开关，工具只读取、不改写。
func (c Crosshair) Convars() []Convar {
	return []Convar{
		{"cl_crosshairstyle", itoa(c.Style)},
		{"cl_crosshair_length", itoa(c.Length)},
		{"cl_crosshair_thickness", itoa(c.Thickness)},
		{"cl_crosshair_gap", itoa(c.Gap)},
		{"cl_crosshairdot", boolToCfg(c.Dot)},
		{"cl_crosshair_t", boolToCfg(c.TStyle)},
		{"cl_crosshair_recoil", boolToCfg(c.FollowRecoil)},
		{"cl_crosshaircolor_r", itoa(c.Color.R)},
		{"cl_crosshaircolor_g", itoa(c.Color.G)},
		{"cl_crosshaircolor_b", itoa(c.Color.B)},
		{"cl_crosshaircolor_a", itoa(c.Color.A)},
		{"cl_crosshair_drawoutline", itoa(c.OutlineMode)},
		{"cl_crosshairoutline_r", itoa(c.OutlineColor.R)},
		{"cl_crosshairoutline_g", itoa(c.OutlineColor.G)},
		{"cl_crosshairoutline_b", itoa(c.OutlineColor.B)},
		{"cl_crosshairoutline_a", itoa(c.OutlineColor.A)},
		{"cl_crosshair_dynamic_spread_limit", itoa(c.SpreadLimit)},
		{"cl_crosshair_dynamic_splitdist", itoa(c.SplitDist)},
		{"cl_crosshair_dynamic_splitalpha_innermod", ftoa(c.InnerAlpha)},
		{"cl_crosshair_dynamic_splitalpha_outermod", ftoa(c.OuterAlpha)},
		{"cl_crosshair_dynamic_maxdist_splitratio", ftoa(c.SplitRatio)},
		{"cl_ironsight_dot_scale", ftoa(c.ScopeDotScale)},
		{"cl_ironsight_usecrosshaircolor", boolToCfg(c.ScopeDotUseColor)},
		// 必须最后：写尺寸类指令会把 screen_height 重置为当前分辨率。
		{"cl_crosshair_screen_height", itoa(c.ScreenHeight)},
	}
}

// DeletedConvars 是 2026-09 重构时从游戏彻底删除的旧键 → 现行替代键。
//
// 留在 autoexec.cfg 里不会让游戏崩溃，但控制台会打 Unknown command 且静默无效，
// 用户会误以为设置生效。工具接管时把它们注释掉并说明替代键。
var DeletedConvars = map[string]string{
	"cl_crosshairgap":                "cl_crosshair_gap",
	"cl_crosshaircolor":              "cl_crosshaircolor_r/g/b",
	"cl_crosshairusealpha":           "cl_crosshaircolor_a",
	"cl_fixedcrosshairgap":           "cl_crosshair_gap",
	"cl_crosshairgap_useweaponvalue": "cl_crosshair_gap",
	"cl_crosshair_outlinethickness":  "cl_crosshair_drawoutline",
}

// LegacyHiddenConvars 是重构后仍存在、但已不描述准星外观的隐藏旧键。
//
// 它们与新键只差一个下划线（cl_crosshairthickness vs cl_crosshair_thickness），
// 混用会「看起来设置了却没变化」，所以一并注释掉并指向新键。
var LegacyHiddenConvars = map[string]string{
	"cl_crosshairsize":      "cl_crosshair_length",
	"cl_crosshairthickness": "cl_crosshair_thickness",
	"cl_crosshairalpha":     "cl_crosshaircolor_a",
}

// DisplayToggleKey 是准星显示总开关。工具只读取它，永不写入或删除。
const DisplayToggleKey = "crosshair"

func itoa(v int) string { return strconv.Itoa(v) }

// ftoa 去掉无意义的小数尾零，让 cfg 里出现 `0.35` 而不是 `0.35000000000000003`。
func ftoa(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

func boolToCfg(v bool) string {
	if v {
		return "1"
	}
	return "0"
}

func clampInt(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func clampFloat(v, lo, hi float64) float64 {
	return math.Min(math.Max(v, lo), hi)
}

func roundToInt(v float64) int { return int(math.Round(v)) }

// CanonicalKey 把大小写混杂的键名归一，便于比对。
func CanonicalKey(key string) string { return strings.ToLower(strings.TrimSpace(key)) }
