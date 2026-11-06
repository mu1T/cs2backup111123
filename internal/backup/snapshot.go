package backup

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"cs2backup/internal/atomicfile"
)

// SnapshotInfo holds metadata for a single snapshot of user configuration.
type SnapshotInfo struct {
	ID         string            `json:"id"`
	AccountID  string            `json:"account_id"`
	CreatedAt  string            `json:"created_at"`
	Note       string            `json:"note"`
	TotalFiles int               `json:"total_files"`
	TotalBytes int64             `json:"total_bytes"`
	FileHashes map[string]string `json:"file_hashes"` // relPath -> sha256
}

// SnapshotManifest tracks all snapshots for a specific account.
type SnapshotManifest struct {
	AccountID string         `json:"account_id"`
	Snapshots []SnapshotInfo `json:"snapshots"`
}

func computeFileSHA256(filePath string) (string, int64, error) { return ComputeFileSHA256(filePath) }

// ComputeFileSHA256 calculates the SHA-256 hash and file size using pooled 64KB buffers.
func ComputeFileSHA256(filePath string) (string, int64, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	fi, err := f.Stat()
	if err != nil {
		return "", 0, err
	}

	h := sha256.New()
	bufPtr := copyBufPool.Get().(*[]byte)
	defer copyBufPool.Put(bufPtr)

	if _, err := io.CopyBuffer(h, f, *bufPtr); err != nil {
		return "", 0, err
	}

	return hex.EncodeToString(h.Sum(nil)), fi.Size(), nil
}

func canonicalManifestPath(backupPath, accountID string) string {
	return filepath.Join(backupPath, "snapshots", accountID, "snapshot_manifest.json")
}

func legacyManifestPath(backupPath, accountID string) string {
	return filepath.Join(backupPath, "snapshots", accountID, "manifest.json")
}

func manifestPath(backupPath, accountID string) string {
	return canonicalManifestPath(backupPath, accountID)
}

// loadManifest loads snapshot_manifest.json (with fallback to manifest.json) for an account or initializes an empty one.
func loadManifest(backupPath, accountID string) (*SnapshotManifest, error) {
	p := canonicalManifestPath(backupPath, accountID)
	data, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			legacyP := legacyManifestPath(backupPath, accountID)
			legacyData, legacyErr := os.ReadFile(legacyP)
			if legacyErr == nil {
				var m SnapshotManifest
				if err := json.Unmarshal(legacyData, &m); err == nil {
					if m.Snapshots == nil {
						m.Snapshots = []SnapshotInfo{}
					}
					return &m, nil
				}
			}
			return &SnapshotManifest{
				AccountID: accountID,
				Snapshots: []SnapshotInfo{},
			}, nil
		}
		return nil, fmt.Errorf("failed to read snapshot manifest %s: %w", p, err)
	}

	var m SnapshotManifest
	if err := json.Unmarshal(data, &m); err != nil {
		return nil, fmt.Errorf("failed to parse snapshot manifest %s: %w", p, err)
	}
	if m.Snapshots == nil {
		m.Snapshots = []SnapshotInfo{}
	}
	return &m, nil
}

// saveManifest writes the manifest atomically to snapshot_manifest.json.
func saveManifest(backupPath, accountID string, m *SnapshotManifest) error {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to serialize snapshot manifest: %w", err)
	}

	p := canonicalManifestPath(backupPath, accountID)
	if _, err := atomicfile.Write(p, data, atomicfile.Options{}); err != nil {
		return fmt.Errorf("failed to write snapshot manifest: %w", err)
	}
	return nil
}

// GetSnapshots lists all snapshots for an account, with automatic v0-legacy migration per ADR 0006.
func GetSnapshots(backupPath, accountID string) ([]SnapshotInfo, error) {
	return GetSnapshotsWithConfig(backupPath, accountID, "config")
}

