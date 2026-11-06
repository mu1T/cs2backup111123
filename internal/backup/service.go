package backup

import (
	"time"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"

	"cs2backup/internal/steam"
)

// Service provides userdata and CS2 game configuration backup and restore functionality.
type Service struct {
	mu         sync.RWMutex
	configDir  string
	cacheMgr   *steam.CacheManager
	steamPath  string
	backupPath string
	// cs2Path 是用户手动指定的 game/csgo/cfg；空串表示交给自动探测。
	// 一台机器装多份 CS2、或游戏不在 Steam 默认库时用它兜底。
	cs2Path string
	// retentionLimit 是每个账号保留的历史版本数，<=0 表示不清理。
	retentionLimit int
}

// NewService creates a new backup Service initialized with saved configuration if available.
func NewService(configDir string, cacheMgr *steam.CacheManager) (*Service, error) {
	if configDir == "" {
		configDir = "config"
	}

	svc := &Service{
		configDir:      configDir,
		cacheMgr:       cacheMgr,
		retentionLimit: DefaultRetentionLimit,
	}

	if cacheMgr != nil {
		svc.steamPath = cacheMgr.GetSavedSteamPath()
		svc.cs2Path = cacheMgr.GetSavedCS2Path()
		if n := cacheMgr.GetBackupRetention(); n > 0 {
			svc.retentionLimit = n
		}
		savedBackup := cacheMgr.GetSavedBackupPath()
		if savedBackup != "" {
			svc.backupPath = savedBackup
		} else {
			svc.backupPath = "backup"
		}
	} else {
		svc.backupPath = "backup"
	}

	return svc, nil
}

// GetBackupPath returns the active backup repository path.
func (s *Service) GetBackupPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.backupPath
}

// SetBackupPath validates, creates if needed, and persists the backup repository path.
func (s *Service) SetBackupPath(path string) error {
	if path == "" {
		return errors.New("backup path cannot be empty")
	}

	cleanPath := filepath.Clean(path)
	if err := os.MkdirAll(cleanPath, 0755); err != nil {
		return fmt.Errorf("failed to create backup directory %s: %w", cleanPath, err)
	}

	s.mu.Lock()
	s.backupPath = cleanPath
	cacheMgr := s.cacheMgr
	s.mu.Unlock()

	if cacheMgr != nil {
		return cacheMgr.SaveBackupPath(cleanPath)
	}
	return nil
}

// GetSteamPath returns the currently configured Steam path.
func (s *Service) GetSteamPath() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.steamPath
}

// SetSteamPath updates the Steam installation path used for operations.
func (s *Service) SetSteamPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.steamPath = filepath.Clean(path)
}

// GetCS2Path returns the manually configured CS2 cfg directory, or "" when auto-detecting.
func (s *Service) GetCS2Path() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.cs2Path
}

// SetCS2Path 手动指定 CS2 的配置目录。
//
// 接受任意层级（CS2 安装目录 / Counter-Strike Global Offensive / game/csgo / cfg 都行），
// 内部归一成 game/csgo/cfg 再存。传空串表示恢复自动探测。
func (s *Service) SetCS2Path(raw string) error {
	trimmed := strings.TrimSpace(raw)

	if trimmed == "" {
		s.mu.Lock()
		s.cs2Path = ""
		s.mu.Unlock()
		if s.cacheMgr != nil {
			return s.cacheMgr.SaveCS2Path("")
		}
		return nil
	}

	normalized, err := NormalizeCS2CFGPath(trimmed)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.cs2Path = normalized
	s.mu.Unlock()

	if s.cacheMgr != nil {
		return s.cacheMgr.SaveCS2Path(normalized)
	}
	return nil
}

// GetCS2CFGPath locates the CS2 game cfg directory based on the configured Steam installation.
//
// 手动指定优先；没指定时按 Steam 安装目录自动探测（会读 libraryfolders.vdf，
// 能覆盖游戏装在非默认库的情况）。
func (s *Service) GetCS2CFGPath() (string, error) {
	s.mu.RLock()
	sp := s.steamPath
	manual := s.cs2Path
	s.mu.RUnlock()

	if manual != "" {
		// 手动路径可能因为盘符变化 / 目录被删而失效，给一句能直接照做的提示，
		// 而不是让后续读写报一个看不懂的错。
		if fi, err := os.Stat(manual); err != nil || !fi.IsDir() {
			return "", fmt.Errorf("自定义的 CS2 配置目录已不存在：%s，请在「路径设置」里重新选择", manual)
		}
		return manual, nil
	}

	if sp == "" {
		return "", errors.New("steam installation path is not set")
	}

	return DetectCS2CFGPath(sp)
}

