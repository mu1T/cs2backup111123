package crosshair

// Preset 是一份准星模板。
//
// 内置预设全部是职业选手的实际准星，取值来自 2026-11 的现行像素制数据
// （见 docs/research/cs2-pro-crosshairs.md）。用户也可以自己存预设，
// 两者在 UI 里混排，靠 Builtin 区分。
type Preset struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Builtin     bool      `json:"builtin"`
	Crosshair   Crosshair `json:"crosshair"`
}

// proBase 返回一位职业选手的准星基准：静态十字、无中心点、无 T 型、无描边。
//
// 十六位选手全部是静态十字、无 T 型；只有 jL 旧版开过中心点，在各自条目里单独处理。
func proBase(length, thickness, gap, screenHeight int, color RGBA) Crosshair {
	c := Default()
	c.Style = StyleStaticCross
	c.Length = length
	c.Thickness = thickness
	c.Gap = gap
	c.ScreenHeight = screenHeight
	c.Color = color
	c.OutlineMode = OutlineNone
	c.OutlineColor = RGBA{R: 0, G: 0, B: 0, A: 255}
	c.Dot = false
	c.TStyle = false
	c.FollowRecoil = true
	return c
}

// BuiltinPresets 返回内置的职业选手准星（十六位，按用户指定名单与顺序）。
//
// 数值口径：现行像素制（length/thickness/gap 均为像素，screenHeight 是作者基准高度），
// 取自 docs/research/cs2-pro-crosshairs.md 的「新版现状」。
// 十六人里 flameZ 是唯一的 style 5（射击反馈）且开中心点，其余都是静态十字。
func BuiltinPresets() []Preset {
	return []Preset{
		{
			ID: "pro-zywoo", Name: "ZywOo", Builtin: true,
			Description: "Vitality · 细线小间距，参考高 768",
			Crosshair:   proBase(2, 2, 2, 768, RGBA{R: 0, G: 233, B: 255, A: 200}),
		},
		{
			ID: "pro-donk", Name: "donk", Builtin: true,
			Description: "Spirit · 青绿偏冷，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 37, G: 255, B: 0, A: 255}),
		},
		{
			ID: "pro-monesy", Name: "m0NESY", Builtin: true,
			Description: "Falcons · 白色细线，参考高 960",
			Crosshair:   proBase(3, 1, 2, 960, RGBA{R: 255, G: 255, B: 255, A: 255}),
		},
		{
			ID: "pro-kscerato", Name: "kscerato", Builtin: true,
			Description: "FURIA · 最短的准星，参考高 864",
			Crosshair:   proBase(1, 2, 1, 864, RGBA{R: 0, G: 255, B: 226, A: 255}),
		},
		{
			ID: "pro-sh1ro", Name: "sh1ro", Builtin: true,
			Description: "Spirit · 绿色细线，不透明度 200，参考高 960",
			Crosshair:   proBase(2, 1, 1, 960, RGBA{R: 0, G: 255, B: 0, A: 200}),
		},
		{
			ID: "pro-ropz", Name: "ropz", Builtin: true,
			Description: "Vitality · 较长较细，参考高 1080",
			Crosshair:   proBase(4, 1, 2, 1080, RGBA{R: 0, G: 255, B: 0, A: 255}),
		},
		{
			ID: "pro-molodoy", Name: "molodoy", Builtin: true,
			Description: "MongolZ · 青色，参考高 768",
			Crosshair:   proBase(2, 2, 2, 768, RGBA{R: 16, G: 255, B: 255, A: 200}),
		},
		{
			ID: "pro-spinx", Name: "spinx", Builtin: true,
			Description: "Vitality · 绿色，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 0, G: 255, B: 0, A: 255}),
		},
		{
			ID: "pro-makazze", Name: "makazze", Builtin: true,
			Description: "MOUZ · 青色，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 0, G: 255, B: 255, A: 255}),
		},
		{
			ID: "pro-niko", Name: "NiKo", Builtin: true,
			Description: "Falcons · 当前绿色版；2021-2024 曾是白色，参考高 960",
			Crosshair:   proBase(2, 2, 2, 960, RGBA{R: 0, G: 255, B: 145, A: 255}),
		},
		{
			ID: "pro-kyousuke", Name: "kyousuke", Builtin: true,
			Description: "Falcons · 步枪手，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 0, G: 255, B: 0, A: 200}),
		},
		{
			ID: "pro-b1t", Name: "b1t", Builtin: true,
			Description: "NAVI · 经典绿色小十字，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 0, G: 255, B: 0, A: 200}),
		},
		{
			ID: "pro-torzsi", Name: "torzsi", Builtin: true,
			Description: "MOUZ · 绿色，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 0, G: 255, B: 0, A: 255}),
		},
		{
			ID: "pro-910", Name: "910", Builtin: true,
			Description: "MongolZ · 白色，参考高 960",
			Crosshair:   proBase(2, 2, 1, 960, RGBA{R: 255, G: 255, B: 255, A: 200}),
		},
		{
			ID: "pro-flamez", Name: "flameZ", Builtin: true,
			Description: "Vitality · 唯一用射击反馈样式并开中心点，参考高 960",
			Crosshair: func() Crosshair {
				c := proBase(3, 1, 2, 960, RGBA{R: 0, G: 229, B: 255, A: 255})
				c.Style = StyleStaticCrossFeedback
				c.Dot = true
				return c
			}(),
		},
		{
			ID: "pro-yekindar", Name: "YEKINDAR", Builtin: true,
			Description: "Team Liquid · 基准高 1050，偏绿的黄绿",
			Crosshair:   proBase(2, 2, 1, 1050, RGBA{R: 38, G: 241, B: 27, A: 255}),
		},
	}
}

// FindBuiltinPreset 按 ID 找内置预设，供「恢复被隐藏的预设」使用。
func FindBuiltinPreset(id string) (Preset, bool) {
	for _, p := range BuiltinPresets() {
		if p.ID == id {
			return p, true
		}
	}
	return Preset{}, false
}
