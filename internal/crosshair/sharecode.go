package crosshair

import (
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
)

// Alphabet 是分享码使用的 57 字符表。
//
// 相比 62 个字母数字，剔除了 5 个易混字符：大写 I、小写 g、小写 l、数字 0、数字 1。
// 注意大写 O **保留**（索引 13），Valve 官方样例里就含 O。
const Alphabet = "ABCDEFGHJKLMNOPQRSTUVWXYZabcdefhijkmnopqrstuvwxyz23456789"

const (
	legacyChars   = 25
	legacyBytes   = 18
	cs2Chars      = 44
	cs2Bytes      = 32
	legacyPrefix  = "CSGO"
	cs2Prefix     = "CS"
	defaultRefHgt = 1080
)

var (
	// ErrEmptyCode 表示用户没输入内容。
	ErrEmptyCode = errors.New("分享码为空")
	// ErrInvalidChar 表示出现了字符表之外的字符。
	ErrInvalidChar = errors.New("分享码包含非法字符")
	// ErrBadLength 表示长度不符合任何已知格式。
	ErrBadLength = errors.New("分享码长度不正确")
	// ErrBadChecksum 表示校验和不匹配，通常是抄漏或抄错字符。
	ErrBadChecksum = errors.New("分享码校验和不匹配，请检查是否抄错字符")
	// ErrUnsupportedVersion 表示版本号未知或已被客户端拒绝。
	ErrUnsupportedVersion = errors.New("不支持的分享码版本")
	// ErrBadScreenHeight 表示基准屏幕高度为 0，客户端同样判为非法码。
	ErrBadScreenHeight = errors.New("分享码的基准屏幕高度非法")
	// ErrOverflow 表示字符序列换算出的数值超出容器容量。
	ErrOverflow = errors.New("分享码数值溢出")
)

var radix57 = big.NewInt(57)

// charsToBytes 把「最低位字符在前」的 base-57 字符串换算成定长大端字节数组。
func charsToBytes(s string, n int) ([]byte, error) {
	v := new(big.Int)
	for i := len(s) - 1; i >= 0; i-- {
		idx := strings.IndexByte(Alphabet, s[i])
		if idx < 0 {
			return nil, fmt.Errorf("%w: %q", ErrInvalidChar, string(s[i]))
		}
		v.Mul(v, radix57)
		v.Add(v, big.NewInt(int64(idx)))
	}
	if v.BitLen() > n*8 {
		return nil, ErrOverflow
	}
	out := make([]byte, n)
	v.FillBytes(out)
	return out, nil
}

// bytesToChars 把定长大端字节数组换算成 n 个字符，低位字符在前。
func bytesToChars(b []byte, n int) string {
	v := new(big.Int).SetBytes(b)
	q, r := new(big.Int), new(big.Int)
	out := make([]byte, 0, n)
	for i := 0; i < n; i++ {
		q.QuoRem(v, radix57, r)
		out = append(out, Alphabet[r.Int64()])
		v.Set(q)
	}
	return string(out)
}

// checksum 是准星码特有的校验字节：byte[0] == sum(byte[1..]) % 256。
func checksum(b []byte) byte {
	var s uint32
	for _, x := range b[1:] {
		s += uint32(x)
	}
	return byte(s & 0xff)
}

// normalizeCode 去掉空白与短横线，并统一 CSGO 前缀的大小写。
func normalizeCode(code string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case ' ', '\t', '\n', '\r', '-':
			return -1
		}
		return r
	}, code)
	if len(cleaned) >= len(legacyPrefix) && strings.EqualFold(cleaned[:len(legacyPrefix)], legacyPrefix) {
		cleaned = legacyPrefix + cleaned[len(legacyPrefix):]
	}
	return cleaned
}

