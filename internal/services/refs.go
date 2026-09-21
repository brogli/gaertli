package services

import (
	"bytes"
	"context"
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

// parsedRef is one for-each-ref record, before bucketing into core.Refs'
// three lists or extracting Summary's current-branch fields.
type parsedRef struct {
	Name          string // full refname, e.g. "refs/heads/main"
	SHA           string // %(*objectname) if non-empty, else %(objectname)
	Upstream      string
	Ahead, Behind int
	UpstreamGone  bool
	Current       bool
	Worktree      string
}

// parseRefRecords splits on the format's own %00 and strips each record's
// leading \n (same framing established for git log/for-each-ref), then
// splits fields on %1f.
func parseRefRecords(out []byte) []parsedRef {
	var refs []parsedRef
	for _, record := range bytes.Split(out, []byte{0}) {
		record = bytes.TrimPrefix(record, []byte("\n"))
		if len(record) == 0 {
			continue
		}
		refs = append(refs, parseRefRecord(string(record)))
	}
	return refs
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
	for _, part := range strings.Split(s, ", ") {
		if n, ok := strings.CutPrefix(part, "ahead "); ok {
			ahead, _ = strconv.Atoi(n)
		}
		if n, ok := strings.CutPrefix(part, "behind "); ok {
			behind, _ = strconv.Atoi(n)
		}
	}
	return ahead, behind, false
}

// toRefs implements core.API's Refs. Buckets parsed by refname prefix
// (for-each-ref's output is sorted globally by refname, not grouped by
// input pattern order — bucketing, not slicing by position, is
// required), grouping refs/remotes/<remote>/... by <remote> in
// first-seen order (each group's own branches, and each bucket as a
// whole, arrive already alphabetically sorted, so no separate sort step
// is needed).
func toRefs(parsed []parsedRef) core.Refs {
	var refs core.Refs
	remotes := map[string]*core.Remote{}
	var order []string

	for _, p := range parsed {
		switch {
		case strings.HasPrefix(p.Name, "refs/heads/"):
			refs.Branches = append(refs.Branches, toBranch(p))
		case strings.HasPrefix(p.Name, "refs/tags/"):
			refs.Tags = append(refs.Tags, core.Tag{Name: strings.TrimPrefix(p.Name, "refs/tags/"), SHA: p.SHA})
		case strings.HasPrefix(p.Name, "refs/remotes/"):
			if remote, branch, ok := splitRemoteBranch(p.Name); ok {
				addRemoteBranch(remotes, &order, remote, branch, p.SHA)
			}
		}
	}

	for _, name := range order {
		refs.Remotes = append(refs.Remotes, *remotes[name])
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

// splitRemoteBranch splits "refs/remotes/<remote>/<branch>" into remote
// and branch.
func splitRemoteBranch(name string) (remote, branch string, ok bool) {
	rest, ok := strings.CutPrefix(name, "refs/remotes/")
	if !ok {
		return "", "", false
	}
	remote, branch, ok = strings.Cut(rest, "/")
	return remote, branch, ok
}

func addRemoteBranch(remotes map[string]*core.Remote, order *[]string, remoteName, branch, sha string) {
	r, ok := remotes[remoteName]
	if !ok {
		r = &core.Remote{Name: remoteName}
		remotes[remoteName] = r
		*order = append(*order, remoteName)
	}
	r.Branches = append(r.Branches, core.RemoteBranch{Name: branch, SHA: sha})
}

// loadRefs runs one for-each-ref call over patterns and parses it.
func loadRefs(ctx context.Context, r git.Runner, dir string, patterns ...string) ([]parsedRef, error) {
	result, err := r.Run(ctx, git.Call{
		Dir: dir, ReadOnly: true,
		Args: append([]string{"for-each-ref", "--format=" + refsFormat}, patterns...),
	})
	if err != nil {
		return nil, err
	}
	return parseRefRecords(result.Stdout), nil
}

// Refs implements core.API.
func (c *Core) Refs(ctx context.Context, id core.WorktreeID) (core.Refs, error) {
	wt, err := c.sessions.Get(id)
	if err != nil {
		return core.Refs{}, err
	}
	parsed, err := loadRefs(ctx, wt.Runner, wt.Path, "refs/heads", "refs/remotes", "refs/tags")
	if err != nil {
		return core.Refs{}, err
	}
	return toRefs(parsed), nil
}
