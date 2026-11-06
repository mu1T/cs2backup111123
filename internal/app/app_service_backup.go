package app

import (
	"strings"
	"fmt"

	"cs2backup/internal/backup"
)

// BackupUser copies userdata/<accountID>/730 to backup repository.
func (s *AppService) BackupUser(accountID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("开始备份用户 [AccountID %s] Userdata 配置...", accountID))
	dropped, err := svc.BackupUserWithReport(accountID)
	if err != nil {
		s.LogError(fmt.Sprintf("备份用户 [AccountID %s] 失败: %v", accountID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("用户 [AccountID %s] Userdata 配置备份成功", accountID))

	// 自动裁剪不弹窗打断用户，但必须留痕：删了哪几个版本要能在日志里查到。
	if len(dropped) > 0 {
		s.LogInfo(fmt.Sprintf(
			"账号 [%s] 历史版本超出上限 %d，已清理最旧的 %d 个: %s",
			accountID, svc.GetRetentionLimit(), len(dropped), strings.Join(dropped, ", "),
		))
	}
	return nil
}

// RestoreVersion 把账号还原到指定历史版本。
//
// 还原前会自动留一个「还原前自动留底」的版本，所以还原本身可以再撤销。
func (s *AppService) RestoreVersion(accountID, versionID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在把账号 [%s] 还原到历史版本 [%s]...", accountID, versionID))
	dropped, err := svc.RestoreVersion(accountID, versionID)
	if err != nil {
		s.LogError(fmt.Sprintf("还原账号 [%s] 到版本 [%s] 失败: %v", accountID, versionID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("账号 [%s] 已还原到版本 [%s]", accountID, versionID))
	if len(dropped) > 0 {
		s.LogInfo(fmt.Sprintf("历史版本超出上限，已清理最旧的 %d 个: %s", len(dropped), strings.Join(dropped, ", ")))
	}
	return nil
}

// ListGameCFGVersions 返回通用 CFG 的历史版本，最新的在前。
func (s *AppService) ListGameCFGVersions() ([]string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return []string{}, nil
	}
	return svc.ListGameCFGVersions()
}

// RestoreGameCFGVersion 把通用 CFG 还原到指定历史版本。
func (s *AppService) RestoreGameCFGVersion(versionID string) (int, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return 0, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在把通用 CFG 还原到版本 [%s]...", versionID))
	n, err := svc.RestoreGameCFGVersion(versionID)
	if err != nil {
		s.LogError(fmt.Sprintf("还原通用 CFG 到版本 [%s] 失败: %v", versionID, err))
		return 0, err
	}
	s.LogSuccess(fmt.Sprintf("通用 CFG 已还原到版本 [%s]，共 %d 个文件", versionID, n))
	return n, nil
}

// PreviewBackupPrune 返回「如果现在收敛会删掉哪些历史版本」，一个都不删。
func (s *AppService) PreviewBackupPrune(accountID string) ([]string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return []string{}, nil
	}
	return svc.PreviewPrune(accountID)
}

// ApplyBackupPrune 按当前上限收敛，并把被删的版本 ID 写进日志。
func (s *AppService) ApplyBackupPrune(accountID string) ([]string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return []string{}, fmt.Errorf("backup service not initialized")
	}

	dropped, err := svc.ApplyPrune(accountID)
	if err != nil {
		s.LogError(fmt.Sprintf("清理账号 [%s] 的历史版本失败: %v", accountID, err))
		return nil, err
	}
	if len(dropped) > 0 {
		s.LogSuccess(fmt.Sprintf(
			"账号 [%s] 历史版本已收敛到 %d 个，清理了 %d 个: %s",
			accountID, svc.GetRetentionLimit(), len(dropped), strings.Join(dropped, ", "),
		))
	}
	return dropped, nil
}

// GetBackupRetention 返回每个账号保留的历史版本数。0 表示不清理。
func (s *AppService) GetBackupRetention() int {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return 0
	}
	return svc.GetRetentionLimit()
}

// SetBackupRetention 设置每个账号保留的历史版本数，并持久化。
//
// 只影响之后的备份：改小不会立刻删掉现有历史，等下一次备份时才收敛。
func (s *AppService) SetBackupRetention(n int) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}
	if err := svc.SetRetentionLimit(n); err != nil {
		s.LogError(fmt.Sprintf("保存历史版本上限失败: %v", err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("历史版本保留上限已设为 %d", svc.GetRetentionLimit()))
	return nil
}

// RestoreUser restores userdata/<sourceAccountID>/730 into target userdata/<targetAccountID>/730.
func (s *AppService) RestoreUser(targetAccountID, sourceAccountID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("开始恢复用户配置至 [目标 AccountID %s] (来源: %s)...", targetAccountID, sourceAccountID))
	if err := svc.RestoreUser(targetAccountID, sourceAccountID); err != nil {
		s.LogError(fmt.Sprintf("恢复用户 [AccountID %s] 失败: %v", targetAccountID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("用户 [AccountID %s] Userdata 配置恢复成功 (已生成安全回滚存档)", targetAccountID))
	return nil
}

// ListUserCFGFiles scans userdata/<accountID>/730/local/cfg for the config editor.
func (s *AppService) ListUserCFGFiles(accountID string) ([]backup.GameCFGFile, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}
	return svc.ListUserCFGFiles(accountID)
}

// ListGameCFGFiles scans game/csgo/cfg and classifies each file by origin.
func (s *AppService) ListGameCFGFiles() ([]backup.GameCFGFile, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("backup service not initialized")
	}
	return svc.ListGameCFGFiles()
}

