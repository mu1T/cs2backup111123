package lan

import (
	"archive/zip"
	"bytes"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

var pkgModTime = time.Date(2026, 11, 6, 18, 0, 0, 0, time.UTC)

// buildZip 造一个内容确定的 ZIP，用于验证「续传拼出来的包还是好包」。
func buildZip(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	f, err := zw.Create("userdata/readme.txt")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.Write(bytes.Repeat([]byte("x"), 4096)); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// startRangeServer 起一个支持 Range 的下载服务，记录客户端发来的 Range 头。
func startRangeServer(t *testing.T, payload []byte) (*httptest.Server, *string) {
	t.Helper()
	var sawRange string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sawRange = r.Header.Get("Range")
		w.Header().Set("Content-Type", "application/zip")
		http.ServeContent(w, r, "pkg.zip", pkgModTime, bytes.NewReader(payload))
	}))
	t.Cleanup(srv.Close)
	return srv, &sawRange
}

func hostPort(t *testing.T, srv *httptest.Server) (string, int) {
	t.Helper()
	host, portStr, err := net.SplitHostPort(srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	port, _ := strconv.Atoi(portStr)
	return host, port
}

/**
 * 断点续传的端到端行为：本地已有一半，服务端支持 Range，最终拿到的还是完整好包。
 *
 * 这条测试上一轮跑出过 `negative offset`——根因是拿 206 的 Content-Length
 * （那一段的长度）当成了整个包的长度，于是好断点被判失效、只写了后半段。
 * 现在总长度改从 Content-Range 取，这条测试就是那个 bug 的回归。
 */
func TestDownloadRemoteBackup_ResumesFromPartial(t *testing.T) {
	payload := buildZip(t)
	srv, sawRange := startRangeServer(t, payload)
	host, port := hostPort(t, srv)

	const backupID = "pkg-1"
	partPath := partialPath(host, port, backupID)
	t.Cleanup(func() {
		_ = os.Remove(partPath)
		_ = os.Remove(partPath + ".meta")
	})

	// 预置断点：前一半已落盘，元信息记着完整包长与修改时间
	half := len(payload) / 2
	if err := os.WriteFile(partPath, payload[:half], 0o644); err != nil {
		t.Fatal(err)
	}
	if err := savePartialMeta(partPath+".meta", ResumeMeta{Total: int64(len(payload)), ModTime: pkgModTime}); err != nil {
		t.Fatal(err)
	}

	backupDir := testutil.TempDir(t)
	if _, err := NewClient().DownloadRemoteBackup(host, port, "", backupID, backupDir); err != nil {
		t.Fatalf("续传下载失败: %v", err)
	}

	if *sawRange != "bytes="+strconv.Itoa(half)+"-" {
		t.Errorf("Range 头应为 bytes=%d-，实际 %q", half, *sawRange)
	}

	// 解出来的文件长度必须与原始内容一致——拼坏了这里就露馅
	entries, err := filepath.Glob(filepath.Join(backupDir, "userdata_backup", "remote_*", "userdata", "readme.txt"))
	if err != nil || len(entries) != 1 {
		t.Fatalf("解包结果不对: %v, %v", entries, err)
	}
	data, err := os.ReadFile(entries[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(data) != 4096 {
		t.Errorf("解出的文件应为 4096 字节，实际 %d —— 断点拼接坏了", len(data))
	}

	// 成功后断点文件应该被清理
	if _, err := os.Stat(partPath); !os.IsNotExist(err) {
		t.Error("下载成功后断点文件应被清理")
	}
}

/**
 * 包变了（长度不同）时断点作废，从头下——不能把两个版本拼在一起。
 */
func TestDownloadRemoteBackup_StalePartialStartsOver(t *testing.T) {
	payload := buildZip(t)
	srv, sawRange := startRangeServer(t, payload)
	host, port := hostPort(t, srv)

	const backupID = "pkg-2"
	partPath := partialPath(host, port, backupID)
	t.Cleanup(func() {
		_ = os.Remove(partPath)
		_ = os.Remove(partPath + ".meta")
	})

	// 断点元信息里的总长度与真实包不一致 → 断点属于另一个包
	if err := os.WriteFile(partPath, []byte("垃圾数据"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := savePartialMeta(partPath+".meta", ResumeMeta{Total: 999999, ModTime: pkgModTime}); err != nil {
		t.Fatal(err)
	}

	backupDir := testutil.TempDir(t)
	if _, err := NewClient().DownloadRemoteBackup(host, port, "", backupID, backupDir); err != nil {
		t.Fatalf("下载失败: %v", err)
	}

	// 断点无效时仍会发 Range（发了才知道服务端认不认），但结果必须是完整包
	if *sawRange == "" {
		t.Log("没有发 Range：断点被判无效，符合预期")
	}
	entries, _ := filepath.Glob(filepath.Join(backupDir, "userdata_backup", "remote_*", "userdata", "readme.txt"))
	if len(entries) != 1 {
		t.Fatalf("解包结果不对: %v", entries)
	}
	data, _ := os.ReadFile(entries[0])
	if len(data) != 4096 {
		t.Errorf("包变了之后应从头上重下，解出 %d 字节（期望 4096）", len(data))
	}
}
