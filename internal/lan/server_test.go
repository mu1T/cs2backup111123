package lan

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"cs2backup/internal/testutil"
)

func TestServerInfoAndLifecycle(t *testing.T) {
	s := NewServer()
	s.SetPort(0) // dynamic port
	s.SetShareName("TestHost")
	s.SetAppVersion("2.0.0")

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	if !s.IsRunning() {
		t.Fatalf("server should be marked running")
	}

	port := s.GetPort()
	if port <= 0 {
		t.Fatalf("server port not assigned: %d", port)
	}

	resp, err := http.Get(fmt.Sprintf("http://127.0.0.1:%d/api/lan/info", port))
	if err != nil {
		t.Fatalf("failed GET /api/lan/info: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK, got %d", resp.StatusCode)
	}

	var info map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		t.Fatalf("decode info response failed: %v", err)
	}

	if info["name"] != "TestHost" {
		t.Errorf("info name = %v; want TestHost", info["name"])
	}
	if info["app_version"] != "2.0.0" {
		t.Errorf("info app_version = %v; want 2.0.0", info["app_version"])
	}

	if err := s.Stop(); err != nil {
		t.Errorf("stop failed: %v", err)
	}
	if s.IsRunning() {
		t.Errorf("server still running after Stop")
	}
}

func TestPINAuthAndRateLimit(t *testing.T) {
	s := NewServer()
	s.SetPort(0)
	s.SetPINEnabled(true)
	pin := s.GetPIN()
	if len(pin) != 6 {
		t.Fatalf("expected 6-digit PIN, got %s", pin)
	}

	if err := s.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}
	defer s.Stop()

	port := s.GetPort()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 1. Unauthenticated request to protected endpoint should fail (401)
	resp, err := http.Get(baseURL + "/api/lan/backups")
	if err != nil {
		t.Fatalf("GET /api/lan/backups failed: %v", err)
	}
	resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", resp.StatusCode)
	}

	// 2. Wrong PIN attempts trigger rate limiting after 5 attempts
	client := &http.Client{Timeout: 2 * time.Second}
	for i := 1; i <= 5; i++ {
		reqBody := `{"pin":"000000"}`
		if pin == "000000" {
			reqBody = `{"pin":"111111"}`
		}
		authResp, err := client.Post(baseURL+"/api/lan/auth", "application/json", strings.NewReader(reqBody))
		if err != nil {
			t.Fatalf("attempt %d failed: %v", i, err)
		}
		authResp.Body.Close()
		if i < 5 && authResp.StatusCode != http.StatusUnauthorized {
			t.Errorf("attempt %d: expected 401, got %d", i, authResp.StatusCode)
		}
	}

	// 6th attempt should be rate limited (429)
	reqBody := `{"pin":"000000"}`
	authResp, err := client.Post(baseURL+"/api/lan/auth", "application/json", strings.NewReader(reqBody))
	if err != nil {
		t.Fatalf("attempt 6 failed: %v", err)
	}
	authResp.Body.Close()
	if authResp.StatusCode != http.StatusTooManyRequests {
		t.Errorf("expected 429 Too Many Requests, got %d", authResp.StatusCode)
	}

	// 3. Clear rate limit by regenerating PIN / resetting failed attempts
	newPIN := s.RegeneratePIN()
	if len(newPIN) != 6 {
		t.Errorf("regenerated PIN invalid: %s", newPIN)
	}

	// 4. Authenticate with correct PIN
	validBody := fmt.Sprintf(`{"pin":"%s"}`, newPIN)
	authResp, err = client.Post(baseURL+"/api/lan/auth", "application/json", strings.NewReader(validBody))
	if err != nil {
		t.Fatalf("valid auth failed: %v", err)
	}
	defer authResp.Body.Close()

	if authResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200 OK for valid PIN, got %d", authResp.StatusCode)
	}

	var authData struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
	}
	if err := json.NewDecoder(authResp.Body).Decode(&authData); err != nil {
		t.Fatalf("decode auth data failed: %v", err)
	}
	if !authData.Success || authData.Token == "" {
		t.Fatalf("auth data invalid: %+v", authData)
	}

	// 5. Access /api/lan/backups with token
	req, _ := http.NewRequest("GET", baseURL+"/api/lan/backups", nil)
	req.Header.Set("Authorization", "Bearer "+authData.Token)
	backupsResp, err := client.Do(req)
	if err != nil {
		t.Fatalf("authenticated GET /api/lan/backups failed: %v", err)
	}
	defer backupsResp.Body.Close()

	if backupsResp.StatusCode != http.StatusOK {
		t.Errorf("expected 200 OK with valid token, got %d", backupsResp.StatusCode)
	}
}

