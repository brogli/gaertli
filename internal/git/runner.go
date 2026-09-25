// Package git executes the system git as a subprocess.
package git

import (
	"bytes"
	"cmp"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/proc"
)

// Call is one git invocation.
type Call struct {
	Dir   string   // working directory: a repo or worktree root
	Args  []string // git's arguments; "git" itself is not included
	Stdin []byte   // piped to the process, e.g. "commit -F -"

	ReadOnly   bool     // adds GIT_OPTIONAL_LOCKS=0: the call performs no writes
	NoEditor   bool     // adds GIT_EDITOR=true: no editor ever waits for input
	Background bool     // logged at Debug, not Info, in the app log
	Env        []string // added to the environment as "KEY=value"; never logged

	// Timeout bounds this call when ctx has no deadline. Zero means 30s.
	Timeout time.Duration
}

// Result is what a call produced, populated whether or not err is nil.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Entry is one call, as Runner.Log gets it. Never carries environment
// variables; a URL with embedded credentials in Command is masked.
type Entry struct {
	Command    string // "git " + args, masked
	Dir        string
	Background bool
	Duration   time.Duration
	Result     Result
	Err        error // the *core.Error Run returned, or nil
}

// Runner executes the system git. The zero value runs "git" resolved via
// PATH with no log.
type Runner struct {
	Git string      // git executable; "" resolves "git" via exec.LookPath
	Log func(Entry) // called once per call, success or failure; nil is fine
}

// defaultTimeout bounds a call whose ctx has no deadline and whose
// Call.Timeout is zero.
const defaultTimeout = 30 * time.Second

// stopGrace is how long a git interrupted by a done ctx has to exit (git
// removes its lock files then) before it is killed; also how long its
// pipes stay open after it exited, for a grandchild that still holds them.
const stopGrace = 2 * time.Second

// Run executes call and waits for it to finish, ctx to be done, or the
// effective timeout to expire, whichever comes first.
//
// err is nil on exit 0. Otherwise err is *core.Error:
//   - ctx already done, or done before/while git runs: Code Cancelled
//   - git started and exited non-zero: Code GitFailed
//   - git could not be started (LookPath failure, bad Dir, ...): Code Internal
func (r Runner) Run(ctx context.Context, call Call) (Result, error) {
	ctx, cancel := withTimeout(ctx, call.Timeout)
	defer cancel()
	cmd, command, err := r.prepare(ctx, call)
	if err != nil {
		return r.notStarted(call, command, err)
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: cmd.ProcessState.ExitCode()}
	err = classify(ctx, cmd, runErr, result, command)
	r.log(call, command, result, duration, err)
	return result, err
}

// prepare builds the *exec.Cmd for call, shared by Run and Stream, and the
// command line as logged. cmd is nil and err is the Cancelled error to
// return at once if ctx is already done.
func (r Runner) prepare(ctx context.Context, call Call) (cmd *exec.Cmd, command string, err error) {
	command = formatCommand(call.Args, call.Stdin != nil)
	if ctx.Err() != nil {
		return nil, command, core.Errorf(core.Cancelled, "%s", ctx.Err())
	}

	cmd = exec.CommandContext(ctx, cmp.Or(r.Git, "git"), call.Args...)
	cmd.Dir = call.Dir
	cmd.Env = buildEnv(call)
	cmd.SysProcAttr = proc.Interruptible()
	cmd.Cancel = func() error { return interrupt(cmd.Process) }
	cmd.WaitDelay = stopGrace
	if call.Stdin != nil {
		cmd.Stdin = bytes.NewReader(call.Stdin)
	}
	return cmd, command, nil
}

// interrupt is a call's Cancel: it interrupts git's process group and kills
// what of the group still runs stopGrace later, since exec's own kill then
// reaches git alone, not a hook or transport that ignored the interrupt.
// A git that ended before its ctx did is neither interrupted nor killed.
func interrupt(git *os.Process) error {
	if err := proc.Interrupt(git); err != nil {
		return err
	}
	time.AfterFunc(stopGrace, func() { proc.KillGroup(git) })
	return nil
}

// notStarted logs and returns a call that never ran.
func (r Runner) notStarted(call Call, command string, err error) (Result, error) {
	result := Result{ExitCode: -1}
	r.log(call, command, result, 0, err)
	return result, err
}

func (r Runner) log(call Call, command string, result Result, duration time.Duration, err error) {
	if r.Log == nil {
		return
	}
	r.Log(Entry{
		Command:    command,
		Dir:        call.Dir,
		Background: call.Background,
		Duration:   duration,
		Result:     result,
		Err:        err,
	})
}

// withTimeout bounds ctx with the call's timeout when ctx has no deadline
// of its own.
func withTimeout(ctx context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	if _, hasDeadline := ctx.Deadline(); hasDeadline {
		return ctx, func() {}
	}
	if timeout == 0 {
		timeout = defaultTimeout
	}
	return context.WithTimeout(ctx, timeout)
}

