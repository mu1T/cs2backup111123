package crosshair

import (
	"math"
	"testing"
)

// 验证向量来自 docs/research/cs2-crosshair.md §4.5。
// 前两条是 CS2 1.41.8.8 客户端自己生成的码（端到端可信），后两条是 legacy 容器。
var cs2Vectors = []struct {
	code string
	want Crosshair
}{
	{
		code: "CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe",
		want: Crosshair{
			Format: FormatCS2V1, Style: 2, Dot: true, FollowRecoil: false, TStyle: false,
			Color: RGBA{255, 0, 0, 255}, Gap: 0, Length: 5, Thickness: 1,
			SpreadLimit: 181, SplitDist: 3, InnerAlpha: 1, OuterAlpha: 0.35,
			SplitRatio: 0, ScreenHeight: 768, ScopeDotScale: 1, OutlineMode: 0,
		},
	},
	{
		code: "CSHt3ObzKS3ikGJLRBVrKjPUrhOCikyke53na8cJFtNZKa",
		want: Crosshair{
			Format: FormatCS2V1, Style: 2, Dot: true,
			Color: RGBA{255, 0, 0, 255}, Gap: 0, Length: 5, Thickness: 1,
			SpreadLimit: 181, SplitDist: 3, InnerAlpha: 1, OuterAlpha: 0.35,
			SplitRatio: 0, ScreenHeight: 768, ScopeDotScale: 0.55, ScopeDotUseColor: true,
		},
	},
}

func TestDecodeCS2Vectors(t *testing.T) {
	for _, tc := range cs2Vectors {
		got, err := Decode(tc.code)
		if err != nil {
			t.Fatalf("Decode(%s) 失败: %v", tc.code, err)
		}
		if got.Format != FormatCS2V1 {
			t.Errorf("格式 = %s, 期望 cs2-v1", got.Format)
		}
		assertIntField(t, tc.code, "style", got.Style, tc.want.Style)
		assertIntField(t, tc.code, "gap", got.Gap, tc.want.Gap)
		assertIntField(t, tc.code, "length", got.Length, tc.want.Length)
		assertIntField(t, tc.code, "thickness", got.Thickness, tc.want.Thickness)
		assertIntField(t, tc.code, "spread_limit", got.SpreadLimit, tc.want.SpreadLimit)
		assertIntField(t, tc.code, "split_dist", got.SplitDist, tc.want.SplitDist)
		assertIntField(t, tc.code, "screen_height", got.ScreenHeight, tc.want.ScreenHeight)
		assertIntField(t, tc.code, "outline_mode", got.OutlineMode, tc.want.OutlineMode)
		assertFloatField(t, tc.code, "inner_alpha", got.InnerAlpha, tc.want.InnerAlpha)
		assertFloatField(t, tc.code, "outer_alpha", got.OuterAlpha, tc.want.OuterAlpha)
		assertFloatField(t, tc.code, "split_ratio", got.SplitRatio, tc.want.SplitRatio)
		assertFloatField(t, tc.code, "scope_dot_scale", got.ScopeDotScale, tc.want.ScopeDotScale)
		if got.Dot != tc.want.Dot {
			t.Errorf("%s: dot = %v, 期望 %v", tc.code, got.Dot, tc.want.Dot)
		}
		if got.ScopeDotUseColor != tc.want.ScopeDotUseColor {
			t.Errorf("%s: scope_dot_use_color = %v, 期望 %v", tc.code, got.ScopeDotUseColor, tc.want.ScopeDotUseColor)
		}
		if got.Color != tc.want.Color {
			t.Errorf("%s: color = %+v, 期望 %+v", tc.code, got.Color, tc.want.Color)
		}
	}
}

// TestEncodeRoundTripCS2 要求「解码再编码」字节级回到原串。
func TestEncodeRoundTripCS2(t *testing.T) {
	for _, tc := range cs2Vectors {
		decoded, err := Decode(tc.code)
		if err != nil {
			t.Fatalf("Decode(%s) 失败: %v", tc.code, err)
		}
		if got := Encode(*decoded); got != tc.code {
			t.Errorf("往返不一致:\n  输入 %s\n  输出 %s", tc.code, got)
		}
	}
}

