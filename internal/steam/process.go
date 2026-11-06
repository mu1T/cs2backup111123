package steam

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

var (
	runningChecker     = isSteamRunningOS
	processTerminator  = terminateSteamOS
	processStarter     = startSteamOS
	commandRunner      = defaultCommandRunner
)

func defaultCommandRunner(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	return cmd.Start()
}

// SetProcessStarterForTesting allows unit and integration tests to safely mock processStarter.
func SetProcessStarterForTesting(fn func(steamPath string, args ...string) error) func() {
	orig := processStarter
	processStarter = fn
	return func() {
		processStarter = orig
	}
}

// IsSteamRunning checks if steam.exe or steamwebhelper.exe is currently active.
func IsSteamRunning() (bool, error) {
	return runningChecker()
}

// ShutdownSteam initiates Valve's official graceful shutdown signal (steam.exe -shutdown)
// and waits up to 5 seconds. If Steam fails to exit within 5 seconds, fallback termination is applied.
func ShutdownSteam(steamPath string) error {
	return shutdownSteamWithInterval(steamPath, 5*time.Second, 200*time.Millisecond)
}

func shutdownSteamWithInterval(steamPath string, timeout, checkInterval time.Duration) error {
	running, err := runningChecker()
	if err != nil {
		return fmt.Errorf("failed to check steam status: %w", err)
	}
	if !running {
		return nil
	}

	// Locate steam.exe
	steamExe := ""
	if steamPath != "" {
		candidate := filepath.Join(steamPath, "steam.exe")
		if _, err := os.Stat(candidate); err == nil {
			steamExe = candidate
		}
	}
	if steamExe == "" {
		if detected, err := DetectSteamPath(); err == nil && detected != "" {
			candidate := filepath.Join(detected, "steam.exe")
			if _, err := os.Stat(candidate); err == nil {
				steamExe = candidate
			}
		}
	}

	// 1. Send graceful shutdown command
	if steamExe != "" {
		_ = commandRunner(steamExe, "-shutdown")
	}

	// 2. Poll until timeout
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		time.Sleep(checkInterval)
		active, err := runningChecker()
		if err == nil && !active {
			return nil
		}
	}

	// 3. Fallback force termination if still running after timeout
	if err := processTerminator(); err != nil {
		return fmt.Errorf("fallback process termination failed: %w", err)
	}

	// Brief verify
	time.Sleep(100 * time.Millisecond)
	if active, _ := runningChecker(); active {
		return fmt.Errorf("steam process still active after termination")
	}

	return nil
}

// StartSteam launches steam.exe in detached background mode without blocking.
// If launchGame is true, it passes -applaunch 730 to launch CS2 immediately.
func StartSteam(steamPath string, launchGame ...bool) error {
	if steamPath == "" {
		detected, err := DetectSteamPath()
		if err != nil {
			return fmt.Errorf("steam path not found: %w", err)
		}
		steamPath = detected
	}

	steamExe := filepath.Join(steamPath, "steam.exe")
	if _, err := os.Stat(steamExe); err != nil {
		return fmt.Errorf("steam.exe not found at %s: %w", steamPath, err)
	}

	var args []string
	if len(launchGame) > 0 && launchGame[0] {
		args = append(args, "-applaunch", "730")
	}

	return processStarter(steamPath, args...)
}

// StartCS2Game launches CS2 directly via steam.exe -applaunch 730.
func StartCS2Game(steamPath ...string) error {
	var path string
	if len(steamPath) > 0 {
		path = steamPath[0]
	}
	return StartSteam(path, true)
}
