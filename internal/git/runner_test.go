package git

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/testbin"
)

// fakegit is testdata/fakegit, built once for every test in this package.
var fakegit string

func TestMain(m *testing.M) {
	testbin.Main(m, testbin.Bin{Path: &fakegit, Pkg: "./testdata/fakegit"})
}

func TestTR_09_DefaultEnvHasNoLocksOrEditorOverride(t *testing.T) {
	t.Setenv("NEBEGIT_TEST_MARKER", "present")
	unsetenvForTest(t, "GIT_EDITOR")
	env := runEnvDump(t, Call{})
	assertEnv(t, env, "GIT_TERMINAL_PROMPT", "0")
	assertEnv(t, env, "LC_MESSAGES", "C")
	assertEnv(t, env, "LANGUAGE", "C")
	assertEnv(t, env, "NEBEGIT_TEST_MARKER", "present")
	assertEnvAbsent(t, env, "GIT_OPTIONAL_LOCKS")
	assertEnvAbsent(t, env, "GIT_EDITOR")
	assertEnvAbsent(t, env, "LC_ALL")
}

// A git started from a terminal must not reach it: a prompt on /dev/tty
// fails at once instead of stopping git in a background process group.
func TestTR_09_GitRunsInASessionOfItsOwnWithoutATerminal(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	result, err := (Runner{Git: "sh"}).Run(t.Context(), Call{Args: []string{"-c", `echo $$; cut -d' ' -f6 /proc/$$/stat`}})
	if err != nil {
		t.Fatal(err)
	}

	pid, session, _ := strings.Cut(strings.TrimSpace(string(result.Stdout)), "\n")
	if session != pid {
		t.Errorf("session = %s, want git's own (%s)", session, pid)
	}
}

func TestTR_09_ReadOnlyAddsOptionalLocks(t *testing.T) {
	env := runEnvDump(t, Call{ReadOnly: true})
	assertEnv(t, env, "GIT_OPTIONAL_LOCKS", "0")
}

func TestTR_09_NoEditorAddsGitEditor(t *testing.T) {
	env := runEnvDump(t, Call{NoEditor: true})
	assertEnv(t, env, "GIT_EDITOR", "true")
}

func TestTR_09_EnvAddsTheCallsOwnVariables(t *testing.T) {
	env := runEnvDump(t, Call{Env: []string{"GIT_SEQUENCE_EDITOR=:", "NEBEGIT_TODO=pick 1"}})
	assertEnv(t, env, "GIT_SEQUENCE_EDITOR", ":")
	assertEnv(t, env, "NEBEGIT_TODO", "pick 1")
}

// unsetenvForTest clears key for the duration of the test, restoring it
// afterward. Some environments set GIT_EDITOR globally; the env tests need a
// clean baseline to prove what buildEnv itself adds.
func unsetenvForTest(t *testing.T, key string) {
	t.Helper()
	t.Setenv(key, "") // restores the original value at cleanup
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
}

func runEnvDump(t *testing.T, call Call) map[string]string {
	t.Helper()
	call.Dir = t.TempDir()
	call.Args = []string{"env"}
	result, err := (Runner{Git: fakegit}).Run(t.Context(), call)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	return parseEnvDump(result.Stdout)
}

func parseEnvDump(out []byte) map[string]string {
	env := map[string]string{}
	for line := range strings.SplitSeq(strings.TrimRight(string(out), "\n"), "\n") {
		if k, v, ok := strings.Cut(line, "="); ok {
			env[k] = v
		}
	}
	return env
}

func assertEnv(t *testing.T, env map[string]string, key, want string) {
	t.Helper()
	if got := env[key]; got != want {
		t.Errorf("%s = %q, want %q", key, got, want)
	}
}

func assertEnvAbsent(t *testing.T, env map[string]string, key string) {
	t.Helper()
	if v, ok := env[key]; ok {
		t.Errorf("%s = %q, want absent", key, v)
	}
}

func TestTR_10_ArgvPassedVerbatimNoShell(t *testing.T) {
	dangerous := []string{"; touch marker;", "`id`", "with spaces"}
	dir := t.TempDir()
	result, err := (Runner{Git: fakegit}).Run(t.Context(), Call{
		Dir:  dir,
		Args: append([]string{"argv"}, dangerous...),
	})
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}

	got := strings.Split(strings.TrimRight(string(result.Stdout), "\n"), "\n")
	if !slices.Equal(got, dangerous) {
		t.Errorf("argv = %q, want %q", got, dangerous)
	}
	if _, err := os.Stat(filepath.Join(dir, "marker")); !os.IsNotExist(err) {
		t.Error("shell metacharacters were interpreted: marker file exists")
	}
}

