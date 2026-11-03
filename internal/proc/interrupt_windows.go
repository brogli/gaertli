//go:build windows

package proc

import (
	"os"
	"sync"
	"syscall"

	"golang.org/x/sys/windows"
)

var (
	kernel32      = windows.NewLazySystemDLL("kernel32.dll")
	attachConsole = kernel32.NewProc("AttachConsole")
	freeConsole   = kernel32.NewProc("FreeConsole")

	// consoleMu serialises sendBreak: a process is attached to one console
	// at a time.
	consoleMu sync.Mutex
)

// Interruptible is HiddenConsole plus a process group of the child's own,
// which a CTRL_BREAK event from Interrupt reaches with all its descendants.
func Interruptible() *syscall.SysProcAttr {
	return &syscall.SysProcAttr{CreationFlags: windows.CREATE_NO_WINDOW | windows.CREATE_NEW_PROCESS_GROUP}
}

// Interrupt sends p's process group, started with Interruptible, a
// CTRL_BREAK event. A console program's default handler for it calls
// ExitProcess, which runs git's exit handlers, and those remove the lock
// files git holds. p is killed at once if the event cannot be sent, e.g.
// from a process that has a console of its own.
func Interrupt(p *os.Process) error {
	if sendBreak(uint32(p.Pid)) != nil {
		return p.Kill()
	}
	return nil
}

// KillGroup does nothing on Windows: reaching the other members of p's
// group would take a job object. A child of git that ignored the break
// event runs on until it ends by itself.
func KillGroup(*os.Process) {}

// sendBreak attaches to pid's hidden console, the only way to reach it
// from a process without one, sends CTRL_BREAK to pid's group and detaches.
func sendBreak(pid uint32) error {
	consoleMu.Lock()
	defer consoleMu.Unlock()
	if ok, _, err := attachConsole.Call(uintptr(pid)); ok == 0 {
		return err
	}
	defer func() { _, _, _ = freeConsole.Call() }()
	return windows.GenerateConsoleCtrlEvent(windows.CTRL_BREAK_EVENT, pid)
}
