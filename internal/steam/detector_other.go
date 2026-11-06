//go:build !windows

package steam

import "errors"

func detectSteamPathRegistry() (string, error) {
	return "", errors.New("registry not supported on non-windows OS")
}
