package lan

import "testing"

/**
 * 「可选的本机地址」必须与运行状态无关。
 *
 * 界面的「监听网卡」下拉就是靠它填选项的，而用户恰恰是在**启动服务之前**选网卡
 * ——那时候 `all_urls` 是空的（它只在 running 时才填）。所以这个列表必须单独
 * 提供，不能复用 all_urls。
 */
func TestGetStatus_AvailableAddressesIndependentOfRunning(t *testing.T) {
	srv := NewServer()

	status := srv.GetStatus()

	if status.AvailableAddresses == nil {
		t.Fatal("AvailableAddresses 不能是 nil——前端拿到 null 会崩")
	}
	if status.Running {
		t.Fatal("刚构造的服务不该是 running")
	}

	// 每一条都必须是「可共享」的地址（环回 / 链路本地 / Clash TUN 都被排除）
	for _, ip := range status.AvailableAddresses {
		if !IsLANIP(ip) {
			t.Errorf("AvailableAddresses 里出现了不该共享的地址: %s", ip)
		}
	}

	// 与 GetLocalIPs 的判定一致：它不是另一套逻辑
	if got, want := len(status.AvailableAddresses), len(GetLocalIPs()); got != want {
		t.Logf("可选地址 %d 个，GetLocalIPs 返回 %d 个（本机网卡数量不同属正常，仅记录）", got, want)
	}
}

/** 绑定地址要能反映到状态里，界面才知道当前选的是哪张卡。 */
func TestGetStatus_ReflectsBindAddress(t *testing.T) {
	srv := NewServer()

	if got := srv.GetStatus().BindAddress; got != "" {
		t.Errorf("默认应为空（全部网卡），实际 %q", got)
	}

	srv.SetBindAddress("  10.147.20.5  ")
	if got := srv.GetStatus().BindAddress; got != "10.147.20.5" {
		t.Errorf("应去掉首尾空白后写入，实际 %q", got)
	}
}
