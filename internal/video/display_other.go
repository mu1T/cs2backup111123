//go:build !windows

package video

import "fmt"

// DisplayMode 描述当前显示器支持的一种显示模式。
type DisplayMode struct {
	Width       int `json:"width"`
	Height      int `json:"height"`
	RefreshRate int `json:"refresh_rate"`
	BitsPerPel  int `json:"bits_per_pel"`
}

// GetDisplayModes 在非 Windows 平台不可用。
func GetDisplayModes() ([]DisplayMode, error) {
	return nil, fmt.Errorf("display mode enumeration is only supported on Windows")
}
