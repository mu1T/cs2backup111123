package steam

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	"cs2backup/internal/testutil"
)

func TestIsSteamRunning_Default(t *testing.T) {
	// Call actual OS detection
	running, err := IsSteamRunning()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Logf("Host Steam running status: %v", running)
}

func TestIsSteamRunning_Mock(t *testing.T) {
	orig := runningChecker
	defer func() { runningChecker = orig }()

	runningChecker = func() (bool, error) {
		return true, nil
	}

	running, err := IsSteamRunning()
	if err != nil || !running {
		t.Fatalf("expected running true, got %v, err: %v", running, err)
	}

	runningChecker = func() (bool, error) {
		return false, nil
	}

	running, err = IsSteamRunning()
	if err != nil || running {
		t.Fatalf("expected running false, got %v, err: %v", running, err)
	}
}

func TestShutdownSteam_AlreadyStopped(t *testing.T) {
	orig := runningChecker
	defer func() { runningChecker = orig }()

	runningChecker = func() (bool, error) {
		return false, nil
	}

	err := ShutdownSteam("")
	if err != nil {
		t.Fatalf("expected nil when already stopped, got %v", err)
	}
}

func TestShutdownSteam_GracefulExit(t *testing.T) {
	origChecker := runningChecker
	origRunner := commandRunner
	defer func() {
		runningChecker = origChecker
		commandRunner = origRunner
	}()

	calls := 0
	runningChecker = func() (bool, error) {
		calls++
		if calls <= 2 {
			return true, nil
		}
		// Stops after -shutdown called
		return false, nil
	}

	shutdownCalled := false
	commandRunner = func(name string, args ...string) error {
		if filepath.Base(name) == "steam.exe" && len(args) > 0 && args[0] == "-shutdown" {
			shutdownCalled = true
		}
		return nil
	}

	tmpDir := testutil.TempDir(t)
	steamExe := filepath.Join(tmpDir, "steam.exe")
	if err := os.WriteFile(steamExe, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	// Use short wait timeout for test
	err := shutdownSteamWithInterval(tmpDir, 500*time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !shutdownCalled {
		t.Errorf("expected steam.exe -shutdown to be called")
	}
}

func TestShutdownSteam_TimeoutFallback(t *testing.T) {
	origChecker := runningChecker
	origRunner := commandRunner
	origTerm := processTerminator
	defer func() {
		runningChecker = origChecker
		commandRunner = origRunner
		processTerminator = origTerm
	}()

	// Always returns running = true until terminated
	runningChecker = func() (bool, error) {
		return true, nil
	}

	commandRunner = func(name string, args ...string) error {
		return nil
	}

	fallbackTerminated := false
	processTerminator = func() error {
		fallbackTerminated = true
		runningChecker = func() (bool, error) {
			return false, nil
		}
		return nil
	}

	tmpDir := testutil.TempDir(t)
	steamExe := filepath.Join(tmpDir, "steam.exe")
	if err := os.WriteFile(steamExe, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	err := shutdownSteamWithInterval(tmpDir, 100*time.Millisecond, 20*time.Millisecond)
	if err != nil {
		t.Fatalf("expected success after fallback termination, got: %v", err)
	}

	if !fallbackTerminated {
		t.Errorf("expected fallback termination to be triggered after timeout")
	}
}

func TestStartSteam_MissingPath(t *testing.T) {
	origStarter := processStarter
	defer func() { processStarter = origStarter }()

	processStarter = func(path string, args ...string) error {
		return errors.New("steam.exe not found")
	}

	err := StartSteam("non_existent_path")
	if err == nil {
		t.Fatal("expected error for missing steam path, got nil")
	}
}

func TestStartSteam_Success(t *testing.T) {
	origStarter := processStarter
	defer func() { processStarter = origStarter }()

	started := false
	processStarter = func(path string, args ...string) error {
		started = true
		return nil
	}

	tmpDir := testutil.TempDir(t)
	steamExe := filepath.Join(tmpDir, "steam.exe")
	if err := os.WriteFile(steamExe, []byte("fake"), 0755); err != nil {
		t.Fatal(err)
	}

	err := StartSteam(tmpDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !started {
		t.Errorf("expected processStarter to be called")
	}
}

func TestStartCS2Game(t *testing.T) {
	origStarter := processStarter
	defer func() { processStarter = origStarter }()

	var capturedArgs []string
	processStarter = func(path string, args ...string) error {
		capturedArgs = args
		return nil
	}

	tmpDir := testutil.TempDir(t)
	steamExe := filepath.Join(tmpDir, "steam.exe")
	_ = os.WriteFile(steamExe, []byte("fake"), 0755)

	if err := StartCS2Game(tmpDir); err != nil {
		t.Fatalf("StartCS2Game failed: %v", err)
	}

	if len(capturedArgs) < 2 || capturedArgs[0] != "-applaunch" || capturedArgs[1] != "730" {
		t.Fatalf("expected [-applaunch 730], got %v", capturedArgs)
	}
}
