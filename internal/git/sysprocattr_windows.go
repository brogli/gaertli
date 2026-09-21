//go:build windows

package git

import (
	"syscall"

	"golang.org/x/sys/windows"
)

// sysProcAttr suppresses the console window git would otherwise flash open
// (TR-12). HideWindow alone is a STARTF_USESHOWWINDOW hint, not console
// suppression (research/go/os-exec-on-windows.md).
func sysProcAttr() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW}
}
