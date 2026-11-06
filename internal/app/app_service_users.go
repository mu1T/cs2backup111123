package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"cs2backup/internal/steam"
)

// GetUsers returns the local Steam accounts discovered from loginusers.vdf / userdata.
// 下载来的远程备份不算账号（它们是归档标签），由 GetRemoteBackups 单独提供，
// 否则列表里会出现点「登录」也没反应的假账号。
func (s *AppService) GetUsers() ([]steam.UserCardInfo, error) {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return nil, fmt.Errorf("steam service not initialized")
	}
	users, err := svc.GetUsers()
	if err != nil {
		s.LogWarning(fmt.Sprintf("读取 Steam 用户失败: %v", err))
		return nil, err
	}

	// 远程备份只是备份仓库里的一个存档，不是本机 Steam 账号：
	// 不进入账号列表（否则会出现可点「登录」的假账号），仅在「还原此备份」的来源里可选。
	return users, nil
}

// GetRemoteBackups returns display labels for downloaded remote backup archives.
// 它们不是 Steam 账号，只作为「还原来源」出现。
func (s *AppService) GetRemoteBackups() []steam.UserCardInfo {
	s.mu.RLock()
	bkSvc := s.backupService
	s.mu.RUnlock()
	if bkSvc == nil {
		return nil
	}
	backedUp, err := bkSvc.GetBackedUpAccounts()
	if err != nil {
		return nil
	}
	bp := bkSvc.GetBackupPath()
	cards := make([]steam.UserCardInfo, 0)
	for _, acc := range backedUp {
		if strings.HasPrefix(acc, "remote_") {
			cards = append(cards, s.buildRemoteUserCard(bp, acc))
		}
	}
	return cards
}

func (s *AppService) buildRemoteUserCard(backupPath, accountID string) steam.UserCardInfo {
	infoPath := filepath.Join(backupPath, "userdata_backup", accountID, "remote_info.json")
	username := "来自其它设备的备份"
	sourceName := ""
	downloadTime := ""

	if data, err := os.ReadFile(infoPath); err == nil {
		var rInfo struct {
			AccountID    string `json:"account_id"`
			Username     string `json:"username"`
			SourceName   string `json:"source_name"`
			DownloadTime string `json:"download_time"`
			Type         string `json:"type"`
		}
		if err := json.Unmarshal(data, &rInfo); err == nil {
			if rInfo.Username != "" {
				username = rInfo.Username
			}
			sourceName = rInfo.SourceName
			downloadTime = rInfo.DownloadTime
		}
	}

	notes := ""
	if sourceName != "" && downloadTime != "" {
		notes = fmt.Sprintf("来源设备: %s (下载时间: %s)", sourceName, downloadTime)
	} else if sourceName != "" {
		notes = fmt.Sprintf("来源设备: %s", sourceName)
	} else if downloadTime != "" {
		notes = fmt.Sprintf("下载时间: %s", downloadTime)
	}

	displayName := username
	if sourceName != "" {
		displayName = fmt.Sprintf("%s (%s)", username, sourceName)
	}

	return steam.UserCardInfo{
		AccountID: accountID,
		Username:  displayName,
		Notes:     notes,
		IsRecent:  false,
	}
}

// SaveUserNote persists custom user notes for an account ID.
func (s *AppService) SaveUserNote(accountID string, note string) error {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("steam service not initialized")
	}
	if err := svc.SaveUserNote(accountID, note); err != nil {
		s.LogError(fmt.Sprintf("保存用户备注失败 [AccountID %s]: %v", accountID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已保存用户备注 [AccountID %s]: %s", accountID, note))
	return nil
}

// ResolveUserName returns the username for an account ID.
func (s *AppService) ResolveUserName(accountID string) (string, error) {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return accountID, fmt.Errorf("steam service not initialized")
	}
	return svc.ResolveUserName(accountID)
}

// GetBackedUpAccounts returns a list of account IDs that have existing backups.
func (s *AppService) GetBackedUpAccounts() ([]string, error) {
	s.mu.RLock()
	svc := s.backupService
	s.mu.RUnlock()
	if svc == nil {
		return []string{}, nil
	}
	return svc.GetBackedUpAccounts()
}
