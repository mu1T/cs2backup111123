package lan

import (
	"context"
	"archive/zip"
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RemoteBackupItem represents a backup item available from a remote LAN device.
type RemoteBackupItem struct {
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

// DownloadedBackupResult describes the unpacked remote backup on disk.
type DownloadedBackupResult struct {
	AccountID    string `json:"account_id"`
	DestDir      string `json:"dest_dir"`
	TotalFiles   int    `json:"total_files"`
	TotalBytes   int64  `json:"total_bytes"`
	SourceName   string `json:"source_name"`
	DownloadTime string `json:"download_time"`
}

// RemoteInfoMetadata describes the metadata written into remote_info.json.
type RemoteInfoMetadata struct {
	AccountID    string `json:"account_id"`
	Username     string `json:"username"`
	SourceName   string `json:"source_name"`
	DownloadTime string `json:"download_time"`
	Type         string `json:"type"`
}

// idleResetReader 每读到数据就把空闲超时的计时器往后推。
//
// 放在这里而不是塞进 Transport：Transport 只能设「等响应头」的超时，
// 一旦开始读 body 它就不管了，而大包恰恰是在读 body 的阶段卡住的。
// timerResetter 让 idleResetReader 不直接依赖 *time.Timer——测试里塞一个
// 计数用的假实现，就能断言「什么时候重置」而不用真的等 60 秒。
type timerResetter interface {
	Reset(d time.Duration) bool
}

type idleResetReader struct {
	r     io.Reader
	timer timerResetter
}

func (t *idleResetReader) Read(p []byte) (int, error) {
	n, err := t.r.Read(p)
	if n > 0 && t.timer != nil {
		t.timer.Reset(downloadIdleTimeout)
	}
	return n, err
}

// Client provides an HTTP client for communicating with remote LAN sharing servers.
type Client struct {
	httpClient *http.Client
}

// downloadIdleTimeout 是一次下载「多久收不到字节」算超时。
//
// 以前 http.Client 设的是 30 秒**总超时**——那是整个请求的生命周期，不是空闲
// 超时。局域网传一个 GB 级的备份包必然超过 30 秒，于是下到一半被直接掐断，
// 表现为「下载失败」，其实只是没传完。
const downloadIdleTimeout = 60 * time.Second

// NewClient creates a new Client configured with resilient connection timeouts.
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			// 不设总超时：大包需要多久就多久。真正的护栏是下面的空闲超时。
			Timeout: 0,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout: 5 * time.Second,
				}).DialContext,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
	}
}

// Close closes any idle HTTP connections.
func (c *Client) Close() {
	if c.httpClient != nil {
		c.httpClient.CloseIdleConnections()
	}
}

// Authenticate verifies the PIN code with the remote device and returns a session token.
func (c *Client) Authenticate(ip string, port int, pin string) (string, error) {
	reqBody, _ := json.Marshal(map[string]string{
		"pin": pin,
	})

	url := fmt.Sprintf("http://%s:%d/api/lan/auth", ip, port)
	req, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(reqBody))
	if err != nil {
		return "", fmt.Errorf("failed to create auth request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("failed to connect to remote device at %s:%d: %w", ip, port, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusTooManyRequests {
		return "", errors.New("尝试次数过多，已被临时锁定，请 30 秒后再试")
	}

	var authResp struct {
		Success bool   `json:"success"`
		Token   string `json:"token"`
		Message string `json:"message"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&authResp); err != nil {
		return "", fmt.Errorf("invalid response from remote server (status %d)", resp.StatusCode)
	}

	if !authResp.Success || resp.StatusCode != http.StatusOK {
		msg := authResp.Message
		if msg == "" {
			msg = "PIN 码错误或认证失败"
		}
		return "", errors.New(msg)
	}

	return authResp.Token, nil
}

// GetRemoteBackups retrieves the list of backups exposed by the target device.
func (c *Client) GetRemoteBackups(ip string, port int, pin string) ([]RemoteBackupItem, error) {
	var token string
	if pin != "" {
		tok, err := c.Authenticate(ip, port, pin)
		if err != nil {
			return nil, err
		}
		token = tok
	}

	url := fmt.Sprintf("http://%s:%d/api/lan/backups", ip, port)
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("X-LAN-Token", token)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to request backups from %s:%d: %w", ip, port, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("该设备启用了 PIN 保护，请输入正确的 PIN 码进行连接")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch backups (status %d)", resp.StatusCode)
	}

	var data struct {
		Backups []RemoteBackupItem `json:"backups"`
		Error   string             `json:"error"`
		Message string             `json:"message"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, fmt.Errorf("failed to decode backups response: %w", err)
	}

	return data.Backups, nil
}

