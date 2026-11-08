// Requirements: FR-CONF-11

package services

import (
	"reflect"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
)

// sampleMergeDraft is a draft made for path's conflict as it is now.
func sampleMergeDraft(t *testing.T, c *Core, wt core.WorktreeID, path string) *core.MergeDraft {
	t.Helper()
	return &core.MergeDraft{
		Text: "one\nmine\n", Fingerprint: "fp", ConflictID: readConflict(t, c, wt, path).ConflictID,
		Regions: []core.DraftRegion{{Range: core.LineRange{Start: 2, Lines: 1}, Resolved: true}},
	}
}

func TestMergeDraftReadsBackWhatWasSet(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("one\nbase\n"), version("one\ncurrent\n"), version("one\nincoming\n"))
	want := sampleMergeDraft(t, c, wt, "a.txt")

	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", want); err != nil {
		t.Fatal(err)
	}
	got, err := c.MergeDraft(t.Context(), wt, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("MergeDraft = %+v, want %+v", got, want)
	}

	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", nil); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.MergeDraft(t.Context(), wt, "a.txt"); got != nil {
		t.Errorf("MergeDraft after clearing = %+v, want nil", got)
	}
}

func TestMergeDraftIsDroppedOnceTheFileIsNoLongerConflicted(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("one\nbase\n"), version("one\ncurrent\n"), version("one\nincoming\n"))
	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", sampleMergeDraft(t, c, wt, "a.txt")); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "add", "a.txt")

	got, err := c.MergeDraft(t.Context(), wt, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("MergeDraft = %+v, want nil", got)
	}
}

func TestMergeDraftOfAnEarlierConflictIsDropped(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version("one\nbase\n"), version("one\ncurrent\n"), version("one\nincoming\n"))
	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", sampleMergeDraft(t, c, wt, "a.txt")); err != nil {
		t.Fatal(err)
	}
	gittest.Run(t, dir, "merge", "--abort")
	gittest.Run(t, dir, "checkout", "-q", "-b", "other", "master~1")
	commitVersion(t, dir, "a.txt", version("one\nother\n"), "other change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	tryGit(t, dir, "merge", "other")
	if unmergedList(t, dir, "a.txt") == "" {
		t.Fatal("a.txt is not conflicted in the second merge")
	}

	got, err := c.MergeDraft(t.Context(), wt, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("MergeDraft = %+v, want nil", got)
	}
}

func TestMergeDraftWrittenLateForAnEarlierConflictIsRefused(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, "a.txt", version("one\ntwo\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("ONE\nTWO\n"), "side change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one-m\ntwo\n"), "first change")
	commitVersion(t, dir, "a.txt", version("one-m\ntwo-m\n"), "second change")
	tryGit(t, dir, "rebase", "side")
	c, wt := openWorktree(t, dir)
	draft := sampleMergeDraft(t, c, wt, "a.txt")
	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", draft); err != nil {
		t.Fatal(err)
	}
	tryGit(t, dir, "rebase", "--skip")
	if unmergedList(t, dir, "a.txt") == "" {
		t.Fatal("a.txt is not conflicted in the second step")
	}
	if got, _ := c.MergeDraft(t.Context(), wt, "a.txt"); got != nil {
		t.Fatalf("MergeDraft after the skip = %+v, want nil", got)
	}

	if err := c.SetMergeDraft(t.Context(), wt, "a.txt", draft); err != nil {
		t.Fatal(err)
	}

	got, err := c.MergeDraft(t.Context(), wt, "a.txt")
	if err != nil {
		t.Fatal(err)
	}
	if got != nil {
		t.Errorf("MergeDraft after a late write = %+v, want nil", got)
	}
}
