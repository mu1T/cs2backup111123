package lan

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// ResumeMeta 是一个断点的元信息：它属于「哪一次传输」。
//
// 断点属于一次传输，不属于一个包。包内容变了（长度或修改时间不同），
// 断点就失效——否则会把两个版本的包拼成一个坏包。
type ResumeMeta struct {
	// Total 是完整包的长度，来自响应头 Content-Length。
	Total int64 `json:"total"`
	// ModTime 是包的修改时间，来自响应头 Last-Modified（零值表示服务端没给）。
	ModTime time.Time `json:"mod_time"`
}

// ResumeDecision 描述「这次能不能接着下、从哪开始」。
type ResumeDecision struct {
	// Skip 是从头跳过的字节数；0 表示从头下。
	Skip int64
	// Valid 为 false 时，即使有断点文件也要丢掉重下。
	Valid bool
}

// DecideResume 判断已有的断点还能不能用。
//
// 三个条件同时满足才续传：
//  1. 断点文件里有已落盘的字节（have > 0）；
//  2. 包没变过——长度相同，且（服务端给了修改时间时）修改时间相同；
//  3. 断点没超过包本身（have < total，等于 total 说明已经下完了）。
//
// 这个函数不碰磁盘也不碰网络，所以「包变了要作废断点」这条最容易写错的规则
// 可以毫秒级地测出来。
func DecideResume(have int64, stored ResumeMeta, respTotal int64, respMod time.Time) ResumeDecision {
	if have <= 0 {
		return ResumeDecision{}
	}

	// 长度对不上：包换了，断点作废。
	if stored.Total != respTotal || respTotal <= 0 {
		return ResumeDecision{}
	}

	// 修改时间：只在两边都有值时才比较。服务端不给 Last-Modified 时，
	// 长度相同就当作同一个包——比因为「没给时间」而永远不能续传要好。
	if !stored.ModTime.IsZero() && !respMod.IsZero() && !stored.ModTime.Equal(respMod) {
		return ResumeDecision{}
	}

	// 已经下完了，没什么可续的。
	if have >= respTotal {
		return ResumeDecision{}
	}

	return ResumeDecision{Skip: have, Valid: true}
}

// partialPath 是某个包在本地断点的固定位置。
//
// 用 IP+端口+包 ID 做键：同一个包在同一个来源上只有一个断点，换一台机器或换一个
// 包都是不同的断点，不会互相干扰。
func partialPath(ip string, port int, backupID string) string {
	sum := sha256.Sum256([]byte(fmt.Sprintf("%s:%d/%s", ip, port, backupID)))
	return filepath.Join(os.TempDir(), fmt.Sprintf("cs2_remote_dl_%x.zip", sum[:8]))
}

// loadPartial 读出「已落盘多少字节」与它的元信息。
//
// 元信息读不到或损坏时返回零值 + 实际大小：调用方拿到 have > 0 但 stored 为空，
// DecideResume 会因为长度对不上而判无效，从而从头下——宁可重下也不要拼坏包。
func loadPartial(partPath, metaPath string) (ResumeMeta, int64) {
	var meta ResumeMeta
	if data, err := os.ReadFile(metaPath); err == nil {
		_ = json.Unmarshal(data, &meta)
	}

	fi, err := os.Stat(partPath)
	if err != nil || fi.IsDir() {
		return meta, 0
	}
	return meta, fi.Size()
}

// savePartialMeta 落盘断点元信息。
func savePartialMeta(metaPath string, meta ResumeMeta) error {
	data, err := json.Marshal(meta)
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0o644)
}

// parseHTTPTime 解析 Last-Modified。解析不出来返回零值，由 DecideResume
// 按「只比长度」处理。
func parseHTTPTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := http.ParseTime(raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

// parseContentRangeTotal 从 `Content-Range: bytes 100-199/200` 里取出总长度。
//
// **这是续传最容易踩的坑**：206 响应的 `Content-Length` 是「这一段」的长度，
// 不是整个包的长度。拿它当总长度去比断点，会得出「包变了」的错误结论，
// 于是丢掉断点只写后半段——最后得到一个解压失败的残包。
func parseContentRangeTotal(raw string) int64 {
	if raw == "" {
		return 0
	}
	slash := strings.LastIndex(raw, "/")
	if slash < 0 {
		return 0
	}
	tail := strings.TrimSpace(raw[slash+1:])
	if tail == "" || tail == "*" {
		return 0
	}
	total, err := strconv.ParseInt(tail, 10, 64)
	if err != nil || total <= 0 {
		return 0
	}
	return total
}
