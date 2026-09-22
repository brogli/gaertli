package core

import "fmt"

// Code is the machine-readable failure class (errors-security.md).
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
	EditorNotFound Code = "EDITOR_NOT_FOUND"
	Cancelled      Code = "CANCELLED"
	GitFailed      Code = "GIT_FAILED"
	// Internal marks a failure that is not git's: a panic, a bad request, a
	// broken transport.
	Internal Code = "INTERNAL"
	// WslNotFound: wsl.exe not on PATH (Windows), or any WSL call attempted
	// on a non-Windows shell (spec 30).
	WslNotFound Code = "WSL_NOT_FOUND"
	// DistroNotFound: the named distro is not among ListDistros's result, or
	// wsl.exe -d <name> itself refuses it (spec 30).
	DistroNotFound Code = "DISTRO_NOT_FOUND"
	// DeployRefused: the consent dialog to copy the core into a distro was
	// declined (spec 30, QA-PORT-02).
	DeployRefused Code = "DEPLOY_REFUSED"
)

// Error is the one error type that crosses the core boundary. GitStderr and
// Command feed the UI's "show details" (FR-UX-01).
type Error struct {
	Code      Code   `json:"code"`
	Message   string `json:"message"`
	GitStderr string `json:"gitStderr,omitempty"`
	Command   string `json:"command,omitempty"`
}

func (e *Error) Error() string { return fmt.Sprintf("%s: %s", e.Code, e.Message) }

// Errorf builds an Error without git context.
func Errorf(code Code, format string, args ...any) *Error {
	return &Error{Code: code, Message: fmt.Sprintf(format, args...)}
}
