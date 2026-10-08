package services

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_LOG_01_ParseTrackParsesBracketedForms(t *testing.T) {
	t.Parallel()
	tests := []struct {
		track                 string
		wantAhead, wantBehind int
		wantGone              bool
	}{
		{"", 0, 0, false},
		{"[ahead 2]", 2, 0, false},
		{"[behind 3]", 0, 3, false},
		{"[ahead 2, behind 3]", 2, 3, false},
		{"[gone]", 0, 0, true},
	}
	for _, tt := range tests {
		t.Run(tt.track, func(t *testing.T) {
			ahead, behind, gone := parseTrack(tt.track)
			if ahead != tt.wantAhead || behind != tt.wantBehind || gone != tt.wantGone {
				t.Errorf("parseTrack(%q) = (%d, %d, %v), want (%d, %d, %v)",
					tt.track, ahead, behind, gone, tt.wantAhead, tt.wantBehind, tt.wantGone)
			}
		})
	}
}

func TestFR_LOG_01_ParseRefRecordsThenToRefsBucketsAndDereferencesTags(t *testing.T) {
	t.Parallel()
	const (
		shaMain        = "1111111111111111111111111111111111111111"
		shaFeature     = "2222222222222222222222222222222222222222"
		shaOriginMain  = "3333333333333333333333333333333333333333"
		shaOriginFeat  = "4444444444444444444444444444444444444444"
		shaLightweight = "5555555555555555555555555555555555555555"
		shaTagObject   = "6666666666666666666666666666666666666666"
		shaAnnotated   = "7777777777777777777777777777777777777777"
	)
	records := []ref{
		{"refs/heads/main", shaMain, "", "origin/main", "[ahead 1, behind 1]", "*", ""},
		{"refs/heads/feature", shaFeature, "", "", "", "", ""},
		{"refs/remotes/origin/HEAD", shaOriginMain, "", "", "", "", ""}, // the remote's symbolic HEAD: not a branch
		{"refs/remotes/origin/main", shaOriginMain, "", "", "", "", ""},
		{"refs/remotes/origin/feature", shaOriginFeat, "", "", "", "", ""},
		{"refs/tags/v1", shaLightweight, "", "", "", "", ""},
		{"refs/tags/v2", shaTagObject, shaAnnotated, "", "", "", ""},
	}
	out := buildForEachRefFixture(records)

	refs := toRefs(parseRefRecords(out), nil)

	if len(refs.Branches) != 2 {
		t.Fatalf("Branches = %+v, want 2 entries", refs.Branches)
	}
	main, feature := refs.Branches[0], refs.Branches[1]
	if main.Name != "main" || !main.Current || main.Upstream != "origin/main" || main.Ahead != 1 || main.Behind != 1 {
		t.Errorf("main = %+v, want Current true, Upstream origin/main, Ahead 1, Behind 1", main)
	}
	if feature.Name != "feature" || feature.Current || feature.Upstream != "" {
		t.Errorf("feature = %+v, want Current false, no Upstream", feature)
	}

	if len(refs.Remotes) != 1 || refs.Remotes[0].Name != "origin" || len(refs.Remotes[0].Branches) != 2 {
		t.Fatalf("Remotes = %+v, want one origin group with 2 branches", refs.Remotes)
	}

	if len(refs.Tags) != 2 {
		t.Fatalf("Tags = %+v, want 2 entries", refs.Tags)
	}
	lightweight, annotated := refs.Tags[0], refs.Tags[1]
	if lightweight.SHA != shaLightweight {
		t.Errorf("lightweight tag SHA = %q, want %q", lightweight.SHA, shaLightweight)
	}
	if annotated.SHA != shaAnnotated {
		t.Errorf("annotated tag SHA = %q, want the dereferenced commit %q, not the tag object", annotated.SHA, shaAnnotated)
	}
}

// ref is one for-each-ref record's fields, in refsFormat's own order.
type ref struct {
	name, sha, deref, upstream, track, head, worktree string
}

// buildForEachRefFixture renders records the way for-each-ref's own
// %1f/%00 framing does: each record ends in "\x00\n".
func buildForEachRefFixture(records []ref) []byte {
	var b strings.Builder
	for _, r := range records {
		b.WriteString(strings.Join([]string{r.name, r.sha, r.deref, r.upstream, r.track, r.head, r.worktree}, "\x1f"))
		b.WriteString("\x00\n")
	}
	return []byte(b.String())
}

