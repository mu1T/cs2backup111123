package steam

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

func TestAvatarManager_GetLocalAvatarDataURI(t *testing.T) {
	tempDir := testutil.TempDir(t)
	configDir := filepath.Join(tempDir, "config")
	steamDir := filepath.Join(tempDir, "steam")

	cm, err := NewCacheManager(configDir)
	if err != nil {
		t.Fatal(err)
	}

	am, err := NewAvatarManager(configDir, cm, nil)
	if err != nil {
		t.Fatal(err)
	}
	am.SetSteamPath(steamDir)

	steam64 := uint64(76561198083722517)

	// Initially not found
	uri, found := am.GetLocalAvatarDataURI(steam64)
	if found || uri != "" {
		t.Fatalf("expected not found initially, got uri=%s", uri)
	}

	// Place an image in steam/config/avatarcache/
	steamAvatarDir := filepath.Join(steamDir, "config", "avatarcache")
	_ = os.MkdirAll(steamAvatarDir, 0755)
	dummyPNG := []byte("\x89PNG\r\n\x1a\n\x00\x00\x00\rIHDR...")
	_ = os.WriteFile(filepath.Join(steamAvatarDir, fmt.Sprintf("%d.png", steam64)), dummyPNG, 0644)

	// Now it should be discovered and copied into config/avatarcache
	uri, found = am.GetLocalAvatarDataURI(steam64)
	if !found {
		t.Fatal("expected avatar to be found via steam local cache")
	}
	if len(uri) == 0 {
		t.Fatal("expected non-empty data URI")
	}

	// Verify file was copied to config/avatarcache
	localFile := filepath.Join(configDir, "avatarcache", fmt.Sprintf("%d.png", steam64))
	if _, err := os.Stat(localFile); err != nil {
		t.Fatalf("expected file to be copied to %s: %v", localFile, err)
	}
}

func TestAvatarManager_FailureCooldown(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cm, _ := NewCacheManager(tempDir)

	// Custom HTTP client that always fails
	mockClient := &mockErrorClient{}
	am, err := NewAvatarManager(tempDir, cm, mockClient)
	if err != nil {
		t.Fatal(err)
	}

	accountID := uint32(123456789)
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Second)
	defer cancel()

	// First fetch attempt fails
	_, _, err = am.FetchProfileAndAvatarSync(ctx, accountID)
	if err == nil {
		t.Fatal("expected fetch to fail with error")
	}

	// Second attempt should be immediately rejected due to failure cooldown
	_, _, err = am.FetchProfileAndAvatarSync(ctx, accountID)
	if err == nil || err.Error() != "in failure cooldown" {
		t.Fatalf("expected failure cooldown error, got: %v", err)
	}
}

func TestAvatarManager_FetchSuccess(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cm, _ := NewCacheManager(tempDir)

	avatarBytes := []byte("fake-png-data")
	mockServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/avatar.png" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write(avatarBytes)
			return
		}
		// Return XML
		xmlResp := fmt.Sprintf(`<profile>
			<steamID><![CDATA[TestGamer]]></steamID>
			<avatarFull>%s/avatar.png</avatarFull>
		</profile>`, "http://"+r.Host)
		w.Header().Set("Content-Type", "text/xml")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(xmlResp))
	}))
	defer mockServer.Close()

	customDoer := &testRewriteDoer{
		targetHost: mockServer.Listener.Addr().String(),
		client:     mockServer.Client(),
	}

	am, err := NewAvatarManager(tempDir, cm, customDoer)
	if err != nil {
		t.Fatal(err)
	}

	accountID := uint32(123456789)
	ctx := context.Background()
	name, uri, err := am.FetchProfileAndAvatarSync(ctx, accountID)
	if err != nil {
		t.Fatalf("unexpected fetch error: %v", err)
	}

	if name != "TestGamer" {
		t.Errorf("expected name 'TestGamer', got %q", name)
	}
	if len(uri) == 0 {
		t.Error("expected non-empty avatar data URI")
	}

	// Verify cached in CacheManager
	cachedName, ok := cm.GetUsername("123456789")
	if !ok || cachedName != "TestGamer" {
		t.Errorf("expected name saved to CacheManager, got %q", cachedName)
	}
}

type mockErrorClient struct{}

func (m *mockErrorClient) Do(req *http.Request) (*http.Response, error) {
	return nil, fmt.Errorf("network connection refused")
}

type testRewriteDoer struct {
	targetHost string
	client     *http.Client
}

func (d *testRewriteDoer) Do(req *http.Request) (*http.Response, error) {
	req.URL.Scheme = "http"
	req.URL.Host = d.targetHost
	return d.client.Do(req)
}

func TestAvatarManager_FailCacheBounded(t *testing.T) {
	tempDir := testutil.TempDir(t)
	cm, _ := NewCacheManager(tempDir)
	am, err := NewAvatarManager(tempDir, cm, nil)
	if err != nil {
		t.Fatal(err)
	}

	for i := 0; i < 250; i++ {
		am.recordFailure(fmt.Sprintf("account_%d", i))
	}

	am.mu.RLock()
	cacheLen := len(am.failCache)
	am.mu.RUnlock()

	if cacheLen > MaxFailCacheEntries {
		t.Fatalf("failCache exceeded maximum bound: %d > %d", cacheLen, MaxFailCacheEntries)
	}
}
