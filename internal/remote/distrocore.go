package remote

import (
	"bufio"
	"context"
	"errors"
	"io"
	"log/slog"
	"os/exec"
	"slices"
	"strings"
	"sync"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

// DistroCore owns one distro's headless core process: spawns it (deploying
// first if needed), holds the live client, and respawns transparently if
// the process dies.
type DistroCore struct {
	distro, version string
	guiExe          string // the Windows app the nebegit command opens
	run             wslRunner
	listDistros     func(context.Context) ([]Distro, error)
	consent         Consent
	sink            core.EventSink
	log             *slog.Logger

	mu        sync.Mutex
	client    *Client
	dead      <-chan struct{} // closed when the current child's Wait() returns
	onConnect func(context.Context, *Client)
}

// NewDistroCore builds a DistroCore for distro, not yet spawned. guiExe is
// the Windows app's own path, for the nebegit command. log is where a
// failed start, a reconnect, and the core process's own stderr are
// reported.
func NewDistroCore(distro, version, guiExe string, run wslRunner, consent Consent, sink core.EventSink, log *slog.Logger) *DistroCore {
	return &DistroCore{distro: distro, version: version, guiExe: guiExe, run: run, listDistros: ListDistros, consent: consent, sink: sink, log: log}
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

// Ensure returns a live client, spawning the core process if none runs.
// respawned is true only when a spawn just happened: a fresh core has no
// sessions, so the caller re-opens its worktrees. A spawn also sets up the
// nebegit command. Concurrent callers during a spawn share the one attempt.
// A failed start is logged; a distro not installed is core.DistroNotFound,
// one on WSL 1 core.WslUnsupported.
func (d *DistroCore) Ensure(ctx context.Context) (*Client, bool, error) {
	// One start lists the distros at most once, for the WSL 1 check and a
	// failure alike.
	listed := sync.OnceValues(func() ([]Distro, error) { return d.listDistros(ctx) })
	client, respawned, onConnect, err := d.ensure(ctx, listed)
	if err != nil {
		err = d.notInstalled(err, listed)
		d.log.Warn("WSL core failed to start", "distro", d.distro, "err", err)
		return nil, false, err
	}
	if respawned {
		// outside the lock: a wsl.exe start or an RPC round trip must not
		// serialise other callers
		d.setUpCommand(ctx)
		if onConnect != nil {
			onConnect(ctx, client)
		}
	}
	return client, respawned, nil
}

// distroListing is one start's listing of the installed distros.
type distroListing func() ([]Distro, error)

// ensure is Ensure's locked half: the live client, whether it was just
// spawned, and the connect callback for Ensure to run once mu is released.
func (d *DistroCore) ensure(ctx context.Context, listed distroListing) (client *Client, respawned bool, onConnect func(context.Context, *Client), err error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	if d.client != nil && !closed(d.dead) {
		return d.client, false, nil, nil
	}
	if err := d.requireWSL2(listed); err != nil {
		return nil, false, nil, err
	}

	client, err = d.spawn(ctx)
	if errors.Is(err, errStaleCore) {
		// A stale binary despite a matching size (a same-version dev
		// rebuild): force one fresh copy, skipping the size check, and
		// retry once.
		if err := d.deployCore(ctx); err != nil {
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
		_ = d.client.Close()
	}
	d.client = client
	return client, true, d.onConnect, nil
}

// notInstalled turns err, a failed start, into core.DistroNotFound when
// wsl.exe exited non-zero and the distro list lacks d's distro. Any other
// error, or a failed listing, is returned as is.
func (d *DistroCore) notInstalled(err error, listed distroListing) error {
	if _, exited := errors.AsType[*exec.ExitError](err); !exited {
		return err
	}
	distros, listErr := listed()
	if listErr != nil || slices.ContainsFunc(distros, d.matches) {
		return err
	}
	return core.ErrorfArgs(core.DistroNotFound, "distroNotFound", map[string]string{"distro": d.distro}, "WSL distro %s is not installed", d.distro)
}

// requireWSL2 refuses a distro listed as running on WSL 1, before anything
// is deployed or started in it. A failed listing does not refuse: the
// start then says what is wrong.
func (d *DistroCore) requireWSL2(listed distroListing) error {
	distros, err := listed()
	if err != nil {
		return nil //nolint:nilerr // a failed listing does not refuse the start
	}
	i := slices.IndexFunc(distros, d.matches)
	if i >= 0 && distros[i].Version == 1 {
		return core.ErrorfArgs(core.WslUnsupported, "wsl1Unsupported", map[string]string{"distro": d.distro}, "WSL distro %s runs on WSL 1; nebegit needs WSL 2.", d.distro)
	}
	return nil
}

// matches reports whether listed is d's distro. wsl.exe matches names
// regardless of case.
func (d *DistroCore) matches(listed Distro) bool { return strings.EqualFold(listed.Name, d.distro) }

// outputGrace bounds how long a core that exited can keep the shell
// waiting on output a grandchild still holds open.
const outputGrace = time.Second

// errStaleCore is spawn's error for a core whose protocol is not this
// build's.
var errStaleCore = errors.New("core protocol mismatch")

// spawn deploys the core if needed, starts wsl.exe -d <distro> --
// <deployTarget> --stdio through a distro shell (so $HOME expands), and
// wires a client plus the dead channel that closes when the process exits.
// A core that fails the version handshake is killed: errStaleCore.
//
// exec copies the core's stdout and stderr into pipes the client and the
// logger read, and Wait returns only once those copies are done, so both
// read everything the core wrote before they see EOF.
func (d *DistroCore) spawn(ctx context.Context) (*Client, error) {
	if err := d.ensureDeployed(ctx); err != nil {
		return nil, err
	}
	cmd, err := d.run(ctx, d.distro, "sh", "-c", deployTarget(d.version)+" --stdio")
	if err != nil {
		return nil, err
	}
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, err
	}
	stdout, stdoutW := io.Pipe()
	stderr, stderrW := io.Pipe()
	cmd.Stdout, cmd.Stderr = stdoutW, stderrW
	cmd.WaitDelay = outputGrace
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	go d.logStderr(stderr)

	dead := make(chan struct{})
	go func() {
		_ = cmd.Wait()
		_ = stdoutW.Close()
		_ = stderrW.Close()
		close(dead)
	}()
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

// discard kills the core that failed the handshake and closes its client.
func discard(client *Client, cmd *exec.Cmd) {
	_ = cmd.Process.Kill()
	_ = client.Close()
}

// logStderr reads distro's core process's stderr and writes one Warn record
// per line, masking an embedded credential, until the process closes it.
// A line too long to log ends the logging; the rest is read and dropped.
func (d *DistroCore) logStderr(r io.Reader) {
	scanner := bufio.NewScanner(r)
	for scanner.Scan() {
		d.log.Warn(git.MaskCredentials(scanner.Text()), "distro", d.distro)
	}
	if err := scanner.Err(); err != nil {
		d.log.Warn("WSL core stderr no longer logged", "distro", d.distro, "err", err)
	}
	_, _ = io.Copy(io.Discard, r) // a core blocked writing its stderr would never exit
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
