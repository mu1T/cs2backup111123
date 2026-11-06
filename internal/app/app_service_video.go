package app

import (
	"fmt"

	"cs2backup/internal/video"
)

// Video Service Delegations

// GetDisplayModes enumerates display modes supported by the primary monitor.
func (s *AppService) GetDisplayModes() ([]video.DisplayMode, error) {
	return video.GetDisplayModes()
}

// GetVideoSettings reads and parses cs2_video.txt for the specified account ID.
func (s *AppService) GetVideoSettings(accountID string) (*video.VideoConfigData, error) {
	s.mu.RLock()
	steamSvc := s.steamService
	s.mu.RUnlock()

	if steamSvc == nil {
		return nil, fmt.Errorf("steam service not initialized")
	}
	steamPath := steamSvc.GetSteamPath()
	if steamPath == "" {
		return nil, fmt.Errorf("未配置 Steam 路径，请先指定 Steam 根目录")
	}

	targetPath := video.GetAccountVideoPath(steamPath, accountID)
	cf, err := video.LoadConfigFile(targetPath)
	if err != nil {
		s.LogWarning(fmt.Sprintf("读取账号 [%s] 视频设置失败: %v", accountID, err))
		return nil, err
	}

	return &video.VideoConfigData{
		FilePath: cf.FilePath,
		ReadOnly: cf.ReadOnly,
		Settings: cf.Settings,
	}, nil
}

// SaveVideoSettings updates cs2_video.txt with updated settings and configures read-only lock.
func (s *AppService) SaveVideoSettings(accountID string, settings map[string]string, readOnly bool) error {
	s.mu.RLock()
	steamSvc := s.steamService
	s.mu.RUnlock()

	if steamSvc == nil {
		return fmt.Errorf("steam service not initialized")
	}
	steamPath := steamSvc.GetSteamPath()
	if steamPath == "" {
		return fmt.Errorf("未配置 Steam 路径，请先指定 Steam 根目录")
	}

	targetPath := video.GetAccountVideoPath(steamPath, accountID)
	s.LogInfo(fmt.Sprintf("正在保存账号 [%s] 视频配置 (只读锁定: %v)...", accountID, readOnly))

	if err := video.SaveConfigFile(targetPath, settings, readOnly); err != nil {
		s.LogError(fmt.Sprintf("保存账号 [%s] 视频配置失败: %v", accountID, err))
		return err
	}

	lockStr := "已解锁可写"
	if readOnly {
		lockStr = "已锁定只读"
	}
	s.LogSuccess(fmt.Sprintf("成功保存账号 [%s] 的视频配置 (%s)", accountID, lockStr))
	return nil
}

// SelectVideoConfigFile prompts the user to select a .txt video configuration file.
func (s *AppService) SelectVideoConfigFile() (string, error) {
	s.mu.RLock()
	picker := s.singlePicker
	s.mu.RUnlock()

	if picker == nil {
		return "", fmt.Errorf("single file picker not configured")
	}

	file, err := picker("选择 CS2 视频配置文件", "CS2 视频配置文件 (*.txt)", "*.txt")
	if err != nil {
		s.LogError(fmt.Sprintf("选择视频配置文件异常: %v", err))
		return "", err
	}
	return file, nil
}

// ImportVideoPreset imports an external .txt file as a new preset.
func (s *AppService) ImportVideoPreset(filePath, name string, setAsDefault bool) (*video.VideoPreset, error) {
	s.mu.RLock()
	mgr := s.videoManager
	s.mu.RUnlock()

	if mgr == nil {
		return nil, fmt.Errorf("video preset manager not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在导入视频配置预设 [%s] (路径: %s)...", name, filePath))
	preset, err := mgr.ImportPreset(filePath, name, setAsDefault)
	if err != nil {
		s.LogError(fmt.Sprintf("导入视频预设失败: %v", err))
		return nil, err
	}

	s.LogSuccess(fmt.Sprintf("成功导入视频预设 [%s] (ID: %s)", preset.Name, preset.ID))
	return preset, nil
}

