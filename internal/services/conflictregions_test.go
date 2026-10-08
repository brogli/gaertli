package services

import (
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CONF_05_ParseMarkerBlocksInEachStyle(t *testing.T) {
	t.Parallel()
	merge := "pre\n<<<<<<< HEAD\ncur\n=======\ninc\n>>>>>>> side\npost\n"
	diff3 := "pre\n<<<<<<< HEAD\ncur\n||||||| base\nbas\n=======\ninc\n>>>>>>> side\npost\n"
	for style, text := range map[string]string{"merge": merge, "diff3": diff3, "zdiff3": diff3} {
		t.Run(style, func(t *testing.T) {
			t.Parallel()
			blocks := parseMarkerBlocks(text, 7)
			if len(blocks) != 1 {
				t.Fatalf("blocks = %d, want 1", len(blocks))
			}
			b := blocks[0]
			wantClose := len(splitLines(text)) - 2
			if b.open != 1 || b.close != wantClose {
				t.Errorf("open, close = %d, %d, want 1, %d", b.open, b.close, wantClose)
			}
			if !reflect.DeepEqual(b.current, []string{"cur\n"}) || !reflect.DeepEqual(b.incoming, []string{"inc\n"}) {
				t.Errorf("current, incoming = %q, %q", b.current, b.incoming)
			}
		})
	}
}

func TestFR_CONF_05_ParseMarkerBlocksHonoursTheMarkerSize(t *testing.T) {
	t.Parallel()
	text := "<<<<<<<<<< HEAD\ncur\n==========\ninc\n>>>>>>>>>> side\n<<<<<<< x\n=======\n>>>>>>> y\n"

	blocks := parseMarkerBlocks(text, 10)

	if len(blocks) != 1 || blocks[0].open != 0 || blocks[0].close != 4 {
		t.Errorf("blocks = %+v, want one block over lines 0..4", blocks)
	}
}

func TestFR_CONF_05_ParseMarkerBlocksIgnoresIncompleteBlocks(t *testing.T) {
	t.Parallel()
	text := "<<<<<<< HEAD\ncur\n=======\ninc\n<<<<<<< HEAD\na\n=======\nb\n>>>>>>> side\n<<<<<<< HEAD\nno end\n"

	blocks := parseMarkerBlocks(text, 7)

	if len(blocks) != 1 || blocks[0].open != 4 || blocks[0].close != 8 {
		t.Errorf("blocks = %+v, want only the complete block, lines 4..8", blocks)
	}
}

func TestFR_CONF_05_ParseMarkerBlocksIgnoresALoneSeparator(t *testing.T) {
	t.Parallel()

	blocks := parseMarkerBlocks("title\n=======\ntext\n<<<<<<<< not a marker\n", 7)

	if len(blocks) != 0 {
		t.Errorf("blocks = %+v, want none", blocks)
	}
}

// selected returns the lines r selects in content.
func selected(content string, r core.LineRange) string {
	lines := splitLines(content)
	start, end := edges(r)
	if start < 0 || end > len(lines) {
		return "<out of range>"
	}
	return strings.Join(lines[start:end], "")
}

// wantRegion is the base, current and incoming lines a conflict region
// selects.
type wantRegion struct{ base, current, incoming string }

// assertRegions fails unless got's conflict regions select want's lines, in
// order, each over one marker block of the result.
func assertRegions(t *testing.T, got core.ConflictFile, want ...wantRegion) {
	t.Helper()
	regions := conflictRegionsOf(got)
	if len(regions) != len(want) {
		t.Fatalf("regions = %d, want %d; result:\n%s", len(regions), len(want), got.Result)
	}
	for i, r := range regions {
		if s := selected(got.Base.Content, r.Base); s != want[i].base {
			t.Errorf("region %d base = %q (%+v), want %q", i, s, r.Base, want[i].base)
		}
		if s := selected(got.Current.Content, r.Current); s != want[i].current {
			t.Errorf("region %d current = %q (%+v), want %q", i, s, r.Current, want[i].current)
		}
		if s := selected(got.Incoming.Content, r.Incoming); s != want[i].incoming {
			t.Errorf("region %d incoming = %q (%+v), want %q", i, s, r.Incoming, want[i].incoming)
		}
		block := selected(got.Result, r.Result)
		if !strings.HasPrefix(block, "<<<<<<<") || strings.Count(block, ">>>>>>>") != 1 || !strings.HasSuffix(block, "\n") {
			t.Errorf("region %d result = %q, want one marker block", i, block)
		}
	}
}

const (
	twoConflictsBase     = "top\nx1\nf1\nf2\nf3\nf4\nmid\nf5\nf6\nf7\nf8\nx2\nbottom\n"
	twoConflictsCurrent  = "top\nc1\nf1\nf2\nf3\nf4\nmid\nf5\nf6\nf7\nf8\nc2a\nc2b\nbottom\n"
	twoConflictsIncoming = "top\ni1\nf1\nf2\nf3\nf4\nM1\nM2\nM3\nf5\nf6\nf7\nf8\ni2\nbottom\n"
)

func TestFR_CONF_05_ConflictRegionsWithZdiff3AndAnIncomingOnlyChangeBetween(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version(twoConflictsBase), version(twoConflictsCurrent), version(twoConflictsIncoming))
	gittest.Run(t, dir, "config", "merge.conflictStyle", "zdiff3")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"x1\n", "c1\n", "i1\n"}, wantRegion{"x2\n", "c2a\nc2b\n", "i2\n"})
}

