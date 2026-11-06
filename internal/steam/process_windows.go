//go:build windows

package steam

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

func isSteamRunningOS() (bool, error) {
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return false, fmt.Errorf("failed to create toolhelp32 snapshot: %w", err)
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snap, &entry); err != nil {
		return false, fmt.Errorf("failed to enumerate first process: %w", err)
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "steam.exe") || strings.EqualFold(name, "steamwebhelper.exe") {
			return true, nil
		}

		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}

	return false, nil
}

func terminateSteamOS() error {
	// 1. Try taskkill for process tree termination
	_ = exec.Command("taskkill", "/F", "/IM", "steam.exe", "/T").Run()
	_ = exec.Command("taskkill", "/F", "/IM", "steamwebhelper.exe", "/T").Run()

	// 2. Direct Win32 process termination for any lingering PIDs
	snap, err := windows.CreateToolhelp32Snapshot(windows.TH32CS_SNAPPROCESS, 0)
	if err != nil {
		return nil
	}
	defer windows.CloseHandle(snap)

	var entry windows.ProcessEntry32
	entry.Size = uint32(unsafe.Sizeof(entry))

	if err := windows.Process32First(snap, &entry); err != nil {
		return nil
	}

	for {
		name := windows.UTF16ToString(entry.ExeFile[:])
		if strings.EqualFold(name, "steam.exe") || strings.EqualFold(name, "steamwebhelper.exe") {
			if h, err := windows.OpenProcess(windows.PROCESS_TERMINATE, false, entry.ProcessID); err == nil {
				_ = windows.TerminateProcess(h, 1)
				_ = windows.CloseHandle(h)
			}
		}

		if err := windows.Process32Next(snap, &entry); err != nil {
			break
		}
	}

	return nil
}

func startSteamOS(steamPath string, args ...string) error {
	steamExe := filepath.Join(steamPath, "steam.exe")
	cmd := exec.Command(steamExe, args...)
	cmd.Dir = steamPath
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: windows.DETACHED_PROCESS | windows.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start steam.exe in detached mode: %w", err)
	}

	return nil
}
