//go:build windows

package video

import (
	"fmt"
	"sort"
	"syscall"
	"unsafe"
)

// DisplayMode 描述当前显示器支持的一种显示模式。
type DisplayMode struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	RefreshRate int `json:"refresh_rate"`
	BitsPerPel  int `json:"bits_per_pel"`
}

// devModeW 对应 Windows 的 DEVMODEW（大小 220 字节）。
// 字段顺序与 Win32 头文件一致，Go 的自然对齐刚好吻合，无需手工 padding。
type devModeW struct {
	DeviceName         [32]uint16
	SpecVersion        uint16
	DriverVersion      uint16
	Size               uint16
	DriverExtra        uint16
	Fields             uint32
	PositionX          int32
	PositionY          int32
	DisplayOrientation uint32
	DisplayFixedOutput uint32
	Color              int16
	Duplex             int16
	YResolution        int16
	TTOption           int16
	Collate            int16
	FormName           [32]uint16
	LogPixels          uint16
	BitsPerPel         uint32
	PelsWidth          uint32
	PelsHeight         uint32
	DisplayFlags       uint32
	DisplayFrequency   uint32
	ICMMethod          uint32
	ICMIntent          uint32
	MediaType          uint32
	DitherType         uint32
	Reserved1          uint32
	Reserved2          uint32
	PanningWidth       uint32
	PanningHeight      uint32
}

var (
	user32                   = syscall.NewLazyDLL("user32.dll")
	procEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
	procEnumDisplayDevicesW  = user32.NewProc("EnumDisplayDevicesW")
)

// GetDisplayModes 枚举当前显示器支持的全部显示模式（分辨率 + 刷新率）。
// 用于让「刷新率」下拉展示本机真实支持的档位，而不是一份固定列表。
func GetDisplayModes() ([]DisplayMode, error) {
	if err := procEnumDisplaySettingsW.Find(); err != nil {
		return nil, fmt.Errorf("当前系统不支持显示模式枚举: %w", err)
	}

	seen := make(map[string]bool)
	modes := make([]DisplayMode, 0, 64)

	for i := 0; i < 512; i++ {
		var dm devModeW
		dm.Size = uint16(unsafe.Sizeof(dm))
		r, _, _ := procEnumDisplaySettingsW.Call(0, uintptr(i), uintptr(unsafe.Pointer(&dm)))
		if r == 0 {
			break
		}
		if dm.PelsWidth == 0 || dm.PelsHeight == 0 || dm.DisplayFrequency == 0 {
			continue
		}
		key := fmt.Sprintf("%dx%d@%d", dm.PelsWidth, dm.PelsHeight, dm.DisplayFrequency)
		if seen[key] {
			continue
		}
		seen[key] = true
		modes = append(modes, DisplayMode{
			Width:       int(dm.PelsWidth),
			Height:      int(dm.PelsHeight),
			RefreshRate: int(dm.DisplayFrequency),
			BitsPerPel:  int(dm.BitsPerPel),
		})
	}

	// 分辨率降序、同分辨率下刷新率降序，方便前端直接取「最高刷新率」
	sort.SliceStable(modes, func(i, j int) bool {
		if modes[i].Width != modes[j].Width {
			return modes[i].Width > modes[j].Width
		}
		if modes[i].Height != modes[j].Height {
			return modes[i].Height > modes[j].Height
		}
		return modes[i].RefreshRate > modes[j].RefreshRate
	})

	return modes, nil
}