func TestDecodeLegacyV1(t *testing.T) {
	got, err := Decode("CSGO-Cn37R-YE7vo-pLCAL-aURmZ-z6zkG")
	if err != nil {
		t.Fatalf("解码 legacy-v1 失败: %v", err)
	}
	if got.Format != FormatLegacyV1 {
		t.Fatalf("格式 = %s, 期望 legacy-v1", got.Format)
	}
	// legacy-v1 尺寸是 480 高屏幕的比例，按 1080 换算：
	// gap -1.3 → -1.3*1080/480 = -2.925 → -3
	// length 4.6 → 10.35 → 10
	// thickness 1.2 → 2.7 → 3
	assertIntField(t, "legacy-v1", "gap", got.Gap, -3)
	assertIntField(t, "legacy-v1", "length", got.Length, 10)
	assertIntField(t, "legacy-v1", "thickness", got.Thickness, 3)
	assertIntField(t, "legacy-v1", "style", got.Style, 2)
	assertIntField(t, "legacy-v1", "split_dist", got.SplitDist, 6)
	assertFloatField(t, "legacy-v1", "inner_alpha", got.InnerAlpha, 0.6)
	assertFloatField(t, "legacy-v1", "outer_alpha", got.OuterAlpha, 0.4)
	assertFloatField(t, "legacy-v1", "split_ratio", got.SplitRatio, 0.5)
	// color 索引 5 = 自定义，取码里的 RGB；alpha 沿用码里的 137。
	if got.Color != (RGBA{175, 81, 213, 137}) {
		t.Errorf("color = %+v, 期望 {175 81 213 137}", got.Color)
	}
	if !got.Dot || !got.TStyle {
		t.Errorf("dot/t_style 应均为 true，实际 dot=%v t=%v", got.Dot, got.TStyle)
	}
	// 旧码没有「半描边」，outline 位为真即完整描边。
	if got.OutlineMode != OutlineFull {
		t.Errorf("outline_mode = %d, 期望 %d", got.OutlineMode, OutlineFull)
	}
}

func TestDecodeLegacyV1PresetColor(t *testing.T) {
	// 构造一份 color 索引 = 4（红）的 legacy-v1 码，验证预设色覆盖 RGB。
	c := Crosshair{
		Format: FormatCS2V1, Style: 4, Length: 8, Thickness: 2, Gap: 4,
		Color: RGBA{1, 2, 3, 255}, ScreenHeight: 1080,
		OuterAlpha: 1, SplitRatio: 1, SpreadLimit: 255, ScopeDotScale: 1,
	}
	// 先确认新码可解，再单独验证预设表本身。
	if _, err := Decode(Encode(c)); err != nil {
		t.Fatalf("自产码无法解码: %v", err)
	}
	if preset := legacyV1Preset[4]; preset != (RGBA{255, 0, 0, 255}) {
		t.Errorf("预设色 4 = %+v, 期望红色", preset)
	}
}

func TestDecodeLegacyV3(t *testing.T) {
	got, err := Decode("CSGO-MnUCC-89iG7-2cVar-wy7Yn-amCpF")
	if err != nil {
		t.Fatalf("解码 legacy-v3 失败: %v", err)
	}
	if got.Format != FormatLegacyV3 {
		t.Fatalf("格式 = %s, 期望 legacy-v3", got.Format)
	}
	assertIntField(t, "legacy-v3", "style", got.Style, 6)
	assertIntField(t, "legacy-v3", "gap", got.Gap, 4)
	assertIntField(t, "legacy-v3", "length", got.Length, 8)
	assertIntField(t, "legacy-v3", "thickness", got.Thickness, 3)
	assertIntField(t, "legacy-v3", "spread_limit", got.SpreadLimit, 255)
	assertIntField(t, "legacy-v3", "split_dist", got.SplitDist, 7)
	assertIntField(t, "legacy-v3", "screen_height", got.ScreenHeight, 1080)
	assertFloatField(t, "legacy-v3", "inner_alpha", got.InnerAlpha, 1)
	assertFloatField(t, "legacy-v3", "outer_alpha", got.OuterAlpha, 0.45)
	assertFloatField(t, "legacy-v3", "split_ratio", got.SplitRatio, 0.3)
	if got.Color != (RGBA{252, 15, 192, 255}) {
		t.Errorf("color = %+v, 期望 {252 15 192 255}", got.Color)
	}
}

