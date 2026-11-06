package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cs2backup/internal/lan"
)

// LAN Sharing API bindings

// StartLANServer starts the LAN HTTP sharing server.
func (s *AppService) StartLANServer(port int, name string) error {
	s.mu.RLock()
	srv := s.lanServer
	backupSvc := s.backupService
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}

	if port < 0 {
		port = 8765
	}
	srv.SetPort(port)
	if name != "" {
		srv.SetShareName(name)
	}
	if backupSvc != nil {
		srv.SetBackupRoot(backupSvc.GetBackupPath())
	}

	if err := srv.Start(); err != nil {
		s.LogError(fmt.Sprintf("启动局域网共享服务失败: %v", err))
		return err
	}

	status := srv.GetStatus()
	s.LogSuccess(fmt.Sprintf("局域网共享服务已启动: %s (PIN: %s, 保护: %v)", status.URL, status.PIN, status.PINEnabled))
	return nil
}

// StopLANServer stops the LAN HTTP sharing server.
func (s *AppService) StopLANServer() error {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}

	if err := srv.Stop(); err != nil {
		s.LogError(fmt.Sprintf("停止局域网共享服务失败: %v", err))
		return err
	}
	s.LogInfo("局域网共享服务已停止")
	return nil
}

// GetLANServerStatus returns the status of the LAN sharing server.
func (s *AppService) GetLANServerStatus() (*lan.LANServerStatus, error) {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return nil, fmt.Errorf("lan server not initialized")
	}
	return srv.GetStatus(), nil
}

// ToggleLANPin enables or disables PIN authentication.
func (s *AppService) ToggleLANPin(enabled bool) error {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}
	srv.SetPINEnabled(enabled)
	if enabled {
		s.LogSuccess(fmt.Sprintf("已开启局域网 PIN 码保护: %s", srv.GetPIN()))
	} else {
		s.LogInfo("已关闭局域网 PIN 码保护 (任何人均可访问)")
	}
	return nil
}

// RegenerateLANPin generates a new 6-digit random PIN.
func (s *AppService) RegenerateLANPin() (string, error) {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return "", fmt.Errorf("lan server not initialized")
	}
	newPIN := srv.RegeneratePIN()
	s.LogSuccess(fmt.Sprintf("已重新生成局域网访问 PIN 码: %s", newPIN))
	return newPIN, nil
}

// SetLANSharedBackups configures which local accounts and ZIP packages are shared over LAN.
// SetLANBindAddress 指定局域网共享只在哪张网卡的地址上提供服务。
//
// 传空串表示监听全部网卡（默认）。装 ZeroTier / Tailscale 的用户选虚拟网卡的
// 地址，可以把服务限制在那条隧道里，不暴露到物理网卡。
//
// 服务正在跑时改这个不会立即生效——需要先停再启，因为监听地址在启动时绑定。
func (s *AppService) SetLANBindAddress(addr string) error {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}
	srv.SetBindAddress(addr)

	label := "全部网卡"
	if addr != "" {
		label = addr
	}
	s.LogSuccess(fmt.Sprintf("局域网共享的监听地址已设为: %s（重启共享服务后生效）", label))
	return nil
}

// SetLANRateLimit 设置局域网共享的出站限速（bytes/sec）。传 0 表示不限。
//
// 限速挂在写出层，桌面端与浏览器端同时生效，改档立即生效不用重启服务。
func (s *AppService) SetLANRateLimit(bytesPerSec int) error {
	s.mu.RLock()
	srv := s.lanServer
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	srv.SetRateLimit(bytesPerSec)

	label := "不限速"
	if bytesPerSec > 0 {
		label = fmt.Sprintf("%d MB/s", bytesPerSec/(1024*1024))
	}
	s.LogSuccess(fmt.Sprintf("局域网共享限速已调整为: %s", label))
	return nil
}

func (s *AppService) SetLANSharedBackups(accountIDs []string, zipPaths []string) error {
	s.mu.RLock()
	srv := s.lanServer
	backupSvc := s.backupService
	s.mu.RUnlock()

	if srv == nil {
		return fmt.Errorf("lan server not initialized")
	}

	backupRoot := ""
	if backupSvc != nil {
		backupRoot = backupSvc.GetBackupPath()
	}

	srv.ClearBackups()

	for _, aid := range accountIDs {
		name, _ := s.ResolveUserName(aid)
		if name == "" {
			name = aid
		}
		userDir := filepath.Join(backupRoot, "userdata_backup", aid)
		srv.AddLocalBackup(aid, name, userDir)
	}

	for _, zp := range zipPaths {
		srv.AddZIPBackup(zp)
	}

	s.LogInfo(fmt.Sprintf("已更新局域网共享清单: %d 个本地备份, %d 个压缩包", len(accountIDs), len(zipPaths)))
	return nil
}

