//go:build windows

package main

import (
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows"
)

// distroCommand runs the server inside a WSL distro (Design): wsl.exe -d
// <distro> -- <remote> serve. CREATE_NO_WINDOW (TR-12) hides its console;
// windows.HideWindow only sets a STARTUPINFO show-state hint, not real
// console suppression (research/go/os-exec-on-windows.md).
func distroCommand(distro, remote string) (*exec.Cmd, error) {
	cmd := exec.Command("wsl.exe", "-d", distro, "--", remote, "serve")
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
	return cmd, nil
}
