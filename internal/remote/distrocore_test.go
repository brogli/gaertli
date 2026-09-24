package remote_test

import (
	"context"
	"errors"
	"log/slog"
	"os/exec"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// coreBin is the real nebegit-core; badBin reports a wrong protocol version;
// stderrBin also writes to stderr, including a line with a credential.
var coreBin, badBin, stderrBin string

func TestMain(m *testing.M) {
	testbin.Main(m,
		testbin.Bin{Path: &coreBin, Pkg: "../../cmd/nebegit-core"},
		testbin.Bin{Path: &badBin, Pkg: "./testdata/badcore"},
		testbin.Bin{Path: &stderrBin, Pkg: "./testdata/stderrcore"},
	)
}

// syncBuffer is a strings.Builder safe for a background goroutine's writes
// (the stderr reader) racing a test's own reads.
type syncBuffer struct {
	mu  sync.Mutex
	buf strings.Builder
}

func (b *syncBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *syncBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// bufLogger is a *slog.Logger writing plain text to a syncBuffer, for a
// test to inspect what DistroCore logged.
func bufLogger() (logger *slog.Logger, text func() string) {
	buf := &syncBuffer{}
	return slog.New(slog.NewTextHandler(buf, nil)), buf.String
}

// acceptConsent always accepts; deploy_test.go covers consent behaviour.
type acceptConsent struct{}

func (acceptConsent) Ask(string, string) bool { return true }

// fakeWSL intercepts the "sh -c '<target> --stdio'" spawn call to run bin
// locally instead of a real wsl.exe child; every other call (deploy's
// stat/cat dance) runs the given shell command for real against a
// test-sandboxed $HOME. It counts every call, and keeps each spawn cmd so a
// test can kill the live process without DistroCore exposing it.
type fakeWSL struct {
	bin    string
	calls  int
	spawns []*exec.Cmd
}

func (f *fakeWSL) run(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	f.calls++
	if len(args) == 3 && strings.HasSuffix(args[2], "--stdio") {
		cmd := exec.CommandContext(ctx, f.bin, "--stdio")
		f.spawns = append(f.spawns, cmd)
		return cmd, nil
	}
	return exec.CommandContext(ctx, args[0], args[1:]...), nil
}

// newDistroCore sandboxes $HOME and builds Ubuntu-22.04's DistroCore, whose
// core process is bin.
func newDistroCore(t *testing.T, bin string, log *slog.Logger) (*remote.DistroCore, *fakeWSL) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	wsl := &fakeWSL{bin: bin}
	return remote.NewDistroCore("Ubuntu-22.04", "1.0.0", wsl.run, acceptConsent{}, nil, log), wsl
}

func TestADR_3_EnsureSpawnsOnceThenReusesTheLiveClient(t *testing.T) {
	dc, wsl := newDistroCore(t, coreBin, slog.Default())

	client1, respawned1, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !respawned1 {
		t.Error("first Ensure: respawned = false, want true")
	}
	callsAfterFirst := wsl.calls

	client2, respawned2, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if respawned2 {
		t.Error("second Ensure: respawned = true, want false")
	}
	if client2 != client1 {
		t.Error("second Ensure returned a different client instance")
	}
	if wsl.calls != callsAfterFirst {
		t.Errorf("wslRunner called %d more times on the second Ensure, want 0", wsl.calls-callsAfterFirst)
	}
}

func TestADR_3_EnsureForcesARedeployOnAProtocolMismatch(t *testing.T) {
	dc, _ := newDistroCore(t, badBin, slog.Default())

	_, _, err := dc.Ensure(t.Context())

	coretest.AssertCode(t, err, core.Internal)
}

func TestADR_3_OpeningWSLWorktreeTranslatesTheUNCPath(t *testing.T) {
	repo := gittest.New(t).Dir
	dc, _ := newDistroCore(t, coreBin, slog.Default())
	client, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	uncPath := remote.BuildWSLPath("Ubuntu-22.04", repo)
	info, err := remote.NewDistroPathClient(client, "Ubuntu-22.04").OpenWorktree(t.Context(), core.RepoPath(uncPath))
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != repo {
		t.Errorf("OpenWorktree translated path = %q, want %q (not the UNC string)", info.Path, repo)
	}
	if info.ID != core.WorktreeID(uncPath) {
		t.Errorf("ID = %q, want the UNC form %q", info.ID, uncPath)
	}
	if want := repo + "/.git"; info.RepoKey != want {
		t.Errorf("RepoKey = %q, want the POSIX %q", info.RepoKey, want)
	}
}

func TestADR_4_EnsureRespawnsAndTheOldSessionIsGone(t *testing.T) {
	repo := gittest.New(t).Dir
	dc, wsl := newDistroCore(t, coreBin, slog.Default())

	client1, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	uncPath := remote.BuildWSLPath("Ubuntu-22.04", repo)
	info, err := remote.NewDistroPathClient(client1, "Ubuntu-22.04").OpenWorktree(t.Context(), core.RepoPath(uncPath))
	if err != nil {
		t.Fatal(err)
	}

	// kill the underlying process, as `wsl --shutdown` would
	if err := wsl.spawns[len(wsl.spawns)-1].Process.Kill(); err != nil {
		t.Fatal(err)
	}

	client2 := waitForRespawn(t, dc)
	if client2 == client1 {
		t.Fatal("Ensure after the kill returned the same client instance")
	}

	_, err = client2.Summary(t.Context(), core.WorktreeID(info.Path))
	if _, ok := errors.AsType[*core.Error](err); !ok {
		t.Fatalf("bare call on the fresh client err = %v, want a *core.Error (unknown worktree)", err)
	}
}

func TestTR_13_OnConnectFiresOnceForAFreshConnectNotOnAReusedClient(t *testing.T) {
	dc, _ := newDistroCore(t, coreBin, slog.Default())
	var got []*remote.Client
	dc.OnConnect(func(_ context.Context, c *remote.Client) { got = append(got, c) })

	client1, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != client1 {
		t.Fatalf("onConnect calls = %d, want 1 with the fresh client", len(got))
	}

	client2, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if client2 != client1 {
		t.Fatal("second Ensure returned a different client")
	}
	if len(got) != 1 {
		t.Errorf("onConnect calls after a reused client = %d, want still 1", len(got))
	}
}

func TestADR_4_LiveClientReportsTheCurrentConnectionState(t *testing.T) {
	dc, _ := newDistroCore(t, coreBin, slog.Default())

	if _, ok := dc.LiveClient(); ok {
		t.Fatal("LiveClient before any Ensure reports true")
	}

	client, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	got, ok := dc.LiveClient()
	if !ok || got != client {
		t.Errorf("LiveClient = (%v, %v), want (%v, true)", got, ok, client)
	}
}

// repoPathMethods finds every core.API method with a core.RepoPath
// parameter by reflection, so a method added to the interface without a
// matching case in distroPathClient fails the tests below instead of going
// unnoticed.
func repoPathMethods() []reflect.Method {
	apiType := reflect.TypeFor[core.API]()
	repoPathType := reflect.TypeFor[core.RepoPath]()

	var methods []reflect.Method
	for m := range apiType.Methods() {
		for in := range m.Type.Ins() {
			if in == repoPathType {
				methods = append(methods, m)
				break
			}
		}
	}
	return methods
}

// callArgs builds reflect.Call arguments for fnType, m's signature: ctx,
// path for the RepoPath parameter, and, for AddWorktree/RemoveWorktree
// only, target at its fixed position (both take it third); every other
// parameter is left zero.
func callArgs(ctx context.Context, fnType reflect.Type, m string, path, target string) []reflect.Value {
	args := make([]reflect.Value, fnType.NumIn())
	for i := range args {
		switch paramType := fnType.In(i); {
		case paramType == reflect.TypeFor[context.Context]():
			args[i] = reflect.ValueOf(ctx)
		case paramType == reflect.TypeFor[core.RepoPath]():
			args[i] = reflect.ValueOf(core.RepoPath(path))
		case (m == "AddWorktree" || m == "RemoveWorktree") && i == 2:
			args[i] = reflect.ValueOf(target)
		default:
			args[i] = reflect.Zero(paramType)
		}
	}
	return args
}

// lastError reads a Call result's trailing error, core.API's rule for every
// method.
func lastError(results []reflect.Value) error {
	last := results[len(results)-1]
	if last.IsNil() {
		return nil
	}
	return last.Interface().(error)
}

func TestFR_REPO_07_TranslatesEveryRepoPathMethod(t *testing.T) {
	const uncPath = `\\wsl.localhost\Ubuntu-22.04\home\u\r`
	const uncTarget = `\\wsl.localhost\Ubuntu-22.04\home\u\r\wt`

	f := &fake{}
	client := pipeClient(t, f, nil)
	wrapped := reflect.ValueOf(remote.NewDistroPathClient(client, "Ubuntu-22.04"))

	for _, m := range repoPathMethods() {
		t.Run(m.Name, func(t *testing.T) {
			method := wrapped.MethodByName(m.Name)
			if err := lastError(method.Call(callArgs(t.Context(), method.Type(), m.Name, uncPath, uncTarget))); err != nil {
				t.Fatalf("%s(%s) = %v, want success", m.Name, uncPath, err)
			}

			if got := f.paths[m.Name]; got != `/home/u/r` {
				t.Errorf("%s: fake received path %q, want %q", m.Name, got, `/home/u/r`)
			}
			if m.Name == "AddWorktree" || m.Name == "RemoveWorktree" {
				if got := f.paths[m.Name+":target"]; got != `/home/u/r/wt` {
					t.Errorf("%s: fake received target %q, want %q", m.Name, got, `/home/u/r/wt`)
				}
			}
		})
	}
}

func TestFR_REPO_07_RejectsAPathForAnotherDistroOrLocal(t *testing.T) {
	cases := []struct{ name, path string }{
		{"another distro", `\\wsl.localhost\Debian\home\u\r`},
		{"local path", `C:\Users\u\r`},
	}

	f := &fake{}
	client := pipeClient(t, f, nil)
	wrapped := reflect.ValueOf(remote.NewDistroPathClient(client, "Ubuntu-22.04"))

	for _, m := range repoPathMethods() {
		for _, c := range cases {
			t.Run(m.Name+"/"+c.name, func(t *testing.T) {
				method := wrapped.MethodByName(m.Name)
				err := lastError(method.Call(callArgs(t.Context(), method.Type(), m.Name, c.path, c.path)))
				coretest.AssertCode(t, err, core.Internal)

				if _, called := f.paths[m.Name]; called {
					t.Errorf("%s: fake was called with %q, want nothing forwarded", m.Name, c.path)
				}
			})
		}
	}
}

// waitForRespawn polls Ensure until it reports a fresh spawn: the killed
// process's exit is observed by DistroCore's own goroutine asynchronously.
func waitForRespawn(t *testing.T, dc *remote.DistroCore) *remote.Client {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		client, respawned, err := dc.Ensure(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if respawned {
			return client
		}
		time.Sleep(20 * time.Millisecond)
	}
	t.Fatal("Ensure did not respawn within 5s of the process being killed")
	return nil
}

func TestFR_UX_02_EnsureLogsAFailedCoreStartAtWarn(t *testing.T) {
	logger, text := bufLogger()
	dc, _ := newDistroCore(t, badBin, logger)

	_, _, err := dc.Ensure(t.Context())

	coretest.AssertCode(t, err, core.Internal)
	if got := text(); !strings.Contains(got, "level=WARN") || !strings.Contains(got, "distro=Ubuntu-22.04") {
		t.Errorf("log = %q, want a Warn record naming the distro", got)
	}
}

func TestFR_UX_02_EnsureLogsAReconnectAtWarn(t *testing.T) {
	logger, text := bufLogger()
	dc, wsl := newDistroCore(t, coreBin, logger)
	if _, _, err := dc.Ensure(t.Context()); err != nil {
		t.Fatal(err)
	}

	if err := wsl.spawns[len(wsl.spawns)-1].Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitForRespawn(t, dc)

	if got := text(); !strings.Contains(got, "level=WARN") || !strings.Contains(got, "distro=Ubuntu-22.04") {
		t.Errorf("log = %q, want a Warn record for the reconnect naming the distro", got)
	}
}

func TestFR_UX_02_TheCoresStderrReachesTheLogAtWarnMasked(t *testing.T) {
	logger, text := bufLogger()
	dc, _ := newDistroCore(t, stderrBin, logger)

	if _, _, err := dc.Ensure(t.Context()); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(2 * time.Second)
	var got string
	for time.Now().Before(deadline) {
		got = text()
		if strings.Contains(got, "https://***@host/repo") {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, "https://***@host/repo") {
		t.Errorf("log = %q, want the masked stderr line at Warn", got)
	}
	if strings.Contains(got, "secret") {
		t.Errorf("log = %q, want no credential", got)
	}
}
