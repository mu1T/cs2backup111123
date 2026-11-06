package backup

import (
	"path/filepath"
	"strings"
)

// AllowedSelectedCFGExts defines valid file extensions for selective CFG backup.
var AllowedSelectedCFGExts = map[string]bool{
	".cfg":  true,
	".vcfg": true,
	".txt":  true,
}

// DefaultGameCFGs 是 **Valve 官方**下发文件的兜底清单，供「仅备份自定义」判定用。
//
// 这里**只放 Valve 的文件**。第三方平台（完美 / 5E）的文件一律靠 `ClassifyGameCFG`
// 里的前缀规则识别，不要往这张表里塞：
//   - 5E 的 `f5eautoexec.cfg` / `f5enostickjump.cfg` 由 `f5e` 前缀命中；
//   - 完美的 `pwa_userconfig<SteamID64>.cfg` / `gamestate_integration_pwa.cfg`
//     由 `pwa_` / `gamestate_integration_pwa` 前缀命中。
//
// 以前这里混进了 3 个 `pwa_userconfig<SteamID64>.cfg`（还带着真实的 SteamID），
// 既是错误归类（它们不是 Valve 的默认配置），也把用户隐私写进了源码。
// 文件名里带 SteamID 的写法尤其不该硬编码——前缀规则对所有 ID 都成立。
var DefaultGameCFGs = map[string]bool{
	"boot.vcfg":                           true,
	"gamemode_armsrace.cfg":               true,
	"gamemode_casual.cfg":                 true,
	"gamemode_competitive.cfg":            true,
	"gamemode_competitive2v2.cfg":         true,
	"gamemode_competitive2v2_offline.cfg": true,
	"gamemode_competitive_offline.cfg":    true,
	"gamemode_competitive_short.cfg":      true,
	"gamemode_competitive_tmm.cfg":        true,
	"gamemode_cooperative.cfg":            true,
	"gamemode_coopmission.cfg":            true,
	"gamemode_custom.cfg":                 true,
	"gamemode_deathmatch.cfg":             true,
	"gamemode_deathmatch_short.cfg":       true,
	"gamemode_deathmatch_tmm.cfg":         true,
	"gamemode_demolition.cfg":             true,
	"gamemode_dm_freeforall.cfg":          true,
	"gamemode_new_user_training.cfg":      true,
	"gamemode_retakecasual.cfg":           true,
	"gamemode_teamdeathmatch.cfg":         true,
	"gamemode_workshop.cfg":               true,
	"machine_convars_default.vcfg":        true,
	"perftest.cfg":                        true,
	"server.cfg":                          true,
	"server_default.cfg":                  true,
	"user_keys_default.vcfg":              true,
	"user_keys_dev_default.vcfg":          true,
}

// IsDefaultGameCFG returns true when the file is NOT written by the player,
// i.e. it comes from Valve, a third-party platform (5E) or Perfect World Arena.
// 分类逻辑集中在 ClassifyGameCFG，避免「扫描结果」与「仅备份自定义」两套判定漂移。
func IsDefaultGameCFG(filename string) bool {
	return ClassifyGameCFG(filename) != CFGSourcePlayer
}

// FilterSelectedCFGFiles filters the input list to only include existing files with allowed extensions (.cfg, .vcfg, .txt).
func FilterSelectedCFGFiles(files []string) []string {
	var filtered []string
	for _, f := range files {
		clean := filepath.Clean(f)
		ext := strings.ToLower(filepath.Ext(clean))
		if AllowedSelectedCFGExts[ext] {
			filtered = append(filtered, clean)
		}
	}
	return filtered
}
