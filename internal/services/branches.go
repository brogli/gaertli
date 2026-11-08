package services

import (
	"bytes"
	"context"
	"errors"
	"slices"
	"strconv"
	"strings"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git"
)

// CreateBranch implements core.API.
func (c *Core) CreateBranch(ctx context.Context, id core.WorktreeID, name, start string, checkout bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}

	if err := validateBranchName(ctx, wt.Runner, wt.Path, name); err != nil {
		return err // a read-only check: not worth holding a write lock for
	}

	if checkout {
		wt.Lock() // create+checkout writes this worktree's HEAD/index
		defer wt.Unlock()
	} else {
		wt.Repo.Lock() // a plain ref create is a repo-wide one-off
		defer wt.Repo.Unlock()
	}

	args := []string{"branch", "--", name, start}
	if checkout {
		args = []string{"switch", "-c", name, "--end-of-options", start} // atomic: no orphan branch if the checkout fails
	}
	if startsAtARemoteBranch(start) { // its own first push gives it its own upstream, not this one's
		args = slices.Insert(args, 1, "--no-track")
	}
	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: args})
	return remapDirty(err, result)
}

// startsAtARemoteBranch reports whether start is a remote-tracking branch's
// full ref.
func startsAtARemoteBranch(start string) bool {
	return strings.HasPrefix(start, "refs/remotes/")
}

// validateBranchName rejects an invalid name before any mutating call as
// Code InvalidName, from check-ref-format's exit code.
func validateBranchName(ctx context.Context, r git.Runner, dir, name string) error {
	_, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: []string{"check-ref-format", "--branch", name}, // safe without a -- of its own
	})
	if ce, ok := errors.AsType[*core.Error](err); ok {
		ce.Retag(core.InvalidName, "invalidBranchName", map[string]string{"name": name}, "%q is not a valid branch name", name)
	}
	return err
}

// Checkout implements core.API. A remote-tracking branch goes by its full
// ref with --track: a bare name's DWIM refuses when a tag has that name.
func (c *Core) Checkout(ctx context.Context, id core.WorktreeID, branch string) error {
	args := []string{"switch", "--end-of-options", branch}
	if startsAtARemoteBranch(branch) {
		args = []string{"switch", "--track", "--end-of-options", branch}
	}
	result, err := c.runLocked(ctx, id, args...)
	return remapDirty(err, result)
}

// CheckoutCommit implements core.API.
func (c *Core) CheckoutCommit(ctx context.Context, id core.WorktreeID, sha string) error {
	result, err := c.runLocked(ctx, id, "switch", "--detach", "--end-of-options", sha)
	return remapDirty(err, result)
}

// Reset implements core.API.
func (c *Core) Reset(ctx context.Context, id core.WorktreeID, sha string, mode core.ResetMode) (core.ResetResult, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.ResetResult{}, err
	}
	flag, err := resetFlag(mode)
	if err != nil {
		return core.ResetResult{}, err
	}

	wt.Lock() // reset writes this worktree's index/files
	defer wt.Unlock()

	previous, err := verifyRev(ctx, wt.Runner, wt.Path, "HEAD") // "" only for an unborn HEAD
	if err != nil {
		return core.ResetResult{}, err
	}
	// no --end-of-options: sha is always an already-resolved hash
	if _, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"reset", flag, sha}}); err != nil {
		return core.ResetResult{}, err
	}
	return core.ResetResult{PreviousHead: previous}, nil
}

// resetFlag maps mode to reset's flag. An unknown mode is a caller bug: Code
// Internal.
func resetFlag(mode core.ResetMode) (string, error) {
	switch mode {
	case core.ResetSoft:
		return "--soft", nil
	case core.ResetMixed:
		return "--mixed", nil
	case core.ResetHard:
		return "--hard", nil
	case core.ResetKeep:
		return "--keep", nil
	default:
		return "", core.Errorf(core.Internal, "unknown reset mode %q", mode)
	}
}

// remapDirty turns a switch refused with exit 1 (a local change or untracked
// file would be overwritten) into Code DirtyWorktree. Every other failure
// (exit 128) stays GitFailed with git's own message.
func remapDirty(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && result.ExitCode == 1 {
		ce.Code = core.DirtyWorktree
	}
	return err
}

