package services

import (
	"bytes"
	"context"
	"slices"
	"strconv"
	"strings"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
)

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
	return toRefs(parseRefRecords(result.Stdout), names), nil
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