// BackupUser 为一个账号创建一个带时间戳的历史版本，并把镜像刷新到该版本。
//
// 以前是「先 RemoveAll 再 CopyDir」的单份覆盖：用户点第二次备份，第一次的状态
// 就没了。现在走快照的存储（时间戳版本 + 哈希去重 + 硬链接），顺带把
// userdata_backup 镜像刷新到最新——镜像路径保持不变，因为 GetBackedUpAccounts
// 和局域网打包都还指着它。
//
// 代价：备份现在要算一遍 SHA-256，比纯复制慢一点，换来的是第二次备份不再覆盖
// 第一次。ADR 0002 关心的内存占用没有变化（哈希是流式的）。
func (s *Service) BackupUser(accountID string) error {
	if accountID == "" {
		return errors.New("account ID cannot be empty")
	}

	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	if sp == "" || bp == "" {
		return errors.New("steam path or backup path is not configured")
	}

	_, err := s.BackupUserWithReport(accountID)
	return err
}

// BackupUserWithReport 与 BackupUser 相同，但返回被裁剪掉的历史版本 ID。
//
// 自动删除不该弹窗打断用户，但也不能毫无痕迹——上层拿到这个列表写进日志。
func (s *Service) BackupUserWithReport(accountID string) ([]string, error) {
	if accountID == "" {
		return nil, errors.New("account ID cannot be empty")
	}

	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	keep := s.retentionLimit
	s.mu.RUnlock()

	if sp == "" || bp == "" {
		return nil, errors.New("steam path or backup path is not configured")
	}

	// 备注留空：备份是快捷路径，要写备注的用户走「创建快照」。
	_, dropped, err := CreateSnapshotWithRetention(sp, bp, accountID, "", keep)
	return dropped, err
}

// RestoreVersion 把账号还原到指定历史版本，并在还原前自动留底。
//
// 与 RestoreUser 的区别：RestoreUser 只认 userdata_backup 镜像（最新那份），
// 这个方法能回到任意一个历史版本，并且还原后会把镜像刷到该版本。
func (s *Service) RestoreVersion(accountID, versionID string) ([]string, error) {
	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	keep := s.retentionLimit
	s.mu.RUnlock()

	if sp == "" || bp == "" {
		return nil, errors.New("steam path or backup path is not configured")
	}
	return RestoreVersion(sp, bp, accountID, versionID, keep)
}

// PreviewPrune 返回「如果现在按上限收敛，会删掉哪些版本」，一个都不删。
//
// 存量收敛要用它：用户机器上可能已经堆了远超上限的历史（这个上限是新加的），
// 直接删是破坏性的。先让他看清要删哪些、再决定。
func (s *Service) PreviewPrune(accountID string) ([]string, error) {
	s.mu.RLock()
	bp := s.backupPath
	keep := s.retentionLimit
	s.mu.RUnlock()

	if bp == "" || accountID == "" {
		return []string{}, nil
	}
	_, dropped, err := planPrune(bp, accountID, keep)
	if err != nil {
		return nil, err
	}
	return VersionIDs(dropped), nil
}

// ApplyPrune 按当前上限收敛，返回被删的版本 ID。
func (s *Service) ApplyPrune(accountID string) ([]string, error) {
	s.mu.RLock()
	bp := s.backupPath
	keep := s.retentionLimit
	s.mu.RUnlock()

	if bp == "" || accountID == "" {
		return []string{}, nil
	}
	return pruneSnapshots(bp, accountID, keep)
}

// GetRetentionLimit 返回每个账号保留的历史版本数。0 表示不清理。
func (s *Service) GetRetentionLimit() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.retentionLimit
}

// SetRetentionLimit 设置保留的历史版本数。0 或负数表示不清理。
//
// 只影响之后的备份：改小不会立刻删掉现有历史，等下一次备份时才收敛。
// 立刻删存量是另一件事（需要用户确认），不在这个 setter 里做。
func (s *Service) SetRetentionLimit(n int) error {
	if n < 0 {
		n = 0
	}
	s.mu.Lock()
	s.retentionLimit = n
	cm := s.cacheMgr
	s.mu.Unlock()

	if cm != nil {
		return cm.SaveBackupRetention(n)
	}
	return nil
}