// RenameBranch implements core.API.
func (c *Core) RenameBranch(ctx context.Context, id core.WorktreeID, name, newName string) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if err := validateBranchName(ctx, wt.Runner, wt.Path, newName); err != nil {
		return err
	}
	if err := refuseBranchRebasingOrBisecting(ctx, wt.Runner, wt.Path, name); err != nil {
		return err // a read-only check: not worth holding a write lock for
	}

	wt.Repo.Lock() // branch rename is a repo-wide one-off
	defer wt.Repo.Unlock()

	_, err = wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"branch", "-m", "--", name, newName}})
	return err
}

// DeleteBranch implements core.API.
func (c *Core) DeleteBranch(ctx context.Context, id core.WorktreeID, name string, force bool) error {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return err
	}
	if err := refuseBranchInUse(ctx, wt.Runner, wt.Path, name); err != nil {
		return err // a read-only check: not worth holding a write lock for
	}

	flag := "-d"
	if force {
		flag = "-D"
	}

	wt.Repo.Lock() // branch delete is a repo-wide one-off
	defer wt.Repo.Unlock()

	result, err := wt.Runner.Run(ctx, git.Call{Dir: wt.Path, Args: []string{"branch", flag, "--", name}})
	return remapUnmerged(err, result)
}

// remapUnmerged reclassifies -d's "not fully merged" refusal as Code
// Unmerged, so the caller can offer a forced retry. Every other exit-1
// refusal ("used by worktree") stays plain GitFailed: forcing does not fix
// that one.
func remapUnmerged(err error, result git.Result) error {
	if ce := gitFailure(err); ce != nil && result.ExitCode == 1 && bytes.Contains(result.Stderr, []byte("is not fully merged")) {
		ce.Code = core.Unmerged
	}
	return err
}

// refsFormat: %1f/%00 is for-each-ref's own hex-escape syntax, not git
// log's %x1f/%x00. %(*objectname) (deref) is requested alongside
// %(objectname) so an annotated tag's SHA is the commit it points to, not
// the tag object.
const refsFormat = "%(refname)%1f%(objectname)%1f%(*objectname)%1f%(upstream:short)%1f%(upstream:track)%1f%(HEAD)%1f%(worktreepath)%00"

// Refs implements core.API.
func (c *Core) Refs(ctx context.Context, id core.WorktreeID) (core.Refs, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.Refs{}, err
	}
	result, err := wt.Runner.Run(ctx, git.Call{
		Dir: wt.Path, ReadOnly: true, Background: true,
		Args: []string{"for-each-ref", "--format=" + refsFormat, "refs/heads", "refs/remotes", "refs/tags"},
	})
	if err != nil {
		return core.Refs{}, err
	}
	names, err := remoteNames(ctx, wt.Runner, wt.Path)
	if err != nil {
		return core.Refs{}, err
	}
	inUse, err := branchesInUse(ctx, wt.Runner, wt.Path, true)
	if err != nil {
		return core.Refs{}, err
	}
	refs := toRefs(parseRefRecords(result.Stdout), names)
	markBranchesInUse(refs.Branches, inUse)
	return refs, nil
}

// markBranchesInUse fills CheckedOutIn and InUse for a branch some other
// worktree is rebasing or bisecting: for-each-ref's %(worktreepath) is
// empty for it whenever that worktree's HEAD is detached mid-operation. A
// use in this worktree itself is left unmarked: it is not held "elsewhere",
// and for a plain checkout %(worktreepath) already covers it.
func markBranchesInUse(branches []core.Branch, inUse map[string]branchInUse) {
	for i, b := range branches {
		if use, ok := inUse[b.Name]; ok && !use.own {
			branches[i].CheckedOutIn = use.path
			branches[i].InUse = core.BranchInUse(use.kind)
		}
	}
}

// parsedRef is one for-each-ref record, before bucketing into core.Refs'
// three lists.
type parsedRef struct {
	Name          string // full refname, e.g. "refs/heads/main"
	SHA           string // %(*objectname) if non-empty, else %(objectname)
	Upstream      string
	Ahead, Behind int
	UpstreamGone  bool
	Current       bool
	Worktree      string
}

// parseRefRecords splits refsFormat's %00-terminated records and their
// %1f-separated fields.
func parseRefRecords(out []byte) []parsedRef {
	var refs []parsedRef
	for _, record := range nulRecords(out) {
		refs = append(refs, parseRefRecord(record))
	}
	return refs
}

// nulRecords splits a %00-terminated git stream into records, stripping the
// '\n' git appends after each one and dropping empties.
func nulRecords(out []byte) []string {
	var records []string
	for record := range bytes.SplitSeq(out, []byte{0}) {
		record = bytes.TrimPrefix(record, []byte("\n"))
		if len(record) > 0 {
			records = append(records, string(record))
		}
	}
	return records
}