// DownloadRemoteBackup streams a remote ZIP package, validates against Zip-Slip,
// unpacks into <backupPath>/userdata_backup/remote_<timestamp>/, writes remote_info.json,
// and returns summary details.
func (c *Client) DownloadRemoteBackup(ip string, port int, pin, backupID, backupPath string) (*DownloadedBackupResult, error) {
	if backupID == "" {
		return nil, errors.New("backupID cannot be empty")
	}

	var token string
	if pin != "" {
		tok, err := c.Authenticate(ip, port, pin)
		if err != nil {
			return nil, err
		}
		token = tok
	}

	url := fmt.Sprintf("http://%s:%d/api/lan/download/%s", ip, port, backupID)

	// 断点用「确定的路径」而不是 os.CreateTemp 的随机名——随机名活不过一次
	// 失败，下次进来又是一个新文件，续传无从谈起。
	partPath := partialPath(ip, port, backupID)
	metaPath := partPath + ".meta"
	stored, have := loadPartial(partPath, metaPath)

	// 空闲超时：每读到一段数据就重置计时器，长时间一个字节都收不到才取消。
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	idle := time.AfterFunc(downloadIdleTimeout, cancel)
	defer idle.Stop()

	// 造请求。Range 只在「先试探一次」时带上：如果服务端回的包与断点对不上，
	// 还要用同一个闭包重造一个不带 Range 的，拿完整包。
	newReq := func(resumeFrom int64) (*http.Request, error) {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
			req.Header.Set("X-LAN-Token", token)
			q := req.URL.Query()
			q.Set("token", token)
			req.URL.RawQuery = q.Encode()
		}
		if resumeFrom > 0 {
			req.Header.Set("Range", fmt.Sprintf("bytes=%d-", resumeFrom))
		}
		return req, nil
	}

	req, err := newReq(have)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to initiate download: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, errors.New("下载未授权，请检查 PIN 码")
	}
	if resp.StatusCode == http.StatusNotFound {
		return nil, errors.New("远端备份文件未找到或已被移除")
	}
	// 206 = 服务端接受了 Range；200 = 从头给（断点无效，或者本来就没有）。
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
	}

	// 总长度：200 看 Content-Length；**206 必须看 Content-Range**，因为那时
	// Content-Length 是这一段的长度。搞错这一点会把好断点判成失效，只写后半段，
	// 最后得到一个解压失败的残包。
	respTotal := resp.ContentLength
	if resp.StatusCode == http.StatusPartialContent {
		if total := parseContentRangeTotal(resp.Header.Get("Content-Range")); total > 0 {
			respTotal = total
		}
	}
	respMod := parseHTTPTime(resp.Header.Get("Last-Modified"))

	decision := DecideResume(have, stored, respTotal, respMod)
	if resp.StatusCode == http.StatusOK {
		// 服务端没接受 Range：不管断点怎么说，都从头写。
		decision = ResumeDecision{}
	}

	// 关键的一步：断点被判无效，但服务端已经按 Range 只回了后半截（206）。
	// 这时候既不能 append（会拼坏），也不能 truncate 后只写这半截（残包）——
	// 必须重发一个不带 Range 的请求，把整包拿回来。
	if resp.StatusCode == http.StatusPartialContent && !decision.Valid {
		_ = resp.Body.Close()

		retryReq, rerr := newReq(0)
		if rerr != nil {
			return nil, rerr
		}
		resp, rerr = c.httpClient.Do(retryReq)
		if rerr != nil {
			return nil, fmt.Errorf("failed to re-download full package: %w", rerr)
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("download failed with status %d", resp.StatusCode)
		}
		respTotal = resp.ContentLength
		respMod = parseHTTPTime(resp.Header.Get("Last-Modified"))
		decision = ResumeDecision{}
	}

	// 1. 落盘：续传时接着写，否则从头写。
	flags := os.O_CREATE | os.O_WRONLY
	if decision.Valid {
		flags |= os.O_APPEND
	} else {
		flags |= os.O_TRUNC
	}
	tmpFile, err := os.OpenFile(partPath, flags, 0o644)
	if err != nil {
		return nil, fmt.Errorf("failed opening partial download file: %w", err)
	}
	tmpPath := partPath

	// 元信息要在开始写之前落盘：中途崩了，下次还能判断这个断点属于哪个包。
	if respTotal > 0 {
		_ = savePartialMeta(metaPath, ResumeMeta{Total: respTotal, ModTime: respMod})
	}

	bufPtr := copyBufPool.Get().(*[]byte)
	_, err = io.CopyBuffer(tmpFile, &idleResetReader{r: resp.Body, timer: idle}, *bufPtr)
	copyBufPool.Put(bufPtr)
	_ = tmpFile.Close()
	if err != nil {
		return nil, fmt.Errorf("failed writing downloaded stream: %w", err)
	}

	// 下载成功：断点文件与元信息都该走了。
	defer func() {
		_ = os.Remove(partPath)
		_ = os.Remove(metaPath)
	}()

	// 2. Open ZIP and validate against Zip-Slip
	zr, err := zip.OpenReader(tmpPath)
	if err != nil {
		return nil, fmt.Errorf("invalid zip archive downloaded: %w", err)
	}
	defer zr.Close()

	for _, f := range zr.File {
		if isZipSlip(f.Name) {
			return nil, fmt.Errorf("zip-slip path traversal attempt detected in entry: %s", f.Name)
		}
	}

	// 3. Prepare target destination
	now := time.Now()
	timestamp := now.Format("20060102_150405")
	accountID := fmt.Sprintf("remote_%s", timestamp)

	if backupPath == "" {
		backupPath = "backup"
	}
	destDir := filepath.Join(backupPath, "userdata_backup", accountID)
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create unpacked directory %s: %w", destDir, err)
	}

	// 4. Unpack files
	totalFiles := 0
	var totalBytes int64
	var foundMeta *PackageMetadata

	for _, f := range zr.File {
		if f.FileInfo().IsDir() {
			continue
		}

		norm := filepath.ToSlash(f.Name)
		var targetRelPath string

		if idx := strings.Index(norm, "730/"); idx >= 0 {
			targetRelPath = norm[idx:]
		} else if idx := strings.Index(norm, "cfg/"); idx >= 0 {
			targetRelPath = norm[idx:]
		} else if strings.HasSuffix(norm, "metadata.json") {
			targetRelPath = "metadata.json"
		} else if strings.HasSuffix(norm, "restore.bat") {
			targetRelPath = "restore.bat"
		} else {
			targetRelPath = norm
		}

		targetFullPath := filepath.Join(destDir, filepath.FromSlash(targetRelPath))
		if err := os.MkdirAll(filepath.Dir(targetFullPath), 0755); err != nil {
			return nil, fmt.Errorf("failed creating subfolder: %w", err)
		}

		if err := extractZipEntry(f, targetFullPath); err != nil {
			return nil, fmt.Errorf("failed extracting entry %s: %w", f.Name, err)
		}

		totalFiles++
		totalBytes += f.FileInfo().Size()

		// Read metadata if found
		if targetRelPath == "metadata.json" {
			if data, err := os.ReadFile(targetFullPath); err == nil {
				var meta PackageMetadata
				if err := json.Unmarshal(data, &meta); err == nil {
					foundMeta = &meta
				}
			}
		}
	}

	// 5. Determine source name
	sourceName := fmt.Sprintf("%s:%d", ip, port)
	if foundMeta != nil && foundMeta.Name != "" {
		sourceName = foundMeta.Name
	}

	// 6. Write remote_info.json
	downloadTimeStr := now.Format("2006-01-02 15:04:05")
	info := RemoteInfoMetadata{
		AccountID:    accountID,
		Username:     "来自其它设备的备份",
		SourceName:   sourceName,
		DownloadTime: downloadTimeStr,
		Type:         "remote",
	}

	infoBytes, err := json.MarshalIndent(info, "", "  ")
	if err == nil {
		_ = os.WriteFile(filepath.Join(destDir, "remote_info.json"), infoBytes, 0644)
	}

	return &DownloadedBackupResult{
		AccountID:    accountID,
		DestDir:      destDir,
		TotalFiles:   totalFiles,
		TotalBytes:   totalBytes,
		SourceName:   sourceName,
		DownloadTime: downloadTimeStr,
	}, nil
}

func extractZipEntry(zf *zip.File, destPath string) error {
	rc, err := zf.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	_, err = io.CopyBuffer(out, rc, *bufPtr)
	return err
}
