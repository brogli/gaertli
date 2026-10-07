package services

import (
	"reflect"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CONF_05_ConflictSideChangesMarkWhereASideDiffersFromTheBase(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\n"
	current := "l1\nc2\nl3\nl4\nc5\n"
	incoming := "l1\nl2\nl3\nl4\ni5\nend\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version(current), version(incoming))

	got := readConflict(t, c, wt, "a.txt")

	wantCurrent := []core.SideChange{{Lines: core.LineRange{Start: 2, Lines: 1}}, {Lines: core.LineRange{Start: 5, Lines: 1}}}
	if !reflect.DeepEqual(got.CurrentChanges, wantCurrent) {
		t.Errorf("CurrentChanges = %+v, want %+v", got.CurrentChanges, wantCurrent)
	}
	wantIncoming := []core.SideChange{{Lines: core.LineRange{Start: 5, Lines: 2}}}
	if !reflect.DeepEqual(got.IncomingChanges, wantIncoming) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, wantIncoming)
	}
	if len(conflictRegionsOf(got)) != 1 || conflictRegionsOf(got)[0].Current.Start != 5 || conflictRegionsOf(got)[0].Incoming.Start != 5 {
		t.Errorf("Regions = %+v, want one at line 5 in both sides", conflictRegionsOf(got))
	}
}

func TestFR_CONF_05_ConflictSideChangesMarkAddedLines(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("l1\nl2\nl3\n"), version("l1\nl2\nc3\n"), version("l1\nnew\nl2\ni3\n"))

	got := readConflict(t, c, wt, "a.txt")

	want := []core.SideChange{{Lines: core.LineRange{Start: 2, Lines: 1}, Added: true}, {Lines: core.LineRange{Start: 4, Lines: 1}}}
	if !reflect.DeepEqual(got.IncomingChanges, want) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, want)
	}
}

func TestFR_CONF_05_ConflictSideChangesWithoutABaseAreTheWholeSide(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, "other.txt", version("x\n"), "start")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("i1\ni2\n"), "incoming add")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("c1\nc2\nc3\n"), "current add")
	tryGit(t, dir, "merge", "side")
	c := New(func(core.Event) {})

	got := readConflict(t, c, openOn(t, c, dir).ID, "a.txt")

	if got.Base.Present {
		t.Fatal("base present, want an add/add conflict")
	}
	if want := []core.SideChange{{Lines: core.LineRange{Start: 1, Lines: 3}, Added: true}}; !reflect.DeepEqual(got.CurrentChanges, want) {
		t.Errorf("CurrentChanges = %+v, want %+v", got.CurrentChanges, want)
	}
	if want := []core.SideChange{{Lines: core.LineRange{Start: 1, Lines: 2}, Added: true}}; !reflect.DeepEqual(got.IncomingChanges, want) {
		t.Errorf("IncomingChanges = %+v, want %+v", got.IncomingChanges, want)
	}
}

func TestFR_CONF_05_ConflictRegionsListEveryChangeWithItsKindAndRanges(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\nl6\nl7\nl8\nl9\nl10\n"
	current := "l1\nc2\nl3\nl4\nc5\nl6\nl7\nl8\nl9\nl10\n"
	incoming := "l1\nl2\nl3\nl4\ni5\nl6\nl7\ni8\nl9\nl10\nend\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version(current), version(incoming))

	got := readConflict(t, c, wt, "a.txt")

	rng := func(start, lines int) core.LineRange { return core.LineRange{Start: start, Lines: lines} }
	want := []core.ConflictRegion{
		{Kind: core.RegionCurrent, Result: rng(2, 1), Base: rng(2, 1), Current: rng(2, 1), Incoming: rng(2, 1)},
		{Kind: core.RegionConflict, Result: rng(5, 5), Base: rng(5, 1), Current: rng(5, 1), Incoming: rng(5, 1)},
		{Kind: core.RegionIncoming, Result: rng(12, 1), Base: rng(8, 1), Current: rng(8, 1), Incoming: rng(8, 1)},
		{Kind: core.RegionIncoming, Added: true, Result: rng(15, 1), Base: rng(10, 0), Current: rng(10, 0), Incoming: rng(11, 1)},
	}
	if !reflect.DeepEqual(got.Regions, want) {
		t.Errorf("Regions = %+v\nwant %+v\nresult:\n%s", got.Regions, want, got.Result)
	}
}

func TestFR_CONF_05_ConflictRegionsLeaveOutAChangeBothSidesMadeAlike(t *testing.T) {
	t.Parallel()
	base := "l1\nl2\nl3\nl4\nl5\n"
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version(base), version("l1\nsame\nl3\nl4\nc5\n"), version("l1\nsame\nl3\nl4\ni5\n"))

	got := readConflict(t, c, wt, "a.txt")

	if len(got.Regions) != 1 || got.Regions[0].Kind != core.RegionConflict {
		t.Errorf("Regions = %+v, want only the conflict", got.Regions)
	}
}