func parseRefRecord(record string) parsedRef {
	f := strings.Split(record, "\x1f")
	sha := f[1]
	if f[2] != "" {
		sha = f[2]
	}
	ahead, behind, gone := parseTrack(f[4])
	return parsedRef{
		Name: f[0], SHA: sha, Upstream: f[3],
		Ahead: ahead, Behind: behind, UpstreamGone: gone,
		Current: f[5] == "*", Worktree: f[6],
	}
}

// parseTrack parses %(upstream:track)'s bracketed text: "" (no upstream,
// or unchanged), "[ahead N]", "[behind M]", "[ahead N, behind M]", or
// "[gone]".
func parseTrack(s string) (ahead, behind int, gone bool) {
	if s == "[gone]" {
		return 0, 0, true
	}
	s = strings.TrimSuffix(strings.TrimPrefix(s, "["), "]")
	for part := range strings.SplitSeq(s, ", ") {
		if n, ok := strings.CutPrefix(part, "ahead "); ok {
			ahead, _ = strconv.Atoi(n)
		}
		if n, ok := strings.CutPrefix(part, "behind "); ok {
			behind, _ = strconv.Atoi(n)
		}
	}
	return ahead, behind, false
}

// toRefs buckets parsed refs by refname prefix and groups remote branches by
// remote. for-each-ref sorts by refname, so every list arrives sorted.
// remoteNames are the configured remotes: a remote's name may contain a
// slash.
func toRefs(parsed []parsedRef, remoteNames []string) core.Refs {
	var refs core.Refs
	for _, name := range remoteNames {
		refs.Remotes = append(refs.Remotes, core.Remote{Name: name})
	}
	for _, p := range parsed {
		switch {
		case strings.HasPrefix(p.Name, "refs/heads/"):
			refs.Branches = append(refs.Branches, toBranch(p))
		case strings.HasPrefix(p.Name, "refs/tags/"):
			refs.Tags = append(refs.Tags, core.Tag{Name: strings.TrimPrefix(p.Name, "refs/tags/"), SHA: p.SHA})
		case strings.HasPrefix(p.Name, "refs/remotes/"):
			remote, branch, ok := splitRemoteBranch(strings.TrimPrefix(p.Name, "refs/remotes/"), remoteNames)
			if ok && branch != "HEAD" { // <remote>/HEAD is the remote's symbolic HEAD, not a branch
				addRemoteBranch(&refs, remote, core.RemoteBranch{Name: branch, SHA: p.SHA})
			}
		}
	}
	return refs
}

func toBranch(p parsedRef) core.Branch {
	return core.Branch{
		Name: strings.TrimPrefix(p.Name, "refs/heads/"), SHA: p.SHA,
		Upstream: p.Upstream, Ahead: p.Ahead, Behind: p.Behind,
		UpstreamGone: p.UpstreamGone, Current: p.Current, CheckedOutIn: p.Worktree,
	}
}

// splitRemoteBranch splits "<remote>/<branch>" at the longest configured
// remote name that prefixes it; a ref of a remote no longer configured
// splits at its first slash.
func splitRemoteBranch(rest string, remoteNames []string) (remote, branch string, ok bool) {
	for _, r := range remoteNames {
		if b, found := strings.CutPrefix(rest, r+"/"); found && len(r) > len(remote) {
			remote, branch = r, b
		}
	}
	if remote != "" {
		return remote, branch, true
	}
	return strings.Cut(rest, "/")
}

// remoteNames lists the configured remotes.
func remoteNames(ctx context.Context, r git.Runner, dir string) ([]string, error) {
	result, err := r.Run(ctx, git.Call{Dir: dir, ReadOnly: true, Background: true, Args: []string{"remote"}})
	if err != nil {
		return nil, err
	}
	return strings.Fields(string(result.Stdout)), nil
}

// addRemoteBranch appends b to remote's entry in refs.Remotes, adding the
// entry the first time remote is seen.
func addRemoteBranch(refs *core.Refs, remote string, b core.RemoteBranch) {
	i := slices.IndexFunc(refs.Remotes, func(r core.Remote) bool { return r.Name == remote })
	if i < 0 {
		i = len(refs.Remotes)
		refs.Remotes = append(refs.Remotes, core.Remote{Name: remote})
	}
	refs.Remotes[i].Branches = append(refs.Remotes[i].Branches, b)
}
