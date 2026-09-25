package core

import (
	"errors"
	"fmt"
)

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
	// Stopped: an operation stopped mid-way for another reason than
	// conflicts (a hook, signing, a file in the way); it is left in
	// progress for continue or abort.
	Stopped     Code = "STOPPED"
	Locked      Code = "LOCKED"
	InvalidName Code = "INVALID_NAME"
	NoUpstream  Code = "NO_UPSTREAM"
	// Unmerged: DeleteBranch without force refused a branch with commits
	// not merged into its upstream, or into HEAD if it has none. Force
	// fixes it, unlike a GitFailed "used by worktree" refusal.
	Unmerged       Code = "UNMERGED_BRANCH"
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
	// DistroNotFound: wsl.exe failed for a distro that ListDistros does not
	// list.
	DistroNotFound Code = "DISTRO_NOT_FOUND"
	// DeployRefused: the consent dialog to copy the core into a distro was
	// declined.
	DeployRefused Code = "DEPLOY_REFUSED"
	// WslUnsupported: the distro runs on WSL 1.
	WslUnsupported Code = "WSL_UNSUPPORTED"
)

// Error is the one error type that crosses the core boundary. GitStderr and
// Command feed the UI's "show details".
type Error struct {
	Code    Code   `json:"code"`
	Message string `json:"message"`
	// Key names Message's wording for the UI to translate; one Code can
	// have several (GitFailed's "worktree exists" and "merge commit not
	// pickable"). "" where Message is untranslatable (Internal, git's own
	// text). ErrorKeys lists every Key.
	Key string `json:"key,omitempty"`
	// Args holds Message's variable parts by name, for Key's translation.
	Args map[string]string `json:"args,omitempty"`
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

// ErrorfArgs is Errorf plus the Key and Args to translate the message by.
// format still renders the English Message, for the app log and as the
// UI's fallback.
func ErrorfArgs(code Code, key string, args map[string]string, format string, a ...any) *Error {
	e := Errorf(code, format, a...)
	e.Key, e.Args = key, args
	return e
}

// Retag reclassifies e in place, as ErrorfArgs builds a fresh Error, once
// a remapper recognises git's stderr. GitStderr and Command stay for "show
// details". A remapper that keeps git's own message as the headline sets
// Code alone.
func (e *Error) Retag(code Code, key string, args map[string]string, format string, a ...any) {
	e.Code, e.Key, e.Args = code, key, args
	e.Message = fmt.Sprintf(format, a...)
}

// ErrorOf returns the *Error in err's chain, or wraps a non-nil err of any
// other type as Code Internal.
func ErrorOf(err error) *Error {
	if ce, ok := errors.AsType[*Error](err); ok {
		return ce
	}
	return Errorf(Internal, "%v", err)
}
