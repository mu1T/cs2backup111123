package lan

import (
	"context"
	"encoding/json"
	"fmt"
	"net"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	// DefaultUDPPort is the standard UDP port for CS2 config sharing discovery.
	DefaultUDPPort = 8766
	// MagicString identifies valid CS2 configuration broadcast packets.
	MagicString = "CS2CFG"
	// BroadcastInterval defines the frequency of announcement packets.
	BroadcastInterval = 3 * time.Second
	// DeviceTimeout defines when an inactive device is evicted.
	DeviceTimeout = 10 * time.Second
)

// DiscoveryMessage represents the broadcast payload format.
type DiscoveryMessage struct {
	Magic   string `json:"magic"`
	Name    string `json:"name"`
	Port    int    `json:"port"`
	Version string `json:"version"`
}

// DiscoveredDevice represents an active peer discovered on the local network.
type DiscoveredDevice struct {
	IP       string    `json:"ip"`
	Port     int       `json:"port"`
	Name     string    `json:"name"`
	Version  string    `json:"version"`
	LastSeen time.Time `json:"last_seen"`
	IsManual bool      `json:"is_manual"`
}

// DiscoveryService coordinates broadcasting local service availability and discovering peers.
type DiscoveryService struct {
	mu         sync.RWMutex
	udpPort    int
	httpPort   int
	shareName  string
	appVersion string
	running    bool
	allowSelf  bool
	devices    map[string]*DiscoveredDevice
	listener   net.PacketConn
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewDiscoveryService creates a new discovery service instance.
func NewDiscoveryService(httpPort int, shareName, appVersion string) *DiscoveryService {
	if shareName == "" {
		shareName = "CS2-Client"
	}
	if appVersion == "" {
		appVersion = "2.0.0"
	}
	return &DiscoveryService{
		udpPort:    DefaultUDPPort,
		httpPort:   httpPort,
		shareName:  shareName,
		appVersion: appVersion,
		devices:    make(map[string]*DiscoveredDevice),
	}
}

// SetUDPPort configures the UDP broadcast/listening port (defaults to 8766).
func (s *DiscoveryService) SetUDPPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.udpPort = port
}

// SetHTTPPort updates the HTTP port announced in discovery broadcasts.
func (s *DiscoveryService) SetHTTPPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.httpPort = port
}

// SetShareName updates the device announcement name.
func (s *DiscoveryService) SetShareName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shareName = name
}

// SetAllowSelf controls whether packets from the local device are recorded.
func (s *DiscoveryService) SetAllowSelf(allow bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.allowSelf = allow
}

// IsRunning reports whether the discovery service is actively listening and broadcasting.
func (s *DiscoveryService) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// AddManualDevice explicitly registers a device IP and port.
func (s *DiscoveryService) AddManualDevice(ip string, port int, name string) {
	ip = strings.TrimSpace(ip)
	if ip == "" || port <= 0 {
		return
	}
	if name == "" {
		name = fmt.Sprintf("手动设备 (%s:%d)", ip, port)
	}

	key := fmt.Sprintf("%s:%d", ip, port)
	s.mu.Lock()
	defer s.mu.Unlock()

	s.devices[key] = &DiscoveredDevice{
		IP:       ip,
		Port:     port,
		Name:     name,
		Version:  "manual",
		LastSeen: time.Now(),
		IsManual: true,
	}
}

// GetDevices returns a snapshot of all discovered active and manual devices.
// Devices not seen for more than 10 seconds (excluding manual devices) are pruned.
func (s *DiscoveryService) GetDevices() []DiscoveredDevice {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.pruneExpiredLocked()

	list := make([]DiscoveredDevice, 0, len(s.devices))
	for _, dev := range s.devices {
		list = append(list, *dev)
	}

	// Sort: manual first, then by LastSeen descending
	sort.Slice(list, func(i, j int) bool {
		if list[i].IsManual != list[j].IsManual {
			return list[i].IsManual
		}
		return list[i].LastSeen.After(list[j].LastSeen)
	})

	return list
}

func (s *DiscoveryService) pruneExpiredLocked() {
	now := time.Now()
	for key, dev := range s.devices {
		if !dev.IsManual && now.Sub(dev.LastSeen) > DeviceTimeout {
			delete(s.devices, key)
		}
	}
}

