package app

import (
	"fmt"

	"cs2backup/internal/steam"
)

// IsSteamRunning checks whether steam.exe or steamwebhelper.exe is currently running.
func (s *AppService) IsSteamRunning() (bool, error) {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return steam.IsSteamRunning()
	}
	return svc.IsSteamRunning()
}

// ShutdownSteam gracefully exits Steam, falling back to process termination if not closed within 5 seconds.
func (s *AppService) ShutdownSteam() error {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()

	s.LogInfo("正在向 Steam 客户端发送优雅退出指令 (-shutdown)...")
	var err error
	if svc != nil {
		err = svc.ShutdownSteam()
	} else {
		err = steam.ShutdownSteam("")
	}

	if err != nil {
		s.LogError(fmt.Sprintf("退出 Steam 客户端失败: %v", err))
		return err
	}
	s.LogSuccess("Steam 客户端已成功安全退出")
	return nil
}

// StartSteam launches steam.exe in detached background mode without blocking.
// If launchGame is true, CS2 is launched directly via -applaunch 730.
func (s *AppService) StartSteam(launchGame ...bool) error {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()

	shouldLaunchGame := len(launchGame) > 0 && launchGame[0]
	if shouldLaunchGame {
		s.LogInfo("正在启动 Steam 并运行 CS2...")
	} else {
		s.LogInfo("正在后台启动 Steam 客户端...")
	}

	var err error
	if svc != nil {
		err = svc.StartSteam(shouldLaunchGame)
	} else {
		err = steam.StartSteam("", shouldLaunchGame)
	}

	if err != nil {
		s.LogError(fmt.Sprintf("启动 Steam 客户端失败: %v", err))
		return err
	}
	if shouldLaunchGame {
		s.LogSuccess("Steam 客户端已在后台启动并拉起 CS2")
	} else {
		s.LogSuccess("Steam 客户端已在后台启动")
	}
	return nil
}

// StartCS2Game launches CS2 directly via steam.exe -applaunch 730.
func (s *AppService) StartCS2Game() error {
	return s.StartSteam(true)
}

// SwitchSteamAccount switches the active Steam account to the specified accountName.
func (s *AppService) SwitchSteamAccount(accountName string) error {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()

	s.LogInfo(fmt.Sprintf("正在切换 Steam 登录账号至 [%s]...", accountName))
	var err error
	if svc != nil {
		err = svc.SwitchSteamAccount(accountName)
	} else {
		err = steam.SwitchSteamAccount("", accountName)
	}

	if err != nil {
		s.LogError(fmt.Sprintf("切换 Steam 账号失败 [%s]: %v", accountName, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("成功切换 Steam 账号至 [%s]，Steam 客户端正在启动", accountName))
	return nil
}

// GetLaunchOptions retrieves CS2 launch options for the specified account ID.
func (s *AppService) GetLaunchOptions(accountID string) (string, error) {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return "", fmt.Errorf("steam service not initialized")
	}

	return svc.GetLaunchOptions(accountID)
}

// SetLaunchOptions updates CS2 launch options for the specified account ID.
func (s *AppService) SetLaunchOptions(accountID, options string) error {
	s.mu.RLock()
	svc := s.steamService
	s.mu.RUnlock()
	if svc == nil {
		return fmt.Errorf("steam service not initialized")
	}

	s.LogInfo(fmt.Sprintf("正在更新账号 [%s] 的 CS2 启动项配置...", accountID))
	if err := svc.SetLaunchOptions(accountID, options); err != nil {
		s.LogError(fmt.Sprintf("更新启动项失败 [AccountID %s]: %v", accountID, err))
		return err
	}
	s.LogSuccess(fmt.Sprintf("成功更新账号 [%s] 的 CS2 启动项配置", accountID))
	return nil
}