// SaveAsVideoPreset saves the account's cs2_video.txt as a new preset.
func (s *AppService) SaveAsVideoPreset(accountID, name string, setAsDefault bool) (*video.VideoPreset, error) {
	s.mu.RLock()
	steamSvc := s.steamService
	mgr := s.videoManager
	s.mu.RUnlock()

	if steamSvc == nil || mgr == nil {
		return nil, fmt.Errorf("services not initialized")
	}
	steamPath := steamSvc.GetSteamPath()
	if steamPath == "" {
		return nil, fmt.Errorf("未配置 Steam 路径")
	}

	s.LogInfo(fmt.Sprintf("正在将账号 [%s] 的当前视频配置保存为预设 [%s]...", accountID, name))
	preset, err := mgr.SaveAccountAsPreset(steamPath, accountID, name, setAsDefault)
	if err != nil {
		s.LogError(fmt.Sprintf("保存账号视频预设失败: %v", err))
		return nil, err
	}

	s.LogSuccess(fmt.Sprintf("成功将账号 [%s] 视频配置存为新预设 [%s]", accountID, preset.Name))
	return preset, nil
}

// GetVideoPresets returns all saved presets.
func (s *AppService) GetVideoPresets() ([]video.VideoPreset, error) {
	s.mu.RLock()
	mgr := s.videoManager
	s.mu.RUnlock()

	if mgr == nil {
		return []video.VideoPreset{}, nil
	}
	return mgr.GetPresets()
}

// SetDefaultVideoPreset sets the designated preset as global default.
func (s *AppService) SetDefaultVideoPreset(presetID string) error {
	s.mu.RLock()
	mgr := s.videoManager
	s.mu.RUnlock()

	if mgr == nil {
		return fmt.Errorf("video preset manager not initialized")
	}

	if err := mgr.SetDefaultPreset(presetID); err != nil {
		s.LogError(fmt.Sprintf("设置默认视频预设失败: %v", err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已成功将预设 [%s] 设为全局默认预设", presetID))
	return nil
}

// DeleteVideoPreset deletes a preset by ID.
func (s *AppService) DeleteVideoPreset(presetID string) error {
	s.mu.RLock()
	mgr := s.videoManager
	s.mu.RUnlock()

	if mgr == nil {
		return fmt.Errorf("video preset manager not initialized")
	}

	if err := mgr.DeletePreset(presetID); err != nil {
		s.LogError(fmt.Sprintf("删除视频预设失败: %v", err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("已成功删除视频预设 [%s]", presetID))
	return nil
}

// ApplyVideoPreset applies a preset to the target account.
func (s *AppService) ApplyVideoPreset(accountID, presetID string, setReadOnly bool) error {
	s.mu.RLock()
	steamSvc := s.steamService
	mgr := s.videoManager
	s.mu.RUnlock()

	if steamSvc == nil || mgr == nil {
		return fmt.Errorf("services not initialized")
	}
	steamPath := steamSvc.GetSteamPath()
	if steamPath == "" {
		return fmt.Errorf("未配置 Steam 路径")
	}

	s.LogInfo(fmt.Sprintf("正在将预设 [%s] 应用至账号 [%s] (只读锁定: %v)...", presetID, accountID, setReadOnly))
	if err := mgr.ApplyPreset(steamPath, accountID, presetID, setReadOnly); err != nil {
		s.LogError(fmt.Sprintf("应用视频预设失败 [AccountID %s]: %v", accountID, err))
		return err
	}

	s.LogSuccess(fmt.Sprintf("成功将视频预设 [%s] 应用至账号 [%s]", presetID, accountID))
	return nil
}

// ApplyDefaultVideoPreset applies the global default preset to the target account.
func (s *AppService) ApplyDefaultVideoPreset(accountID string, setReadOnly bool) error {
	s.mu.RLock()
	steamSvc := s.steamService
	mgr := s.videoManager
	s.mu.RUnlock()

	if steamSvc == nil || mgr == nil {
		return fmt.Errorf("services not initialized")
	}
	steamPath := steamSvc.GetSteamPath()
	if steamPath == "" {
		return fmt.Errorf("未配置 Steam 路径")
	}

	s.LogInfo(fmt.Sprintf("正在将全局默认视频预设应用至账号 [%s] (只读锁定: %v)...", accountID, setReadOnly))
	if err := mgr.ApplyDefaultPreset(steamPath, accountID, setReadOnly); err != nil {
		s.LogError(fmt.Sprintf("应用默认视频预设失败 [AccountID %s]: %v", accountID, err))
		return err
	}

	s.LogSuccess(fmt.Sprintf("成功将全局默认视频预设应用至账号 [%s]", accountID))
	return nil
}
