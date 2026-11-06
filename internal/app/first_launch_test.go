package app

import (
	"os"
	"path/filepath"
	"testing"

	"cs2backup/internal/steam"
	"cs2backup/internal/testutil"
)

// fakeSteamInstall 造一个结构齐全的 Steam 安装目录，返回 (steam 目录, 游戏 cfg 目录)。
func fakeSteamInstall(t *testing.T, accountIDs ...string) (string, string) {
	t.Helper()
	steamDir := testutil.TempDir(t)

	for _, id := range accountIDs {
		_ = os.MkdirAll(filepath.Join(steamDir, "userdata", id, "730", "local", "cfg"), 0o755)
	}
	gameCfg := filepath.Join(steamDir, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	_ = os.MkdirAll(gameCfg, 0o755)
	return steamDir, gameCfg
}

/**
 * 初次启动：配置目录里没有保存过 Steam 路径，但机器上装了 Steam。
 * 这时应当自动探测一次，并且「用上」——不只是账号列表能出来，
 * 备份服务那边的游戏 cfg 目录也得跟着有值。
 *
 * 这是用户报的那个症状：首启后路径栏是空的，得手点「自动检测」才正常。
 */
func TestFirstLaunchDetectsAndUsesSteamPath(t *testing.T) {
	steamDir, gameCfg := fakeSteamInstall(t, "123456789")

	restore := steam.SetSteamPathDetectorForTesting(func() (string, error) {
		return steamDir, nil
	})
	defer restore()

	configDir := testutil.TempDir(t)
	svc := NewAppServiceWithConfigDir(configDir)
	defer svc.WaitBackground()

	// app.startup 里做的就是这一步
	svc.EnsureSteamPath()

	// 1. 界面首屏读的就是这个
	if got := svc.GetSteamPath(); got != steamDir {
		t.Errorf("首启 GetSteamPath 应为 %q，实际 %q", steamDir, got)
	}

	// 2. 备份服务必须同步拿到，否则游戏 cfg 目录、快照、准星全都用不了
	got, err := svc.GetCS2CFGPath()
	if err != nil {
		t.Errorf("首启 GetCS2CFGPath 失败: %v", err)
	}
	if got != gameCfg {
		t.Fatalf("首启 GetCS2CFGPath 应为 %q，实际 %q", gameCfg, got)
	}

	// 3. 探测结果要落盘，第二次启动不该再探一遍
	cm, err := steam.NewCacheManager(configDir)
	if err != nil {
		t.Fatalf("NewCacheManager failed: %v", err)
	}
	if saved := cm.GetSavedSteamPath(); saved != steamDir {
		t.Fatalf("探测结果应写入配置，实际保存的是 %q", saved)
	}
}

/**
 * 已经保存过路径时不该再探一遍：探测要读注册表 + 扫盘，
 * 而且用户手动指定的路径可能不在常规位置，扫也扫不到，
 * 探出别的目录再覆盖回去就是灾难。
 */
func TestEnsureSteamPathSkipsWhenAlreadyConfigured(t *testing.T) {
	steamDir, _ := fakeSteamInstall(t, "123456789")

	restore := steam.SetSteamPathDetectorForTesting(func() (string, error) {
		t.Errorf("已经配好路径时不该再探测")
		return "", steam.ErrSteamNotFound
	})
	defer restore()

	configDir := testutil.TempDir(t)
	cm, err := steam.NewCacheManager(configDir)
	if err != nil {
		t.Fatalf("NewCacheManager failed: %v", err)
	}
	if err := cm.SaveSteamPath(steamDir); err != nil {
		t.Fatalf("SaveSteamPath failed: %v", err)
	}

	svc := NewAppServiceWithConfigDir(configDir)
	defer svc.WaitBackground()
	svc.EnsureSteamPath()

	if got := svc.GetSteamPath(); got != steamDir {
		t.Fatalf("应保留已保存的路径 %q，实际 %q", steamDir, got)
	}
}

/** 机器上没装 Steam 时，探测失败不能阻断启动。 */
func TestEnsureSteamPathToleratesMissingSteam(t *testing.T) {
	restore := steam.SetSteamPathDetectorForTesting(func() (string, error) {
		return "", steam.ErrSteamNotFound
	})
	defer restore()

	svc := NewAppServiceWithConfigDir(testutil.TempDir(t))
	defer svc.WaitBackground()

	svc.EnsureSteamPath() // 不 panic、不 fatal 即通过

	if got := svc.GetSteamPath(); got != "" {
		t.Fatalf("探测失败时应保持空路径，实际 %q", got)
	}
	if _, err := svc.GetCS2CFGPath(); err == nil {
		t.Fatalf("没有 Steam 目录时 GetCS2CFGPath 应当报错，而不是返回空路径")
	}
}