func TestFR_CONF_05_ConflictRegionsSkipABlockTheUserResolved(t *testing.T) {
	t.Parallel()
	c, wt, dir := stoppedOn(t, "merge", "a.txt", version(twoConflictsBase), version(twoConflictsCurrent), version(twoConflictsIncoming))
	path := filepath.Join(dir, "a.txt")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(data)
	afterFirst := strings.Index(text, ">>>>>>>")
	afterFirst += strings.Index(text[afterFirst:], "\n") + 1
	if err := os.WriteFile(path, []byte("top\nmine\n"+text[afterFirst:]), 0o644); err != nil {
		t.Fatal(err)
	}

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"x2\n", "c2a\nc2b\n", "i2\n"})
}

func TestFR_CONF_05_ConflictRegionsUseTheMarkerSizeAttribute(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, ".gitattributes", version("* conflict-marker-size=10\n"), "attributes")
	commitVersion(t, dir, "a.txt", version("one\nbase\ntwo\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("one\nincoming\ntwo\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one\ncurrent\ntwo\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	if got.MarkerSize != 10 {
		t.Errorf("MarkerSize = %d, want 10", got.MarkerSize)
	}
	assertRegions(t, got, wantRegion{"base\n", "current\n", "incoming\n"})
}

func TestFR_CONF_05_ConflictRegionsIgnoreTheDiffAttribute(t *testing.T) {
	t.Parallel()
	dir := gittest.New(t).Dir
	commitVersion(t, dir, ".gitattributes", version("* -diff\n"), "attributes")
	commitVersion(t, dir, "a.txt", version("one\nbase\ntwo\n"), "base")
	gittest.Run(t, dir, "checkout", "-q", "-b", "side")
	commitVersion(t, dir, "a.txt", version("one\nincoming\ntwo\n"), "incoming change")
	gittest.Run(t, dir, "checkout", "-q", "master")
	commitVersion(t, dir, "a.txt", version("one\ncurrent\ntwo\n"), "current change")
	tryGit(t, dir, "merge", "side")
	c, wt := openWorktree(t, dir)

	got := readConflict(t, c, wt, "a.txt")

	assertRegions(t, got, wantRegion{"base\n", "current\n", "incoming\n"})
}

// conflictRegionsOf returns the regions of f that are conflicts.
func conflictRegionsOf(f core.ConflictFile) []core.ConflictRegion {
	return slices.DeleteFunc(slices.Clone(f.Regions), func(r core.ConflictRegion) bool { return r.Kind != core.RegionConflict })
}