// GetSnapshotsWithConfig lists all snapshots for an account using the provided configuration directory.
func GetSnapshotsWithConfig(backupPath, accountID, configDir string) ([]SnapshotInfo, error) {
	if backupPath == "" {
		return nil, errors.New("backup path cannot be empty")
	}
	if accountID == "" {
		return nil, errors.New("account ID cannot be empty")
	}

	manifestExists := false
	if _, err := os.Stat(canonicalManifestPath(backupPath, accountID)); err == nil {
		manifestExists = true
	} else if _, err := os.Stat(legacyManifestPath(backupPath, accountID)); err == nil {
		manifestExists = true
	}

	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		return nil, err
	}

	// Legacy Python backup compatibility (ADR 0006):
	// Auto-import if snapshot_manifest.json / manifest.json did not exist yet (i.e. snapshots/<accountID> is empty/uninitialized)
	if !manifestExists && len(m.Snapshots) == 0 {
		var legacyDir string
		if accountID == "cs2_cfg" || accountID == "cfg" || accountID == "global_cfg" {
			legacyDir = filepath.Join(backupPath, "cs2_cfg")
		} else {
			legacyDir = filepath.Join(backupPath, "userdata_backup", accountID, "730")
			if _, err := os.Stat(legacyDir); err != nil {
				altDir := filepath.Join(backupPath, "userdata_backup", accountID)
				if fi, err := os.Stat(altDir); err == nil && fi.IsDir() {
					legacyDir = altDir
				}
			}
		}

		if fi, err := os.Stat(legacyDir); err == nil && fi.IsDir() {
			legacySnap, err := importLegacySnapshot(backupPath, accountID, legacyDir, configDir)
			if err == nil && legacySnap != nil {
				m.Snapshots = append(m.Snapshots, *legacySnap)
				_ = saveManifest(backupPath, accountID, m)
			}
		}
	}

	// Sort snapshots by CreatedAt descending (newest first)
	sort.Slice(m.Snapshots, func(i, j int) bool {
		return m.Snapshots[i].CreatedAt > m.Snapshots[j].CreatedAt
	})

	return m.Snapshots, nil
}

