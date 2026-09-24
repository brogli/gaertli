package remote

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"sync"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// DistroCore owns one distro's headless core process: spawns it (deploying
// first if needed), holds the live client, and respawns transparently if
// the process dies.
type DistroCore struct {
	distro, version string
	run             wslRunner
	consent         Consent
	sink            core.EventSink
	log             *slog.Logger

	mu        sync.Mutex
	client    *Client
	dead      <-chan struct{} // closed when the current child's Wait() returns
	onConnect func(context.Context, *Client)
}

// OnConnect registers fn to run once per fresh connect, after the version
// handshake succeeds and before Ensure returns to its caller.
func (d *DistroCore) OnConnect(fn func(context.Context, *Client)) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.onConnect = fn
}

// LiveClient returns the currently connected client, if any, without
// triggering a spawn.
func (d *DistroCore) LiveClient() (*Client, bool) {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.client, d.client != nil
}

// NewDistroCore builds a DistroCore for distro, not yet spawned. log is
// where a failed start, a reconnect, and the core process's own stderr are
// reported.
func NewDistroCore(distro, version string, run wslRunner, consent Consent, sink core.EventSink, log *slog.Logger) *DistroCore {
	return &DistroCore{distro: distro, version: version, run: run, consent: consent, sink: sink, log: log}
}

// Ensure returns a live client, (re)spawning the core process if none is
// running or the last one exited. respawned is true only when a spawn just
// happened; the caller then re-opens every worktree it had open against
// this distro, since a fresh core has no sessions. Concurrent
// callers during a spawn share the one attempt. A failure to start is
// logged at Warn, naming distro.
func (d *DistroCore) Ensure(ctx context.Context) (*Client, bool, error) {
	client, respawned, onConnect, err := d.ensure(ctx)
	if err != nil {
		d.log.Warn("WSL core failed to start", "distro", d.distro, "err", err)
		return nil, false, err
	}
	if respawned && onConnect != nil {
		onConnect(ctx, client) // outside the lock: an RPC round trip must not serialise other callers
	}
	return client, respawned, nil
}

// ensure is Ensure's locked half: the live client, whether it was just
// spawned, and the connect callback for Ensure to run once mu is released.
func (d *DistroCore) ensure(ctx context.Context) (client *Client, respawned bool, onConnect func(context.Context, *Client), err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.client != nil && !closed(d.dead) {
		return d.client, false, nil, nil
	}

	client, err = d.spawn(ctx)
	if errors.Is(err, errStaleCore) {
		// A stale binary despite a matching size (a same-version dev
		// rebuild): force one fresh copy, skipping the size check, and
		// retry once.
		if err := deployCore(ctx, d.run, d.consent, d.distro, deployTarget(d.version)); err != nil {
			return nil, false, nil, err
		}
		client, err = d.spawn(ctx)
		if errors.Is(err, errStaleCore) {
			err = core.Errorf(core.Internal, "distro %s: core protocol mismatch persists after a fresh deploy", d.distro)
		}
	}
	if err != nil {
		return nil, false, nil, err
	}

	if d.client != nil {
		d.log.Warn("WSL core reconnected", "distro", d.distro)
		_ = d.client.Close() // its process is gone (d.dead closed), so this returns at once
	}
	d.client = client
	return client, true, d.onConnect, nil
}

// errStaleCore is spawn's error for a core whose protocol is not this
// build's.
var errStaleCore = errors.New("core protocol mismatch")

// spawn deploys the core if needed, starts wsl.exe -d <distro> --
// <deployTarget> --stdio through a distro shell (so $HOME expands), and
// wires a client plus the dead channel that closes when the process exits.
// A core that fails the version handshake is killed: errStaleCore.
func (d *DistroCore) spawn(ctx context.Context) (*Client, error) {
	if err := ensureDeployed(ctx, d.run, d.consent, d.distro, d.version); err != nil {
		return nil, err
	}
	cmd, err := d.run(ctx, d.distro, "sh", "-c", deployTarget(d.version)+" --stdio")
	if err != nil {
		return nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go d.logStderr(stderr)

	dead := make(chan struct{})
	go func() { _ = cmd.Wait(); close(dead) }()
	d.dead = dead

	client := NewClient(stdout, stdin, d.sink)
	if !versionOK(ctx, client) {
		discard(client, cmd)
		return nil, errStaleCore
	}
	return client, nil
}

// versionOK checks the connected client's protocol against this build's.
func versionOK(ctx context.Context, client *Client) bool {
	v, err := client.Version(ctx)
	return err == nil && v.Protocol == core.Protocol
}

// discard kills cmd before closing client: Close does not close the pipes,
// so without the kill it would block until the process closes its stdout.
func discard(client *Client, cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
	_ = client.Close()
}

// logStderr reads distro's core process's stderr and writes one Warn record
// per line, masking an embedded credential, until the process closes it.
func (d *DistroCore) logStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		d.log.Warn(git.MaskCredentials(scanner.Text()), "distro", d.distro)
	}
}