// RestoreUser creates a rollback snapshot of current target userdata/730,
// and copies backupPath/userdata_backup/<sourceAccountID>/730 into target userdata/730.
func (s *Service) RestoreUser(targetAccountID, sourceAccountID string) error {
	if targetAccountID == "" {
		return errors.New("target account ID cannot be empty")
	}
	if sourceAccountID == "" {
		sourceAccountID = targetAccountID
	}

	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	if sp == "" || bp == "" {
		return errors.New("steam path or backup path is not configured")
	}

	src := filepath.Join(bp, "userdata_backup", sourceAccountID, "730")
	if fi, err := os.Stat(src); err != nil || !fi.IsDir() {
		return fmt.Errorf("恢复源不存在: %s", src)
	}

	dst := filepath.Join(sp, "userdata", targetAccountID, "730")
	rollbackDir := filepath.Join(bp, "restore_originals", "user_730", targetAccountID)

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

	return CopyDir(src, dst)
}

// BackupGameCFG backs up CS2 game configuration files according to the specified mode ("all", "custom", "selected").
// Returns the number of files backed up.
func (s *Service) BackupGameCFG(mode string, selectedFiles []string) (int, error) {
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()

	if bp == "" {
		return 0, errors.New("backup path is not configured")
	}

	// 通用 CFG 也走版本化：每次备份写进 cs2_cfg_versions/<时间戳>/，
	// 再把 cs2_cfg 镜像刷到最新。镜像路径不变，还原与局域网打包都还指着它。
	versionsRoot := filepath.Join(bp, "cs2_cfg_versions")
	versionID := time.Now().Format("20060102_150405")
	for i := 2; ; i++ {
		if _, err := os.Stat(filepath.Join(versionsRoot, versionID)); os.IsNotExist(err) {
			break
		}
		versionID = fmt.Sprintf("%s_%d", time.Now().Format("20060102_150405"), i)
	}
	targetDir := filepath.Join(versionsRoot, versionID)
	if err := os.MkdirAll(targetDir, 0755); err != nil {
		return 0, fmt.Errorf("failed to create target backup directory: %w", err)
	}

	fileCount := 0

	if mode == "selected" {
		allowedFiles := FilterSelectedCFGFiles(selectedFiles)
		if len(allowedFiles) == 0 {
			return 0, errors.New("未选择任何有效的 .cfg/.vcfg/.txt 配置文件")
		}

		for _, srcFile := range allowedFiles {
			if fi, err := os.Stat(srcFile); err == nil && !fi.IsDir() {
				dstFile := filepath.Join(targetDir, filepath.Base(srcFile))
				if err := CopyFile(srcFile, dstFile); err == nil {
					fileCount++
				}
			}
		}
	} else {
		entries, err := os.ReadDir(cfgDir)
		if err != nil {
			return 0, fmt.Errorf("failed to read CS2 cfg directory %s: %w", cfgDir, err)
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}

			filename := entry.Name()
			shouldBackup := false

			switch mode {
			case "all":
				shouldBackup = true
			case "custom":
				if !IsDefaultGameCFG(filename) {
					shouldBackup = true
				}
			default:
				return 0, fmt.Errorf("unknown backup mode: %s", mode)
			}

			if shouldBackup {
				srcFile := filepath.Join(cfgDir, filename)
				dstFile := filepath.Join(targetDir, filename)
				if err := CopyFile(srcFile, dstFile); err != nil {
					return fileCount, fmt.Errorf("failed to copy %s: %w", filename, err)
				}
				fileCount++
			}
		}
	}

	if fileCount == 0 {
		_ = os.RemoveAll(targetDir)
		return 0, fmt.Errorf("备份完成，但没有文件被复制 (模式: %s)", mode)
	}

	// 镜像刷到最新版本。先写版本再刷镜像：镜像中途失败也不影响历史。
	mirrorDir := filepath.Join(bp, "cs2_cfg")
	_ = os.RemoveAll(mirrorDir)
	if err := CopyDir(targetDir, mirrorDir); err != nil {
		return fileCount, fmt.Errorf("备份成功但刷新镜像失败: %w", err)
	}

	// 按同一个上限裁剪历史。失败不影响本次备份成功。
	s.mu.RLock()
	keep := s.retentionLimit
	s.mu.RUnlock()
	_, _ = pruneVersionDirs(versionsRoot, keep)

	return fileCount, nil
}

