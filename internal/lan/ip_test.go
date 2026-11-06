package lan

import (
	"net"
	"testing"
)

func TestIsLANIP(t *testing.T) {
	// 这个函数判定的是「本机这张网卡上的地址，别人有没有机会连到」，
	// 不是「是不是私网地址」。所以它只看黑名单：环回、链路本地、Clash TUN
	// 这三类排除，其余一律可共享。
	tests := []struct {
		ip       string
		expected bool
		why      string
	}{
		// 传统私网：可共享
		{"192.168.1.1", true, "常见家庭网段"},
		{"10.0.0.1", true, "常见企业/虚拟网段"},
		{"172.20.10.5", true, "172.16/12"},

		// 新增放行：这些以前被白名单挡掉了
		{"100.64.0.1", true, "CGNAT，Tailscale 常用"},
		{"100.127.255.254", true, "CGNAT 上边界"},
		{"172.32.0.1", true, "172.16/12 之外，但 ZeroTier 可以配到任意段"},
		{"192.0.2.10", true, "TEST-NET-1，ZeroTier 也可能分配"},
		{"8.8.8.8", true, "本机 NIC 上真有公网地址时它是可达的，不该藏起来"},

		// 仍然排除
		{"127.0.0.1", false, "环回"},
		{"127.0.1.1", false, "环回"},
		{"169.254.1.1", false, "链路本地（APIPA）"},
		{"198.18.0.1", false, "Clash / Mihomo TUN fake-IP"},
		{"198.19.100.1", false, "Clash TUN 上边界"},

		// 非法输入
		{"::1", false, "IPv6"},
		{"fe80::1", false, "IPv6"},
		{"", false, "空串"},
		{"invalid", false, "不是 IP"},
		{"999.999.999.999", false, "越界"},
	}

	for _, tt := range tests {
		got := IsLANIP(tt.ip)
		if got != tt.expected {
			t.Errorf("IsLANIP(%q) = %v; want %v (%s)", tt.ip, got, tt.expected, tt.why)
		}
	}
}

func TestLANRank(t *testing.T) {
	if r := LANRank("192.168.1.1"); r != 0 {
		t.Errorf("LANRank(192.168.1.1) = %d; want 0", r)
	}
	if r := LANRank("10.0.0.1"); r != 1 {
		t.Errorf("LANRank(10.0.0.1) = %d; want 1", r)
	}
	if r := LANRank("172.20.0.1"); r != 2 {
		t.Errorf("LANRank(172.20.0.1) = %d; want 2", r)
	}
	if r := LANRank("8.8.8.8"); r != 3 {
		t.Errorf("LANRank(8.8.8.8) = %d; want 3", r)
	}
}

func TestGetLocalIPs(t *testing.T) {
	ips := GetLocalIPs()
	if len(ips) == 0 {
		t.Fatalf("GetLocalIPs returned empty list")
	}

	for _, ip := range ips {
		parsed := net.ParseIP(ip)
		if parsed == nil {
			t.Errorf("GetLocalIPs returned invalid IP: %s", ip)
		}
	}

	primary := GetLocalIP()
	if primary == "" {
		t.Errorf("GetLocalIP returned empty string")
	}
	if primary != ips[0] {
		t.Errorf("GetLocalIP() = %s; want %s", primary, ips[0])
	}
}