// TestDecodeClampsOutOfRange 覆盖客户端也会做的夹取行为。
func TestDecodeClampsOutOfRange(t *testing.T) {
	got, err := Decode("CS7uw3wMkJBJ4rMDLEaYbyfTJFFPvkLAhOmBFi8zOM6H2d")
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if got.Gap != GapMin {
		t.Errorf("gap = %d, 期望被夹到 %d", got.Gap, GapMin)
	}
	if got.Thickness != ThicknessMax {
		t.Errorf("thickness = %d, 期望被夹到 %d", got.Thickness, ThicknessMax)
	}
	if got.ScreenHeight != ScreenHeightMin {
		t.Errorf("screen_height = %d, 期望被抬到 %d", got.ScreenHeight, ScreenHeightMin)
	}
	if got.ScopeDotScale != ScopeDotMax {
		t.Errorf("scope_dot_scale = %v, 期望被夹到 %v", got.ScopeDotScale, ScopeDotMax)
	}
}

func TestDecodeGapUpperClamp(t *testing.T) {
	got, err := Decode("CSDfPcMpj4e9NRxx6SGiVoFCHzYRYGmb34WHpbYHy6E6id")
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if got.Gap != GapMax {
		t.Errorf("gap = %d, 期望被夹到 %d", got.Gap, GapMax)
	}
}

func TestDecodeRejectsBadInput(t *testing.T) {
	cases := []struct {
		name string
		code string
	}{
		{"空字符串", ""},
		{"纯空白", "   "},
		{"长度不对", "CSabc"},
		{"非法字符", "CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZR0"},
		{"校验和错误", "CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRf"},
		{"旧码版本 2", "CSGO-aaaaa-aaaaa-aaaaa-aaaaa-aaaaa"},
	}
	for _, tc := range cases {
		if _, err := Decode(tc.code); err == nil {
			t.Errorf("%s: 期望报错，实际解码成功", tc.name)
		}
	}
}

// TestDecodeAcceptsLooseFormatting 允许粘贴时夹带空格、短横线与小写前缀。
func TestDecodeAcceptsLooseFormatting(t *testing.T) {
	canonical := "CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe"
	loose := "  CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe\n"
	if _, err := Decode(loose); err != nil {
		t.Fatalf("带空白应可解码: %v", err)
	}
	if _, err := Decode(canonical[:2] + "-" + canonical[2:]); err != nil {
		t.Fatalf("带短横线应可解码: %v", err)
	}
	// 只有前缀允许大小写混写；字符表本身区分大小写，正文不能整体小写。
	legacyLoose := "csgo-Cn37R-YE7vo-pLCAL-aURmZ-z6zkG"
	if _, err := Decode(legacyLoose); err != nil {
		t.Fatalf("小写 CSGO 前缀应可解码: %v", err)
	}
	// 正文被整体小写属于真实抄错，必须报错而不是静默解出另一个准星。
	if _, err := Decode("CSGO-cn37r-ye7vo-plcal-aurmz-z6zkg"); err == nil {
		t.Fatal("正文被小写的码应被拒绝")
	}
}

func TestDetectFormat(t *testing.T) {
	cases := map[string]Format{
		"CSvbPubOq37zTGqtsPTP5QTrp5CB4xFXiKRLfzJsm49ZRe": FormatCS2V1,
		"CSGO-Cn37R-YE7vo-pLCAL-aURmZ-z6zkG":             FormatLegacyV1,
		"CSGO-MnUCC-89iG7-2cVar-wy7Yn-amCpF":             FormatLegacyV3,
	}
	for code, want := range cases {
		got, err := DetectFormat(code)
		if err != nil {
			t.Fatalf("DetectFormat(%s) 失败: %v", code, err)
		}
		if got != want {
			t.Errorf("DetectFormat(%s) = %s, 期望 %s", code, got, want)
		}
	}
}