func TestFR_LOG_01_RefsListsEveryRefShape(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := gittest.HeadSHA(t, repo.Dir)
	bare := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "master")
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")
	gittest.Commit(t, linked, "local.txt", "local", "local-only")
	remoteAppendCommitOnBranch(t, bare, "feature", "remote.txt", "remote", "remote-only")
	gittest.Run(t, bare, "update-ref", "-d", "refs/heads/master")
	gittest.Run(t, repo.Dir, "fetch", "--prune", "origin")
	gittest.Run(t, repo.Dir, "tag", "v1")
	gittest.Run(t, repo.Dir, "remote", "add", "empty", "https://example.invalid/empty.git")
	gittest.Run(t, repo.Dir, "remote", "add", "team", "https://example.invalid/team.git")
	// A newer git's "remote add" refuses a name nested under an existing
	// remote's; a repo set up by an older git or by hand still has both.
	gittest.Run(t, repo.Dir, "config", "remote.team/x.url", "https://example.invalid/x.git")
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/team/main", sha)
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/team/x/master", sha)
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/gone/topic", sha) // a remote no longer configured

	c, wt := openWorktree(t, repo.Dir)
	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("branches and the current one", func(t *testing.T) {
		if len(refs.Branches) != 2 {
			t.Fatalf("Branches = %+v, want 2 entries", refs.Branches)
		}
		if current := findBranch(refs.Branches, "master"); current == nil || !current.Current {
			t.Errorf("Branches = %+v, want master marked Current", refs.Branches)
		}
		if other := findBranch(refs.Branches, "feature"); other == nil || other.Current {
			t.Errorf("Branches = %+v, want feature not Current", refs.Branches)
		}
	})
	t.Run("worktree holding a branch", func(t *testing.T) {
		want := map[string]string{"master": gitPath(t, repo.Dir), "feature": gitPath(t, linked)}
		for _, b := range refs.Branches {
			if b.CheckedOutIn != want[b.Name] {
				t.Errorf("branch %q CheckedOutIn = %q, want %q", b.Name, b.CheckedOutIn, want[b.Name])
			}
		}
	})
	t.Run("ahead and behind upstream", func(t *testing.T) {
		feature := findBranch(refs.Branches, "feature")
		if feature == nil || feature.Upstream != "origin/feature" || feature.Ahead != 1 || feature.Behind != 1 {
			t.Errorf("feature = %+v, want Upstream origin/feature, Ahead 1, Behind 1", feature)
		}
	})
	t.Run("upstream gone after prune", func(t *testing.T) {
		if master := findBranch(refs.Branches, "master"); master == nil || !master.UpstreamGone {
			t.Errorf("master = %+v, want UpstreamGone true", master)
		}
	})
	t.Run("remotes, including empty, slashed and unconfigured ones", func(t *testing.T) {
		got := map[string][]string{}
		for _, r := range refs.Remotes {
			got[r.Name] = []string{}
			for _, b := range r.Branches {
				got[r.Name] = append(got[r.Name], b.Name)
			}
		}
		want := map[string][]string{
			"origin": {"feature"}, "empty": {}, "gone": {"topic"}, "team": {"main"}, "team/x": {"master"},
		}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("remote branches = %v, want %v", got, want)
		}
	})
	t.Run("tags", func(t *testing.T) {
		if len(refs.Tags) != 1 || refs.Tags[0].Name != "v1" {
			t.Errorf("Tags = %+v, want [v1]", refs.Tags)
		}
	})
}

func TestFR_LOG_01_RefsOnUnbornHEADReturnsEmptyListsNoError(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs.Branches) != 0 || len(refs.Remotes) != 0 || len(refs.Tags) != 0 {
		t.Errorf("Refs = %+v, want all empty", refs)
	}
}

func findBranch(branches []core.Branch, name string) *core.Branch {
	for i, b := range branches {
		if b.Name == name {
			return &branches[i]
		}
	}
	return nil
}

