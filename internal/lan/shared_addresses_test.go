package lan

import (
	"reflect"
	"testing"
)

/**
 * 共享网卡地址的筛选：能测 CIDR 边界，不用真装 ZeroTier。
 *
 * 输入是「网卡名 + 地址」的扁平对，和真实网卡解耦。
 */
func TestSharedAddresses(t *testing.T) {
	tests := []struct {
		name  string
		pairs [][2]string
		want  []string
	}{
		{
			name:  "空输入",
			pairs: nil,
			want:  []string{},
		},
		{
			name: "普通家庭网卡：留下",
			pairs: [][2]string{
				{"以太网", "192.168.1.20"},
			},
			want: []string{"192.168.1.20"},
		},
		{
			name: "ZeroTier / Tailscale 的地址现在能显示",
			pairs: [][2]string{
				{"ZeroTier One", "10.147.20.5"},
				{"Tailscale", "100.101.102.103"},
			},
			// 10/8 排在 100.64/10 前面（LANRank：10.x 是第 1 档，其余是最后一档）
			want: []string{"10.147.20.5", "100.101.102.103"},
		},
		{
			name: "环回 / 链路本地 / Clash TUN 被排除",
			pairs: [][2]string{
				{"Loopback", "127.0.0.1"},
				{"自动专用", "169.254.10.10"},
				{"Clash", "198.18.0.5"},
				{"以太网", "192.168.1.20"},
			},
			want: []string{"192.168.1.20"},
		},
		{
			name: "重复地址只留一次",
			pairs: [][2]string{
				{"以太网", "192.168.1.20"},
				{"以太网 IPv4", "192.168.1.20"},
			},
			want: []string{"192.168.1.20"},
		},
		{
			name: "空地址与非法地址跳过",
			pairs: [][2]string{
				{"以太网", ""},
				{"坏的", "not-an-ip"},
				{"以太网", "192.168.1.20"},
			},
			want: []string{"192.168.1.20"},
		},
		{
			name: "按 LANRank 排序：192.168 先于 10，再先于其它",
			pairs: [][2]string{
				{"ZeroTier", "172.25.0.2"},
				{"以太网", "10.0.0.5"},
				{"Wi-Fi", "192.168.1.20"},
			},
			want: []string{"192.168.1.20", "10.0.0.5", "172.25.0.2"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SharedAddresses(tt.pairs)
			if got == nil {
				t.Fatal("应返回空切片而不是 nil——前端拿到 null 会崩")
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("SharedAddresses = %v, want %v", got, tt.want)
			}
		})
	}
}
