package services

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
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

func TestFR_LOG_01_RefsListsBranchesRemotesAndTags(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := gittest.HeadSHA(t, repo.Dir)
	gittest.Run(t, repo.Dir, "branch", "feature")
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/origin/master", sha)
	gittest.Run(t, repo.Dir, "update-ref", "refs/remotes/origin/feature", sha)
	gittest.Run(t, repo.Dir, "tag", "v1")

	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	if len(refs.Branches) != 2 {
		t.Fatalf("Branches = %+v, want 2 entries", refs.Branches)
	}
	current := findBranch(refs.Branches, "master")
	if current == nil || !current.Current {
		t.Errorf("Branches = %+v, want master marked Current", refs.Branches)
	}
	other := findBranch(refs.Branches, "feature")
	if other == nil || other.Current {
		t.Errorf("Branches = %+v, want feature not Current", refs.Branches)
	}

	if len(refs.Remotes) != 1 || refs.Remotes[0].Name != "origin" || len(refs.Remotes[0].Branches) != 2 {
		t.Errorf("Remotes = %+v, want one origin group with 2 branches", refs.Remotes)
	}
	if len(refs.Tags) != 1 || refs.Tags[0].Name != "v1" {
		t.Errorf("Tags = %+v, want [v1]", refs.Tags)
	}
}

func TestFR_LOG_01_RefsMarksTheWorktreeWithABranchCheckedOut(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "branch", "feature")
	linked := filepath.Join(t.TempDir(), "linked")
	gittest.Run(t, repo.Dir, "worktree", "add", linked, "feature")

	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]string{"master": gitPath(t, repo.Dir), "feature": gitPath(t, linked)}
	for _, b := range refs.Branches {
		if b.CheckedOutIn != want[b.Name] {
			t.Errorf("branch %q CheckedOutIn = %q, want %q", b.Name, b.CheckedOutIn, want[b.Name])
		}
	}
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

func TestFR_REPO_03_RefsReportsAheadAndBehindUpstream(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	gittest.Run(t, repo.Dir, "switch", "-c", "feature")
	bare := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "feature")
	gittest.Commit(t, repo.Dir, "local.txt", "local", "local-only")
	remoteAppendCommitOnBranch(t, bare, "feature", "remote.txt", "remote", "remote-only")
	gittest.Run(t, repo.Dir, "fetch", "origin")

	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	feature := findBranch(refs.Branches, "feature")
	if feature == nil {
		t.Fatalf("Branches = %+v, want a feature entry", refs.Branches)
	}
	if feature.Upstream != "origin/feature" || feature.Ahead != 1 || feature.Behind != 1 {
		t.Errorf("feature = %+v, want Upstream origin/feature, Ahead 1, Behind 1", feature)
	}
}

func TestFR_REPO_03_RefsReportsUpstreamGoneAfterPrune(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	bare := bareRemote(t, repo, "origin")
	gittest.Run(t, repo.Dir, "push", "-u", "origin", "master")
	gittest.Run(t, bare, "update-ref", "-d", "refs/heads/master")
	gittest.Run(t, repo.Dir, "fetch", "--prune", "origin")

	c, wt := openWorktree(t, repo.Dir)

	refs, err := c.Refs(t.Context(), wt)
	if err != nil {
		t.Fatal(err)
	}
	branch := findBranch(refs.Branches, "master")
	if branch == nil || !branch.UpstreamGone {
		t.Errorf("master = %+v, want UpstreamGone true", branch)
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

func TestFR_LOG_01_RefsGroupsARemoteWhoseNameHasASlashUnderItsWholeName(t *testing.T) {
	t.Parallel()
	repo := gittest.New(t)
	gittest.Commit(t, repo.Dir, "seed.txt", "x", "seed")
	sha := gittest.HeadSHA(t, repo.Dir)
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

	got := map[string][]string{}
	for _, r := range refs.Remotes {
		for _, b := range r.Branches {
			got[r.Name] = append(got[r.Name], b.Name)
		}
	}
	want := map[string][]string{"gone": {"topic"}, "team": {"main"}, "team/x": {"master"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("remote branches = %v, want %v", got, want)
	}
}