// TestFR_BR_05_RefsMarksBranchesBeingRebasedOrBisectedElsewhere covers the
// four linked worktrees a branch can be held by (and Y asking about itself): X stopped mid-rebase of B,
// Y mid-bisect of C, Z mid-rebase from a detached HEAD (not attributable to
// any branch), and W with D plainly checked out.
func TestFR_BR_05_RefsMarksBranchesBeingRebasedOrBisectedElsewhere(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	seedSHA := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "B")
	gittest.Run(t, repo.Dir, "branch", "C")
	gittest.Run(t, repo.Dir, "branch", "D")
	gittest.Run(t, repo.Dir, "branch", "E") // never checked out or touched

	xDir := filepath.Join(t.TempDir(), "x")
	gittest.Run(t, repo.Dir, "worktree", "add", xDir, "B")
	gittest.Commit(t, xDir, "conflict.txt", "b-side\n", "b change")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master-side\n", "master change")
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: xDir, Args: []string{"rebase", "master"}}); err == nil {
		t.Fatal("X's rebase did not conflict as the fixture expects")
	}

	yDir := filepath.Join(t.TempDir(), "y")
	gittest.Run(t, repo.Dir, "worktree", "add", yDir, "C")
	gittest.Commit(t, yDir, "c1.txt", "1", "c1")
	gittest.Commit(t, yDir, "c2.txt", "2", "c2")
	gittest.Run(t, yDir, "bisect", "start")
	gittest.Run(t, yDir, "bisect", "bad", "HEAD")
	gittest.Run(t, yDir, "bisect", "good", "HEAD~2")

	wDir := filepath.Join(t.TempDir(), "w")
	gittest.Run(t, repo.Dir, "worktree", "add", wDir, "D")

	zDir := filepath.Join(t.TempDir(), "z")
	gittest.Run(t, repo.Dir, "worktree", "add", "--detach", zDir, seedSHA)
	gittest.Commit(t, zDir, "conflict2.txt", "z-side\n", "z change")
	gittest.Commit(t, wDir, "conflict2.txt", "d-side\n", "d change")
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: zDir, Args: []string{"rebase", "D"}}); err == nil {
		t.Fatal("Z's rebase did not conflict as the fixture expects")
	}

	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	b, cb, d, e := findBranch(refs.Branches, "B"), findBranch(refs.Branches, "C"), findBranch(refs.Branches, "D"), findBranch(refs.Branches, "E")
	if b == nil || b.CheckedOutIn != gitPath(t, xDir) || b.InUse != core.BranchRebasing {
		t.Errorf("B = %+v, want CheckedOutIn %s, InUse rebase", b, gitPath(t, xDir))
	}
	if cb == nil || cb.CheckedOutIn != gitPath(t, yDir) || cb.InUse != core.BranchBisecting {
		t.Errorf("C = %+v, want CheckedOutIn %s, InUse bisect", cb, gitPath(t, yDir))
	}
	if d == nil || d.CheckedOutIn != gitPath(t, wDir) || d.InUse != "" {
		t.Errorf("D = %+v, want CheckedOutIn %s, InUse \"\"", d, gitPath(t, wDir))
	}
	if e == nil || e.CheckedOutIn != "" || e.InUse != "" {
		t.Errorf("E = %+v, want CheckedOutIn and InUse empty", e)
	}

	// Y's own bisect is not "elsewhere" from Y's point of view.
	cY, wtY := openWorktree(t, yDir)
	refsFromY, err := cY.Refs(t.Context(), wtY)
	if err != nil {
		t.Fatal(err)
	}
	if cb := findBranch(refsFromY.Branches, "C"); cb == nil || cb.CheckedOutIn != "" || cb.InUse != "" {
		t.Errorf("C from Y's own Refs = %+v, want CheckedOutIn and InUse empty", cb)
	}
}

// TestFR_BR_05_RefsMarksBothARebaseAndAStaleBisectFromOneWorktree covers a
// worktree that started a bisect on C, then moved on: first a plain checkout
// of D (not detached, C's BISECT_START left stale), then a rebase of D that
// conflicts (detached again). Both checkpoints must still show C bisecting.
func TestFR_BR_05_RefsMarksBothARebaseAndAStaleBisectFromOneWorktree(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "C")
	gittest.Run(t, repo.Dir, "branch", "D")
	gittest.Commit(t, repo.Dir, "conflict.txt", "master-side\n", "master change") // master diverges from D

	wtDir := filepath.Join(t.TempDir(), "wt")
	gittest.Run(t, repo.Dir, "worktree", "add", wtDir, "C")
	gittest.Commit(t, wtDir, "c1.txt", "1", "c1")
	gittest.Commit(t, wtDir, "c2.txt", "2", "c2")
	gittest.Run(t, wtDir, "bisect", "start")
	gittest.Run(t, wtDir, "bisect", "bad", "HEAD")
	gittest.Run(t, wtDir, "bisect", "good", "HEAD~2")
	gittest.Run(t, wtDir, "switch", "D")
	wtPath := gitPath(t, wtDir)

	c, wt := openWorktree(t, repo.Dir)
	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	cb, d := findBranch(refs.Branches, "C"), findBranch(refs.Branches, "D")
	if cb == nil || cb.CheckedOutIn != wtPath || cb.InUse != core.BranchBisecting {
		t.Errorf("C (not detached, stale bisect) = %+v, want CheckedOutIn %s, InUse bisect", cb, wtPath)
	}
	if d == nil || d.CheckedOutIn != wtPath || d.InUse != "" {
		t.Errorf("D (plainly checked out) = %+v, want CheckedOutIn %s, InUse \"\"", d, wtPath)
	}

	gittest.Commit(t, wtDir, "conflict.txt", "wt-side\n", "wt change")
	if _, err := (git.Runner{}).Run(t.Context(), git.Call{Dir: wtDir, Args: []string{"rebase", "master"}}); err == nil {
		t.Fatal("wt's rebase did not conflict as the fixture expects")
	}

	refs, err = c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	cb, d = findBranch(refs.Branches, "C"), findBranch(refs.Branches, "D")
	if cb == nil || cb.CheckedOutIn != wtPath || cb.InUse != core.BranchBisecting {
		t.Errorf("C (stale bisect, wt now rebasing D) = %+v, want CheckedOutIn %s, InUse bisect", cb, wtPath)
	}
	if d == nil || d.CheckedOutIn != wtPath || d.InUse != core.BranchRebasing {
		t.Errorf("D (mid-rebase) = %+v, want CheckedOutIn %s, InUse rebase", d, wtPath)
	}
}
