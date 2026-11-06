//go:build !windows

package steam

import (
	"errors"
)

func isSteamRunningOS() (bool, error) {
	return false, nil
}

func terminateSteamOS() error {
	return nil
}

func startSteamOS(steamPath string, args ...string) error {
	return errors.New("startSteam is only supported on Windows")
}
