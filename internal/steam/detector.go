package steam

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

var (
	// ErrSteamNotFound is returned when no valid Steam installation can be detected.
	ErrSteamNotFound = errors.New("steam installation not found")
	// ErrInvalidSteamPath is returned when the given path does not contain Steam structure.
	ErrInvalidSteamPath = errors.New("invalid steam installation path")
)

// ValidateSteamPath checks whether a given directory is a valid Steam installation path.
func ValidateSteamPath(path string) error {
	if path == "" {
		return fmt.Errorf("%w: path is empty", ErrInvalidSteamPath)
	}

	cleanPath := filepath.Clean(path)
	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("%w: directory does not exist: %v", ErrInvalidSteamPath, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: path is not a directory", ErrInvalidSteamPath)
	}

	// Check for indicators of a Steam directory
	steamExe := filepath.Join(cleanPath, "steam.exe")
	userdataDir := filepath.Join(cleanPath, "userdata")
	loginUsersVDF := filepath.Join(cleanPath, "config", "loginusers.vdf")

	if _, err := os.Stat(steamExe); err == nil {
		return nil
	}
	if info, err := os.Stat(userdataDir); err == nil && info.IsDir() {
		return nil
	}
	if _, err := os.Stat(loginUsersVDF); err == nil {
		return nil
	}

	return fmt.Errorf("%w: neither steam.exe, userdata nor loginusers.vdf found in %s", ErrInvalidSteamPath, cleanPath)
}

// detectSteamPathFn 是自动探测的实现。测试注入一个确定结果，
// 免得用例隐式依赖宿主机装没装 Steam。
var detectSteamPathFn = DetectSteamPath

// SetSteamPathDetectorForTesting 替换自动探测实现，返回恢复函数。
func SetSteamPathDetectorForTesting(fn func() (string, error)) func() {
	prev := detectSteamPathFn
	detectSteamPathFn = fn
	return func() { detectSteamPathFn = prev }
}

// DetectSteamPath tries to automatically find the Steam directory via registry then common drive fallbacks.
func DetectSteamPath() (string, error) {
	// 1. Try Windows Registry
	if regPath, err := detectSteamPathRegistry(); err == nil && regPath != "" {
		clean := filepath.Clean(regPath)
		if err := ValidateSteamPath(clean); err == nil {
			return clean, nil
		}
	}

	// 2. Fallback drive scan
	drives := []string{"C", "D", "E", "F", "G", "H"}
	subPaths := []string{
		filepath.Join("Program Files (x86)", "Steam"),
		filepath.Join("Program Files", "Steam"),
		"Steam",
	}

	for _, d := range drives {
		for _, sp := range subPaths {
			candidate := filepath.Join(d+":\\", sp)
			if err := ValidateSteamPath(candidate); err == nil {
				return candidate, nil
			}
		}
	}

	return "", ErrSteamNotFound
}