func TestBackupsListAndDownload(t *testing.T) {
	tempDir := testutil.TempDir(t)

	// Create a fake ZIP package
	zipFile := filepath.Join(tempDir, "CS2Backup_Test_1.zip")
	_, err := CreateLANPackage(PackageOptions{
		AccountID:       "123456789",
		Name:            "ZywOo_Shared",
		IncludeUserdata: false,
		IncludeCfg:      false,
		OutputPath:      zipFile,
	})
	if err != nil {
		t.Fatalf("failed creating zip package: %v", err)
	}

	// Create a fake local backup folder: userdata_backup/12345/730/local/cfg/
	localDir := filepath.Join(tempDir, "userdata_backup", "12345", "730", "local", "cfg")
	os.MkdirAll(localDir, 0755)
	os.WriteFile(filepath.Join(localDir, "cs2_video.txt"), []byte("video_settings"), 0644)

	s := NewServer()
	s.SetPort(0)
	s.SetPINEnabled(false) // public mode for ease of test
	s.SetBackupRoot(tempDir)
	s.AddZIPBackup(zipFile)
	s.AddLocalBackup("12345", "Local_s1mple", filepath.Join(tempDir, "userdata_backup", "12345"))

	if err := s.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer s.Stop()

	port := s.GetPort()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// List backups
	resp, err := http.Get(baseURL + "/api/lan/backups")
	if err != nil {
		t.Fatalf("GET /api/lan/backups failed: %v", err)
	}
	defer resp.Body.Close()

	var data struct {
		Backups []SharedBackupItem `json:"backups"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		t.Fatalf("decode backups failed: %v", err)
	}

	if len(data.Backups) != 2 {
		t.Fatalf("expected 2 backups, got %d", len(data.Backups))
	}

	var zipItem, localItem *SharedBackupItem
	for i := range data.Backups {
		if data.Backups[i].IsLocal {
			localItem = &data.Backups[i]
		} else {
			zipItem = &data.Backups[i]
		}
	}

	if zipItem == nil || localItem == nil {
		t.Fatalf("missing zipItem or localItem in backups: %+v", data.Backups)
	}

	// Download ZIP package
	zipResp, err := http.Get(baseURL + "/api/lan/download/" + zipItem.ID)
	if err != nil {
		t.Fatalf("download zip failed: %v", err)
	}
	defer zipResp.Body.Close()
	if zipResp.StatusCode != http.StatusOK {
		t.Errorf("download zip status = %d; want 200", zipResp.StatusCode)
	}
	zipBytes, _ := io.ReadAll(zipResp.Body)
	if len(zipBytes) == 0 {
		t.Errorf("downloaded zip is empty")
	}

	// Download local backup (streams as zip on the fly)
	locResp, err := http.Get(baseURL + "/api/lan/download/" + localItem.ID)
	if err != nil {
		t.Fatalf("download local failed: %v", err)
	}
	defer locResp.Body.Close()
	if locResp.StatusCode != http.StatusOK {
		t.Errorf("download local status = %d; want 200", locResp.StatusCode)
	}
	locBytes, _ := io.ReadAll(locResp.Body)
	if len(locBytes) == 0 {
		t.Errorf("downloaded local backup is empty")
	}

	// Verify local downloaded zip contains restore.bat
	zr, err := zip.NewReader(bytes.NewReader(locBytes), int64(len(locBytes)))
	if err != nil {
		t.Fatalf("failed reading downloaded local zip: %v", err)
	}
	hasRestore := false
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, "restore.bat") {
			hasRestore = true
		}
	}
	if !hasRestore {
		t.Errorf("downloaded local zip missing restore.bat")
	}
}

func TestStaticSPAServing(t *testing.T) {
	mockFS := fstest.MapFS{
		"index.html": &fstest.MapFile{
			Data: []byte("<!DOCTYPE html><html><body>CS2 App</body></html>"),
		},
		"assets/app.js": &fstest.MapFile{
			Data: []byte("console.log('CS2');"),
		},
	}

	s := NewServer()
	s.SetPort(0)
	s.SetAssets(mockFS)

	if err := s.Start(); err != nil {
		t.Fatalf("start failed: %v", err)
	}
	defer s.Stop()

	port := s.GetPort()
	baseURL := fmt.Sprintf("http://127.0.0.1:%d", port)

	// 1. Root / -> index.html
	resp, err := http.Get(baseURL + "/")
	if err != nil {
		t.Fatalf("GET / failed: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if !strings.Contains(string(body), "CS2 App") {
		t.Errorf("expected index.html, got: %s", string(body))
	}

	// 2. Asset /assets/app.js
	respJs, err := http.Get(baseURL + "/assets/app.js")
	if err != nil {
		t.Fatalf("GET /assets/app.js failed: %v", err)
	}
	defer respJs.Body.Close()
	jsBody, _ := io.ReadAll(respJs.Body)
	if !strings.Contains(string(jsBody), "console.log('CS2')") {
		t.Errorf("expected js asset, got: %s", string(jsBody))
	}

	// 3. Unknown route /lan-sharing/unknown -> SPA fallback to index.html
	respFallback, err := http.Get(baseURL + "/lan-sharing/unknown")
	if err != nil {
		t.Fatalf("GET fallback failed: %v", err)
	}
	defer respFallback.Body.Close()
	fallbackBody, _ := io.ReadAll(respFallback.Body)
	if !strings.Contains(string(fallbackBody), "CS2 App") {
		t.Errorf("expected SPA fallback to index.html, got: %s", string(fallbackBody))
	}
}

func TestServer_SessionsBounded(t *testing.T) {
	s := NewServer()
	for i := 0; i < 250; i++ {
		_ = s.createSession()
	}

	s.mu.RLock()
	sessionsLen := len(s.sessions)
	s.mu.RUnlock()

	if sessionsLen > maxSessions {
		t.Fatalf("sessions exceeded maximum bound: %d > %d", sessionsLen, maxSessions)
	}
}

func TestServer_FailedAttemptsBounded(t *testing.T) {
	s := NewServer()
	for i := 0; i < 250; i++ {
		s.recordFailedAttempt(fmt.Sprintf("192.168.1.%d", i))
	}

	s.mu.RLock()
	failedLen := len(s.failedAttempts)
	s.mu.RUnlock()

	if failedLen > maxFailedAttemptIPs {
		t.Fatalf("failedAttempts exceeded maximum bound: %d > %d", failedLen, maxFailedAttemptIPs)
	}
}
