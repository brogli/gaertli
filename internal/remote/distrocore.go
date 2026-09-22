package remote

import (
	"context"
	"os/exec"
	"sync"

	"github.com/brogli/nebegit/internal/core"
)

// DistroCore owns one distro's headless core process (ADR-3: "One RPC core
// per distro in use"): spawns it (deploying first if needed), holds the
// live client, and respawns transparently if the process dies (reconnect,
// ADR-4).
type DistroCore struct {
	distro, version string
	run             wslRunner
	consent         Consent
	sink            core.EventSink

	mu        sync.Mutex
	client    *Client
	dead      <-chan struct{} // closed when the current child's Wait() returns
	onConnect func(context.Context, *Client)
}

// OnConnect registers fn to run once per fresh connect (TR-13: "the shell
// pushes the relevant subset to each core on connect"), after the version
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

// NewDistroCore builds a DistroCore for distro, not yet spawned.
func NewDistroCore(distro, version string, run wslRunner, consent Consent, sink core.EventSink) *DistroCore {
	return &DistroCore{distro: distro, version: version, run: run, consent: consent, sink: sink}
}

// Ensure returns a live client, (re)spawning the core process if none is
// running or the last one exited. respawned is true only when a spawn just
// happened; the caller then re-opens every worktree it had open against
// this distro, since a fresh core has no sessions (ADR-4). Concurrent
// callers during a spawn share the one attempt.
func (d *DistroCore) Ensure(ctx context.Context) (*Client, bool, error) {
	client, respawned, onConnect, err := d.ensure(ctx)
	if err != nil {
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

	client, cmd, err := d.spawn(ctx)
	if err != nil {
		return nil, false, nil, err
	}
	if !d.versionOK(ctx, client) {
		discard(client, cmd)
		// A stale binary despite a matching size (a same-version dev
		// rebuild): force one fresh copy, skipping the size check, and
		// retry once.
		if err := deployCore(ctx, d.run, d.consent, d.distro, deployTarget(d.version)); err != nil {
			return nil, false, nil, err
		}
		client, cmd, err = d.spawn(ctx)
		if err != nil {
			return nil, false, nil, err
		}
		if !d.versionOK(ctx, client) {
			discard(client, cmd)
			return nil, false, nil, core.Errorf(core.Internal, "distro %s: core protocol mismatch persists after a fresh deploy", d.distro)
		}
	}

	if d.client != nil {
		_ = d.client.Close() // its process is gone (d.dead closed), so this returns at once
	}
	d.client = client
	return client, true, d.onConnect, nil
}

// discard kills cmd before closing client: Close's underlying channel does
// not close the pipes (remote.go: "the caller owns stdout and pipes"), so
// without the kill it would block forever waiting for the still-running
// process to close its own stdout.
func discard(client *Client, cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
	_ = client.Close()
}

// versionOK checks the connected client's protocol against this build's own
// (ADR-3: "Version is checked on connect").
func (d *DistroCore) versionOK(ctx context.Context, client *Client) bool {
	v, err := client.Version(ctx)
	return err == nil && v.Protocol == core.Protocol
}

// spawn deploys the core if needed, starts wsl.exe -d <distro> --
// <deployTarget> --stdio through a distro shell (so $HOME expands), and
// wires a client plus the dead channel that closes when the process exits.
// Returns cmd too, so a caller discarding a stale client can kill it.
func (d *DistroCore) spawn(ctx context.Context) (*Client, *exec.Cmd, error) {
	if err := ensureDeployed(ctx, d.run, d.consent, d.distro, d.version); err != nil {
		return nil, nil, err
	}
	cmd, err := d.run(ctx, d.distro, "sh", "-c", deployTarget(d.version)+" --stdio")
	if err != nil {
		return nil, nil, err
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	dead := make(chan struct{})
	go func() { _ = cmd.Wait(); close(dead) }()
	d.dead = dead

	return NewClient(stdout, stdin, d.sink), cmd, nil
}

func closed(ch <-chan struct{}) bool {
	if ch == nil {
		return true
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

// openingWSLWorktree adapts a live DistroCore client to core.API for the
// one call that needs path translation: only OpenWorktree takes a RepoPath,
// every other method takes a WorktreeID the distro's own core minted.
// Embedding promotes the rest straight through to *Client.
type openingWSLWorktree struct {
	*Client
	distro string
}

// NewOpener wraps client so OpenWorktree translates a WSL UNC path
// (\\wsl.localhost\<distro>\... or \\wsl$\<distro>\...) to the POSIX path
// distro's own core expects.
func NewOpener(client *Client, distro string) core.API {
	return openingWSLWorktree{Client: client, distro: distro}
}

func (o openingWSLWorktree) OpenWorktree(ctx context.Context, path core.RepoPath) (core.WorktreeInfo, error) {
	_, linuxPath, ok := ClassifyWSLPath(string(path))
	if !ok {
		return core.WorktreeInfo{}, core.Errorf(core.Internal, "not a WSL path for distro %s: %s", o.distro, path)
	}
	return o.Client.OpenWorktree(ctx, core.RepoPath(linuxPath))
}
