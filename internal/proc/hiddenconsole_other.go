//go:build !windows

package proc

import "syscall"

func HiddenConsole() *syscall.SysProcAttr { return nil }
