//go:build !windows

package proc

import (
	"os/exec"
	"syscall"
)

// Detach makes cmd survive its parent's exit: its own session, so
// an editor or the handed-off GUI keeps running after gaertli is gone.
func Detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