// CreateLANPackage builds a standard ZIP package and registers it into LAN sharing.
func (s *AppService) CreateLANPackage(accountID, name string, includeCfg, includeUserdata bool) (string, error) {
	s.mu.RLock()
	srv := s.lanServer
	backupSvc := s.backupService
	steamSvc := s.steamService
	s.mu.RUnlock()

	if srv == nil || backupSvc == nil {
		return "", fmt.Errorf("services not initialized")
	}

	backupRoot := backupSvc.GetBackupPath()
	pkgDir := filepath.Join(backupRoot, "lan_packages")
	if err := os.MkdirAll(pkgDir, 0755); err != nil {
		return "", fmt.Errorf("failed creating package dir: %w", err)
	}

	if name == "" {
		name = "CS2Backup"
		if accountID != "" {
			name = fmt.Sprintf("Backup_%s", accountID)
		}
	}

	// 1. Locate userdata source
	var userdataDir string
	if includeUserdata && accountID != "" {
		localBackup730 := filepath.Join(backupRoot, "userdata_backup", accountID, "730")
		if fi, err := os.Stat(localBackup730); err == nil && fi.IsDir() {
			userdataDir = localBackup730
		} else if steamSvc != nil && steamSvc.GetSteamPath() != "" {
			live730 := filepath.Join(steamSvc.GetSteamPath(), "userdata", accountID, "730")
			if fi, err := os.Stat(live730); err == nil && fi.IsDir() {
				userdataDir = live730
			}
		}
	}

	// 2. Locate game cfg source
	var cfgDir string
	if includeCfg {
		localCfg := filepath.Join(backupRoot, "cs2_cfg")
		if fi, err := os.Stat(localCfg); err == nil && fi.IsDir() {
			cfgDir = localCfg
		} else {
			liveCfg, _ := s.GetCS2CFGPath()
			if fi, err := os.Stat(liveCfg); err == nil && fi.IsDir() {
				cfgDir = liveCfg
			}
		}
	}

	ts := time.Now().Format("20060102_150405")
	safeName := lan.SanitizeFilename(name)
	zipPath := filepath.Join(pkgDir, fmt.Sprintf("CS2Backup_%s_%s.zip", safeName, ts))

	s.LogInfo(fmt.Sprintf("正在打包局域网分发包 [%s]...", name))
	createdPath, err := lan.CreateLANPackage(lan.PackageOptions{
		AccountID:       accountID,
		Name:            name,
		IncludeUserdata: includeUserdata && userdataDir != "",
		UserdataDir:     userdataDir,
		IncludeCfg:      includeCfg && cfgDir != "",
		CfgDir:          cfgDir,
		OutputPath:      zipPath,
	})
	if err != nil {
		s.LogError(fmt.Sprintf("创建局域网备份包失败: %v", err))
		return "", err
	}

	srv.AddZIPBackup(createdPath)
	s.LogSuccess(fmt.Sprintf("局域网备份包创建成功: %s", filepath.Base(createdPath)))
	return createdPath, nil
}

// GetLANPackages lists all ZIP backup packages in the backup repository.
func (s *AppService) GetLANPackages() ([]lan.PackageInfo, error) {
	s.mu.RLock()
	backupSvc := s.backupService
	s.mu.RUnlock()

	if backupSvc == nil {
		return []lan.PackageInfo{}, nil
	}

	pkgDir := filepath.Join(backupSvc.GetBackupPath(), "lan_packages")
	if _, err := os.Stat(pkgDir); os.IsNotExist(err) {
		return []lan.PackageInfo{}, nil
	}

	entries, err := os.ReadDir(pkgDir)
	if err != nil {
		return nil, err
	}

	var packages []lan.PackageInfo
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".zip") || strings.HasSuffix(entry.Name(), ".part") {
			continue
		}
		info, err := entry.Info()
		if err != nil {
			continue
		}
		packages = append(packages, lan.PackageInfo{
			Filename: entry.Name(),
			Path:     filepath.Join(pkgDir, entry.Name()),
			Size:     info.Size(),
		})
	}

	sort.Slice(packages, func(i, j int) bool {
		return packages[i].Size > packages[j].Size
	})

	return packages, nil
}

