//go:build windows

package steam

import (
	"golang.org/x/sys/windows/registry"
)

func detectSteamPathRegistry() (string, error) {
	k, err := registry.OpenKey(registry.CURRENT_USER, `Software\Valve\Steam`, registry.QUERY_VALUE)
	if err != nil {
		return "", err
	}
	defer k.Close()

	val, _, err := k.GetStringValue("SteamPath")
	if err != nil {
		return "", err
	}
	return val, nil
}