// Start launches the UDP discovery listener and background broadcast loops.
func (s *DiscoveryService) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	s.ctx = ctx
	s.cancel = cancel

	port := s.udpPort
	s.mu.Unlock()

	lc := net.ListenConfig{
		Control: controlReusePort,
	}

	addr := fmt.Sprintf("0.0.0.0:%d", port)
	conn, err := lc.ListenPacket(ctx, "udp4", addr)
	if err != nil {
		cancel()
		return fmt.Errorf("failed to bind UDP discovery port %d: %w", port, err)
	}

	s.mu.Lock()
	s.listener = conn
	s.running = true
	s.mu.Unlock()

	s.wg.Add(3)
	go s.listenLoop(conn)
	go s.broadcastLoop(ctx)
	go s.evictionLoop(ctx)

	return nil
}

// Stop terminates discovery listening and broadcasting loops.
func (s *DiscoveryService) Stop() error {
	s.mu.Lock()
	if !s.running {
		s.mu.Unlock()
		return nil
	}
	s.running = false
	if s.cancel != nil {
		s.cancel()
	}
	if s.listener != nil {
		_ = s.listener.Close()
	}
	s.mu.Unlock()

	s.wg.Wait()
	return nil
}

func (s *DiscoveryService) listenLoop(conn net.PacketConn) {
	defer s.wg.Done()
	buffer := make([]byte, 2048)

	for {
		n, remoteAddr, err := conn.ReadFrom(buffer)
		if err != nil {
			// Listener closed or shutdown
			return
		}

		if n <= 0 {
			continue
		}

		var msg DiscoveryMessage
		if err := json.Unmarshal(buffer[:n], &msg); err != nil {
			continue
		}

		if msg.Magic != MagicString || msg.Port <= 0 || msg.Port > 65535 {
			continue
		}

		remoteUDP, ok := remoteAddr.(*net.UDPAddr)
		if !ok || remoteUDP.IP == nil {
			continue
		}
		remoteIP := remoteUDP.IP.String()

		s.handleIncomingMessage(remoteIP, msg)
	}
}

func (s *DiscoveryService) handleIncomingMessage(remoteIP string, msg DiscoveryMessage) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.allowSelf {
		// Filter out own broadcast: match HTTP port and local IP
		if msg.Port == s.httpPort {
			if remoteIP == "127.0.0.1" || remoteIP == "::1" {
				return
			}
			for _, localIP := range GetLocalIPs() {
				if remoteIP == localIP {
					return
				}
			}
		}
	}

	key := fmt.Sprintf("%s:%d", remoteIP, msg.Port)
	if existing, found := s.devices[key]; found {
		existing.LastSeen = time.Now()
		existing.Name = msg.Name
		existing.Version = msg.Version
	} else {
		s.devices[key] = &DiscoveredDevice{
			IP:       remoteIP,
			Port:     msg.Port,
			Name:     msg.Name,
			Version:  msg.Version,
			LastSeen: time.Now(),
			IsManual: false,
		}
	}
}

func (s *DiscoveryService) broadcastLoop(ctx context.Context) {
	defer s.wg.Done()

	lc := net.ListenConfig{
		Control: controlBroadcast,
	}

	bconn, err := lc.ListenPacket(ctx, "udp4", ":0")
	if err != nil {
		return
	}
	defer bconn.Close()

	// Initial broadcast immediately
	s.sendBroadcast(bconn)

	ticker := time.NewTicker(BroadcastInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.sendBroadcast(bconn)
		}
	}
}

func (s *DiscoveryService) sendBroadcast(conn net.PacketConn) {
	s.mu.RLock()
	msg := DiscoveryMessage{
		Magic:   MagicString,
		Name:    s.shareName,
		Port:    s.httpPort,
		Version: s.appVersion,
	}
	udpPort := s.udpPort
	s.mu.RUnlock()

	data, err := json.Marshal(msg)
	if err != nil {
		return
	}

	// 1. General broadcast to 255.255.255.255:udpPort
	bcastAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("255.255.255.255:%d", udpPort))
	if err == nil {
		_, _ = conn.WriteTo(data, bcastAddr)
	}

	// 2. Local loopback broadcast for same-host testing
	loopAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("127.0.0.1:%d", udpPort))
	if err == nil {
		_, _ = conn.WriteTo(data, loopAddr)
	}
}

func (s *DiscoveryService) evictionLoop(ctx context.Context) {
	defer s.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			s.mu.Lock()
			s.pruneExpiredLocked()
			s.mu.Unlock()
		}
	}
}
