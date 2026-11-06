package lan

import (
	"context"
	"crypto/md5"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// SharedBackupItem represents a backup item available for LAN download.
type SharedBackupItem struct {
	ID               string `json:"id"`
	Filename         string `json:"filename"`
	Size             int64  `json:"size"`
	BackupName       string `json:"backup_name"`
	CreatedAt        string `json:"created_at"`
	ContainsCfg      bool   `json:"contains_cfg"`
	ContainsUserdata bool   `json:"contains_userdata"`
	IsLocal          bool   `json:"is_local"`
	AccountID        string `json:"account_id,omitempty"`
}

// SharedLocalInfo holds tracking info for a shared local backup directory.
type SharedLocalInfo struct {
	AccountID string `json:"account_id"`
	Name      string `json:"name"`
	Dir       string `json:"dir"`
}

// LANServerStatus contains running state, network URLs, PIN configuration, and connection stats.
type LANServerStatus struct {
	Running        bool     `json:"running"`
	URL            string   `json:"url"`
	AllURLs        []string `json:"all_urls"`
	Port           int      `json:"port"`
	ShareName      string   `json:"share_name"`
	PIN            string   `json:"pin"`
	PINEnabled     bool     `json:"pin_enabled"`
	SharedCount    int      `json:"shared_count"`
	Connections    int      `json:"connections"`
	SharedLocalIDs []string `json:"shared_local_ids"`
	SharedZIPPaths []string `json:"shared_zip_paths"`
	// RateLimit 是出站限速（bytes/sec），0 表示不限。
	RateLimit int `json:"rate_limit"`
	// BindAddress 是只提供服务的网卡地址，空表示全部网卡。
	BindAddress string `json:"bind_address"`
	// AvailableAddresses 是本机所有「可共享」的地址，与运行状态无关。
	//
	// 界面用它填「监听网卡」下拉的选项。**不能复用 AllURLs**——那个只在服务
	// 运行时才填，而用户恰恰是在启动之前选网卡。
	AvailableAddresses []string `json:"available_addresses"`
}

// Server provides the lightweight HTTP server for LAN sharing.
const (
	maxSessions         = 200
	maxFailedAttemptIPs = 200
)

type Server struct {
	mu             sync.RWMutex
	port           int
	shareName      string
	appVersion     string
	pin            string
	pinEnabled     bool
	running        bool
	httpServer     *http.Server
	listener       net.Listener
	assets         fs.FS
	backupRoot     string
	sharedZIPs     []string
	sharedLocals   map[string]SharedLocalInfo
	// rateLimit 是出站限速（bytes/sec），0 表示不限。
	rateLimit int
	// bindAddress 非空时只在该地址上监听（Shared NIC），空表示全部网卡。
	bindAddress string
	sessions       map[string]time.Time
	failedAttempts map[string][]time.Time
	activeConns    atomic.Int64
}

// NewServer creates a new Server instance with default settings.
func NewServer() *Server {
	hostname, err := os.Hostname()
	if err != nil || hostname == "" {
		hostname = "CS2-LAN-Server"
	}

	s := &Server{
		port:           8765,
		shareName:      hostname,
		appVersion:     "2.0.0",
		pinEnabled:     false,
		sharedLocals:   make(map[string]SharedLocalInfo),
		sessions:       make(map[string]time.Time),
		failedAttempts: make(map[string][]time.Time),
	}
	s.pin = s.generateRandomPIN()
	return s
}

func (s *Server) generateRandomPIN() string {
	n, err := rand.Int(rand.Reader, big.NewInt(1000000))
	if err != nil {
		return "123456"
	}
	return fmt.Sprintf("%06d", n.Int64())
}

// SetPort configures the listening port (0 for dynamic).
func (s *Server) SetPort(port int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.port = port
}

// SetBindAddress 指定只在哪张网卡的地址上提供服务。传空串表示监听全部网卡。
//
// 装 ZeroTier / Tailscale 的用户会选虚拟网卡的地址：既能把服务限制在那条
// 隧道里（不暴露到物理网卡），也让对方知道该连哪个 IP。
func (s *Server) SetBindAddress(addr string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.bindAddress = strings.TrimSpace(addr)
}

// GetBindAddress 返回当前绑定的地址，空串表示全部网卡。
func (s *Server) GetBindAddress() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.bindAddress
}