// RestoreGameCFG saves a rollback snapshot to restore_originals/cs2_cfg_global,
// then restores all files and directories from backupPath/cs2_cfg to the CS2 game cfg directory.
func (s *Service) RestoreGameCFG() (int, error) {
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()

	if bp == "" {
		return 0, errors.New("backup path is not configured")
	}

	return s.restoreGameCFFrom(cfgDir, bp, filepath.Join(bp, "cs2_cfg"))
}

// ListGameCFGVersions 返回通用 CFG 的历史版本 ID，最新的在前。
//
// 目录名是 `YYYYMMDD_HHMMSS`，字典序即时间序，不用解析时间。
func (s *Service) ListGameCFGVersions() ([]string, error) {
	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()
	if bp == "" {
		return []string{}, nil
	}

	root := filepath.Join(bp, "cs2_cfg_versions")
	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var names []string
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	return names, nil
}

// RestoreGameCFGVersion 把通用 CFG 还原到指定历史版本。
//
// 与 RestoreGameCFG 的区别：那个只认 `cs2_cfg` 镜像（最新一份），这个能回到
// 任意一个历史版本。还原前同样会把当前状态存进 restore_originals。
func (s *Service) RestoreGameCFGVersion(versionID string) (int, error) {
	if versionID == "" {
		return 0, errors.New("version ID cannot be empty")
	}
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()
	if bp == "" {
		return 0, errors.New("backup path is not configured")
	}

	versionDir := filepath.Join(bp, "cs2_cfg_versions", versionID)
	if fi, err := os.Stat(versionDir); err != nil || !fi.IsDir() {
		return 0, fmt.Errorf("通用 CFG 版本不存在: %s", versionID)
	}

	return s.restoreGameCFFrom(cfgDir, bp, versionDir)
}

// restoreGameCFFrom 把 srcDir 里的文件铺到 cfgDir，并在动之前留一份还原点。
func (s *Service) restoreGameCFFrom(cfgDir, bp, srcDir string) (int, error) {
	entries, err := os.ReadDir(srcDir)
	if err != nil || len(entries) == 0 {
		return 0, errors.New("找不到 CS2 配置备份或备份目录为空")
	}

	// Create rollback point of current game cfg
	rollbackDir := filepath.Join(bp, "restore_originals", "cs2_cfg_global")
	if fi, err := os.Stat(cfgDir); err == nil && fi.IsDir() {
		if _, err := os.Stat(rollbackDir); err == nil {
			_ = os.RemoveAll(rollbackDir)
		}
		_ = CopyDir(cfgDir, rollbackDir)
	}

	fileCount := 0
	for _, entry := range entries {
		src := filepath.Join(srcDir, entry.Name())
		dst := filepath.Join(cfgDir, entry.Name())

		if entry.IsDir() {
			if _, err := os.Stat(dst); err == nil {
				_ = os.RemoveAll(dst)
			}
			if err := CopyDir(src, dst); err == nil {
				fileCount++
			}
		} else {
			if err := CopyFile(src, dst); err == nil {
				fileCount++
			}
		}
	}

	return fileCount, nil
}

// HasUserRollback checks if an automatic pre-restore rollback snapshot exists for the target account.
func (s *Service) HasUserRollback(accountID string) bool {
	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()
	if bp == "" {
		return false
	}
	rollbackDir := filepath.Join(bp, "restore_originals", "user_730", accountID)
	fi, err := os.Stat(rollbackDir)
	return err == nil && fi.IsDir()
}

// RollbackUserRestore restores the pre-restore rollback snapshot back to the account's userdata/730 directory.
func (s *Service) RollbackUserRestore(accountID string) error {
	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	if sp == "" || bp == "" {
		return errors.New("steam path or backup path is not configured")
	}

	rollbackDir := filepath.Join(bp, "restore_originals", "user_730", accountID)
	fi, err := os.Stat(rollbackDir)
	if err != nil || !fi.IsDir() {
		return fmt.Errorf("未找到账号 [%s] 的恢复前回退备份", accountID)
	}

	dst := filepath.Join(sp, "userdata", accountID, "730")
	if _, err := os.Stat(dst); err == nil {
		if err := os.RemoveAll(dst); err != nil {
			return fmt.Errorf("failed to clear destination directory %s: %w", dst, err)
		}
	}

	return CopyDir(rollbackDir, dst)
}

