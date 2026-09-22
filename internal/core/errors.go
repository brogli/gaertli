package core

import "fmt"

// Code is the machine-readable failure class.
type Code string

const (
	NotARepo       Code = "NOT_A_REPO"
	GitTooOld      Code = "GIT_TOO_OLD"
	AuthFailed     Code = "AUTH_FAILED"
	PushRejected   Code = "PUSH_REJECTED"
	NotFastForward Code = "NOT_FAST_FORWARD"
	DirtyWorktree  Code = "DIRTY_WORKTREE"
	Conflicts      Code = "CONFLICTS"
	Locked         Code = "LOCKED"
	InvalidName    Code = "INVALID_NAME"
	NoUpstream     Code = "NO_UPSTREAM"
	EditorNotFound Code = "EDITOR_NOT_FOUND"
	// FileNotFound: OpenFile's path no longer exists in the worktree (a
	// deleted or since-renamed file shown from a historical diff).
	FileNotFound Code = "FILE_NOT_FOUND"
	Cancelled    Code = "CANCELLED"
	GitFailed    Code = "GIT_FAILED"
	// Internal marks a failure that is not git's: a panic, a bad request, a
	// broken transport.
	Internal Code = "INTERNAL"
	// WslNotFound: wsl.exe not on PATH (Windows), or any WSL call attempted
	// on a non-Windows shell.
	WslNotFound Code = "WSL_NOT_FOUND"
	// DistroNotFound: the named distro is not among ListDistros's result, or
	// wsl.exe -d <name> itself refuses it.
	DistroNotFound Code = "DISTRO_NOT_FOUND"
	// DeployRefused: the consent dialog to copy the core into a distro was
	// declined.
	DeployRefused Code = "DEPLOY_REFUSED"
)

// Error is the one error type that crosses the core boundary. GitStderr and
// Command feed the UI's "show details".
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	// GitStderr is git's own explanation of the failure: stderr, or stdout
	// when stderr is empty.
	GitStderr string `json:"gitStderr,omitempty"`
	Command   string `json:"command,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Errorf builds an Error without git context.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