// SetRateLimit 设置出站限速（bytes/sec）。传 0 表示不限。
//
// 限速挂在写出层，所以桌面端与浏览器端同时生效。
func (s *Server) SetRateLimit(bytesPerSec int) {
	if bytesPerSec < 0 {
		bytesPerSec = 0
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.rateLimit = bytesPerSec
}

// GetRateLimit 返回当前出站限速（bytes/sec），0 表示不限。
func (s *Server) GetRateLimit() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.rateLimit
}

// GetPort returns the currently configured or bound port.
func (s *Server) GetPort() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.port
}

// SetShareName sets the host display name.
func (s *Server) SetShareName(name string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.shareName = name
}

// GetShareName returns the host display name.
func (s *Server) GetShareName() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.shareName
}

// SetAppVersion sets the application version string.
func (s *Server) SetAppVersion(ver string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.appVersion = ver
}

// SetBackupRoot sets the root folder where backups reside.
func (s *Server) SetBackupRoot(root string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.backupRoot = root
}

// SetAssets configures embedded assets for serving the SPA.
func (s *Server) SetAssets(assets fs.FS) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// If the embedded filesystem has a frontend/dist subfolder, use that as root
	if sub, err := fs.Sub(assets, "frontend/dist"); err == nil {
		if _, err := sub.Open("index.html"); err == nil {
			s.assets = sub
			return
		}
	}
	s.assets = assets
}

// SetPINEnabled enables or disables PIN authentication requirement.
func (s *Server) SetPINEnabled(enabled bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pinEnabled = enabled
	if !enabled {
		s.sessions = make(map[string]time.Time)
		s.failedAttempts = make(map[string][]time.Time)
	}
}

// IsPINEnabled reports whether PIN protection is active.
func (s *Server) IsPINEnabled() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pinEnabled
}

// SetPIN sets a specific PIN for testing or explicit configuration.
func (s *Server) SetPIN(pin string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pin = pin
	s.sessions = make(map[string]time.Time)
	s.failedAttempts = make(map[string][]time.Time)
}

// GetPIN returns the current PIN.
func (s *Server) GetPIN() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.pin
}

// RegeneratePIN generates a new 6-digit random PIN and invalidates previous sessions.
func (s *Server) RegeneratePIN() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pin = s.generateRandomPIN()
	s.sessions = make(map[string]time.Time)
	s.failedAttempts = make(map[string][]time.Time)
	return s.pin
}

// AddZIPBackup adds a ZIP package to shared packages.
func (s *Server) AddZIPBackup(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range s.sharedZIPs {
		if p == path {
			return
		}
	}
	if _, err := os.Stat(path); err == nil {
		s.sharedZIPs = append(s.sharedZIPs, path)
	}
}

// RemoveZIPBackup removes a ZIP package from shared packages.
func (s *Server) RemoveZIPBackup(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var updated []string
	for _, p := range s.sharedZIPs {
		if p != path {
			updated = append(updated, p)
		}
	}
	s.sharedZIPs = updated
}

// AddLocalBackup registers a local account backup directory for sharing.
func (s *Server) AddLocalBackup(accountID, name, dir string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sharedLocals[accountID] = SharedLocalInfo{
		AccountID: accountID,
		Name:      name,
		Dir:       dir,
	}
}

// RemoveLocalBackup unregisters a local account backup.
func (s *Server) RemoveLocalBackup(accountID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sharedLocals, accountID)
}

// ClearBackups clears all shared local and ZIP backups.
func (s *Server) ClearBackups() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sharedZIPs = nil
	s.sharedLocals = make(map[string]SharedLocalInfo)
}

// GetSharedZIPs returns a copy of shared ZIP file paths.
func (s *Server) GetSharedZIPs() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	res := make([]string, len(s.sharedZIPs))
	copy(res, s.sharedZIPs)
	return res
}

