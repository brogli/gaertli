package remote_test

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/testbin"
)

// coreBin and badBin are built once for every test in this file: the real
// nebegit-core (spec 01 case 8 pattern) and a fixture reporting a wrong
// protocol version (case 10).
var coreBin, badBin string

func TestMain(m *testing.M) {
	dir, err := os.MkdirTemp("", "nebegit-distrocore")
	if err != nil {
		panic(err)
	}
	coreBin = filepath.Join(dir, "nebegit-core")
	badBin = filepath.Join(dir, "badcore")
	if err := testbin.Build(coreBin, "../../cmd/nebegit-core"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := testbin.Build(badBin, "./testdata/badcore"); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	code := m.Run()
	_ = os.RemoveAll(dir)
	os.Exit(code)
}

// acceptConsent always accepts (cases 9-12 don't assert consent behaviour;
// deploy.go's own tests cover that).
type acceptConsent struct{}

func (acceptConsent) Ask(string, string) bool { return true }

// fakeWSLRunner intercepts the "sh -c '<target> --stdio'" spawn call to run
// bin locally instead of a real wsl.exe child (spec 01 case 8 pattern);
// every other call (deploy's stat/cat dance) runs the given shell command
// for real against a test-sandboxed $HOME (t.Setenv), same technique as
// deploy_test.go. Every call, spawn or not, increments *calls. Each spawn
// cmd is appended to *spawns, letting a test find the live process to kill
// without DistroCore exposing it.
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

	var ce *core.Error
	if !errors.As(err, &ce) || ce.Code != core.Internal {
		t.Fatalf("err = %v, want *core.Error{Code: Internal}", err)
	}
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
	info, err := remote.NewOpener(client, "Ubuntu-22.04").OpenWorktree(context.Background(), core.RepoPath(uncPath))
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
	info, err := remote.NewOpener(client1, "Ubuntu-22.04").OpenWorktree(context.Background(), core.RepoPath(uncPath))
	if err != nil {
		t.Fatal(err)
	}

	// Kill the underlying process (ADR-4: simulates `wsl --shutdown`).
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