func TestTR_08_ExitZeroSucceedsDespiteStderr(t *testing.T) {
	t.Setenv("FAKEGIT_STDERR", "fatal: looks bad")
	result, err := (Runner{Git: fakegit}).Run(t.Context(), Call{Args: []string{"whatever"}})
	if err != nil {
		t.Fatalf("Run() err = %v, want nil", err)
	}
	if string(result.Stderr) != "fatal: looks bad" {
		t.Errorf("Stderr = %q", result.Stderr)
	}
}

func TestTR_08_ExitNonZeroFailsDespiteBenignStdout(t *testing.T) {
	t.Setenv("FAKEGIT_EXIT", "1")
	t.Setenv("FAKEGIT_STDOUT", "all fine")
	_, err := (Runner{Git: fakegit}).Run(t.Context(), Call{Args: []string{"whatever"}})
	coretest.AssertCode(t, err, core.GitFailed)
}

func TestTR_11_CancelKillsAndReturnsQuickly(t *testing.T) {
	t.Setenv("FAKEGIT_SLEEP_MS", "5000")
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	start := time.Now()
	_, err := (Runner{Git: fakegit}).Run(ctx, Call{Args: []string{"whatever"}})
	elapsed := time.Since(start)

	coretest.AssertCode(t, err, core.Cancelled)
	if elapsed > time.Second {
		t.Errorf("Run() took %s, want well under 5s", elapsed)
	}
}

func TestFR_UX_01_GitStderrFallsBackToStdoutWhenStderrIsEmpty(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "conflict.txt: needs merge\n")
	t.Setenv("FAKEGIT_EXIT", "1")
	_, err := (Runner{Git: fakegit}).Run(t.Context(), Call{Args: []string{"whatever"}})
	gitErr := coretest.AsError(t, err, core.GitFailed)
	if gitErr.GitStderr != "conflict.txt: needs merge\n" {
		t.Errorf("GitStderr = %q, want the stdout fallback", gitErr.GitStderr)
	}
}

func TestFR_UX_01_AGitFailuresMessageIsGitsOwn(t *testing.T) {
	cases := []struct{ name, stderr, want string }{
		{"prefix stripped", "fatal: a branch named 'main' already exists\n", "a branch named 'main' already exists"},
		{
			"hints dropped, following lines kept",
			"error: Your local changes to the following files would be overwritten by checkout:\n\tconflict.txt\nhint: stash them\nAborting\n",
			"Your local changes to the following files would be overwritten by checkout:\nconflict.txt\nAborting",
		},
		{"at most four lines", "a\nb\nc\nd\ne\n", "a\nb\nc\nd"},
		{"leads with the fatal line, not progress before it", "Preparing worktree (new branch 'x')\nfatal: '/t' already exists\n", "'/t' already exists"},
		{"no output: the exit code", "", "git exited 3"},
		{
			"progress redrawn in place with \\r and ESC[K: only its last state",
			"Rebasing (2/5)\rRebasing (3/5)\r\x1b[Kerror: untracked files would be overwritten:\n\tz.txt\nhint: move them\n",
			"untracked files would be overwritten:\nz.txt",
		},
		{
			"a line after progress on the same line keeps its own text",
			"Rebasing (4/6)\x1b[K\rExecuting: git commit --amend\nrefusing\nwarning: execution failed\n",
			"Executing: git commit --amend\nrefusing\nwarning: execution failed",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("FAKEGIT_STDERR", c.stderr)
			t.Setenv("FAKEGIT_EXIT", "3")
			_, err := (Runner{Git: fakegit}).Run(t.Context(), Call{Args: []string{"whatever"}})
			if got := coretest.AsError(t, err, core.GitFailed).Message; got != c.want {
				t.Errorf("Message = %q, want %q", got, c.want)
			}
		})
	}
}

