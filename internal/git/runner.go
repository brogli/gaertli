// Package git executes the system git as a subprocess (ADR-1).
package git

import (
	"bytes"
	"context"
	"net/url"
	"os"
	"os/exec"
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

	ReadOnly   bool // adds GIT_OPTIONAL_LOCKS=0 (TR-09): the call performs no writes
	Continue   bool // adds GIT_EDITOR=true (TR-09): a --continue/--abort/--skip call
	Background bool // hidden by default in the command log (FR-UX-02)

	// Timeout bounds this call when ctx has no deadline. Zero means 30s.
	Timeout time.Duration
}

// Result is what a call produced, populated whether or not err is nil.
type Result struct {
	Stdout   []byte
	Stderr   []byte
	ExitCode int
}

// Entry is one command-log row (FR-UX-02). Never carries environment
// variables; a URL with embedded credentials in Command is masked.
type Entry struct {
	Command    string // "git " + args, masked
	Dir        string
	Background bool
	Duration   time.Duration
	Result     Result
	Err        error // the *core.Error Run returned, or nil
}

// Runner executes the system git as a subprocess (ADR-1). The zero value
// runs "git" resolved via PATH with no log.
type Runner struct {
	Git string      // git executable; "" resolves "git" via exec.LookPath
	Log func(Entry) // called once per Run, success or failure; nil is fine
}

// defaultTimeout bounds a call whose ctx has no deadline and whose
// Call.Timeout is zero.
const defaultTimeout = 30 * time.Second

// Run executes call and waits for it to finish, ctx to be done, or the
// effective timeout to expire, whichever comes first.
//
// err is nil on exit 0. Otherwise err is *core.Error:
//   - ctx already done, or done before/while git runs: Code Cancelled
//   - git started and exited non-zero: Code GitFailed
//   - git could not be started (LookPath failure, bad Dir, ...): Code Internal
func (r Runner) Run(ctx context.Context, call Call) (Result, error) {
	cmd, effectiveCtx, cancel, command, err := r.prepare(ctx, call)
	if err != nil {
		result := Result{ExitCode: -1}
		r.log(call, command, result, 0, err)
		return result, err
	}
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	runErr := cmd.Run()
	duration := time.Since(start)

	result := Result{Stdout: stdout.Bytes(), Stderr: stderr.Bytes(), ExitCode: cmd.ProcessState.ExitCode()}
	err = classify(effectiveCtx, cmd, runErr, result, command)
	r.log(call, command, result, duration, err)
	return result, err
}

// prepare builds the *exec.Cmd for call, shared by Run and Stream. cmd is
// nil and err is the Cancelled error to return at once if ctx is already
// done.
func (r Runner) prepare(ctx context.Context, call Call) (cmd *exec.Cmd, effectiveCtx context.Context, cancel context.CancelFunc, command string, err error) {
	command = formatCommand(call.Args, call.Stdin != nil)
	if ctx.Err() != nil {
		return nil, ctx, func() {}, command, core.Errorf(core.Cancelled, "%s", ctx.Err())
	}

	effectiveCtx, cancel = withTimeout(ctx, call.Timeout)

	git := r.Git
	if git == "" {
		git = "git"
	}
	cmd = exec.CommandContext(effectiveCtx, git, call.Args...)
	cmd.Dir = call.Dir
	cmd.Env = buildEnv(call)
	cmd.SysProcAttr = proc.HiddenConsole()
	cmd.WaitDelay = 2 * time.Second
	if call.Stdin != nil {
		cmd.Stdin = bytes.NewReader(call.Stdin)
	}
	return cmd, effectiveCtx, cancel, command, nil
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
// else is a plain exit-code failure (TR-08 — never inspects Stderr text).
func classify(ctx context.Context, cmd *exec.Cmd, err error, result Result, command string) error {
	switch {
	case ctx.Err() != nil:
		return core.Errorf(core.Cancelled, "%s", ctx.Err())
	case cmd.ProcessState == nil:
		return core.Errorf(core.Internal, "%v", err)
	case err != nil:
		gitErr := core.Errorf(core.GitFailed, "git exited %d", result.ExitCode)
		gitErr.GitStderr = string(result.Stderr)
		gitErr.Command = command
		return gitErr
	default:
		return nil
	}
}

// buildEnv is the child's environment (TR-09): the user's own environment
// unchanged, plus scripting-safety variables. Never LC_ALL, HOME,
// GIT_CONFIG_GLOBAL, or GIT_CONFIG_SYSTEM (QA-COMP-01, QA-SAFE-03) — those
// come from os.Environ() as inherited.
func buildEnv(call Call) []string {
	env := append(os.Environ(), "GIT_TERMINAL_PROMPT=0", "LC_MESSAGES=C", "LANGUAGE=C")
	if call.ReadOnly {
		env = append(env, "GIT_OPTIONAL_LOCKS=0")
	}
	if call.Continue {
		env = append(env, "GIT_EDITOR=true")
	}
	return env
}

// formatCommand renders args for the command log (FR-UX-02), with
// credentials masked and a stdin marker if hasStdin. Display only: never
// re-parsed or re-executed.
func formatCommand(args []string, hasStdin bool) string {
	quoted := make([]string, len(args))
	for i, a := range maskCredentials(args) {
		quoted[i] = quoteArg(a)
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

// maskCredentials replaces the userinfo of every arg that parses as an
// absolute URL with a non-nil User with "***".
func maskCredentials(args []string) []string {
	masked := make([]string, len(args))
	for i, a := range args {
		masked[i] = maskCredential(a)
	}
	return masked
}

func maskCredential(a string) string {
	u, err := url.Parse(a)
	if err != nil || !u.IsAbs() || u.User == nil {
		return a
	}
	// String manipulation, not u.String(): URL escapes "*" as "%2A".
	scheme := len(u.Scheme) + len("://")
	userinfo := strings.Index(a[scheme:], "@")
	return a[:scheme] + "***" + a[scheme+userinfo:]
}
