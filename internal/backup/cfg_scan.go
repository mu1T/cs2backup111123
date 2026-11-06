package backup

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Game CFG 文件来源分类。
// 分类直接决定前端展示与「仅备份自定义」的取舍，因此放在后端统一判定，
// 避免前端再维护一份易过期的硬编码清单。
const (
	CFGSourceValve    = "valve"    // Valve 随游戏下发的官方配置
	CFGSourcePWA      = "pwa"      // 完美世界竞技平台（PWA）生成
	CFGSource5E       = "fivee"    // 5E 对战平台（5EPlay）注入
	CFGSourcePlatform = "platform" // 其他第三方对战平台生成（FACEIT 等，当前无已知前缀）
	CFGSourcePlayer   = "player"   // 玩家自己编写 / 导入
)

// GameCFGFile 描述 game/csgo/cfg 目录下的单个文件。
type GameCFGFile struct {
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	Modified string `json:"modified"`
	Ext      string `json:"ext"`
	Source   string `json:"source"`
	ReadOnly bool   `json:"read_only"`
}

// valveGameCFGExact 列出 Valve 官方下发、但不带通用前缀的固定文件名。
var valveGameCFGExact = map[string]bool{
	"boot.vcfg":                    true,
	"server.cfg":                   true,
	"server_default.cfg":           true,
	"perftest.cfg":                 true,
	"machine_convars_default.vcfg": true,
	"user_keys_default.vcfg":       true,
	"user_keys_dev_default.vcfg":   true,
}

// fiveeGameCFGPrefixes 5E 对战平台（5EPlay）注入的别名脚本与集成配置。
// 早期版本把 f5e 归到「其他第三方平台」，导致 5E 的污染文件与真正的自定义脚本难以区分，
// 现已单列一档；判定顺序上 5E 先于 platform。
var fiveeGameCFGPrefixes = []string{
	"f5e", // 5E 注入的别名脚本（f5eautoexec.cfg / f5enostickjump.cfg）
	"5e",
	"gamestate_integration_5e",
}

// platformGameCFGPrefixes 其他第三方对战平台的前缀。
// 目前没有已知会往 game/csgo/cfg 写文件的平台（完美走 pwaGameCFGPrefixes，5E 走 fiveeGameCFGPrefixes）。
// 保留「第三方平台」这一档是为了让概念有落点，出现新平台时往这里追加前缀即可。
var platformGameCFGPrefixes = []string{}

// pwaGameCFGPrefixes 完美世界竞技平台（Perfect World Arena）的文件前缀。
var pwaGameCFGPrefixes = []string{
	"pwa_",
	"pwa.",
	"gamestate_integration_pwa",
}

// ClassifyGameCFG 根据文件名判断该配置来自哪里。
func ClassifyGameCFG(filename string) string {
	lower := strings.ToLower(filepath.Base(filename))

	for _, p := range pwaGameCFGPrefixes {
		if strings.HasPrefix(lower, p) {
			return CFGSourcePWA
		}
	}

	for _, p := range fiveeGameCFGPrefixes {
		if strings.HasPrefix(lower, p) {
			return CFGSource5E
		}
	}

	for _, p := range platformGameCFGPrefixes {
		if strings.HasPrefix(lower, p) {
			return CFGSourcePlatform
		}
	}

	if valveGameCFGExact[lower] {
		return CFGSourceValve
	}

	// 历史清单兜底：旧版本记录过但未覆盖在固定表里的 Valve 文件
	if DefaultGameCFGs[lower] {
		return CFGSourceValve
	}

	// Valve 官方模式脚本统一以 gamemode_ 开头
	if strings.HasPrefix(lower, "gamemode_") && strings.HasSuffix(lower, ".cfg") {
		return CFGSourceValve
	}

	// 引擎生成的 *_default 模板
	if strings.HasSuffix(lower, "_default.vcfg") || strings.HasSuffix(lower, "_default.cfg") {
		return CFGSourceValve
	}

	return CFGSourcePlayer
}

// ListUserCFGFiles 扫描某个账号的 userdata/<accountID>/730/local/cfg，
// 用于「配置文件编辑器」让用户直接从真实文件里选，而不是手输文件名。
func (s *Service) ListUserCFGFiles(accountID string) ([]GameCFGFile, error) {
	if accountID == "" {
		return nil, fmt.Errorf("账号 ID 为空")
	}

	s.mu.RLock()
	sp := s.steamPath
	s.mu.RUnlock()

	if sp == "" {
		return nil, fmt.Errorf("steam installation path is not set")
	}

	dir := filepath.Join(sp, "userdata", accountID, "730", "local", "cfg")
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("账号 [%s] 的配置目录不存在: %s", accountID, dir)
		}
		return nil, fmt.Errorf("读取账号配置目录失败: %w", err)
	}

	files := make([]GameCFGFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !AllowedSelectedCFGExts[ext] {
			// 过滤掉 cs2_user_keys_0_slot0.vcfg_lastclouded 之类的云同步副本
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, GameCFGFile{
			Name:     name,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
			Ext:      ext,
			Source:   CFGSourcePlayer,
			ReadOnly: info.Mode().Perm()&0200 == 0,
		})
	}

	sort.SliceStable(files, func(i, j int) bool {
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}

// ListGameCFGFiles 扫描 game/csgo/cfg 目录，返回全部可备份的配置文件（不含子目录）。
// 结果按「来源优先级 + 文件名」排序，便于前端稳定展示。
func (s *Service) ListGameCFGFiles() ([]GameCFGFile, error) {
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(cfgDir)
	if err != nil {
		return nil, fmt.Errorf("读取 CS2 cfg 目录失败: %w", err)
	}

	order := map[string]int{
		CFGSourcePlayer:   0,
		CFGSourcePWA:      1,
		CFGSource5E:       2,
		CFGSourcePlatform: 3,
		CFGSourceValve:    4,
	}

	files := make([]GameCFGFile, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		ext := strings.ToLower(filepath.Ext(name))
		if !AllowedSelectedCFGExts[ext] {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		files = append(files, GameCFGFile{
			Name:     name,
			Size:     info.Size(),
			Modified: info.ModTime().Format(time.RFC3339),
			Ext:      ext,
			Source:   ClassifyGameCFG(name),
			ReadOnly: info.Mode().Perm()&0200 == 0,
		})
	}

	sort.SliceStable(files, func(i, j int) bool {
		oi, oj := order[files[i].Source], order[files[j].Source]
		if oi != oj {
			return oi < oj
		}
		return strings.ToLower(files[i].Name) < strings.ToLower(files[j].Name)
	})

	return files, nil
}
