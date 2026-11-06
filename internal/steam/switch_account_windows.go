//go:build windows

package steam

import (
	"fmt"

	"golang.org/x/sys/windows/registry"
)

func setSteamAutoLoginRegistry(accountName string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.SET_VALUE)
	if err != nil {
		return fmt.Errorf("failed to open Steam registry key HKCU\\Software\\Valve\\Steam: %w", err)
	}
	defer k.Close()

	if err := k.SetStringValue("AutoLoginUser", accountName); err != nil {
		return fmt.Errorf("failed to set AutoLoginUser registry value: %w", err)
	}

	if err := k.SetDWordValue("RememberPassword", 1); err != nil {
		return fmt.Errorf("failed to set RememberPassword registry value: %w", err)
	}

	return nil
}