func closed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// distroPathClient is a DistroCore client that translates every
// core.RepoPath argument, and the target argument of AddWorktree and
// RemoveWorktree, from the WSL UNC form the shell holds to the POSIX form
// distro's own core expects; embedding passes every other method straight
// to *Client. Returned paths stay POSIX (the distro core's own); only
// OpenWorktree's ID comes back in UNC form, the shell's id for a WSL
// worktree.
type distroPathClient struct {
	*Client
	distro string
}

// NewDistroPathClient wraps client as a distroPathClient for distro.
func NewDistroPathClient(client *Client, distro string) core.API {
	return distroPathClient{Client: client, distro: distro}
}

// posix translates path, a WSL UNC path naming o.distro, to its POSIX form.
// A path naming another distro, or no WSL path at all, is core.Internal.
func (o distroPathClient) posix(path string) (string, error) {
	distro, linuxPath, ok := ClassifyWSLPath(path)
	if !ok || distro != o.distro {
		return "", core.Errorf(core.Internal, "not a WSL path for distro %s: %s", o.distro, path)
	}
	return linuxPath, nil
}

func (o distroPathClient) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	p, err := o.posix(string(path))
	if err != nil {
		return core.WorktreeInfo{}, err
	}
	info, err := o.Client.OpenWorktree(ctx, core.RepoPath(p))
	if err != nil {
		return core.WorktreeInfo{}, err
	}
	info.ID = core.WorktreeID(BuildWSLPath(o.distro, string(info.ID)))
	return info, nil
}

func (o distroPathClient) OpenFolderAt(ctx context.Context, path core.RepoPath) error {
	p, err := o.posix(string(path))
	if err != nil {
		return err
	}
	return o.Client.OpenFolderAt(ctx, core.RepoPath(p))
}

func (o distroPathClient) ListWorktrees(ctx context.Context, path core.RepoPath) ([]core.Worktree, error) {
	p, err := o.posix(string(path))
	if err != nil {
		return nil, err
	}
	return o.Client.ListWorktrees(ctx, core.RepoPath(p))
}

func (o distroPathClient) ListBranches(ctx context.Context, path core.RepoPath) ([]core.BranchSummary, error) {
	p, err := o.posix(string(path))
	if err != nil {
		return nil, err
	}
	return o.Client.ListBranches(ctx, core.RepoPath(p))
}

func (o distroPathClient) AddWorktree(ctx context.Context, path core.RepoPath, target, branch, start string, newBranch bool) (core.Worktree, error) {
	p, err := o.posix(string(path))
	if err != nil {
		return core.Worktree{}, err
	}
	t, err := o.posix(target)
	if err != nil {
		return core.Worktree{}, err
	}
	return o.Client.AddWorktree(ctx, core.RepoPath(p), t, branch, start, newBranch)
}

func (o distroPathClient) RemoveWorktree(ctx context.Context, path core.RepoPath, target string, force bool) error {
	p, err := o.posix(string(path))
	if err != nil {
		return err
	}
	t, err := o.posix(target)
	if err != nil {
		return err
	}
	return o.Client.RemoveWorktree(ctx, core.RepoPath(p), t, force)
}

func (o distroPathClient) PruneWorktrees(ctx context.Context, path core.RepoPath) error {
	p, err := o.posix(string(path))
	if err != nil {
		return err
	}
	return o.Client.PruneWorktrees(ctx, core.RepoPath(p))
}

func (o distroPathClient) MainWorktree(ctx context.Context, gitDir core.RepoPath) (core.Worktree, error) {
	p, err := o.posix(string(gitDir))
	if err != nil {
		return core.Worktree{}, err
	}
	return o.Client.MainWorktree(ctx, core.RepoPath(p))
}
