package git

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
)

// fakegit is testdata/fakegit, built once for every test in this package.
var fakegit string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "fakegit")
	if err != nil {
		panic(err)
	}
	fakegit = filepath.Join(dir, "fakegit")
	build := exec.Command("go", "build", "-o", fakegit, "./testdata/fakegit")
	build.Env = append(os.Environ(), "CGO_ENABLED=0")
	if out, err := build.CombinedOutput(); err != nil {
		panic(string(out) + err.Error())
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

func TestTR_09_DefaultEnvHasNoLocksOrEditorOverride(t *testing.T) {
	t.Setenv("NEBEGIT_TEST_MARKER", "present")
	unsetenvForTest(t, "GIT_EDITOR") // the sandbox itself sets this; isolate from it
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
// afterward. Some environments (this sandbox included) set GIT_EDITOR
// globally; the TR-09 tests need a clean baseline to prove what buildEnv
// itself adds.
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
	assertCode(t, err, core.GitFailed)
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

	assertCode(t, err, core.Cancelled)
	if elapsed > time.Second {
		t.Errorf("Run() took %s, want well under 5s", elapsed)
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
			assertCode(t, err, tt.want)
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

func assertCode(t *testing.T, err error, want core.Code) {
	t.Helper()
	var gitErr *core.Error
	if !errors.As(err, &gitErr) || gitErr.Code != want {
		t.Errorf("err = %v, want *core.Error{Code: %s}", err, want)
	}
}
