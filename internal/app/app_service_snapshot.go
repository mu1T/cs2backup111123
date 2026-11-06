package app

import (
	"fmt"

	"cs2backup/internal/backup"
)

// Snapshot API Delegations

// GetSnapshots retrieves all snapshots for the specified account ID.
func (s *AppService) GetSnapshots(accountID string) ([]backup.SnapshotInfo, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}

	return svc.GetSnapshots(accountID)
}

// CreateSnapshot creates a new immutable snapshot of the account's userdata/730.
func (s *AppService) CreateSnapshot(accountID, note string) (*backup.SnapshotInfo, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在为账号 [%s] 创建配置快照 (备注: %s)...", accountID, note))
	snap, err := svc.CreateSnapshot(accountID, note)
	if err != nil {
		s.LogError(fmt.Sprintf("创建快照失败 [AccountID %s]: %v", accountID, err))
		return nil, err
	}
	s.LogSuccess(fmt.Sprintf("成功创建配置快照 [AccountID %s, ID %s, 文件数 %d]", accountID, snap.ID, snap.TotalFiles))
	return snap, nil
}

// RestoreSnapshot restores the specified snapshot into the target account userdata.
func (s *AppService) RestoreSnapshot(accountID, snapshotID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在恢复账号 [%s] 的快照版本 [%s] (已自动生成原配置回滚存档)...", accountID, snapshotID))
	if err := svc.RestoreSnapshot(accountID, snapshotID); err != nil {
		s.LogError(fmt.Sprintf("还原快照失败 [AccountID %s, ID %s]: %v", accountID, snapshotID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("成功恢复账号 [%s] 的快照版本 [%s]", accountID, snapshotID))
	return nil
}

// DeleteSnapshot deletes the specified snapshot from disk and manifest.
func (s *AppService) DeleteSnapshot(accountID, snapshotID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在删除账号 [%s] 的历史快照 [%s]...", accountID, snapshotID))
	if err := svc.DeleteSnapshot(accountID, snapshotID); err != nil {
		s.LogError(fmt.Sprintf("删除快照失败 [AccountID %s, ID %s]: %v", accountID, snapshotID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已成功删除账号 [%s] 的历史快照 [%s]", accountID, snapshotID))
	return nil
}

// CompareSnapshot compares a snapshot against live Steam userdata.
func (s *AppService) CompareSnapshot(accountID, snapshotID string) ([]backup.FileDiffItem, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在比对账号 [%s] 快照 [%s] 与本地实时配置差异...", accountID, snapshotID))
	items, err := svc.CompareSnapshot(accountID, snapshotID)
	if err != nil {
		s.LogError(fmt.Sprintf("差异比对失败 [AccountID %s, ID %s]: %v", accountID, snapshotID, err))
		return nil, err
	}
	s.LogSuccess(fmt.Sprintf("比对完成 [AccountID %s, ID %s]: 共比对 %d 个配置文件", accountID, snapshotID, len(items)))
	return items, nil
}

// GetTextDiff compares two strings line-by-line.
func (s *AppService) GetTextDiff(textA, textB string) (*backup.DiffResult, error) {
	return backup.GetTextDiff(textA, textB)
}
