package remote_test

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/git/gittest"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// coreBin is the real nebegit-core; badBin reports a wrong protocol version;
// stderrBin also writes to stderr, including a line with a credential;
// exitBin writes a burst of events and stderr lines, then exits.
var coreBin, badBin, stderrBin, exitBin string

func TestMain(m *testing.M) {
	gittest.Main(m,
		testbin.Bin{Path: &coreBin, Pkg: "../../cmd/nebegit-core"},
		testbin.Bin{Path: &badBin, Pkg: "./testdata/badcore"},
		testbin.Bin{Path: &stderrBin, Pkg: "./testdata/stderrcore"},
		testbin.Bin{Path: &exitBin, Pkg: "./testdata/exitcore"},
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

func (acceptConsent) Ask(string) bool { return true }

// failConsent fails the test if asked.
type failConsent struct{ t *testing.T }

func (c failConsent) Ask(string) bool {
	c.t.Error("consent asked, want none")
	return false
}

// toggleConsent answers accept, changeable between calls: it tests that a
// declined consent is not remembered and the next open asks again.
type toggleConsent struct{ accept bool }

func (c *toggleConsent) Ask(string) bool { return c.accept }

// guiExe is the Windows app the test distro cores set the command up for.
const guiExe = `C:\nebegit\nebegit.exe`

// fakeWSL intercepts the "sh -c '<target> --stdio'" spawn call to run bin
// locally instead of a real wsl.exe child; every other call (deploy's
// stat/cat dance, the command setup) runs the given shell command for real
// against a test-sandboxed $HOME. It counts every call, records each
// script, and keeps each spawn cmd so a test can kill the live process
// without DistroCore exposing it.
type fakeWSL struct {
	bin     string
	calls   int
	scripts []string
	spawns  []*exec.Cmd
}

func (f *fakeWSL) run(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
	f.calls++
	f.scripts = append(f.scripts, strings.Join(args, " "))
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
	return remote.NewDistroCore("Ubuntu-22.04", "1.0.0", guiExe, wsl.run, acceptConsent{}, nil, log), wsl
}

// commandSetUp reads what the command setup left in $HOME: the link's
// target and the recorded app path.
func commandSetUp(t *testing.T) (link, gui string) {
	t.Helper()
	home := os.Getenv("HOME")
	link, err := os.Readlink(filepath.Join(home, ".local/bin/nebegit"))
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(filepath.Join(home, ".cache/nebegit/1.0.0/gui-path"))
	if err != nil {
		t.Fatal(err)
	}
	return link, string(data)
}

func TestFR_REPO_06_EveryFreshConnectSetsUpTheCommandInOneWSLStart(t *testing.T) {
	dc, wsl := newDistroCore(t, coreBin, slog.Default())
	if _, _, err := dc.Ensure(t.Context()); err != nil {
		t.Fatal(err)
	}

	link, gui := commandSetUp(t)
	if want := filepath.Join(os.Getenv("HOME"), ".cache/nebegit/1.0.0/nebegit-core"); link != want {
		t.Errorf("~/.local/bin/nebegit -> %q, want %q", link, want)
	}
	if want := "/mnt/c/nebegit/nebegit.exe"; gui != want {
		t.Errorf("gui-path = %q, want %q", gui, want)
	}
	setups := slices.DeleteFunc(slices.Clone(wsl.scripts), func(s string) bool { return !strings.Contains(s, "gui-path") })
	if len(setups) != 1 || !strings.Contains(setups[0], "ln -sf") {
		t.Errorf("setup scripts = %q, want one that links and records", setups)
	}

	if err := os.Remove(filepath.Join(os.Getenv("HOME"), ".cache/nebegit/1.0.0/gui-path")); err != nil {
		t.Fatal(err)
	}
	if err := wsl.spawns[len(wsl.spawns)-1].Process.Kill(); err != nil {
		t.Fatal(err)
	}
	waitForRespawn(t, dc)

	if _, gui := commandSetUp(t); gui != "/mnt/c/nebegit/nebegit.exe" {
		t.Errorf("gui-path after a respawn = %q, want it written again", gui)
	}
}

func TestFR_REPO_06_AMovedAppTakesOverTheCommandOnItsConnect(t *testing.T) {
	dc, wsl := newDistroCore(t, coreBin, slog.Default())
	if _, _, err := dc.Ensure(t.Context()); err != nil {
		t.Fatal(err)
	}

	moved := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", `D:\tools\nebegit.exe`, wsl.run, failConsent{t}, nil, slog.Default())
	if _, _, err := moved.Ensure(t.Context()); err != nil {
		t.Fatal(err)
	}

	if _, gui := commandSetUp(t); gui != "/mnt/d/tools/nebegit.exe" {
		t.Errorf("gui-path = %q, want the moved app's", gui)
	}
}

// A declined consent is not remembered: the next open asks again, and
// accepting then deploys, spawns and sets up the command as any other
// first open would.
func TestQA_PORT_02_ADeclinedConsentAsksAgainOnTheNextOpen(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	wsl := &fakeWSL{bin: coreBin}
	consent := &toggleConsent{accept: false}
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", guiExe, wsl.run, consent, nil, slog.Default())

	_, _, err := dc.Ensure(t.Context())
	coretest.AssertCode(t, err, core.DeployRefused)

	consent.accept = true
	client, respawned, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !respawned || client == nil {
		t.Errorf("Ensure after accepting = (%v, %v), want a freshly spawned live client", client, respawned)
	}
	if _, gui := commandSetUp(t); gui == "" {
		t.Error("command not set up after accepting on the second open")
	}
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

	waitFor(func() bool { return strings.Contains(text(), "https://***@host/repo") })
	got := text()
	if !strings.Contains(got, "level=WARN") || !strings.Contains(got, "https://***@host/repo") {
		t.Errorf("log = %q, want the masked stderr line at Warn", got)
	}
	if strings.Contains(got, "secret") {
		t.Errorf("log = %q, want no credential", got)
	}
}

// exitBurst is how many events and stderr lines exitcore writes before it
// exits.
const exitBurst = 2000

// exitingCore connects to exitcore and makes it write its burst and exit.
func exitingCore(t *testing.T, sink core.EventSink, log *slog.Logger) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
	wsl := &fakeWSL{bin: exitBin}
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", guiExe, wsl.run, acceptConsent{}, sink, log)
	client, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Jobs(t.Context(), "any"); err == nil {
		t.Fatal("Jobs answered, want the core gone")
	}
}

// Output the client cannot read ends its reading; the rest of the core's
// output must not keep the core's exit from being noticed.
func TestADR_4_ACoreThatExitsAfterUnreadableOutputIsRespawned(t *testing.T) {
	dc, _ := newDistroCore(t, exitBin, slog.Default())
	client, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Summary(t.Context(), "any"); err == nil {
		t.Fatal("Summary answered, want the core gone")
	}

	waitForRespawn(t, dc)
}

// A stderr line too long to log ends the logging; the rest of the core's
// stderr must not keep the core's exit from being noticed.
func TestADR_4_ACoreThatExitsAfterAnOverlongStderrLineIsRespawned(t *testing.T) {
	dc, _ := newDistroCore(t, exitBin, slog.Default())
	client, _, err := dc.Ensure(t.Context())
	if err != nil {
		t.Fatal(err)
	}

	if _, err := client.Refs(t.Context(), "any"); err == nil {
		t.Fatal("Refs answered, want the core gone")
	}

	waitForRespawn(t, dc)
}

// waitFor polls done for up to 3s and reports its last answer.
func waitFor(done func() bool) bool {
	deadline := time.Now().Add(3 * time.Second)
	for !done() && time.Now().Before(deadline) {
		time.Sleep(20 * time.Millisecond)
	}
	return done()
}

func TestADR_3_EveryEventACoreSendsBeforeItExitsArrives(t *testing.T) {
	var events atomic.Int64

	exitingCore(t, func(core.Event) { events.Add(1) }, slog.Default())

	if !waitFor(func() bool { return events.Load() == exitBurst }) {
		t.Errorf("%d events arrived, want %d", events.Load(), exitBurst)
	}
}

func TestFR_UX_02_EveryStderrLineACoreWritesBeforeItExitsReachesTheLog(t *testing.T) {
	logger, text := bufLogger()

	exitingCore(t, nil, logger)

	last := fmt.Sprintf("line %d", exitBurst-1)
	if !waitFor(func() bool { return strings.Contains(text(), last) }) {
		t.Errorf("log lacks %q, the core's last stderr line", last)
	}
}
