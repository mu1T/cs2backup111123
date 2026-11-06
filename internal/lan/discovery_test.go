package lan

import (
	"encoding/json"
	"fmt"
	"net"
	"testing"
	"time"
)

func TestDiscoveryMessage_Serialization(t *testing.T) {
	msg := DiscoveryMessage{
		Magic:   MagicString,
		Name:    "Player-PC",
		Port:    8765,
		Version: "2.0.0",
	}

	data, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("Marshal failed: %v", err)
	}

	var parsed DiscoveryMessage
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("Unmarshal failed: %v", err)
	}

	if parsed.Magic != "CS2CFG" {
		t.Errorf("expected magic 'CS2CFG', got '%s'", parsed.Magic)
	}
	if parsed.Name != "Player-PC" {
		t.Errorf("expected name 'Player-PC', got '%s'", parsed.Name)
	}
	if parsed.Port != 8765 {
		t.Errorf("expected port 8765, got %d", parsed.Port)
	}
	if parsed.Version != "2.0.0" {
		t.Errorf("expected version '2.0.0', got '%s'", parsed.Version)
	}
}

func TestDiscovery_ManualDevice(t *testing.T) {
	disc := NewDiscoveryService(8765, "Local-PC", "2.0.0")

	// Add manual device with explicit name
	disc.AddManualDevice("192.168.1.100", 8765, "My Friend's PC")

	devices := disc.GetDevices()
	if len(devices) != 1 {
		t.Fatalf("expected 1 device, got %d", len(devices))
	}
	if devices[0].IP != "192.168.1.100" || devices[0].Port != 8765 {
		t.Errorf("device IP/Port mismatch: %+v", devices[0])
	}
	if devices[0].Name != "My Friend's PC" {
		t.Errorf("expected name 'My Friend's PC', got '%s'", devices[0].Name)
	}
	if !devices[0].IsManual {
		t.Errorf("expected IsManual to be true")
	}

	// Add manual device with empty name -> defaults
	disc.AddManualDevice("10.0.0.5", 8766, "")
	devices = disc.GetDevices()
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices, got %d", len(devices))
	}
}

func TestDiscovery_TimeoutEviction(t *testing.T) {
	disc := NewDiscoveryService(8765, "Local-PC", "2.0.0")

	// Add an expired device (last seen 15 seconds ago)
	disc.mu.Lock()
	disc.devices["192.168.1.20:8765"] = &DiscoveredDevice{
		IP:       "192.168.1.20",
		Port:     8765,
		Name:     "Old-PC",
		Version:  "1.0.0",
		LastSeen: time.Now().Add(-15 * time.Second),
		IsManual: false,
	}
	// Add an active device (last seen 2 seconds ago)
	disc.devices["192.168.1.21:8765"] = &DiscoveredDevice{
		IP:       "192.168.1.21",
		Port:     8765,
		Name:     "Active-PC",
		Version:  "2.0.0",
		LastSeen: time.Now().Add(-2 * time.Second),
		IsManual: false,
	}
	// Add an old manual device
	disc.devices["192.168.1.22:8765"] = &DiscoveredDevice{
		IP:       "192.168.1.22",
		Port:     8765,
		Name:     "Manual-PC",
		Version:  "2.0.0",
		LastSeen: time.Now().Add(-20 * time.Second),
		IsManual: true,
	}
	disc.mu.Unlock()

	// GetDevices triggers eviction of expired non-manual devices
	devices := disc.GetDevices()
	if len(devices) != 2 {
		t.Fatalf("expected 2 devices after eviction, got %d", len(devices))
	}

	deviceMap := make(map[string]DiscoveredDevice)
	for _, d := range devices {
		deviceMap[d.IP] = d
	}

	if _, found := deviceMap["192.168.1.20"]; found {
		t.Errorf("expired device 192.168.1.20 should have been evicted")
	}
	if _, found := deviceMap["192.168.1.21"]; !found {
		t.Errorf("active device 192.168.1.21 should remain")
	}
	if _, found := deviceMap["192.168.1.22"]; !found {
		t.Errorf("manual device 192.168.1.22 should remain")
	}
}

func TestDiscovery_StartStop(t *testing.T) {
	disc := NewDiscoveryService(8765, "Test-PC", "2.0.0")
	// Use custom test UDP port to avoid conflicting with running services
	disc.SetUDPPort(18766)

	if disc.IsRunning() {
		t.Errorf("should not be running initially")
	}

	if err := disc.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	if !disc.IsRunning() {
		t.Errorf("expected IsRunning to be true")
	}

	// Second start is idempotent
	if err := disc.Start(); err != nil {
		t.Errorf("idempotent start failed: %v", err)
	}

	if err := disc.Stop(); err != nil {
		t.Fatalf("Stop failed: %v", err)
	}

	if disc.IsRunning() {
		t.Errorf("expected IsRunning to be false after Stop")
	}

	// Can start and stop again
	if err := disc.Start(); err != nil {
		t.Fatalf("second Start failed: %v", err)
	}
	_ = disc.Stop()
}

func TestDiscovery_ReceiveBroadcast(t *testing.T) {
	disc := NewDiscoveryService(8765, "Host-PC", "2.0.0")
	testPort := 18767
	disc.SetUDPPort(testPort)
	disc.SetAllowSelf(true)

	if err := disc.Start(); err != nil {
		t.Fatalf("failed to start discovery service: %v", err)
	}
	defer disc.Stop()

	// Send valid discovery broadcast from mock remote sender
	sender, err := net.Dial("udp", fmt.Sprintf("127.0.0.1:%d", testPort))
	if err != nil {
		t.Fatalf("failed to dial UDP listener: %v", err)
	}
	defer sender.Close()

	msg := DiscoveryMessage{
		Magic:   MagicString,
		Name:    "Remote-Player",
		Port:    8888,
		Version: "2.0.0",
	}
	data, _ := json.Marshal(msg)
	if _, err := sender.Write(data); err != nil {
		t.Fatalf("failed to write UDP packet: %v", err)
	}

	// Allow listener loop to process
	var found bool
	for i := 0; i < 20; i++ {
		time.Sleep(50 * time.Millisecond)
		for _, dev := range disc.GetDevices() {
			if dev.Name == "Remote-Player" && dev.Port == 8888 {
				found = true
				break
			}
		}
		if found {
			break
		}
	}

	if !found {
		t.Fatalf("discovery service did not record the broadcast device")
	}

	// Test invalid magic message is rejected
	badMsg := DiscoveryMessage{
		Magic:   "INVALID",
		Name:    "Fake-PC",
		Port:    8765,
		Version: "2.0.0",
	}
	badData, _ := json.Marshal(badMsg)
	_, _ = sender.Write(badData)

	time.Sleep(100 * time.Millisecond)
	for _, dev := range disc.GetDevices() {
		if dev.Name == "Fake-PC" {
			t.Errorf("invalid magic device should not have been recorded")
		}
	}
}