// DetectFormat 只根据容器与版本字节判断格式，不做校验和验证。
func DetectFormat(code string) (Format, error) {
	c := normalizeCode(code)
	if c == "" {
		return "", ErrEmptyCode
	}
	switch {
	case strings.HasPrefix(c, cs2Prefix) && len(c) == len(cs2Prefix)+cs2Chars:
		return FormatCS2V1, nil
	case strings.HasPrefix(c, legacyPrefix) && len(c) == len(legacyPrefix)+legacyChars:
		body := c[len(legacyPrefix):]
		b, err := charsToBytes(body, legacyBytes)
		if err != nil {
			return "", err
		}
		switch b[1] {
		case 1:
			return FormatLegacyV1, nil
		case 3:
			return FormatLegacyV3, nil
		case 4:
			return FormatLegacyV4, nil
		default:
			return "", ErrUnsupportedVersion
		}
	}
	return "", ErrBadLength
}

// Decode 解析任意受支持格式的分享码，返回统一的新制像素模型。
//
// legacy-v1 的尺寸是「480 像素高屏幕的比例」，这里按 1080 近似换算；需要更贴近
// 用户实际分辨率时用 DecodeWithRef 显式传入屏幕高度。
func Decode(code string) (*Crosshair, error) {
	return DecodeWithRef(code, defaultRefHgt)
}

// DecodeWithRef 同 Decode，但允许指定 legacy-v1 换算时使用的屏幕高度。
func DecodeWithRef(code string, refHeight int) (*Crosshair, error) {
	if refHeight < ScreenHeightMin {
		refHeight = defaultRefHgt
	}

	c := normalizeCode(code)
	if c == "" {
		return nil, ErrEmptyCode
	}

	switch {
	case strings.HasPrefix(c, cs2Prefix) && len(c) == len(cs2Prefix)+cs2Chars:
		return decodeCS2(c[len(cs2Prefix):])
	case strings.HasPrefix(c, legacyPrefix) && len(c) == len(legacyPrefix)+legacyChars:
		return decodeLegacy(c[len(legacyPrefix):], refHeight)
	}
	return nil, ErrBadLength
}

// decodeCS2 解 cs2-v1（32 字节）。
func decodeCS2(body string) (*Crosshair, error) {
	b, err := charsToBytes(body, cs2Bytes)
	if err != nil {
		return nil, err
	}
	if b[0] != checksum(b) {
		return nil, ErrBadChecksum
	}
	if b[1] != 1 {
		return nil, ErrUnsupportedVersion
	}

	screenHeight := int(b[2]) | int(b[3])<<8
	if screenHeight == 0 {
		return nil, ErrBadScreenHeight
	}

	bits := uint32(b[18]) | uint32(b[19])<<8 | uint32(b[20])<<16 | uint32(b[21])<<24

	c := &Crosshair{
		Format:           FormatCS2V1,
		Style:            int(b[4] & 0x1f),
		FollowRecoil:     b[4]&0x20 != 0,
		Dot:              b[4]&0x40 != 0,
		TStyle:           b[4]&0x80 != 0,
		Color:            RGBA{R: int(b[5]), G: int(b[6]), B: int(b[7]), A: int(b[8])},
		OutlineColor:     RGBA{R: int(b[9]), G: int(b[10]), B: int(b[11]), A: int(b[12])},
		Thickness:        int(b[13] & 0x3f),
		OutlineMode:      int(b[13] >> 6),
		Gap:              int(int16(uint16(b[14]) | uint16(b[15])<<8)),
		Length:           int(b[16]),
		SpreadLimit:      int(b[17]),
		SplitDist:        int(bits & 0x7f),
		InnerAlpha:       float64((bits>>7)&0x7f) / 100,
		OuterAlpha:       float64((bits>>14)&0x7f)/100 + 0.3,
		SplitRatio:       float64((bits>>21)&0x7f) / 100,
		ScopeDotUseColor: (bits>>28)&1 == 1,
		ScreenHeight:     screenHeight,
		ScopeDotScale:    float64(b[22])/100 + 0.1,
	}
	c.Normalize()
	return c, nil
}

