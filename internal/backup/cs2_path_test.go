package backup

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"cs2backup/internal/testutil"
)

// makeFakeCS2Tree 造一棵和真实安装同构的目录树：
// <root>/steamapps/common/Counter-Strike Global Offensive/game/csgo/cfg/xxx.cfg
func makeFakeCS2Tree(t *testing.T) (root, cfgDir string) {
	t.Helper()
	root = testutil.TempDir(t)
	cfgDir = filepath.Join(root, "steamapps", "common", "Counter-Strike Global Offensive", "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatalf("建目录失败: %v", err)
	}
	// 真实的 cfg 目录里一定有 gamemode_*.cfg，这里放一个最小的样本
	if err := os.WriteFile(filepath.Join(cfgDir, "gamemode_competitive.cfg"), []byte("mp_roundtime 1.92\n"), 0o644); err != nil {
		t.Fatalf("写样例 cfg 失败: %v", err)
	}
	return root, cfgDir
}

func TestNormalizeCS2CFGPathAcceptsAnyLevel(t *testing.T) {
	root, cfgDir := makeFakeCS2Tree(t)
	csgoRoot := filepath.Join(root, "steamapps", "common", "Counter-Strike Global Offensive")
	commonDir := filepath.Join(root, "steamapps", "common")

	// 用户可能选中的各个层级，都应该归一到同一个 cfg 目录
	for _, raw := range []string{
		cfgDir,                          // 直接选 cfg
		filepath.Dir(cfgDir),            // 选 game/csgo
		filepath.Join(csgoRoot, "game"), // 选 game
		csgoRoot,                        // 选 Counter-Strike Global Offensive
		commonDir,                       // 选 steamapps/common
	} {
		got, err := NormalizeCS2CFGPath(raw)
		if err != nil {
			t.Errorf("%s 应被接受，实际报错: %v", raw, err)
			continue
		}
		if got != cfgDir {
			t.Errorf("%s 归一结果 = %s, 期望 %s", raw, got, cfgDir)
		}
	}
}

func TestNormalizeCS2CFGPathAcceptsCustomInstallDir(t *testing.T) {
	// 非 Steam 默认布局：用户自己把 CS2 拷到别处，只要有 game/csgo/cfg 就算数
	root := testutil.TempDir(t)
	cfgDir := filepath.Join(root, "game", "csgo", "cfg")
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		t.Fatal(err)
	}

	got, err := NormalizeCS2CFGPath(root)
	if err != nil {
		t.Fatalf("自定义布局应被接受: %v", err)
	}
	if got != cfgDir {
		t.Errorf("归一结果 = %s, 期望 %s", got, cfgDir)
	}
}

func TestNormalizeCS2CFGPathRejectsUnrelatedDir(t *testing.T) {
	root := testutil.TempDir(t) // 空目录，既不是 cfg 也没有 .cfg 文件
	if _, err := NormalizeCS2CFGPath(root); err == nil {
		t.Fatal("无关目录应被拒绝")
	}
	if _, err := NormalizeCS2CFGPath("   "); err == nil {
		t.Fatal("空路径应被拒绝")
	}
	if _, err := NormalizeCS2CFGPath(filepath.Join(root, "not-exist")); err == nil {
		t.Fatal("不存在的路径应被拒绝")
	}
}

func TestServiceSetCS2PathNormalizesAndPersists(t *testing.T) {
	root, cfgDir := makeFakeCS2Tree(t)

	svc := newTestService(t)
	// 故意不设 steamPath：手动路径应当独立生效
	if err := svc.SetCS2Path(root); err != nil {
		t.Fatalf("设置失败: %v", err)
	}
	if got := svc.GetCS2Path(); got != cfgDir {
		t.Errorf("GetCS2Path = %s, 期望归一后的 %s", got, cfgDir)
	}
	got, err := svc.GetCS2CFGPath()
	if err != nil {
		t.Fatalf("GetCS2CFGPath 失败: %v", err)
	}
	if got != cfgDir {
		t.Errorf("GetCS2CFGPath = %s, 期望 %s", got, cfgDir)
	}
}

func TestServiceSetCS2PathEmptyRestoresAuto(t *testing.T) {
	root, _ := makeFakeCS2Tree(t)
	svc := newTestService(t)

	if err := svc.SetCS2Path(root); err != nil {
		t.Fatal(err)
	}
	if err := svc.SetCS2Path("  "); err != nil {
		t.Fatalf("清空失败: %v", err)
	}
	if got := svc.GetCS2Path(); got != "" {
		t.Errorf("清空后 GetCS2Path = %q, 期望空串", got)
	}
	// 清空后没有 steamPath，应当回到「未配置」的错误
	if _, err := svc.GetCS2CFGPath(); err == nil {
		t.Error("清空手动路径且没有 Steam 路径时应报错")
	}
}

func TestServiceManualPathMissingGivesActionableError(t *testing.T) {
	root, cfgDir := makeFakeCS2Tree(t)
	svc := newTestService(t)
	if err := svc.SetCS2Path(root); err != nil {
		t.Fatal(err)
	}

	// 模拟盘符变化 / 目录被删
	if err := os.RemoveAll(cfgDir); err != nil {
		t.Fatal(err)
	}

	_, err := svc.GetCS2CFGPath()
	if err == nil {
		t.Fatal("目录消失后应报错")
	}
	if !strings.Contains(err.Error(), "路径设置") {
		t.Errorf("错误提示应指引用户去路径设置，实际: %v", err)
	}
}

func TestServiceSetCS2PathRejectsUnrelated(t *testing.T) {
	svc := newTestService(t)
	if err := svc.SetCS2Path(testutil.TempDir(t)); err == nil {
		t.Fatal("无关目录应被拒绝")
	}
	if got := svc.GetCS2Path(); got != "" {
		t.Errorf("拒绝后不应写入，实际 %q", got)
	}
}

// newTestService 构造一个不带 cacheMgr 的服务，路径只活在内存里。
func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := NewService(testutil.TempDir(t), nil)
	if err != nil {
		t.Fatalf("构造服务失败: %v", err)
	}
	return svc
}
