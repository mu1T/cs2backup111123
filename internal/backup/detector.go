package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cs2backup/internal/steam"
)

// DetectCS2CFGPath locates the CS2 game cfg directory (Counter-Strike Global Offensive/game/csgo/cfg)
// by inspecting steamapps/libraryfolders.vdf and falling back to the standard steamapps/common path.
func DetectCS2CFGPath(steamPath string) (string, error) {
	if steamPath == "" {
		return "", fmt.Errorf("steam installation path is empty")
	}

	cleanSteam := filepath.Clean(steamPath)
	if fi, err := os.Stat(cleanSteam); err != nil || !fi.IsDir() {
		return "", fmt.Errorf("invalid steam path: %s", cleanSteam)
	}

	var candidates []string
	seen := make(map[string]bool)

	addCandidate := func(p string) {
		clean := filepath.Clean(p)
		if clean != "" && !seen[clean] {
			seen[clean] = true
			candidates = append(candidates, clean)
		}
	}

	// 1. Inspect steamapps/libraryfolders.vdf
	vdfPath := filepath.Join(cleanSteam, "steamapps", "libraryfolders.vdf")
	if _, err := os.Stat(vdfPath); err == nil {
		if folders, err := steam.ParseLibraryFoldersFile(vdfPath); err == nil {
			// Prioritize libraries that explicitly contain AppID 730
			for _, folder := range folders {
				if folder.Path != "" {
					if _, has730 := folder.Apps["730"]; has730 {
						addCandidate(folder.Path)
					}
				}
			}
			// Add any other libraries
			for _, folder := range folders {
				if folder.Path != "" {
					addCandidate(folder.Path)
				}
			}
		}
	}

	// 2. Always include steamPath as candidate
	addCandidate(cleanSteam)

	// 3. Search candidate libraries for CS2 cfg directories
	for _, libPath := range candidates {
		checkPaths := []string{
			filepath.Join(libPath, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg"),
			filepath.Join(libPath, "steamapps", "common", "Counter-Strike Global Offensive", "csgo", "cfg"),
		}

		for _, p := range checkPaths {
			if fi, err := os.Stat(p); err == nil && fi.IsDir() {
				return p, nil
			}
		}

		// Also check if appmanifest_730.acf exists in library
		manifestPath := filepath.Join(libPath, "steamapps", "appmanifest_730.acf")
		if _, err := os.Stat(manifestPath); err == nil {
			for _, p := range checkPaths {
				if fi, err := os.Stat(p); err == nil && fi.IsDir() {
					return p, nil
				}
			}
		}
	}

	// 4. Default fallback path
	fallback := filepath.Join(cleanSteam, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	if fi, err := os.Stat(fallback); err == nil && fi.IsDir() {
		return fallback, nil
	}

	return "", fmt.Errorf("无法找到 CS2 安装位置或 CFG 文件夹 (Steam 目录: %s)", cleanSteam)
}

// NormalizeCS2CFGPath 把用户随手选中的目录归一成 CS2 的 game/csgo/cfg。
//
// 用户可能选中的层级千差万别：CS2 安装目录、Counter-Strike Global Offensive、
// game/csgo、直接就是 cfg，甚至 steamapps/common。这里按由深到浅的顺序试，
// 返回第一个「确实像个 cfg 目录」的候选，避免把路径写错层级却毫无提示。
func NormalizeCS2CFGPath(raw string) (string, error) {
	clean := filepath.Clean(strings.TrimSpace(raw))
	if clean == "" || clean == "." {
		return "", fmt.Errorf("路径不能为空")
	}

	candidates := []string{
		clean,
		filepath.Join(clean, "cfg"),
		filepath.Join(clean, "csgo", "cfg"),
		filepath.Join(clean, "game", "csgo", "cfg"),
		filepath.Join(clean, "Counter-Strike Global Offensive", "game", "csgo", "cfg"),
		filepath.Join(clean, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg"),
	}

	for _, c := range candidates {
		if looksLikeCS2CFGDir(c) {
			return c, nil
		}
	}

	return "", fmt.Errorf("这个目录看起来不是 CS2 的配置目录：%s\n请选择 game/csgo/cfg，或 Counter-Strike Global Offensive 安装目录", clean)
}

// looksLikeCS2CFGDir 判断目录是否像 CS2 的 cfg 目录。
//
// 判据：目录存在，且（目录名就是 cfg，或者里面至少有一个 .cfg 文件）。
// 后者能挡住「选了个随便的空文件夹」这种误操作。
func looksLikeCS2CFGDir(dir string) bool {
	fi, err := os.Stat(dir)
	if err != nil || !fi.IsDir() {
		return false
	}
	if strings.EqualFold(filepath.Base(dir), "cfg") {
		return true
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ".cfg") {
			return true
		}
	}
	return false
}
