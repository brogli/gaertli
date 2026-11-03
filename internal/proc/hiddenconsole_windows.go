//go:build windows

package proc

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// HiddenConsole suppresses the console window a child process (git,
// wsl.exe) would otherwise flash open. HideWindow alone is a
// STARTF_USESHOWWINDOW hint, not console suppression.
func HiddenConsole() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}
