//go:build !windows

package main

import (
	"errors"
	"os/exec"
)

// distroCommand is the wsl.exe hop (Design); it only exists on Windows.
func distroCommand(distro, remote string) (*exec.Cmd, error) {
	return nil, errors.New("wsl.exe dial is Windows-only")
}
