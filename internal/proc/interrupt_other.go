//go:build !windows

package proc

import (
	"os"
	"syscall"
)

// Interruptible puts a child in a session of its own, and so in a process
// group of its own, so Interrupt reaches it together with its descendants
// (hooks, transport helpers), as an interrupt in a terminal does. The
// session has no controlling terminal: a prompt on /dev/tty fails at once,
// as in a desktop launch, where a background group of the app's terminal
// would be stopped by it.
func Interruptible() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{Setsid: true}
}

// Interrupt sends SIGTERM to the process group of p, started with
// Interruptible. git's handler for it removes the lock files git holds.
// Once p has been waited for, it returns os.ErrProcessDone and signals
// nothing: the group id may no longer be p's.
func Interrupt(p *os.Process) error {
	if err := p.Signal(syscall.Signal(0)); err != nil {
		return err
	}
	return syscall.Kill(-p.Pid, syscall.SIGTERM)
}

// KillGroup kills every member of the process group of p, started with
// Interruptible, that still runs: one that ignored Interrupt. p may have
// been waited for: the group id cannot pass to another process while a
// member lives.
func KillGroup(p *os.Process) {
	_ = syscall.Kill(-p.Pid, syscall.SIGKILL) // ESRCH: nothing is left
}