// TestEncodeThenDecodeSemantic 保证任意合法设置在码里往返后语义不变。
func TestEncodeThenDecodeSemantic(t *testing.T) {
	orig := Crosshair{
		Style: StyleStaticQuad, FollowRecoil: true, Dot: true, TStyle: true,
		OutlineMode: OutlineHalf,
		Color:       RGBA{208, 77, 6, 255}, OutlineColor: RGBA{10, 20, 30, 200},
		Gap: 12, Length: 7, Thickness: 3, SpreadLimit: 200,
		SplitDist: 40, InnerAlpha: 0.75, OuterAlpha: 0.6, SplitRatio: 0.29,
		ScreenHeight: 1440, ScopeDotScale: 1.35, ScopeDotUseColor: true,
	}
	code := Encode(orig)
	got, err := Decode(code)
	if err != nil {
		t.Fatalf("解码自产码失败: %v", err)
	}
	assertIntField(t, code, "style", got.Style, orig.Style)
	assertIntField(t, code, "gap", got.Gap, orig.Gap)
	assertIntField(t, code, "length", got.Length, orig.Length)
	assertIntField(t, code, "thickness", got.Thickness, orig.Thickness)
	assertIntField(t, code, "outline_mode", got.OutlineMode, orig.OutlineMode)
	assertIntField(t, code, "split_dist", got.SplitDist, orig.SplitDist)
	assertFloatField(t, code, "inner_alpha", got.InnerAlpha, orig.InnerAlpha)
	assertFloatField(t, code, "outer_alpha", got.OuterAlpha, orig.OuterAlpha)
	assertFloatField(t, code, "split_ratio", got.SplitRatio, orig.SplitRatio)
	assertFloatField(t, code, "scope_dot_scale", got.ScopeDotScale, orig.ScopeDotScale)
	if got.Color != orig.Color {
		t.Errorf("color = %+v, 期望 %+v", got.Color, orig.Color)
	}
	if got.OutlineColor != orig.OutlineColor {
		t.Errorf("outline_color = %+v, 期望 %+v", got.OutlineColor, orig.OutlineColor)
	}
	if !got.Dot || !got.TStyle || !got.FollowRecoil || !got.ScopeDotUseColor {
		t.Errorf("布尔字段丢失: %+v", got)
	}
}

// TestEncodeNormalizesBeforeEncoding 越界值应先被夹取再编码。
func TestEncodeNormalizesBeforeEncoding(t *testing.T) {
	code := Encode(Crosshair{
		Style: 99, Thickness: 999, Gap: -99999, ScreenHeight: 0,
		InnerAlpha: 5, OuterAlpha: 5, SplitRatio: 5, ScopeDotScale: 9,
	})
	got, err := Decode(code)
	if err != nil {
		t.Fatalf("解码失败: %v", err)
	}
	if got.Style != StyleMax {
		t.Errorf("style = %d, 期望 %d", got.Style, StyleMax)
	}
	if got.Thickness != ThicknessMax {
		t.Errorf("thickness = %d, 期望 %d", got.Thickness, ThicknessMax)
	}
	if got.Gap != GapMin {
		t.Errorf("gap = %d, 期望 %d", got.Gap, GapMin)
	}
	if got.ScreenHeight != ScreenHeightMin {
		t.Errorf("screen_height = %d, 期望 %d", got.ScreenHeight, ScreenHeightMin)
	}
	if got.ScopeDotScale != ScopeDotMax {
		t.Errorf("scope_dot_scale = %v, 期望 %v", got.ScopeDotScale, ScopeDotMax)
	}
}

func assertIntField(t *testing.T, code, field string, got, want int) {
	t.Helper()
	if got != want {
		t.Errorf("%s: %s = %d, 期望 %d", code, field, got, want)
	}
}

func assertFloatField(t *testing.T, code, field string, got, want float64) {
	t.Helper()
	if math.Abs(got-want) > 0.005 {
		t.Errorf("%s: %s = %v, 期望 %v", code, field, got, want)
	}
}