func importLegacySnapshot(backupPath, accountID, legacyDir, configDir string) (*SnapshotInfo, error) {
	v0Dir := filepath.Join(backupPath, "snapshots", accountID, "v0-legacy")
	if err := os.MkdirAll(v0Dir, 0755); err != nil {
		return nil, err
	}

	// Copy legacy data if not already present
	_ = CopyDir(legacyDir, v0Dir)

	fileHashes := make(map[string]string)
	var totalFiles int
	var totalBytes int64

	err := filepath.WalkDir(v0Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		rel, err := filepath.Rel(v0Dir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)
		hash, sz, err := computeFileSHA256(path)
		if err != nil {
			return err
		}
		fileHashes[relSlash] = hash
		totalFiles++
		totalBytes += sz
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Extract existing user note from user_notes.json if available
	note := "历史备份 (Python 原版)"
	if accountID == "cs2_cfg" || accountID == "cfg" || accountID == "global_cfg" {
		note = "历史游戏全局 CFG 备份 (Python 原版)"
	}
	notesFile := filepath.Join(configDir, "user_notes.json")
	if data, err := os.ReadFile(notesFile); err == nil {
		var notesMap map[string]string
		if err := json.Unmarshal(data, &notesMap); err == nil {
			if existingNote, ok := notesMap[accountID]; ok && strings.TrimSpace(existingNote) != "" {
				note = existingNote
			}
		}
	}

	createdAt := time.Now().Format(time.RFC3339)
	if fi, err := os.Stat(legacyDir); err == nil {
		createdAt = fi.ModTime().Format(time.RFC3339)
	}

	return &SnapshotInfo{
		ID:         "v0-legacy",
		AccountID:  accountID,
		CreatedAt:  createdAt,
		Note:       note,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
		FileHashes: fileHashes,
	}, nil
}

// CreateSnapshot captures an immutable snapshot of userdata/<accountID>/730 with SHA-256 deduplication.
func CreateSnapshot(steamPath, backupPath, accountID, note string) (*SnapshotInfo, error) {
	snap, _, err := CreateSnapshotWithRetention(steamPath, backupPath, accountID, note, DefaultRetentionLimit)
	return snap, err
}

// CreateSnapshotWithRetention 与 CreateSnapshot 相同，但指定保留上限，
// 并返回这次被裁剪掉的历史版本 ID（供上层写日志）。
//
// keep <= 0 表示不清理（见 SelectVersionsToKeep 的注释）。
func CreateSnapshotWithRetention(steamPath, backupPath, accountID, note string, keep int) (*SnapshotInfo, []string, error) {
	if steamPath == "" || backupPath == "" {
		return nil, nil, errors.New("steam path and backup path must be configured")
	}
	if accountID == "" {
		return nil, nil, errors.New("account ID cannot be empty")
	}

	srcDir := filepath.Join(steamPath, "userdata", accountID, "730")
	fi, err := os.Stat(srcDir)
	if err != nil || !fi.IsDir() {
		return nil, nil, fmt.Errorf("用户数据不存在: %s", srcDir)
	}

	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		return nil, nil, err
	}

	// Index existing files by SHA-256 for hardlink deduplication
	existingFilesByHash := make(map[string]string)
	for _, snap := range m.Snapshots {
		for relPath, hash := range snap.FileHashes {
			candidate := filepath.Join(backupPath, "snapshots", accountID, snap.ID, filepath.FromSlash(relPath))
			if _, err := os.Stat(candidate); err == nil {
				existingFilesByHash[hash] = candidate
			}
		}
	}

	// 版本 ID 用秒级时间戳，同一秒内多次备份靠后缀区分。
	//
	// 后缀以前是 `UnixNano()%10000`——又回到了「靠时钟保证唯一」的老路，
	// 12 次快速备份里真的会撞，撞上之后 MkdirAll 对已存在的目录返回 nil，
	// 两个版本静默塌成一个。改成从 2 开始顺序试探，直到目录不存在为止。
	baseID := time.Now().Format("20060102_150405")
	snapID := baseID
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(backupPath, "snapshots", accountID, snapID)); os.IsNotExist(err) {
			break
		}
		snapID = fmt.Sprintf("%s_%d", baseID, i)
	}
	targetDir := filepath.Join(backupPath, "snapshots", accountID, snapID)

	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return nil, nil, fmt.Errorf("failed to create snapshot dir %s: %w", targetDir, err)
	}

	fileHashes := make(map[string]string)
	var totalFiles int
	var totalBytes int64

	err = filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		rel, err := filepath.Rel(srcDir, path)
		if err != nil {
			return err
		}
		relSlash := filepath.ToSlash(rel)

		hash, sz, err := computeFileSHA256(path)
		if err != nil {
			return fmt.Errorf("failed to hash %s: %w", path, err)
		}

		dstPath := filepath.Join(targetDir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dstPath), 0755); err != nil {
			return err
		}

		// SHA-256 Deduplication: attempt NTFS hardlink first
		linked := false
		if existingPath, ok := existingFilesByHash[hash]; ok {
			if linkErr := os.Link(existingPath, dstPath); linkErr == nil {
				linked = true
			}
		}

		if !linked {
			// Fallback gracefully to streaming CopyFile with pooled buffer
			if err := CopyFile(path, dstPath); err != nil {
				return fmt.Errorf("failed to copy file %s to snapshot: %w", rel, err)
			}
			existingFilesByHash[hash] = dstPath
		}

		fileHashes[relSlash] = hash
		totalFiles++
		totalBytes += sz
		return nil
	})
	if err != nil {
		_ = os.RemoveAll(targetDir)
		return nil, nil, err
	}

	snapInfo := SnapshotInfo{
		ID:         snapID,
		AccountID:  accountID,
		CreatedAt:  time.Now().Format(time.RFC3339),
		Note:       note,
		TotalFiles: totalFiles,
		TotalBytes: totalBytes,
		FileHashes: fileHashes,
	}

	m.Snapshots = append([]SnapshotInfo{snapInfo}, m.Snapshots...)
	if err := saveManifest(backupPath, accountID, m); err != nil {
		return nil, nil, err
	}

	// Mirror to userdata_backup for legacy compatibility
	legacyDst := filepath.Join(backupPath, "userdata_backup", accountID, "730")
	_ = os.RemoveAll(legacyDst)
	_ = CopyDir(targetDir, legacyDst)

	// 最后才裁剪：先保证新版本和镜像都落好了，再动旧版本。
	// 裁剪失败不影响本次备份成功——用户拿到的是「历史没清掉」，不是「备份没做成」。
	dropped, pruneErr := pruneSnapshots(backupPath, accountID, keep)
	if pruneErr != nil {
		dropped = []string{}
	}

	return &snapInfo, dropped, nil
}

// pruneSnapshots 按上限裁掉最旧的版本，返回被删的 ID 列表。
//
// 判定交给 SelectVersionsToKeep（纯函数），这里只负责删目录 + 改清单。
// planPrune 只算「会删哪些」，不动磁盘。预览与执行共用这一份判定。
func planPrune(backupPath, accountID string, keep int) (kept, dropped []SnapshotInfo, err error) {
	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		return nil, nil, err
	}
	k, d := SelectVersionsToKeep(m.Snapshots, keep)
	return k, d, nil
}

func pruneSnapshots(backupPath, accountID string, keep int) ([]string, error) {
	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		return nil, err
	}

	kept, dropped := SelectVersionsToKeep(m.Snapshots, keep)
	if len(dropped) == 0 {
		return []string{}, nil
	}

	for _, d := range dropped {
		_ = os.RemoveAll(filepath.Join(backupPath, "snapshots", accountID, d.ID))
	}

	m.Snapshots = kept
	if err := saveManifest(backupPath, accountID, m); err != nil {
		return nil, err
	}
	return VersionIDs(dropped), nil
}