// GetSharedLocalBackups returns a list of registered local backups.
func (s *Server) GetSharedLocalBackups() []SharedLocalInfo {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var res []SharedLocalInfo
	for _, info := range s.sharedLocals {
		res = append(res, info)
	}
	return res
}

// IsRunning reports whether the server is actively listening.
func (s *Server) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

// GetStatus returns the current snapshot of LAN server status.
func (s *Server) GetStatus() *LANServerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	primaryIP := GetLocalIP()
	primaryURL := ""
	var allURLs []string
	if s.running && s.port > 0 {
		primaryURL = fmt.Sprintf("http://%s:%d", primaryIP, s.port)
		for _, ip := range GetLocalIPs() {
			allURLs = append(allURLs, fmt.Sprintf("http://%s:%d", ip, s.port))
		}
	}

	// 可选地址与运行状态无关：启动前也得能选网卡。
	available := GetLocalIPs()
	if available == nil {
		available = []string{}
	}

	var localIDs []string
	for aid := range s.sharedLocals {
		localIDs = append(localIDs, aid)
	}
	zipPaths := make([]string, len(s.sharedZIPs))
	copy(zipPaths, s.sharedZIPs)

	return &LANServerStatus{
		Running:        s.running,
		URL:            primaryURL,
		AllURLs:        allURLs,
		Port:           s.port,
		ShareName:      s.shareName,
		PIN:            s.pin,
		PINEnabled:     s.pinEnabled,
		SharedCount:    len(s.sharedLocals) + len(s.sharedZIPs),
		Connections:    int(s.activeConns.Load()),
		SharedLocalIDs: localIDs,
		SharedZIPPaths: zipPaths,
		RateLimit:      s.rateLimit,
		BindAddress:        s.bindAddress,
		AvailableAddresses: available,
	}
}

// Start launches the HTTP server in a background goroutine.
func (s *Server) Start() error {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return nil
	}

	host := s.bindAddress
	if host == "" {
		host = "0.0.0.0"
	}
	addr := fmt.Sprintf("%s:%d", host, s.port)
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		s.mu.Unlock()
		return fmt.Errorf("failed to bind port %d: %w", s.port, err)
	}
	s.listener = ln
	s.port = ln.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	s.registerRoutes(mux)

	s.httpServer = &http.Server{
		Handler:      mux,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 60 * time.Minute, // Allow long package streaming downloads
	}
	s.running = true
	s.mu.Unlock()

	go func() {
		if err := s.httpServer.Serve(ln); err != nil && err != http.ErrServerClosed {
			s.mu.Lock()
			s.running = false
			s.mu.Unlock()
		}
	}()

	return nil
}

// Stop gracefully stops the HTTP server.
func (s *Server) Stop() error {
	s.mu.Lock()
	if !s.running || s.httpServer == nil {
		s.mu.Unlock()
		return nil
	}
	srv := s.httpServer
	s.running = false
	s.sessions = make(map[string]time.Time)
	s.failedAttempts = make(map[string][]time.Time)
	s.mu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	err := srv.Shutdown(ctx)
	_ = srv.Close()
	return err
}

func (s *Server) registerRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/lan/info", s.handleInfo)
	mux.HandleFunc("/api/lan/auth", s.handleAuth)
	mux.HandleFunc("/api/lan/backups", s.requireAuth(s.handleBackups))
	mux.HandleFunc("/api/lan/download/", s.requireAuth(s.handleDownload))
	mux.HandleFunc("/api/lan/download", s.requireAuth(s.handleDownload))

	// Static SPA handler
	mux.HandleFunc("/", s.handleStaticSPA)
}