// Discovery & Remote Sync Service Delegations

// StartDiscovery launches UDP broadcast and peer listening for local network sharing.
func (s *AppService) StartDiscovery() error {
	s.mu.RLock()
	disc := s.discoveryService
	srv := s.lanServer
	s.mu.RUnlock()

	if disc == nil {
		return fmt.Errorf("discovery service not initialized")
	}
	if srv != nil {
		status := srv.GetStatus()
		disc.SetHTTPPort(status.Port)
		disc.SetShareName(status.ShareName)
	}

	if err := disc.Start(); err != nil {
		s.LogError(fmt.Sprintf("启动局域网设备发现失败: %v", err))
		return err
	}
	s.LogSuccess("局域网设备发现服务已启动 (UDP 8766)")
	return nil
}

// StopDiscovery terminates UDP discovery listening and broadcasting.
func (s *AppService) StopDiscovery() error {
	s.mu.RLock()
	disc := s.discoveryService
	s.mu.RUnlock()

	if disc == nil {
		return nil
	}
	if err := disc.Stop(); err != nil {
		s.LogError(fmt.Sprintf("停止局域网设备发现失败: %v", err))
		return err
	}
	s.LogInfo("局域网设备发现服务已停止")
	return nil
}

// GetDiscoveredDevices returns all active and manual devices discovered on the network.
func (s *AppService) GetDiscoveredDevices() ([]lan.DiscoveredDevice, error) {
	s.mu.RLock()
	disc := s.discoveryService
	s.mu.RUnlock()

	if disc == nil {
		return []lan.DiscoveredDevice{}, nil
	}
	return disc.GetDevices(), nil
}

// AddManualDevice explicitly adds a target device by IP and port.
func (s *AppService) AddManualDevice(ip string, port int) error {
	s.mu.RLock()
	disc := s.discoveryService
	s.mu.RUnlock()

	if disc == nil {
		return fmt.Errorf("discovery service not initialized")
	}
	disc.AddManualDevice(ip, port, "")
	s.LogSuccess(fmt.Sprintf("已手动添加局域网设备: %s:%d", ip, port))
	return nil
}

// ConnectRemoteDevice connects to a remote LAN device, optionally authenticates with PIN,
// and retrieves the list of exposed backups.
func (s *AppService) ConnectRemoteDevice(ip string, port int, pin string) ([]lan.RemoteBackupItem, error) {
	s.mu.RLock()
	client := s.lanClient
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("lan client not initialized")
	}

	backups, err := client.GetRemoteBackups(ip, port, pin)
	if err != nil {
		s.LogError(fmt.Sprintf("连接远程设备 [%s:%d] 失败: %v", ip, port, err))
		return nil, err
	}

	s.LogSuccess(fmt.Sprintf("成功连接远程设备 [%s:%d]，获取到 %d 个共享备份", ip, port, len(backups)))
	return backups, nil
}

// DownloadRemoteBackup downloads a remote backup package, unpacks it into the local repository,
// and integrates it into local restorable backups.
func (s *AppService) DownloadRemoteBackup(ip string, port int, pin, backupID string) (*lan.DownloadedBackupResult, error) {
	s.mu.RLock()
	client := s.lanClient
	bkSvc := s.backupService
	s.mu.RUnlock()

	if client == nil {
		return nil, fmt.Errorf("lan client not initialized")
	}

	var backupPath string
	if bkSvc != nil {
		backupPath = bkSvc.GetBackupPath()
	}
	if backupPath == "" {
		backupPath = "backup"
	}

	s.LogInfo(fmt.Sprintf("正在从 [%s:%d] 下载远程备份 [ID: %s]...", ip, port, backupID))
	res, err := client.DownloadRemoteBackup(ip, port, pin, backupID, backupPath)
	if err != nil {
		s.LogError(fmt.Sprintf("下载远程备份失败: %v", err))
		return nil, err
	}

	s.LogSuccess(fmt.Sprintf("成功下载并解压远程备份: %s (共 %d 个文件, %d 字节, 存放于 %s)", res.AccountID, res.TotalFiles, res.TotalBytes, res.DestDir))
	s.mu.RLock()
	emitter := s.emitter
	ctx := s.ctx
	s.mu.RUnlock()
	if emitter != nil && ctx != nil {
		emitter(ctx, "backup:updated", res.AccountID)
	}
	return res, nil
}
