package backup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cs2backup/internal/atomicfile"
)

// resolveConfigFilePath validates accountID and relPath against path traversal attacks
// and returns the sanitized absolute file path within either CS2 game cfg or Steam userdata cfg.
func (s *Service) resolveConfigFilePath(accountID, relPath string) (string, error) {
	if strings.TrimSpace(accountID) == "" {
		return "", errors.New("accountID cannot be empty")
	}
	if strings.TrimSpace(relPath) == "" {
		return "", errors.New("relPath cannot be empty")
	}

	// 1. Strict relPath validation
	if strings.ContainsRune(relPath, 0) {
		return "", errors.New("relPath contains invalid null byte")
	}
	// Check for leading slashes or Windows drive specifier
	if filepath.IsAbs(relPath) || strings.HasPrefix(relPath, "/") || strings.HasPrefix(relPath, "\\") {
		return "", fmt.Errorf("relPath must be a relative path: %s", relPath)
	}
	if len(relPath) >= 2 && relPath[1] == ':' {
		return "", fmt.Errorf("relPath must not contain a drive letter: %s", relPath)
	}

	cleanRel := filepath.Clean(relPath)
	if cleanRel == "." || cleanRel == ".." || strings.HasPrefix(cleanRel, ".."+string(filepath.Separator)) || strings.HasPrefix(cleanRel, "../") || strings.HasPrefix(cleanRel, `..\`) {
		return "", fmt.Errorf("path traversal rejected: %s", relPath)
	}

	// 2. Base directory resolution and accountID validation
	var baseDir string
	if accountID == "game_cfg" {
		cfgDir, err := s.GetCS2CFGPath()
		if err != nil {
			return "", fmt.Errorf("failed to locate CS2 game cfg path: %w", err)
		}
		if cfgDir == "" {
			return "", errors.New("CS2 game cfg path is not set")
		}
		baseDir = cfgDir
	} else {
		// Strict accountID validation
		cleanAccountID := filepath.Clean(accountID)
		if cleanAccountID != accountID || strings.ContainsAny(accountID, `/\:..`) {
			return "", fmt.Errorf("invalid accountID: %s", accountID)
		}

		s.mu.RLock()
		sp := s.steamPath
		s.mu.RUnlock()
		if sp == "" {
			return "", errors.New("steam installation path is not set")
		}

		baseDir = filepath.Join(sp, "userdata", cleanAccountID, "730", "local", "cfg")
	}

	// 3. Ensure target is strictly inside baseDir
	absBase, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute base path: %w", err)
	}
	targetPath := filepath.Join(absBase, cleanRel)
	absTarget, err := filepath.Abs(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to get absolute target path: %w", err)
	}

	rel, err := filepath.Rel(absBase, absTarget)
	if err != nil || strings.HasPrefix(rel, "..") || rel == "." {
		return "", fmt.Errorf("resolved path outside allowed configuration directory: %s", relPath)
	}

	return absTarget, nil
}

// ReadConfigFile safely reads a configuration text file targeting either
// userdata/<accountID>/730/local/cfg/<relPath> or game/csgo/cfg/<relPath> (when accountID == "game_cfg").
func (s *Service) ReadConfigFile(accountID, relPath string) (string, error) {
	targetPath, err := s.resolveConfigFilePath(accountID, relPath)
	if err != nil {
		return "", err
	}

	fi, err := os.Stat(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read config file %s: %w", relPath, err)
	}
	if fi.IsDir() {
		return "", fmt.Errorf("target path is a directory: %s", relPath)
	}

	data, err := os.ReadFile(targetPath)
	if err != nil {
		return "", fmt.Errorf("failed to read file contents: %w", err)
	}

	return string(data), nil
}

// SaveConfigFileText safely writes content with 0644 permissions to either
// userdata/<accountID>/730/local/cfg/<relPath> or game/csgo/cfg/<relPath> (when accountID == "game_cfg").
func (s *Service) SaveConfigFileText(accountID, relPath, content string) error {
	targetPath, err := s.resolveConfigFilePath(accountID, relPath)
	if err != nil {
		return err
	}

	// 建目录、解只读、临时文件 + rename 都在 atomicfile 里。
	// cfg 是游戏加载时才读的内容，写一半崩掉会让游戏读到半截配置，
	// 所以优先原子写；目标被占用时它会自动退回原地覆盖。
	if _, err := atomicfile.Write(targetPath, []byte(content), atomicfile.Options{}); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", relPath, err)
	}

	return nil
}

// windowsReservedNames 是 Windows 不允许用作文件名的设备名。
// 带后缀也不行（`CON.cfg` 一样建不出来），所以校验时比较的是去掉后缀的主名。
var windowsReservedNames = map[string]bool{
	"CON": true, "PRN": true, "AUX": true, "NUL": true,
	"COM1": true, "COM2": true, "COM3": true, "COM4": true, "COM5": true,
	"COM6": true, "COM7": true, "COM8": true, "COM9": true,
	"LPT1": true, "LPT2": true, "LPT3": true, "LPT4": true, "LPT5": true,
	"LPT6": true, "LPT7": true, "LPT8": true, "LPT9": true,
}

// 文件名里不允许出现的字符：路径分隔符、Windows 保留字符、控制字符。
const illegalNameChars = `/\:*?"<>|`

// NormalizeNewCFGName 校验并规范化用户输入的新建文件名，返回最终的文件名。
//
// 抽成纯函数是为了能把 Windows 那一堆边角情况测干净——这些规则一旦漏掉，
// 表现是「点了新建没反应」或者「建出来的名字被系统悄悄截掉」，很难排查。
//
// 规则：
//   - 必须是非空；
//   - 不许含路径分隔符或 Windows 保留字符（别让人跳出 cfg 目录）；
//   - 不许是 Windows 设备名（CON / NUL / COM1 …），带后缀也不行；
//   - 不许以点或空格结尾（Windows 会静默截掉，导致名字和用户想的不一样）；
//   - 只接受 .cfg，没写后缀就自动补上，写了别的后缀直接报错（避免
//     `practice.txt.cfg` 这种莫名其妙的结果）。
func NormalizeNewCFGName(raw string) (string, error) {
	name := strings.TrimSpace(raw)
	if name == "" {
		return "", errors.New("文件名不能为空")
	}
	if len(name) > 100 {
		return "", errors.New("文件名太长（最多 100 个字符）")
	}
	if strings.ContainsAny(name, illegalNameChars) {
		return "", fmt.Errorf("文件名不能包含这些字符：%s", illegalNameChars)
	}
	for _, r := range name {
		if r < 32 {
			return "", errors.New("文件名不能包含控制字符")
		}
	}
	if strings.HasSuffix(name, ".") || strings.HasSuffix(name, " ") {
		return "", errors.New("文件名不能以点或空格结尾")
	}

	// 后缀：有就只认 .cfg，没有就补上
	ext := strings.ToLower(filepath.Ext(name))
	base := name
	if ext != "" {
		if ext != ".cfg" {
			return "", fmt.Errorf("只支持 .cfg 文件，不支持 %s", ext)
		}
		base = name[:len(name)-len(ext)]
	}
	if strings.TrimSpace(base) == "" {
		return "", errors.New("文件名不能只有后缀")
	}
	if windowsReservedNames[strings.ToUpper(base)] {
		return "", fmt.Errorf("%s 是 Windows 保留名，不能用作文件名", base)
	}

	return base + ".cfg", nil
}

// CreateConfigFile 在指定配置目录下新建一个空的 .cfg 文件，返回最终文件名。
//
// 与 SaveConfigFileText 的关键区别：**这个只能新建，绝不覆盖**。
// 保存是覆盖写，如果新建也走那条路，用户输入一个已存在的名字（最典型是
// autoexec）就会把原配置清空——整个工具的前提就是不丢配置。
//
// accountID 传 "game_cfg" 时落在 game/csgo/cfg，否则落在该账号的 userdata cfg 目录。
func (s *Service) CreateConfigFile(accountID, rawName string) (string, error) {
	name, err := NormalizeNewCFGName(rawName)
	if err != nil {
		return "", err
	}

	targetPath, err := s.resolveConfigFilePath(accountID, name)
	if err != nil {
		return "", err
	}

	// 先按目录里现有的名字比一遍（忽略大小写）：Windows 文件系统不区分大小写，
	// `Autoexec.cfg` 和 `autoexec.cfg` 是同一个文件，直接报错比让 O_EXCL 抛
	// 一个系统错误更好懂。
	if entries, err := os.ReadDir(filepath.Dir(targetPath)); err == nil {
		for _, entry := range entries {
			if strings.EqualFold(entry.Name(), name) {
				return "", fmt.Errorf("%s 已存在，换个名字", entry.Name())
			}
		}
	}

	// O_EXCL 是真正的兜底：上面那轮扫描和这里之间理论上有竞态。
	f, err := os.OpenFile(targetPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0644)
	if err != nil {
		if os.IsExist(err) {
			return "", fmt.Errorf("%s 已存在，换个名字", name)
		}
		return "", fmt.Errorf("新建配置文件失败: %w", err)
	}
	if err := f.Close(); err != nil {
		return "", fmt.Errorf("新建配置文件失败: %w", err)
	}

	return name, nil
}
