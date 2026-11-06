package backup

import (
	"testing"
)

func TestIsDefaultGameCFG(t *testing.T) {
	defaultCases := []string{
		"boot.vcfg",
		"BOOT.VCFG",
		"gamemode_competitive.cfg",
		"GAMEMODE_CASUAL.CFG",
		"gamemode_armsrace.cfg",
		"server.cfg",
		"server_default.cfg",
		"machine_convars_default.vcfg",
		"pwa_userconfig76561198083722517.cfg",
		"user_keys_default.vcfg",
	}

	for _, c := range defaultCases {
		if !IsDefaultGameCFG(c) {
			t.Errorf("expected %s to be recognized as default game CFG", c)
		}
	}

	customCases := []string{
		"autoexec.cfg",
		"AUTOEXEC.CFG",
		"practice.cfg",
		"nadepractice.cfg",
		"bhop.cfg",
		"crosshair.cfg",
		"myconfig.vcfg",
		"custom.txt",
	}

	for _, c := range customCases {
		if IsDefaultGameCFG(c) {
			t.Errorf("expected %s to be recognized as custom CFG (not default)", c)
		}
	}
}

func TestFilterSelectedCFGFiles(t *testing.T) {
	input := []string{
		"C:\\configs\\autoexec.cfg",
		"C:\\configs\\binds.vcfg",
		"C:\\configs\\notes.txt",
		"C:\\configs\\trojan.exe",
		"C:\\configs\\script.bat",
		"C:\\configs\\data.json",
	}

	filtered := FilterSelectedCFGFiles(input)

	if len(filtered) != 3 {
		t.Fatalf("expected 3 allowed files, got %d: %v", len(filtered), filtered)
	}

	expected := map[string]bool{
		"C:\\configs\\autoexec.cfg": true,
		"C:\\configs\\binds.vcfg":    true,
		"C:\\configs\\notes.txt":     true,
	}

	for _, f := range filtered {
		if !expected[f] {
			t.Errorf("unexpected file in filtered output: %s", f)
		}
	}
}

// DefaultGameCFGs 只该装 Valve 的文件。
//
// 这张表是「Valve 兜底清单」：第三方平台的文件靠 ClassifyGameCFG 的前缀规则识别，
// 混进来就会两头都错——既被当成 Valve，又可能把带 SteamID 的文件名硬编码进源码。
func TestDefaultGameCFGs_OnlyValveFiles(t *testing.T) {
	for name := range DefaultGameCFGs {
		if got := ClassifyGameCFG(name); got != CFGSourceValve {
			t.Errorf("%s 在 DefaultGameCFGs 里，但分类结果是 %s（这张表只该放 Valve 的文件）", name, got)
		}
	}
}

// 文件名里带 SteamID 的第三方配置，必须靠前缀识别，不能靠硬编码具体 ID。
func TestClassifyGameCFG_PlatformFilesByPrefix(t *testing.T) {
	cases := map[string]string{
		// 任意 SteamID 都要命中，不依赖清单里写过谁
		"pwa_userconfig76561198083722517.cfg": CFGSourcePWA,
		"pwa_userconfig76561190000000001.cfg": CFGSourcePWA,
		"gamestate_integration_pwa.cfg":       CFGSourcePWA,
		"f5eautoexec.cfg":                     CFGSource5E,
		"f5enostickjump.cfg":                  CFGSource5E,
		// 对照组：Valve 的不能被误判
		"gamemode_competitive.cfg": CFGSourceValve,
		"boot.vcfg":                CFGSourceValve,
		// 对照组：玩家自己的还是玩家
		"practice.cfg":     CFGSourcePlayer,
		"my_pwa_notes.cfg": CFGSourcePlayer,
	}
	for name, want := range cases {
		if got := ClassifyGameCFG(name); got != want {
			t.Errorf("%s: got %s, want %s", name, got, want)
		}
	}
}
