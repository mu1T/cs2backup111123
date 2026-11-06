package lan

import (
	"net"
	"sort"
	"strings"
	"time"
)

// rankNetworks 只用于排序「备用网卡地址」的展示顺序（192.168 > 10 > 172.16 > 其它），
// 不参与过滤——过滤看 blockedNetworks。
var rankNetworks = []*net.IPNet{
	mustParseCIDR("192.168.0.0/16"),
	mustParseCIDR("10.0.0.0/8"),
	mustParseCIDR("172.16.0.0/12"),
}

// blockedNetworks 是不该出现在「可共享地址」里的网段。
//
// 用黑名单而不是白名单：白名单只放行 192.168/16、10/8、172.16/12，会把
// ZeroTier / Tailscale 这类虚拟网卡的地址一起滤掉——用户装它们就是为了内网
// 穿透，结果地址在界面上根本不显示，连「念给对方听」都做不到。
//
// 100.64/10（CGNAT，Tailscale 常用）以前也在黑名单里，现在放行。
var blockedNetworks = []*net.IPNet{
	mustParseCIDR("198.18.0.0/15"),  // Clash / Mihomo TUN fake-IP（代理网段，不是共享目标）
	mustParseCIDR("169.254.0.0/16"), // Link-local
	mustParseCIDR("127.0.0.0/8"),    // Loopback
}

func mustParseCIDR(s string) *net.IPNet {
	_, ipnet, err := net.ParseCIDR(s)
	if err != nil {
		panic("invalid CIDR: " + s)
	}
	return ipnet
}

// IsLANIP 判断一个地址能否作为「本机可共享地址」出现在界面上。
//
// 名字里的 LAN 是历史遗留：它现在不是「私网地址」的意思，而是「本机这张网卡
// 上的地址，别人有机会连到」。所以判定只看黑名单——本机 NIC 上真有一个公网
// 地址，那它确实可达，没必要藏起来。
func IsLANIP(ipStr string) bool {
	ipStr = strings.TrimSpace(ipStr)
	if ipStr == "" {
		return false
	}
	ip := net.ParseIP(ipStr)
	if ip == nil {
		return false
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return false
	}

	for _, blocked := range blockedNetworks {
		if blocked.Contains(ipv4) {
			return false
		}
	}

	return true
}

// LANRank returns the ranking of the LAN IP:
// 0: 192.168.x.x
// 1: 10.x.x.x
// 2: 172.16-31.x.x
// 3: any other
func LANRank(ipStr string) int {
	ip := net.ParseIP(strings.TrimSpace(ipStr))
	if ip == nil {
		return len(rankNetworks)
	}
	ipv4 := ip.To4()
	if ipv4 == nil {
		return len(rankNetworks)
	}

	for idx, netw := range rankNetworks {
		if netw.Contains(ipv4) {
			return idx
		}
	}
	return len(rankNetworks)
}

// GetLocalIPs returns all discovered valid LAN IPv4 addresses, ranked 192.168 > 10 > 172.16-31.
// If no valid LAN IP is found, it returns ["127.0.0.1"].
func GetLocalIPs() []string {
	var candidates []string
	seen := make(map[string]bool)

	addCandidate := func(ip string) {
		ip = strings.TrimSpace(ip)
		if ip != "" && !seen[ip] {
			seen[ip] = true
			candidates = append(candidates, ip)
		}
	}

	// 1. UDP probe to discover default routing interface (does not send traffic)
	if conn, err := net.DialTimeout("udp", "8.8.8.8:80", 200*time.Millisecond); err == nil {
		if localAddr, ok := conn.LocalAddr().(*net.UDPAddr); ok && localAddr.IP != nil {
			addCandidate(localAddr.IP.String())
		}
		_ = conn.Close()
	}

	// 2. Enumerate network interfaces
	if ifaces, err := net.Interfaces(); err == nil {
		for _, iface := range ifaces {
			// Skip down or loopback interfaces
			if iface.Flags&net.FlagUp == 0 || iface.Flags&net.FlagLoopback != 0 {
				continue
			}
			addrs, err := iface.Addrs()
			if err != nil {
				continue
			}
			for _, addr := range addrs {
				var ip net.IP
				switch v := addr.(type) {
				case *net.IPNet:
					ip = v.IP
				case *net.IPAddr:
					ip = v.IP
				}
				if ip != nil && ip.To4() != nil {
					addCandidate(ip.String())
				}
			}
		}
	}

	// 筛选与排序都交给 SharedAddresses——它是纯函数，边界情况能测。
	pairs := make([][2]string, 0, len(candidates))
	for _, ip := range candidates {
		pairs = append(pairs, [2]string{"", ip})
	}
	validLANs := SharedAddresses(pairs)

	if len(validLANs) == 0 {
		return []string{"127.0.0.1"}
	}

	return validLANs
}

// GetLocalIP returns the highest-ranked local LAN IPv4 address.
func GetLocalIP() string {
	ips := GetLocalIPs()
	if len(ips) > 0 {
		return ips[0]
	}
	return "127.0.0.1"
}

// SharedAddresses 从一组「网卡名 + 地址」里挑出可共享的，按 LANRank 排序。
//
// 抽成纯函数是为了能测：CIDR 边界这类东西，靠实机装 ZeroTier 去验是不可行的。
// 输入是普通字符串对，不依赖 net.Interface——调用方负责把真实的网卡摊平成
// 这个形状，测试则直接喂假数据。
func SharedAddresses(pairs [][2]string) []string {
	type item struct {
		ip   string
		rank int
	}
	var picked []item
	seen := make(map[string]bool)

	for _, p := range pairs {
		ip := strings.TrimSpace(p[1])
		if ip == "" || seen[ip] {
			continue
		}
		if !IsLANIP(ip) {
			continue
		}
		seen[ip] = true
		picked = append(picked, item{ip: ip, rank: LANRank(ip)})
	}

	sort.SliceStable(picked, func(a, b int) bool { return picked[a].rank < picked[b].rank })

	out := make([]string, 0, len(picked))
	for _, it := range picked {
		out = append(out, it.ip)
	}
	return out
}
