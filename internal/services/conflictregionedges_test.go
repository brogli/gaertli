package services

import (
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git/gittest"
)

func TestFR_CONF_05_ConflictRegionBaseKeepsLinesTheCurrentSideDeleted(t *testing.T) {
	t.Parallel()
	c, wt, _ := stoppedOn(t, "merge", "a.txt", version("a\nb\nc\n"), version("a\nc\n"), version("a\nB\nc\n"))

	got := readConflict(t, c, wt, "a.txt")

	if len(conflictRegionsOf(got)) != 1 {
		t.Fatalf("regions = %d, want 1; result:\n%s", len(conflictRegionsOf(got)), got.Result)
	}
	assertRegion(t, got, 0, "b\n", "", "B\n")
}

// A separate diff of the base against a side can align repeated lines differently than git did;
// in the diff3 style the region's base is the section git wrote.
func TestFR_CONF_05_ConflictRegionBaseIsGitsBaseSectionInDiff3Style(t *testing.T) {
	t.Parallel()
	cases := map[string][3]string{
		"repeated start": {"c\nc\na\na\n", "A\nc\na\n", "D\nc\nD\nc\na\n"},
		"repeated lines": {"d\nd\nc\nc\nc\ne\n", "D\nd\nc\nC\nc\ne\n", "d\nd\nc\nc\ne\n"},
	}
	for name, texts := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := divergedRepo(t, "a.txt", version(texts[0]), version(texts[1]), version(texts[2]))
			gittest.Run(t, dir, "config", "merge.conflictStyle", "diff3")
			tryGit(t, dir, "merge", "side")
			c := New(func(core.Event) {})

			got := readConflict(t, c, openOn(t, c, dir).ID, "a.txt")

			if len(conflictRegionsOf(got)) == 0 {
				t.Fatalf("no regions; result:\n%s", got.Result)
			}
			assertBaseIsMarkerSection(t, got)
		})
	}
}

// checkout --conflict=diff3 writes the diff3 style whatever merge.conflictStyle says. Its merge
// ignores diff.algorithm, which the diffs of the base against each side follow, so only the base
// section places the region.
func TestFR_CONF_05_ConflictRegionBaseIsGitsBaseSectionInAFileRewrittenInDiff3Style(t *testing.T) {
	t.Parallel()
	dir := divergedRepo(t, "a.txt", version("c\nc\nd\nd\n"), version("B\nc\nd\n"), version("c\nc\nD\nd\n"))
	tryGit(t, dir, "merge", "side")
	gittest.Run(t, dir, "checkout", "--conflict=diff3", "--", "a.txt")
	gittest.Run(t, dir, "config", "diff.algorithm", "histogram")
	c := New(func(core.Event) {})

	got := readConflict(t, c, openOn(t, c, dir).ID, "a.txt")

	if len(conflictRegionsOf(got)) == 0 {
		t.Fatalf("no regions; result:\n%s", got.Result)
	}
	assertBaseIsMarkerSection(t, got)
}

// Both sides replaced one base line with the same three lines and one line each: no base line
// stands for the shared lines alone, so the region's base is the whole replaced line.
func TestFR_CONF_05_ConflictRegionBaseOfASharedChangeOfAnotherLengthIsTheReplacedLines(t *testing.T) {
	t.Parallel()
	for _, style := range []string{"merge", "zdiff3", "diff3"} {
		t.Run(style, func(t *testing.T) {
			t.Parallel()
			dir := divergedRepo(t, "a.txt", version("a\nX\nb\n"), version("a\nP\nQ\nR\nC\nb\n"), version("a\nP\nQ\nR\nI\nb\n"))
			gittest.Run(t, dir, "config", "merge.conflictStyle", style)
			tryGit(t, dir, "merge", "side")
			c := New(func(core.Event) {})

			got := readConflict(t, c, openOn(t, c, dir).ID, "a.txt")

			if len(conflictRegionsOf(got)) != 1 {
				t.Fatalf("regions = %d, want 1; result:\n%s", len(conflictRegionsOf(got)), got.Result)
			}
			if style == "diff3" {
				assertRegion(t, got, 0, "X\n", "P\nQ\nR\nC\n", "P\nQ\nR\nI\n")
				return
			}
			assertRegion(t, got, 0, "X\n", "C\n", "I\n")
		})
	}
}

// The merge and zdiff3 styles leave the lines both sides changed alike outside the markers; the
// region's base is the lines that differ only.
func TestFR_CONF_05_ConflictRegionBaseExcludesLinesBothSidesChangedTheSameWay(t *testing.T) {
	t.Parallel()
	// Each way writes the file in a style; the config may name another.
	ways := map[string]struct {
		write func(t *testing.T, dir string)
		diff3 bool
	}{
		"merge":  {write: mergeInStyle("merge")},
		"zdiff3": {write: mergeInStyle("zdiff3")},
		"diff3":  {write: mergeInStyle("diff3"), diff3: true},
		"zdiff3 by checkout under a diff3 config": {write: func(t *testing.T, dir string) {
			t.Helper()
			mergeInStyle("diff3")(t, dir)
			gittest.Run(t, dir, "checkout", "--conflict=zdiff3", "--", "a.txt")
		}},
		"zdiff3 for one merge under a diff3 config": {write: func(t *testing.T, dir string) {
			t.Helper()
			gittest.Run(t, dir, "config", "merge.conflictStyle", "diff3")
			tryGit(t, dir, "-c", "merge.conflictStyle=zdiff3", "merge", "side")
		}},
	}
	for name, way := range ways {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			dir := divergedRepo(t, "a.txt", version("a\nb\nc\nx\n"), version("A\nB\nC\nx\n"), version("A\nY\nC\nx\n"))
			way.write(t, dir)
			c := New(func(core.Event) {})

			got := readConflict(t, c, openOn(t, c, dir).ID, "a.txt")

			if len(conflictRegionsOf(got)) != 1 {
				t.Fatalf("regions = %d, want 1; result:\n%s", len(conflictRegionsOf(got)), got.Result)
			}
			if way.diff3 {
				assertRegion(t, got, 0, "a\nb\nc\n", "A\nB\nC\n", "A\nY\nC\n")
				return
			}
			assertRegion(t, got, 0, "b\n", "B\n", "Y\n")
		})
	}
}

// mergeInStyle merges side with merge.conflictStyle set to style.
func mergeInStyle(style string) func(t *testing.T, dir string) {
	return func(t *testing.T, dir string) {
		t.Helper()
		gittest.Run(t, dir, "config", "merge.conflictStyle", style)
		tryGit(t, dir, "merge", "side")
	}
}