func clientIP(r *http.Request) string {
	if fwd := r.Header.Get("X-Forwarded-For"); fwd != "" {
		parts := strings.Split(fwd, ",")
		return strings.TrimSpace(parts[0])
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}

func (s *Server) checkRateLimit(ip string) bool {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	attempts := s.failedAttempts[ip]
	var recent []time.Time
	for _, t := range attempts {
		if now.Sub(t) < 30*time.Second {
			recent = append(recent, t)
		}
	}
	if len(recent) == 0 {
		delete(s.failedAttempts, ip)
	} else {
		s.failedAttempts[ip] = recent
	}
	return len(recent) < 5
}

func (s *Server) recordFailedAttempt(ip string) {
	now := time.Now()
	s.mu.Lock()
	defer s.mu.Unlock()

	// Bound failedAttempts map: clean expired entries first
	if len(s.failedAttempts) >= maxFailedAttemptIPs {
		for k, att := range s.failedAttempts {
			var valid []time.Time
			for _, t := range att {
				if now.Sub(t) < 30*time.Second {
					valid = append(valid, t)
				}
			}
			if len(valid) == 0 {
				delete(s.failedAttempts, k)
			} else {
				s.failedAttempts[k] = valid
			}
		}
		// If still at capacity, evict oldest IP
		for len(s.failedAttempts) >= maxFailedAttemptIPs {
			var oldestIP string
			var oldestTime time.Time
			first := true
			for k, att := range s.failedAttempts {
				for _, t := range att {
					if first || t.Before(oldestTime) {
						oldestTime = t
						oldestIP = k
						first = false
					}
				}
			}
			if oldestIP != "" {
				delete(s.failedAttempts, oldestIP)
			} else {
				break
			}
		}
	}

	s.failedAttempts[ip] = append(s.failedAttempts[ip], now)
}

func (s *Server) createSession() string {
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	// Bound sessions: evict expired entries first
	if len(s.sessions) >= maxSessions {
		for k, exp := range s.sessions {
			if now.After(exp) {
				delete(s.sessions, k)
			}
		}
		// If still at capacity, evict oldest session (earliest expiry)
		for len(s.sessions) >= maxSessions {
			var oldestToken string
			var earliestExpiry time.Time
			first := true
			for tok, exp := range s.sessions {
				if first || exp.Before(earliestExpiry) {
					earliestExpiry = exp
					oldestToken = tok
					first = false
				}
			}
			if oldestToken != "" {
				delete(s.sessions, oldestToken)
			} else {
				break
			}
		}
	}

	s.sessions[token] = now.Add(24 * time.Hour)
	return token
}

func (s *Server) isSessionValid(token string) bool {
	if token == "" {
		return false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()

	expiry, found := s.sessions[token]
	if !found {
		return false
	}
	return time.Now().Before(expiry)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.RLock()
		pinEnabled := s.pinEnabled
		s.mu.RUnlock()

		if !pinEnabled {
			s.activeConns.Add(1)
			defer s.activeConns.Add(-1)
			next(w, r)
			return
		}

		// Check session token in cookie or headers
		token := ""
		if cookie, err := r.Cookie("lan_token"); err == nil {
			token = cookie.Value
		}
		if token == "" {
			authHeader := r.Header.Get("Authorization")
			if strings.HasPrefix(authHeader, "Bearer ") {
				token = strings.TrimPrefix(authHeader, "Bearer ")
			}
		}
		if token == "" {
			token = r.Header.Get("X-LAN-Token")
		}
		if token == "" {
			token = r.URL.Query().Get("token")
		}

		if !s.isSessionValid(token) {
			w.Header().Set("Content-Type", "application/json; charset=utf-8")
			w.WriteHeader(http.StatusUnauthorized)
			_ = json.NewEncoder(w).Encode(map[string]interface{}{
				"error":   "unauthorized",
				"message": "PIN 认证失败或已过期",
			})
			return
		}

		s.activeConns.Add(1)
		defer s.activeConns.Add(-1)
		next(w, r)
	}
}

func (s *Server) handleInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	status := s.GetStatus()
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"name":         status.ShareName,
		"pin_enabled":  status.PINEnabled,
		"app_version":  s.appVersion,
		"port":         status.Port,
		"shared_count": status.SharedCount,
	})
}

