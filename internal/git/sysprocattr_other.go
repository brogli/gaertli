//go:build !windows

package git

import "syscall"

func sysProcAttr() *syscall.SysProcAttr { return nil }
