package lan

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

/**
 * 从「本地备份目录」下载时，响应必须带 Content-Length 且支持 Range。
 *
 * 以前这条路径是边打包边往 ResponseWriter 写，chunked 传输、没有长度：
 * 浏览器端进度条恒为 0，也没法续传。现在先打包到临时文件再走 ServeContent，
 * 这两件事一起解决。
 */
func TestDownloadLocalBackup_ContentLengthAndRange(t *testing.T) {
	dir := testutil.TempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "cs2_video.txt"), []byte("hello lan"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer()
	srv.AddLocalBackup("123456789", "tester", dir)
	handler := srv.requireAuth(srv.handleDownload)
	id := makeItemID("local_123456789")

	// 1. 不带 Range：200 + 明确的 Content-Length，内容是一个合法 ZIP
	req := httptest.NewRequest(http.MethodGet, "/api/lan/download/"+id, nil)
	rec := httptest.NewRecorder()
	handler(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d，body=%s", rec.Code, rec.Body.String())
	}
	declared := rec.Header().Get("Content-Length")
	if declared == "" {
		t.Fatal("响应缺少 Content-Length——浏览器端进度条会恒为 0")
	}
	body := rec.Body.Bytes()
	if declared != itoa(len(body)) {
		t.Errorf("Content-Length=%s 与实际长度 %d 不符", declared, len(body))
	}
	if _, err := zip.NewReader(bytes.NewReader(body), int64(len(body))); err != nil {
		t.Errorf("下载内容不是合法 ZIP: %v", err)
	}

	// 2. 带 Range：206 + Content-Range，只回请求的那一段
	req2 := httptest.NewRequest(http.MethodGet, "/api/lan/download/"+id, nil)
	req2.Header.Set("Range", "bytes=0-9")
	rec2 := httptest.NewRecorder()
	handler(rec2, req2)

	if rec2.Code != http.StatusPartialContent {
		t.Fatalf("带 Range 时期望 206，实际 %d", rec2.Code)
	}
	if cr := rec2.Header().Get("Content-Range"); cr == "" {
		t.Error("206 响应缺少 Content-Range")
	}
	if rec2.Body.Len() != 10 {
		t.Errorf("期望只回 10 字节，实际 %d 字节", rec2.Body.Len())
	}
	if !bytes.Equal(rec2.Body.Bytes(), body[:10]) {
		t.Error("Range 返回的字节与完整内容的对应片段不一致")
	}
}

/**
 * 临时打包文件在请求结束后必须被清理，不能在系统临时目录里越堆越多。
 */
func TestDownloadLocalBackup_CleansTempPackage(t *testing.T) {
	dir := testutil.TempDir(t)
	if err := os.WriteFile(filepath.Join(dir, "a.cfg"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	srv := NewServer()
	srv.AddLocalBackup("42", "tester", dir)
	handler := srv.requireAuth(srv.handleDownload)

	before := countTempPackages(t)
	rec := httptest.NewRecorder()
	handler(rec, httptest.NewRequest(http.MethodGet, "/api/lan/download/"+makeItemID("local_42"), nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("期望 200，实际 %d", rec.Code)
	}

	if after := countTempPackages(t); after > before {
		t.Errorf("请求结束后残留了临时打包文件：请求前 %d 个，请求后 %d 个", before, after)
	}
}

/**
 * idleResetReader 只在真读到数据时重置计时器。
 *
 * 这是「大包不被 30 秒总超时掐断」的机制：只要还在收字节，计时器就一直往后推；
 * 真的卡住了（读到 0 字节且没有新数据）才让超时生效。
 */
func TestIdleResetReader_ResetsOnlyOnData(t *testing.T) {
	var resets int
	r := &idleResetReader{r: strings.NewReader("abcdef")}
	// 用假的 timer 计数代替真定时器：这里关心的是「什么时候重置」，不是时长。
	r.timer = fakeResetter{onReset: func() { resets++ }}

	buf := make([]byte, 3)
	if _, err := r.Read(buf); err != nil {
		t.Fatal(err)
	}
	if resets != 1 {
		t.Errorf("读到数据后应重置一次，实际 %d 次", resets)
	}

	// 读空 reader：0 字节，不该重置
	r.r = strings.NewReader("")
	if _, err := r.Read(buf); err == nil {
		t.Fatal("空 reader 应该返回 EOF")
	}
	if resets != 1 {
		t.Errorf("读到 0 字节不该重置，实际变成 %d 次", resets)
	}
}

// fakeResetter 只数被重置了几次。
type fakeResetter struct{ onReset func() }

func (f fakeResetter) Reset(time.Duration) bool {
	if f.onReset != nil {
		f.onReset()
	}
	return true
}

// itoa 避免为一个断言引入 strconv。
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

// countTempPackages 数一下系统临时目录里有多少个我们打的包。
func countTempPackages(t *testing.T) int {
	t.Helper()
	entries, err := os.ReadDir(os.TempDir())
	if err != nil {
		return 0
	}
	n := 0
	for _, e := range entries {
		if strings.HasPrefix(e.Name(), "cs2_lan_pkg_") {
			n++
		}
	}
	return n
}