// BackupGameCFG backs up CS2 game configs with mode "all", "custom", or "selected".
func (s *AppService) BackupGameCFG(mode string, selectedFiles []string) (int, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return 0, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("开始备份 CS2 全局游戏配置 (模式: %s)...", mode))
	count, err := svc.BackupGameCFG(mode, selectedFiles)
	if err != nil {
		s.LogError(fmt.Sprintf("备份 CS2 游戏配置失败: %v", err))
		return 0, err
	}
	s.LogSuccess(fmt.Sprintf("成功备份 %d 个 CS2 游戏配置文件 (模式: %s)", count, mode))
	return count, nil
}

// RestoreGameCFG restores CS2 game configs into game/csgo/cfg.
func (s *AppService) RestoreGameCFG() (int, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return 0, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo("开始还原 CS2 全局游戏配置...")
	count, err := svc.RestoreGameCFG()
	if err != nil {
		s.LogError(fmt.Sprintf("还原 CS2 游戏配置失败: %v", err))
		return 0, err
	}
	s.LogSuccess(fmt.Sprintf("成功还原 %d 个 CS2 游戏配置 (原配置已备份至全局回滚点)", count))
	return count, nil
}

// HasUserRollback checks if an automatic pre-restore rollback snapshot exists for the target account.
func (s *AppService) HasUserRollback(accountID string) bool {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return false
	}
	return svc.HasUserRollback(accountID)
}

// RollbackUserRestore restores the pre-restore rollback snapshot back to the account's userdata/730 directory.
func (s *AppService) RollbackUserRestore(accountID string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("开始撤销账号 [%s] 的还原操作并回退...", accountID))
	if err := svc.RollbackUserRestore(accountID); err != nil {
		s.LogError(fmt.Sprintf("撤销还原账号 [%s] 失败: %v", accountID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("成功撤销账号 [%s] 的还原，已恢复至操作前的配置状态", accountID))
	return nil
}

// HasGameCFGRollback checks if an automatic pre-restore rollback snapshot exists for the global game CFG.
func (s *AppService) HasGameCFGRollback() bool {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return false
	}
	return svc.HasGameCFGRollback()
}

// RollbackGameCFG restores the pre-restore rollback snapshot back to the CS2 game cfg directory.
func (s *AppService) RollbackGameCFG() (int, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return 0, fmt.Errorf("backup service not initialized")
	}

	s.LogInfo("开始撤销游戏通用 CFG 的还原操作并回退...")
	count, err := svc.RollbackGameCFG()
	if err != nil {
		s.LogError(fmt.Sprintf("撤销通用配置还原失败: %v", err))
		return 0, err
	}
	s.LogSuccess(fmt.Sprintf("成功撤销通用配置还原，已恢复 %d 个文件至操作前的配置状态", count))
	return count, nil
}

// SelectCFGFiles prompts the user to select .cfg, .vcfg, or .txt files.
func (s *AppService) SelectCFGFiles() ([]string, error) {
	s.mu.RLock()
	picker := s.filePicker
	s.mu.RUnlock()

	if picker == nil {
		return nil, fmt.Errorf("file picker not configured")
	}

	files, err := picker("选择 CS2 配置文件备份", "CS2 配置文件 (*.cfg, *.vcfg, *.txt)", "*.cfg;*.vcfg;*.txt")
	if err != nil {
		s.LogError(fmt.Sprintf("选择配置文件异常: %v", err))
		return nil, err
	}
	return files, nil
}

// ReadConfigFile securely reads the text content of a CS2 config file.
func (s *AppService) ReadConfigFile(accountID, relPath string) (string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return "", fmt.Errorf("backup service not initialized")
	}

	return svc.ReadConfigFile(accountID, relPath)
}

// CreateConfigFile 新建一个空的 cfg 文件，返回最终文件名（已补 .cfg 后缀）。
// 与保存不同，它只能新建：同名文件存在就报错，绝不覆盖。
func (s *AppService) CreateConfigFile(accountID, name string) (string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return "", fmt.Errorf("backup service not initialized")
	}

	created, err := svc.CreateConfigFile(accountID, name)
	if err != nil {
		return "", err
	}
	s.LogInfo(fmt.Sprintf("已新建配置文件 [%s/%s]", accountID, created))
	return created, nil
}

// SaveConfigFileText securely saves text content to a CS2 config file.
func (s *AppService) SaveConfigFileText(accountID, relPath, content string) error {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("backup service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在保存配置文件 [%s/%s]...", accountID, relPath))
	if err := svc.SaveConfigFileText(accountID, relPath, content); err != nil {
		s.LogError(fmt.Sprintf("保存配置文件失败 [%s/%s]: %v", accountID, relPath, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("成功保存配置文件 [%s/%s]", accountID, relPath))
	return nil
}
