package remote_test

import (
	"context"
	"errors"
	"os/exec"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/core/coretest"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// coreBin is the real nebegit-core; badBin reports a wrong protocol version.
var coreBin, badBin string

func TestMain(m *testing.M) {
	testbin.Main(m,
		testbin.Bin{Path: &coreBin, Pkg: "../../cmd/nebegit-core"},
		testbin.Bin{Path: &badBin, Pkg: "./testdata/badcore"},
	)
}

// acceptConsent always accepts; deploy_test.go covers consent behaviour.
type acceptConsent struct{}

func (acceptConsent) Ask(string, string) bool { return true }

// fakeWSLRunner intercepts the "sh -c '<target> --stdio'" spawn call to run
// bin locally instead of a real wsl.exe child; every other call (deploy's
// stat/cat dance) runs the given shell command for real against a
// test-sandboxed $HOME. Every call increments *calls, and each spawn cmd is
// appended to *spawns so a test can kill the live process without
// DistroCore exposing it.
func fakeWSLRunner(bin string, calls *int, spawns *[]*exec.Cmd) func(context.Context, string, ...string) (*exec.Cmd, error) {
	return func(ctx context.Context, distro string, args ...string) (*exec.Cmd, error) {
		*calls++
		if len(args) == 3 && strings.HasSuffix(args[2], "--stdio") {
			cmd := exec.CommandContext(ctx, bin, "--stdio")
			*spawns = append(*spawns, cmd)
			return cmd, nil
		}
		return exec.CommandContext(ctx, args[0], args[1:]...), nil
	}
}

func TestADR_3_EnsureSpawnsOnceThenReusesTheLiveClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls int
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(coreBin, &calls, new([]*exec.Cmd)), acceptConsent{}, nil)

	client1, respawned1, err := dc.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !respawned1 {
		t.Error("first Ensure: respawned = false, want true")
	}
	callsAfterFirst := calls

	client2, respawned2, err := dc.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if respawned2 {
		t.Error("second Ensure: respawned = true, want false")
	}
	if client2 != client1 {
		t.Error("second Ensure returned a different client instance")
	}
	if calls != callsAfterFirst {
		t.Errorf("wslRunner called %d more times on the second Ensure, want 0", calls-callsAfterFirst)
	}
}

func TestADR_3_EnsureForcesARedeployOnAProtocolMismatch(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	var calls int
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(badBin, &calls, new([]*exec.Cmd)), acceptConsent{}, nil)

	_, _, err := dc.Ensure(context.Background())

	coretest.AssertCode(t, err, core.Internal)
}

func TestADR_3_OpeningWSLWorktreeTranslatesTheUNCPath(t *testing.T) {
	repo := t.TempDir()
	if err := exec.Command("git", "init", repo).Run(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(coreBin, new(int), new([]*exec.Cmd)), acceptConsent{}, nil)
	client, _, err := dc.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}

	uncPath := `\\wsl.localhost\Ubuntu-22.04\` + strings.ReplaceAll(strings.TrimPrefix(repo, "/"), "/", `\`)
	info, err := remote.NewDistroPathClient(client, "Ubuntu-22.04").OpenWorktree(context.Background(), core.RepoPath(uncPath))
	if err != nil {
		t.Fatal(err)
	}
	if info.Path != repo {
		t.Errorf("OpenWorktree translated path = %q, want %q (not the UNC string)", info.Path, repo)
	}
}

func TestADR_4_EnsureRespawnsAndTheOldSessionIsGone(t *testing.T) {
	repo := t.TempDir()
	if err := exec.Command("git", "init", repo).Run(); err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", t.TempDir())
	var calls int
	var spawns []*exec.Cmd
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(coreBin, &calls, &spawns), acceptConsent{}, nil)

	client1, _, err := dc.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	uncPath := `\\wsl.localhost\Ubuntu-22.04\` + strings.ReplaceAll(strings.TrimPrefix(repo, "/"), "/", `\`)
	info, err := remote.NewDistroPathClient(client1, "Ubuntu-22.04").OpenWorktree(context.Background(), core.RepoPath(uncPath))
	if err != nil {
		t.Fatal(err)
	}

	// kill the underlying process, as `wsl --shutdown` would
	if err := spawns[len(spawns)-1].Process.Kill(); err != nil {
		t.Fatal(err)
	}

	client2 := waitForRespawn(t, dc)
	if client2 == client1 {
		t.Fatal("Ensure after the kill returned the same client instance")
	}

	_, err = client2.Summary(context.Background(), info.ID)
	var ce *core.Error
	if !errors.As(err, &ce) {
		t.Fatalf("bare call on the fresh client err = %v, want a *core.Error (unknown worktree)", err)
	}
}

func TestTR_13_OnConnectFiresOnceForAFreshConnectNotOnAReusedClient(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(coreBin, new(int), new([]*exec.Cmd)), acceptConsent{}, nil)
	var got []*remote.Client
	dc.OnConnect(func(_ context.Context, c *remote.Client) { got = append(got, c) })

	client1, _, err := dc.Ensure(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != client1 {
		t.Fatalf("onConnect calls = %d, want 1 with the fresh client", len(got))
	}

	client2, _, err := dc.Ensure(context.Background())
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
	t.Setenv("HOME", t.TempDir())
	dc := remote.NewDistroCore("Ubuntu-22.04", "1.0.0", fakeWSLRunner(coreBin, new(int), new([]*exec.Cmd)), acceptConsent{}, nil)

	if _, ok := dc.LiveClient(); ok {
		t.Fatal("LiveClient before any Ensure reports true")
	}

	client, _, err := dc.Ensure(context.Background())
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
func repoPathMethods(t *testing.T) []reflect.Method {
	t.Helper()
	apiType := reflect.TypeOf((*core.API)(nil)).Elem()
	repoPathType := reflect.TypeOf(core.RepoPath(""))

	var methods []reflect.Method
	for i := range apiType.NumMethod() {
		m := apiType.Method(i)
		for j := range m.Type.NumIn() {
			if m.Type.In(j) == repoPathType {
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
func callArgs(fnType reflect.Type, m string, path, target string) []reflect.Value {
	args := make([]reflect.Value, fnType.NumIn())
	for i := range args {
		switch paramType := fnType.In(i); {
		case paramType == reflect.TypeOf((*context.Context)(nil)).Elem():
			args[i] = reflect.ValueOf(context.Background())
		case paramType == reflect.TypeOf(core.RepoPath("")):
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

	f := &fake{version: func() (core.Version, error) { return core.Version{}, nil }}
	client := pipeClient(t, f, nil)
	wrapped := reflect.ValueOf(remote.NewDistroPathClient(client, "Ubuntu-22.04"))

	for _, m := range repoPathMethods(t) {
		t.Run(m.Name, func(t *testing.T) {
			method := wrapped.MethodByName(m.Name)
			if err := lastError(method.Call(callArgs(method.Type(), m.Name, uncPath, uncTarget))); err != nil {
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

	f := &fake{version: func() (core.Version, error) { return core.Version{}, nil }}
	client := pipeClient(t, f, nil)
	wrapped := reflect.ValueOf(remote.NewDistroPathClient(client, "Ubuntu-22.04"))

	for _, m := range repoPathMethods(t) {
		for _, c := range cases {
			t.Run(m.Name+"/"+c.name, func(t *testing.T) {
				method := wrapped.MethodByName(m.Name)
				err := lastError(method.Call(callArgs(method.Type(), m.Name, c.path, c.path)))
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
		client, respawned, err := dc.Ensure(context.Background())
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