func (s *Server) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	ip := clientIP(r)
	if !s.checkRateLimit(ip) {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "尝试次数过多，请 30 秒后再试",
		})
		return
	}

	var req struct {
		PIN string `json:"pin"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(http.StatusBadRequest)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": false,
			"message": "无效的请求格式",
		})
		return
	}

	s.mu.RLock()
	currentPIN := s.pin
	pinEnabled := s.pinEnabled
	s.mu.RUnlock()

	if !pinEnabled || req.PIN == currentPIN {
		token := s.createSession()
		http.SetCookie(w, &http.Cookie{
			Name:     "lan_token",
			Value:    token,
			Path:     "/",
			HttpOnly: true,
			SameSite: http.SameSiteLaxMode,
			MaxAge:   86400,
		})
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"success": true,
			"token":   token,
		})
		return
	}

	s.recordFailedAttempt(ip)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(http.StatusUnauthorized)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": false,
		"message": "PIN 码错误",
	})
}

func makeItemID(raw string) string {
	sum := md5.Sum([]byte(raw))
	return hex.EncodeToString(sum[:])[:8]
}

func (s *Server) handleBackups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	s.mu.RLock()
	zips := make([]string, len(s.sharedZIPs))
	copy(zips, s.sharedZIPs)
	locals := make([]SharedLocalInfo, 0, len(s.sharedLocals))
	for _, l := range s.sharedLocals {
		locals = append(locals, l)
	}
	s.mu.RUnlock()

	var items []SharedBackupItem

	// 1. Process shared ZIPs
	for _, zp := range zips {
		fi, err := os.Stat(zp)
		if err != nil {
			continue
		}
		item := SharedBackupItem{
			ID:          makeItemID(zp),
			Filename:    filepath.Base(zp),
			Size:        fi.Size(),
			BackupName:  strings.TrimSuffix(filepath.Base(zp), filepath.Ext(zp)),
			CreatedAt:   fi.ModTime().Format(time.RFC3339),
			IsLocal:     false,
			ContainsCfg: true,
		}

		if val, err := ValidatePackage(zp); err == nil {
			item.BackupName = val.Metadata.Name
			item.ContainsCfg = val.Metadata.ContainsCfg
			item.ContainsUserdata = val.Metadata.ContainsUserdata
			item.AccountID = val.Metadata.AccountID
			if val.Metadata.Created != "" {
				item.CreatedAt = val.Metadata.Created
			}
		}

		items = append(items, item)
	}

	// 2. Process shared local backups
	for _, loc := range locals {
		fi, err := os.Stat(loc.Dir)
		if err != nil {
			continue
		}

		var totalSize int64
		hasCfg := false
		_ = filepath.Walk(loc.Dir, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				totalSize += info.Size()
				if strings.Contains(filepath.ToSlash(path), "/cfg/") {
					hasCfg = true
				}
			}
			return nil
		})

		items = append(items, SharedBackupItem{
			ID:               makeItemID("local_" + loc.AccountID),
			Filename:         fmt.Sprintf("CS2Backup_Local_%s.zip", loc.AccountID),
			Size:             totalSize,
			BackupName:       loc.Name,
			CreatedAt:        fi.ModTime().Format(time.RFC3339),
			ContainsCfg:      hasCfg,
			ContainsUserdata: true,
			IsLocal:          true,
			AccountID:        loc.AccountID,
		})
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"backups": items,
	})
}

func (s *Server) handleDownload(w http.ResponseWriter, r *http.Request) {
	// 限速挂在写出层，两条下载路径共用。
	w = s.wrapRateLimit(w)

	if r.Method != http.MethodGet {
		http.Error(w, "Method Not Allowed", http.StatusMethodNotAllowed)
		return
	}

	// Extract backup ID from URL path or query
	id := strings.TrimPrefix(r.URL.Path, "/api/lan/download/")
	id = strings.TrimPrefix(id, "/api/lan/download")
	id = strings.Trim(id, "/")
	if id == "" {
		id = r.URL.Query().Get("id")
	}

	if id == "" {
		http.Error(w, "Backup ID required", http.StatusBadRequest)
		return
	}

	s.mu.RLock()
	var matchedZIP string
	for _, zp := range s.sharedZIPs {
		if makeItemID(zp) == id {
			matchedZIP = zp
			break
		}
	}
	var matchedLocal *SharedLocalInfo
	for _, loc := range s.sharedLocals {
		if makeItemID("local_"+loc.AccountID) == id {
			tmp := loc
			matchedLocal = &tmp
			break
		}
	}
	s.mu.RUnlock()

	// 1. If it's a pre-built ZIP package
	if matchedZIP != "" {
		f, err := os.Open(matchedZIP)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}
		defer f.Close()

		fi, err := f.Stat()
		if err != nil {
			http.Error(w, "Cannot read file stats", http.StatusInternalServerError)
			return
		}

		filename := filepath.Base(matchedZIP)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))

		http.ServeContent(w, r, filename, fi.ModTime(), f)
		return
	}

	// 2. 本地备份目录：先打包到临时文件，再交给 ServeContent。
	//
	// 直接往 ResponseWriter 里流式写虽然省一次落盘，但代价是拿不到
	// Content-Length：浏览器端进度条只能恒为 0，也没有 Range 可续传。
	// 多一次落盘换「进度 + 续传 + 统一路径」，值得。
	if matchedLocal != nil {
		filename := fmt.Sprintf("CS2Backup_Local_%s.zip", matchedLocal.AccountID)

		tmp, err := os.CreateTemp("", "cs2_lan_pkg_*.zip")
		if err != nil {
			http.Error(w, "Cannot prepare package", http.StatusInternalServerError)
			return
		}
		tmpPath := tmp.Name()
		// ServeContent 是同步写完才返回的，所以返回后再删是安全的。
		defer func() {
			_ = tmp.Close()
			_ = os.Remove(tmpPath)
		}()

		cfgDir := filepath.Join(matchedLocal.Dir, "cfg")
		hasCfg := false
		if st, err := os.Stat(cfgDir); err == nil && st.IsDir() {
			hasCfg = true
		}

		opts := PackageOptions{
			AccountID:       matchedLocal.AccountID,
			Name:            matchedLocal.Name,
			IncludeUserdata: true,
			UserdataDir:     matchedLocal.Dir,
			IncludeCfg:      hasCfg,
			CfgDir:          cfgDir,
		}

		if _, err := StreamLANPackage(tmp, opts); err != nil {
			http.Error(w, "Failed to build package", http.StatusInternalServerError)
			return
		}
		// Windows 上不先关掉，Stat 拿到的长度可能还没刷出来。
		if err := tmp.Close(); err != nil {
			http.Error(w, "Failed to flush package", http.StatusInternalServerError)
			return
		}

		fi, err := os.Stat(tmpPath)
		if err != nil {
			http.Error(w, "Cannot read package stats", http.StatusInternalServerError)
			return
		}

		f, err := os.Open(tmpPath)
		if err != nil {
			http.Error(w, "Cannot open package", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%q", filename))
		w.Header().Set("Content-Length", fmt.Sprintf("%d", fi.Size()))

		http.ServeContent(w, r, filename, fi.ModTime(), f)
		return
	}

	http.Error(w, "Backup not found", http.StatusNotFound)
}

func (s *Server) handleStaticSPA(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}

	s.mu.RLock()
	assets := s.assets
	s.mu.RUnlock()

	if assets == nil {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`<!DOCTYPE html><html><head><title>CS2 LAN Share</title></head><body style="background:#111;color:#fff;font-family:sans-serif;padding:40px;"><h2>CS2 LAN Share Server</h2><p>Server is running. Embedded assets not mounted in current test session.</p></body></html>`))
		return
	}

	relPath := strings.TrimPrefix(r.URL.Path, "/")
	if relPath == "" {
		relPath = "index.html"
	}

	// Try serving static asset directly
	f, err := assets.Open(relPath)
	if err == nil {
		defer f.Close()
		if fi, err := f.Stat(); err == nil && !fi.IsDir() {
			http.FileServer(http.FS(assets)).ServeHTTP(w, r)
			return
		}
	}

	// SPA fallback: serve index.html
	indexFile, err := assets.Open("index.html")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	defer indexFile.Close()

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = io.Copy(w, indexFile)
}
