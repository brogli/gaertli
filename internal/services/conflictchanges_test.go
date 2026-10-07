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
	if len(got.Regions) != 1 || got.Regions[0].Current.Start != 5 || got.Regions[0].Incoming.Start != 5 {
		t.Errorf("Regions = %+v, want one at line 5 in both sides", got.Regions)
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