func TestFR_UX_01_GitStderrPrefersStderrOverStdoutWhenBothArePresent(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "on stdout")
	t.Setenv("FAKEGIT_STDERR", "on stderr")
	t.Setenv("FAKEGIT_EXIT", "1")
	_, err := (Runner{Git: fakegit}).Run(t.Context(), Call{Args: []string{"whatever"}})
	gitErr := coretest.AsError(t, err, core.GitFailed)
	if gitErr.GitStderr != "on stderr" {
		t.Errorf("GitStderr = %q, want stderr, not the stdout fallback", gitErr.GitStderr)
	}
}

func TestQA_REL_01_NeverPanics(t *testing.T) {
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()

	tests := map[string]struct {
		runner Runner
		ctx    context.Context
		call   Call
		want   core.Code
	}{
		"nonexistent dir": {Runner{Git: fakegit}, t.Context(), Call{Dir: filepath.Join(t.TempDir(), "missing")}, core.Internal},
		"nonexistent git": {Runner{Git: "/does/not/exist"}, t.Context(), Call{}, core.Internal},
		"cancelled ctx":   {Runner{Git: fakegit}, cancelled, Call{}, core.Cancelled},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			_, err := tt.runner.Run(tt.ctx, tt.call)
			coretest.AssertCode(t, err, tt.want)
		})
	}
}

func TestFR_UX_02_LogsEveryCall(t *testing.T) {
	var entries []Entry
	r := Runner{Git: fakegit, Log: func(e Entry) { entries = append(entries, e) }}

	if _, err := r.Run(t.Context(), Call{Args: []string{"whatever"}}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FAKEGIT_EXIT", "1")
	_, _ = r.Run(t.Context(), Call{Args: []string{"whatever"}})

	t.Setenv("FAKEGIT_EXIT", "0")
	_, _ = r.Run(t.Context(), Call{Args: []string{"whatever"}, Background: true})
	_, _ = r.Run(t.Context(), Call{Args: []string{"clone", "https://alice:s3cr3t@example.com/x.git"}})

	if len(entries) != 4 {
		t.Fatalf("got %d entries, want 4", len(entries))
	}
	if entries[0].Err != nil {
		t.Errorf("entries[0].Err = %v, want nil", entries[0].Err)
	}
	if entries[1].Err == nil {
		t.Error("entries[1].Err = nil, want the GitFailed error")
	}
	if !entries[2].Background {
		t.Error("entries[2].Background = false, want true")
	}
	if strings.Contains(entries[3].Command, "s3cr3t") {
		t.Errorf("Command = %q, leaks the credential", entries[3].Command)
	}
	if !strings.Contains(entries[3].Command, "https://***@example.com/x.git") {
		t.Errorf("Command = %q, want the masked URL", entries[3].Command)
	}
}

func TestQA_SAFE_02_CancelActuallyKillsProcess(t *testing.T) {
	heartbeat := sleepWithHeartbeat(t)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(50*time.Millisecond, cancel)

	_, _ = (Runner{Git: fakegit}).Run(ctx, Call{Args: []string{"whatever"}})

	assertStopped(t, heartbeat)
}

// git removes its lock files when interrupted, not when killed.
func TestQA_SAFE_04_CancelInterruptsGitBeforeKillingIt(t *testing.T) {
	skipWithoutPOSIXSignals(t)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)

	result, err := (Runner{Git: "sh"}).Run(ctx, Call{Args: []string{"-c", `trap 'echo interrupted >&2; exit 1' TERM; sleep 10 & wait`}})

	coretest.AssertCode(t, err, core.Cancelled)
	if !strings.Contains(string(result.Stderr), "interrupted") {
		t.Errorf("stderr = %q, want the interrupt's trap to have run", result.Stderr)
	}
}

func TestQA_SAFE_04_AGitIgnoringTheInterruptIsKilledAfterTheGracePeriod(t *testing.T) {
	skipWithoutPOSIXSignals(t)
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(100*time.Millisecond, cancel)

	start := time.Now()
	_, err := (Runner{Git: "sh"}).Run(ctx, Call{Args: []string{"-c", `trap '' TERM; exec sleep 10`}})
	elapsed := time.Since(start)

	coretest.AssertCode(t, err, core.Cancelled)
	if elapsed > 5*time.Second {
		t.Errorf("Run() took %s, want it back soon after the 2s grace period", elapsed)
	}
}