// classify turns cmd's outcome into Run's error: Cancelled beats every
// other cause, a nil ProcessState means git never started, and everything
// else is a plain exit-code failure, GitFailed unless another process holds
// one of git's own lock files (Locked). This is the one place every call
// (Run, Stream, StreamErr) is classified, so every write gets Locked
// detection without action-specific code.
func classify(ctx context.Context, cmd *exec.Cmd, err error, result Result, command string) error {
	switch {
	case ctx.Err() != nil:
		return core.Errorf(core.Cancelled, "%s", ctx.Err())
	case cmd.ProcessState == nil:
		return core.Errorf(core.Internal, "%v", err)
	case err != nil:
		details := gitDetails(result)
		gitErr := lockedError(details)
		if gitErr == nil {
			gitErr = core.Errorf(core.GitFailed, "%s", gitMessage(details, result.ExitCode))
		}
		gitErr.GitStderr = details
		gitErr.Command = command
		return gitErr
	default:
		return nil
	}
}

// lockPattern is how git's lockfile code reports a lock file another
// process still holds, whatever the lock (index, HEAD, a ref, config,
// shallow, …) and whatever command hit it: an "error:" or "fatal:" line
// ending in this text, sometimes with an explanatory hint below.
var lockPattern = regexp.MustCompile(`Unable to create '([^']+)': File exists\.`)

// lockedError returns Code Locked with the lock file's base name if details
// carries lockPattern, else nil.
func lockedError(details string) *core.Error {
	m := lockPattern.FindStringSubmatch(details)
	if m == nil {
		return nil
	}
	lock := filepath.Base(m[1])
	return core.ErrorfArgs(core.Locked, "locked", map[string]string{"lock": lock},
		"Another program is using the repo (holding %s).", lock)
}

// gitMessage is a failure's user-facing message: up to four of git's own
// lines, starting at its first "fatal:"/"error:" line when there is one
// (progress lines come before it), without those prefixes or hint lines;
// the exit code when git printed nothing.
func gitMessage(details string, exitCode int) string {
	var lines []string
	for line := range strings.Lines(details) {
		if line = shownText(line); line != "" && !strings.HasPrefix(line, "hint:") {
			lines = append(lines, line)
		}
	}
	if i := slices.IndexFunc(lines, isGitReason); i > 0 {
		lines = lines[i:]
	}
	if len(lines) == 0 {
		return fmt.Sprintf("git exited %d", exitCode)
	}
	lines = lines[:min(len(lines), 4)]
	for i, line := range lines {
		lines[i] = strings.TrimPrefix(strings.TrimPrefix(line, "fatal: "), "error: ")
	}
	return strings.Join(lines, "\n")
}

// shownText is what a terminal would end up showing of line: git redraws
// progress in place with "\r" (and ESC[K, clear to end of line), so only
// the text after the last "\r" stays.
func shownText(line string) string {
	line = strings.TrimSpace(strings.ReplaceAll(line, "\x1b[K", ""))
	if i := strings.LastIndexByte(line, '\r'); i >= 0 {
		line = strings.TrimSpace(line[i+1:])
	}
	return line
}

func isGitReason(line string) bool {
	return strings.HasPrefix(line, "fatal: ") || strings.HasPrefix(line, "error: ")
}

// gitDetails is the text for GitStderr: git's stderr, or stdout when
// stderr is empty — some refusals (rebase --continue with an unresolved
// file, and merge/cherry-pick conflicts in general) print their message on
// stdout instead.
func gitDetails(result Result) string {
	if strings.TrimSpace(string(result.Stderr)) != "" {
		return string(result.Stderr)
	}
	return string(result.Stdout)
}

// buildEnv is the user's own environment unchanged, plus scripting-safety
// variables. Never sets LC_ALL, HOME, GIT_CONFIG_GLOBAL, or
// GIT_CONFIG_SYSTEM: the user's hooks and config apply as inherited.
func buildEnv(call Call) []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_MESSAGES=C", "LANGUAGE=C")
	if call.ReadOnly {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	if call.NoEditor {
		env = append(env, "GIT_EDITOR=true")
	}
	return append(env, call.Env...)
}

// formatCommand renders args for the app log, with credentials masked
// and a stdin marker if hasStdin. Display only: never re-parsed or
// re-executed.
func formatCommand(args []string, hasStdin bool) string {
	quoted := make([]string, len(args))
	for i, a := range args {
		quoted[i] = quoteArg(MaskCredentials(a))
	}
	command := "git " + strings.Join(quoted, " ")
	if hasStdin {
		command += " < stdin"
	}
	return command
}

// quoteArg wraps a in double quotes if it contains whitespace.
func quoteArg(a string) string {
	if strings.ContainsAny(a, " \t\n") {
		return `"` + a + `"`
	}
	return a
}