// decodeLegacy 解 legacy-v1 / v3 / v4（18 字节）。
func decodeLegacy(body string, refHeight int) (*Crosshair, error) {
	b, err := charsToBytes(body, legacyBytes)
	if err != nil {
		return nil, err
	}
	if b[0] != checksum(b) {
		return nil, ErrBadChecksum
	}

	switch b[1] {
	case 1:
		return decodeLegacyV1(b, refHeight), nil
	case 3, 4:
		return decodeLegacyV34(b), nil
	default:
		return nil, ErrUnsupportedVersion
	}
}

// legacyV1Preset 是 legacy-v1 的 color 索引 → RGB 映射。
//
// 索引 5 表示「自定义」，此时用码里存的 RGB；其余用预设色。
var legacyV1Preset = map[int]RGBA{
	0: {R: 0, G: 255, B: 0, A: 255},
	1: {R: 255, G: 255, B: 0, A: 255},
	2: {R: 0, G: 0, B: 255, A: 255},
	3: {R: 0, G: 255, B: 255, A: 255},
	4: {R: 255, G: 0, B: 0, A: 255},
}

// decodeLegacyV1 解 CS:GO 时代格式，并把「480 像素高屏幕的比例」近似换算成像素。
func decodeLegacyV1(b []byte, refHeight int) *Crosshair {
	// 尺寸以十分之一为单位，且是 480 像素高屏幕的比例。
	toPx := func(tenths int) int {
		ratio := float64(tenths) / 10
		return roundToInt(ratio * float64(refHeight) / 480)
	}

	b13 := b[13]
	colorIdx := int(b[10] & 7)
	color := RGBA{R: int(b[4]), G: int(b[5]), B: int(b[6]), A: int(b[7])}
	if preset, ok := legacyV1Preset[colorIdx]; ok {
		preset.A = color.A
		color = preset
	}

	outline := OutlineNone
	if b[10]&0x08 != 0 {
		outline = OutlineFull
	}

	c := &Crosshair{
		Format:       FormatLegacyV1,
		Gap:          toPx(int(int8(b[2]))),
		Length:       toPx(int(b[14])),
		Thickness:    toPx(int(b[12])),
		Color:        color,
		OutlineColor: RGBA{R: 0, G: 0, B: 0, A: 255},
		OutlineMode:  outline,
		SplitDist:    int(b[8] & 7),
		FollowRecoil: b[8]&0x80 != 0,
		InnerAlpha:   float64(b[10]>>4) / 10,
		OuterAlpha:   float64(b[11]&0x0f) / 10,
		SplitRatio:   float64(b[11]>>4) / 10,
		Style:        int((b13 & 0x0f) >> 1),
		Dot:          b13&0x10 != 0,
		TStyle:       b13&0x80 != 0,
		// legacy-v1 不存基准高度，换算后统一按 refHeight 标注。
		ScreenHeight:  refHeight,
		SpreadLimit:   SpreadLimitMax,
		ScopeDotScale: 1,
	}
	// 旧格式没有「半描边」概念；outerAlpha 下限是 0，需要抬到新制下限。
	if c.OuterAlpha < 0.3 {
		c.OuterAlpha = 0.3
	}
	c.Normalize()
	return c
}