// HasGameCFGRollback checks if an automatic pre-restore rollback snapshot exists for the global CS2 game CFG.
func (s *Service) HasGameCFGRollback() bool {
	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()
	if bp == "" {
		return false
	}
	rollbackDir := filepath.Join(bp, "restore_originals", "cs2_cfg_global")
	fi, err := os.Stat(rollbackDir)
	return err == nil && fi.IsDir()
}

// RollbackGameCFG restores the pre-restore rollback snapshot back to the CS2 game cfg directory.
func (s *Service) RollbackGameCFG() (int, error) {
	cfgDir, err := s.GetCS2CFGPath()
	if err != nil {
		return 0, err
	}

	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()

	if bp == "" {
		return 0, errors.New("backup path is not configured")
	}

	rollbackDir := filepath.Join(bp, "restore_originals", "cs2_cfg_global")
	entries, err := os.ReadDir(rollbackDir)
	if err != nil || len(entries) == 0 {
		return 0, errors.New("未找到游戏通用配置的回退备份")
	}

	fileCount := 0
	for _, entry := range entries {
		src := filepath.Join(rollbackDir, entry.Name())
		dst := filepath.Join(cfgDir, entry.Name())

		if entry.IsDir() {
			if _, err := os.Stat(dst); err == nil {
				_ = os.RemoveAll(dst)
			}
			if err := CopyDir(src, dst); err == nil {
				fileCount++
			}
		} else {
			if err := CopyFile(src, dst); err == nil {
				fileCount++
			}
		}
	}

	return fileCount, nil
}

// GetBackedUpAccounts returns all account IDs that have backups in backupPath/userdata_backup.
// Numeric IDs are sorted numerically ascending, and remote_ accounts are sorted alphabetically at the end.
func (s *Service) GetBackedUpAccounts() ([]string, error) {
	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()

	if bp == "" {
		return []string{}, nil
	}

	backupUserdata := filepath.Join(bp, "userdata_backup")
	entries, err := os.ReadDir(backupUserdata)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, err
	}

	var numericAccounts []uint32
	var remoteAccounts []string

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()

		if aid, err := strconv.ParseUint(name, 10, 32); err == nil {
			numericAccounts = append(numericAccounts, uint32(aid))
		} else if len(name) > 7 && name[:7] == "remote_" {
			remoteAccounts = append(remoteAccounts, name)
		}
	}

	sort.Slice(numericAccounts, func(i, j int) bool {
		return numericAccounts[i] < numericAccounts[j]
	})
	sort.Strings(remoteAccounts)

	result := make([]string, 0, len(numericAccounts)+len(remoteAccounts))
	for _, aid := range numericAccounts {
		result = append(result, strconv.FormatUint(uint64(aid), 10))
	}
	result = append(result, remoteAccounts...)

	return result, nil
}

// GetSnapshots retrieves all snapshots for the specified account ID.
func (s *Service) GetSnapshots(accountID string) ([]SnapshotInfo, error) {
	s.mu.RLock()
	bp := s.backupPath
	cd := s.configDir
	s.mu.RUnlock()

	return GetSnapshotsWithConfig(bp, accountID, cd)
}

// CreateSnapshot creates a new immutable snapshot of the account's userdata/730.
func (s *Service) CreateSnapshot(accountID, note string) (*SnapshotInfo, error) {
	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	return CreateSnapshot(sp, bp, accountID, note)
}

// RestoreSnapshot restores the specified snapshot into the target account userdata.
func (s *Service) RestoreSnapshot(targetAccountID, snapshotID string) error {
	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	return RestoreSnapshot(sp, bp, targetAccountID, snapshotID)
}

// DeleteSnapshot deletes the specified snapshot from disk and manifest.
func (s *Service) DeleteSnapshot(accountID, snapshotID string) error {
	s.mu.RLock()
	bp := s.backupPath
	s.mu.RUnlock()

	return DeleteSnapshot(bp, accountID, snapshotID)
}

// CompareSnapshot compares a snapshot against live Steam userdata.
func (s *Service) CompareSnapshot(accountID, snapshotID string) ([]FileDiffItem, error) {
	s.mu.RLock()
	sp := s.steamPath
	bp := s.backupPath
	s.mu.RUnlock()

	return CompareSnapshotWithLive(sp, bp, accountID, snapshotID)
}

// GetTextDiff compares two strings line-by-line.
func (s *Service) GetTextDiff(textA, textB string) (*DiffResult, error) {
	return GetTextDiff(textA, textB)
}