// The kill after the grace period reaches git alone; a hook or transport
// that ignores the interrupt must not outlive the cancel.
func TestQA_SAFE_04_CancelKillsAChildOfGitThatIgnoresTheInterrupt(t *testing.T) {
	dir := t.TempDir()

	err := runCancelledAfter(t, 100*time.Millisecond,
		`(trap '' TERM; exec sleep 20) >/dev/null 2>&1 & echo $! >`+dir+`/pid; wait`)

	coretest.AssertCode(t, err, core.Cancelled)
	pid := readTrimmed(t, dir+"/pid")
	if !within(4*time.Second, func() bool { return processGone(pid) }) {
		t.Fatal("the child that ignored the interrupt still runs after the grace period")
	}
}

// A hook or transport that ends its own way on the interrupt, after its
// output left git's pipes, gets the grace period to do so.
func TestQA_SAFE_04_AChildOfGitGetsTheGracePeriodToCleanUp(t *testing.T) {
	dir := t.TempDir()

	err := runCancelledAfter(t, 200*time.Millisecond,
		`(exec >/dev/null 2>&1; trap 'sleep 0.3; echo x >`+dir+`/cleaned; exit 0' TERM; sleep 30 & wait) & wait`)

	coretest.AssertCode(t, err, core.Cancelled)
	if !within(2*time.Second, func() bool { _, err := os.Stat(dir + "/cleaned"); return err == nil }) {
		t.Fatal("the child was killed before its clean-up ended")
	}
}

// git that ended before the cancel was never interrupted, so nothing of
// its group is killed either, though a child still held its output.
func TestQA_SAFE_04_NothingIsKilledWhenGitEndedBeforeTheCancel(t *testing.T) {
	dir := t.TempDir()

	err := runCancelledAfter(t, 300*time.Millisecond, `(sleep 5 & echo $! >`+dir+`/pid; wait) &`)

	coretest.AssertCode(t, err, core.Cancelled)
	pid := readTrimmed(t, dir+"/pid")
	t.Cleanup(func() { _ = exec.Command("kill", pid).Run() })
	time.Sleep(time.Second) // Run returned when the pipes closed, 2s in: now past cancel plus the grace period
	if processGone(pid) {
		t.Error("the child of a git that ended before the cancel was killed")
	}
}

// runCancelledAfter runs script through sh as the git of a call whose ctx
// ends after delay.
func runCancelledAfter(t *testing.T, delay time.Duration, script string) error {
	t.Helper()
	if runtime.GOOS != "linux" {
		t.Skip("reads /proc")
	}
	ctx, cancel := context.WithCancel(t.Context())
	time.AfterFunc(delay, cancel)
	_, err := (Runner{Git: "sh"}).Run(ctx, Call{Args: []string{"-c", script}})
	return err
}

func readTrimmed(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.TrimSpace(string(data))
}

// within polls done until it holds or timeout passes.
func within(timeout time.Duration, done func() bool) bool {
	deadline := time.Now().Add(timeout)
	for !done() {
		if time.Now().After(deadline) {
			return false
		}
		time.Sleep(20 * time.Millisecond)
	}
	return true
}

// processGone reports whether pid has exited: no longer listed, or a
// zombie its new parent has not reaped yet.
func processGone(pid string) bool {
	stat, err := os.ReadFile("/proc/" + pid + "/stat")
	if err != nil {
		return true
	}
	_, rest, _ := strings.Cut(string(stat), ") ")
	return strings.HasPrefix(rest, "Z")
}

func skipWithoutPOSIXSignals(t *testing.T) {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("needs a POSIX shell and signals")
	}
}

// sleepWithHeartbeat makes fakegit sleep 5s while touching the returned
// file every 50ms.
func sleepWithHeartbeat(t *testing.T) (heartbeat string) {
	t.Helper()
	heartbeat = filepath.Join(t.TempDir(), "heartbeat")
	t.Setenv("FAKEGIT_SLEEP_MS", "5000")
	t.Setenv("FAKEGIT_HEARTBEAT", heartbeat)
	return heartbeat
}

// assertStopped fails if heartbeat is still being touched, i.e. fakegit
// was not killed.
func assertStopped(t *testing.T, heartbeat string) {
	t.Helper()
	stopped := mtime(t, heartbeat)
	time.Sleep(300 * time.Millisecond)
	if got := mtime(t, heartbeat); !got.Equal(stopped) {
		t.Errorf("heartbeat mtime advanced after cancel: %v -> %v (process not killed)", stopped, got)
	}
}

func mtime(t *testing.T, path string) time.Time {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return info.ModTime()
}