// decodeLegacyV34 解 v3 / v4，两者尺寸已是像素，只差描边的表达方式。
func decodeLegacyV34(b []byte) *Crosshair {
	bits := uint32(b[10]) | uint32(b[11])<<8 | uint32(b[12])<<16 | uint32(b[13])<<24

	outline := OutlineNone
	if b[1] == 3 {
		// v3 用 byte2 bit5 的单布尔表示描边开关。
		if b[2]&0x20 != 0 {
			outline = OutlineFull
		}
	} else {
		// v4 起改为 0/1/2 三态，位域 bit28-29。
		outline = int((bits >> 28) & 3)
	}

	screenHeight := int(b[14]) | int(b[15])<<8
	if screenHeight == 0 {
		screenHeight = defaultRefHgt
	}

	c := &Crosshair{
		Style:         int(b[2] & 0x0f),
		FollowRecoil:  b[2]&0x10 != 0,
		Dot:           b[2]&0x40 != 0,
		TStyle:        b[2]&0x80 != 0,
		Color:         RGBA{R: int(b[3]), G: int(b[4]), B: int(b[5]), A: int(b[6])},
		OutlineColor:  RGBA{R: 0, G: 0, B: 0, A: 255},
		OutlineMode:   outline,
		Gap:           int(b[7]),
		Length:        int(b[8]),
		SpreadLimit:   int(b[9]),
		SplitDist:     int(bits & 0x7f),
		InnerAlpha:    float64((bits>>7)&0x1f) / 20,
		OuterAlpha:    float64((bits>>12)&0x0f)/20 + 0.3,
		SplitRatio:    float64((bits>>16)&0x7f) / 100,
		Thickness:     int((bits >> 23) & 0x1f),
		ScreenHeight:  screenHeight,
		ScopeDotScale: 1,
	}
	if b[1] == 3 {
		c.Format = FormatLegacyV3
	} else {
		c.Format = FormatLegacyV4
	}
	c.Normalize()
	return c
}

// Encode 把准星设置编码成当前可用的 cs2-v1 分享码。
//
// 旧容器已不被游戏接受，因此这里不提供 legacy 导出。
func Encode(c Crosshair) string {
	c.Normalize()

	b := make([]byte, cs2Bytes)
	b[1] = 1
	b[2] = byte(c.ScreenHeight & 0xff)
	b[3] = byte(c.ScreenHeight >> 8)
	b[4] = byte(c.Style&0x1f) | boolBit(c.FollowRecoil)<<5 | boolBit(c.Dot)<<6 | boolBit(c.TStyle)<<7
	b[5], b[6], b[7], b[8] = byte(c.Color.R), byte(c.Color.G), byte(c.Color.B), byte(c.Color.A)
	b[9], b[10], b[11], b[12] = byte(c.OutlineColor.R), byte(c.OutlineColor.G), byte(c.OutlineColor.B), byte(c.OutlineColor.A)
	b[13] = byte(c.Thickness&0x3f) | byte(c.OutlineMode&0x03)<<6

	g := int16(c.Gap)
	b[14] = byte(g)
	b[15] = byte(uint16(g) >> 8)

	b[16] = byte(c.Length)
	b[17] = byte(c.SpreadLimit)

	// 浮点陷阱：0.29*100 = 28.999999999999996，必须 round 而不是截断。
	inner := uint32(roundToInt(c.InnerAlpha * 100))
	outer := uint32(roundToInt(c.OuterAlpha*100) - 30)
	ratio := uint32(roundToInt(c.SplitRatio * 100))
	bits := uint32(c.SplitDist&0x7f) | inner<<7 | outer<<14 | ratio<<21
	if c.ScopeDotUseColor {
		bits |= 1 << 28
	}
	b[18], b[19], b[20], b[21] = byte(bits), byte(bits>>8), byte(bits>>16), byte(bits>>24)

	b[22] = byte(roundToInt(c.ScopeDotScale*100) - 10)

	b[0] = checksum(b)
	return cs2Prefix + bytesToChars(b, cs2Chars)
}

func boolBit(v bool) byte {
	if v {
		return 1
	}
	return 0
}

// ScreenHeightFromDisplay 把显示器高度换算成合法的基准高度。
func ScreenHeightFromDisplay(height int) int {
	if height < ScreenHeightMin {
		return defaultRefHgt
	}
	return int(math.Min(float64(height), ScreenHeightMax))
}
