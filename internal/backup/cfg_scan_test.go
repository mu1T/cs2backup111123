package backup

import "testing"

func TestClassifyGameCFG(t *testing.T) {
	cases := map[string]string{
		// 完美世界竞技平台
		"pwa.cfg":                             CFGSourcePWA,
		"pwa_userconfig76561198083722517.cfg": CFGSourcePWA,
		"PWA_UserConfig76561198515265839.CFG": CFGSourcePWA,
		"gamestate_integration_pwa.cfg":       CFGSourcePWA,
		// 5E 对战平台（f5e / 5e 前缀）
		"f5eautoexec.cfg":              CFGSource5E,
		"f5enostickjump.cfg":           CFGSource5E,
		"F5E_UserConfig76561198.CFG":   CFGSource5E,
		"5e_bind.cfg":                  CFGSource5E,
		"gamestate_integration_5e.cfg": CFGSource5E,
		// Valve 官方
		"gamemode_competitive.cfg":     CFGSourceValve,
		"gamemode_retakecasual.cfg":    CFGSourceValve,
		"GAMEMODE_CASUAL.CFG":          CFGSourceValve,
		"boot.vcfg":                    CFGSourceValve,
		"server.cfg":                   CFGSourceValve,
		"server_default.cfg":           CFGSourceValve,
		"perftest.cfg":                 CFGSourceValve,
		"machine_convars_default.vcfg": CFGSourceValve,
		"user_keys_default.vcfg":       CFGSourceValve,
		"user_keys_dev_default.vcfg":   CFGSourceValve,
		// 玩家自定义
		"autoexec.cfg":  CFGSourcePlayer,
		"surf.cfg":      CFGSourcePlayer,
		"pt.cfg":        CFGSourcePlayer,
		"myconfig.vcfg": CFGSourcePlayer,
		"crosshair.txt": CFGSourcePlayer,
	}

	for name, want := range cases {
		if got := ClassifyGameCFG(name); got != want {
			t.Errorf("ClassifyGameCFG(%q) = %q, want %q", name, got, want)
		}
	}
}

// 5E 注入的配置必须独立归类，不能再混进「第三方平台」桶里。
func TestClassifyGameCFG_5EPrefixIsNotPlatform(t *testing.T) {
	names := []string{
		"f5eautoexec.cfg",
		"f5enostickjump.cfg",
		"f5e.cfg",
		"F5E_UserConfig.cfg",
		"5e_bind.cfg",
		"5eplay.cfg",
		"gamestate_integration_5e.cfg",
	}
	for _, name := range names {
		got := ClassifyGameCFG(name)
		if got == CFGSourcePlatform {
			t.Errorf("ClassifyGameCFG(%q) = %q，5E 文件不应再算作第三方平台", name, got)
		}
		if got != CFGSource5E {
			t.Errorf("ClassifyGameCFG(%q) = %q, want %q", name, got, CFGSource5E)
		}
	}
}

func TestIsDefaultGameCFGMatchesClassifier(t *testing.T) {
	// 「仅备份自定义」必须与扫描分类保持一致：非玩家文件一律视为默认
	for _, name := range []string{
		"pwa.cfg",
		"pwa_userconfig76561198083722517.cfg",
		"f5eautoexec.cfg",
		"gamemode_casual.cfg",
		"autoexec.cfg",
		"surf.cfg",
	} {
		want := ClassifyGameCFG(name) != CFGSourcePlayer
		if got := IsDefaultGameCFG(name); got != want {
			t.Errorf("IsDefaultGameCFG(%q) = %v, want %v", name, got, want)
		}
	}
}
