package steam

import (
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

// blockingClient 让头像抓取卡在 HTTP 调用上，直到测试放行。
// 用它把「异步」变成可控的时序，从而确定性地验证 WaitBackground 的行为。
type blockingClient struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
}

func (c *blockingClient) Do(_ *http.Request) (*http.Response, error) {
	c.once.Do(func() { close(c.entered) })
	<-c.release
	return nil, http.ErrHandlerTimeout
}

// WaitBackground 必须等异步抓取真正结束才返回，否则调用方会在写入落盘前就清理目录
// （Windows 上表现为 TempDir「目录非空」）。
func TestAvatarManager_WaitBackgroundBlocksUntilFetchDone(t *testing.T) {
	configDir := testutil.TempDir(t)
	cacheMgr, err := NewCacheManager(configDir)
	if err != nil {
		t.Fatalf("NewCacheManager failed: %v", err)
	}
	client := &blockingClient{entered: make(chan struct{}), release: make(chan struct{})}
	am, err := NewAvatarManager(configDir, cacheMgr, client)
	if err != nil {
		t.Fatalf("NewAvatarManager failed: %v", err)
	}

	am.EnqueueFetch(123456789)

	// 等抓取协程真正进到 HTTP 调用，否则下面可能测的是「还没开始」
	select {
	case <-client.entered:
	case <-time.After(2 * time.Second):
		t.Fatal("抓取协程没有启动")
	}

	done := make(chan struct{})
	go func() {
		am.WaitBackground()
		close(done)
	}()

	select {
	case <-done:
		t.Fatal("WaitBackground 在抓取仍阻塞时就返回了")
	case <-time.After(80 * time.Millisecond):
		// 正确：仍在等待
	}

	close(client.release)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("抓取结束后 WaitBackground 仍未返回")
	}

	// 放行后失败缓存必须已经落盘——这正是此前与 TempDir 清理撞车的文件
	if _, err := os.Stat(filepath.Join(configDir, ".avatar_failed_cache")); err != nil {
		t.Fatalf("等待返回后失败缓存应已写入: %v", err)
	}
}
