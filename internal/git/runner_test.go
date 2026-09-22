package git

import (
	"context"
	"os"
	"path/filepath"
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

func TestTR_09_ReadOnlyAddsOptionalLocks(t *testing.T) {
	env := runEnvDump(t, Call{ReadOnly: true})
	assertEnv(t, env, "GIT_OPTIONAL_LOCKS", "0")
}

func TestTR_09_ContinueAddsGitEditor(t *testing.T) {
	env := runEnvDump(t, Call{Continue: true})
	assertEnv(t, env, "GIT_EDITOR", "true")
}

// unsetenvForTest clears key for the duration of the test, restoring it
// afterward. Some environments set GIT_EDITOR globally; the env tests need a
// clean baseline to prove what buildEnv itself adds.
func unsetenvForTest(t *testing.T, key string) {
	t.Helper()
	orig, had := os.LookupEnv(key)
	if err := os.Unsetenv(key); err != nil {
		t.Fatal(err)
	}
	if had {
		t.Cleanup(func() { _ = os.Setenv(key, orig) })
	}
}

func runEnvDump(t *testing.T, call Call) map[string]string {
	t.Helper()
	call.Dir = t.TempDir()
	call.Args = []string{"env"}
	result, err := (Runner{Git: fakegit}).Run(context.Background(), call)
	if err != nil {
		t.Fatalf("Run() err = %v", err)
	}
	return parseEnvDump(result.Stdout)
}

func parseEnvDump(out []byte) map[string]string {
	env := map[string]string{}
	for _, line := range strings.Split(strings.TrimRight(string(out), "\n"), "\n") {
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
	result, err := (Runner{Git: fakegit}).Run(context.Background(), Call{
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
	result, err := (Runner{Git: fakegit}).Run(context.Background(), Call{Args: []string{"whatever"}})
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
	_, err := (Runner{Git: fakegit}).Run(context.Background(), Call{Args: []string{"whatever"}})
	coretest.AssertCode(t, err, core.GitFailed)
}

func TestTR_11_CancelKillsAndReturnsQuickly(t *testing.T) {
	t.Setenv("FAKEGIT_SLEEP_MS", "5000")
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

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
	_, err := (Runner{Git: fakegit}).Run(context.Background(), Call{Args: []string{"whatever"}})
	gitErr := coretest.AsError(t, err, core.GitFailed)
	if gitErr.GitStderr != "conflict.txt: needs merge\n" {
		t.Errorf("GitStderr = %q, want the stdout fallback", gitErr.GitStderr)
	}
}

func TestFR_UX_01_GitStderrPrefersStderrOverStdoutWhenBothArePresent(t *testing.T) {
	t.Setenv("FAKEGIT_STDOUT", "on stdout")
	t.Setenv("FAKEGIT_STDERR", "on stderr")
	t.Setenv("FAKEGIT_EXIT", "1")
	_, err := (Runner{Git: fakegit}).Run(context.Background(), Call{Args: []string{"whatever"}})
	gitErr := coretest.AsError(t, err, core.GitFailed)
	if gitErr.GitStderr != "on stderr" {
		t.Errorf("GitStderr = %q, want stderr, not the stdout fallback", gitErr.GitStderr)
	}
}

func TestQA_REL_01_NeverPanics(t *testing.T) {
	cancelled, cancel := context.WithCancel(context.Background())
	cancel()

	tests := map[string]struct {
		runner Runner
		ctx    context.Context
		call   Call
		want   core.Code
	}{
		"nonexistent dir": {Runner{Git: fakegit}, context.Background(), Call{Dir: filepath.Join(t.TempDir(), "missing")}, core.Internal},
		"nonexistent git": {Runner{Git: "/does/not/exist"}, context.Background(), Call{}, core.Internal},
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

	if _, err := r.Run(context.Background(), Call{Args: []string{"whatever"}}); err != nil {
		t.Fatal(err)
	}

	t.Setenv("FAKEGIT_EXIT", "1")
	_, _ = r.Run(context.Background(), Call{Args: []string{"whatever"}})

	t.Setenv("FAKEGIT_EXIT", "0")
	_, _ = r.Run(context.Background(), Call{Args: []string{"whatever"}, Background: true})
	_, _ = r.Run(context.Background(), Call{Args: []string{"clone", "https://alice:s3cr3t@example.com/x.git"}})

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
	t.Setenv("FAKEGIT_SLEEP_MS", "5000")
	heartbeat := filepath.Join(t.TempDir(), "heartbeat")
	t.Setenv("FAKEGIT_HEARTBEAT", heartbeat)

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()
	_, _ = (Runner{Git: fakegit}).Run(ctx, Call{Args: []string{"whatever"}})

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