// RestoreSnapshot creates a pre-restore rollback then restores the specified snapshot files.
func RestoreSnapshot(steamPath, backupPath, targetAccountID, snapshotID string) error {
	if steamPath == "" || backupPath == "" {
		return errors.New("steam path and backup path must be configured")
	}
	if targetAccountID == "" || snapshotID == "" {
		return errors.New("target account ID and snapshot ID cannot be empty")
	}

	snapDir := filepath.Join(backupPath, "snapshots", targetAccountID, snapshotID)
	fi, err := os.Stat(snapDir)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("快照不存在: %s", snapDir)
	}

	dst := filepath.Join(steamPath, "userdata", targetAccountID, "730")
	rollbackDir := filepath.Join(backupPath, "restore_originals", "user_730", targetAccountID)

	// Save rollback point of existing destination data if present
	if fi, err := os.Stat(dst); err == nil && fi.IsDir() {
		if _, err := os.Stat(rollbackDir); err == nil {
			_ = os.RemoveAll(rollbackDir)
		}
		if err := CopyDir(dst, rollbackDir); err != nil {
			return fmt.Errorf("failed to create rollback snapshot at %s: %w", rollbackDir, err)
		}
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("failed to clear destination directory %s: %w", dst, err)
		}
	}

	return CopyDir(snapDir, dst)
}

// DeleteSnapshot removes the snapshot folder and updates manifest.json.
func DeleteSnapshot(backupPath, accountID, snapshotID string) error {
	if backupPath == "" || accountID == "" || snapshotID == "" {
		return errors.New("backup path, account ID, and snapshot ID cannot be empty")
	}

	m, err := loadManifest(backupPath, accountID)
	if err != nil {
		return err
	}

	foundIdx := -1
	for idx, snap := range m.Snapshots {
		if snap.ID == snapshotID {
			foundIdx = idx
			break
		}
	}

	if foundIdx == -1 {
		return fmt.Errorf("快照 %s 不存在", snapshotID)
	}

	// Remove snapshot folder
	snapDir := filepath.Join(backupPath, "snapshots", accountID, snapshotID)
	_ = os.RemoveAll(snapDir)

	// Remove from manifest
	m.Snapshots = append(m.Snapshots[:foundIdx], m.Snapshots[foundIdx+1:]...)
	return saveManifest(backupPath, accountID, m)
}

// RestoreVersion 把一个账号还原到指定的历史版本。
//
// 与直接调 RestoreSnapshot 的区别有两点，都是「还原这件事本身要可撤销」的要求：
//
//  1. 还原前先把当前状态存成一个**历史版本**（带备注），而不只是塞进
//     restore_originals。这样用户能在历史列表里看到「我动过手」的那一步，
//     而不是一个藏在别处、界面上看不见的目录。
//  2. 还原后把 userdata_backup 镜像刷到被还原的那个版本。否则局域网共享
//     给对方的是「最新备份」，和用户刚还原到的状态对不上。
func RestoreVersion(steamPath, backupPath, accountID, versionID string, keep int) ([]string, error) {
	if steamPath == "" || backupPath == "" {
		return nil, errors.New("steam path and backup path must be configured")
	}
	if accountID == "" || versionID == "" {
		return nil, errors.New("account ID and version ID cannot be empty")
	}

	versionDir := filepath.Join(backupPath, "snapshots", accountID, versionID)
	if fi, err := os.Stat(versionDir); err != nil || !fi.IsDir() {
		return nil, fmt.Errorf("版本不存在: %s", versionID)
	}

	// 1. 还原前留底：这一步失败就不该继续，否则还原不可撤销。
	_, dropped, err := CreateSnapshotWithRetention(
		steamPath, backupPath, accountID, "还原前自动留底", keep,
	)
	if err != nil {
		return nil, fmt.Errorf("还原前留底失败，已中止: %w", err)
	}

	// 2. 还原文件。RestoreSnapshot 自己会再往 restore_originals 写一份兼容备份。
	if err := RestoreSnapshot(steamPath, backupPath, accountID, versionID); err != nil {
		return dropped, err
	}

	// 3. 镜像刷到被还原的版本——局域网共享打的包要跟着变。
	legacyDst := filepath.Join(backupPath, "userdata_backup", accountID, "730")
	_ = os.RemoveAll(legacyDst)
	if err := CopyDir(versionDir, legacyDst); err != nil {
		return dropped, fmt.Errorf("还原成功但刷新镜像失败: %w", err)
	}

	return dropped, nil
}
