// Requirements: FR-CONF-05, FR-CONF-10

package services

import (
	"math/rand"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/brogli/gaertli/internal/git/gittest"
)

// mutateLines inserts, deletes and replaces a few lines.
func mutateLines(r *rand.Rand, lines []string) []string {
	out := append([]string(nil), lines...)
	letter := func() string { return string(rune('A' + r.Intn(4))) }
	for n := r.Intn(3) + 1; n > 0; n-- {
		i := r.Intn(len(out) + 1)
		switch r.Intn(3) {
		case 0:
			out = append(out[:i], append([]string{letter()}, out[i:]...)...)
		case 1:
			if i < len(out) {
				out = append(out[:i], out[i+1:]...)
			}
		case 2:
			if i < len(out) {
				out[i] = letter()
			}
		}
	}
	return out
}

func joinLines(lines []string) string {
	if len(lines) == 0 {
		return ""
	}
	return strings.Join(lines, "\n") + "\n"
}

// assertBaseIsMarkerSection checks that each region's base lines are the base section git wrote
// in its marker block.
func assertBaseIsMarkerSection(t *testing.T, got core.ConflictFile) {
	t.Helper()
	blocks := parseMarkerBlocks(got.Result, got.MarkerSize)
	for i, region := range conflictRegionsOf(got) {
		if have, want := selected(got.Base.Content, region.Base), strings.Join(blocks[i].base, ""); have != want {
			t.Errorf("base %q, current %q, incoming %q: region %d base = %q, git's base section %q",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, have, want)
		}
	}
}

// assertBaseInBounds checks that each region's base lines lie within the base.
func assertBaseInBounds(t *testing.T, got core.ConflictFile) {
	t.Helper()
	lines := len(splitLines(got.Base.Content))
	for i, region := range conflictRegionsOf(got) {
		if start, end := edges(region.Base); start < 0 || end > lines {
			t.Errorf("base %q, current %q, incoming %q: region %d base %+v lies outside the base's %d lines",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, region.Base, lines)
		}
	}
}

// regionTake is the lines a take puts in a region's place.
type regionTake func(core.ConflictFile, core.ConflictRegion) []string

func takeCurrent(f core.ConflictFile, r core.ConflictRegion) []string {
	return linesAt(f.Current.Content, r.Current)
}

func takeIncoming(f core.ConflictFile, r core.ConflictRegion) []string {
	return linesAt(f.Incoming.Content, r.Incoming)
}

// applyingNonConflicting takes the changed side of each change only one side made and conflict
// for each conflict.
func applyingNonConflicting(conflict regionTake) regionTake {
	return func(f core.ConflictFile, r core.ConflictRegion) []string {
		switch r.Kind {
		case core.RegionIncoming:
			return takeIncoming(f, r)
		case core.RegionCurrent:
			return takeCurrent(f, r)
		}
		return conflict(f, r)
	}
}

// resolvedWith is the merge editor's result: the working file with each region's lines replaced by
// the take's.
func resolvedWith(t *testing.T, got core.ConflictFile, take regionTake) string {
	t.Helper()
	result := splitLines(got.Result)
	var out []string
	next := 0
	for _, region := range got.Regions {
		start, end := edges(region.Result)
		if start < next {
			t.Fatalf("region %+v overlaps the one before it", region)
		}
		out = append(out, result[next:start]...)
		out = append(out, take(got, region)...)
		next = end
	}
	return strings.Join(append(out, result[next:]...), "")
}

// A smoke check over random merges. Every failure it found is pinned as a table case in
// TestChangeRegionsCoverTheSameChangeInAllFourTexts and conflictregionedges_test.go.
func TestTakingASideInEveryRegionMatchesMergingWithThatSidePreferred(t *testing.T) {
	t.Parallel()
	if runtime.GOOS == "windows" || testing.Short() { // git starts slowly there; the region logic is platform-independent
		t.Skip("fuzz test")
	}
	// Each style is set up before the merge (config) and may rewrite the file after it.
	styles := map[string]struct {
		config  []string
		rewrite []string
		diff3   bool
	}{
		"merge":                              {config: []string{"merge.conflictStyle", "merge"}},
		"zdiff3":                             {config: []string{"merge.conflictStyle", "zdiff3"}},
		"diff3":                              {config: []string{"merge.conflictStyle", "diff3"}, diff3: true},
		"histogram diff3":                    {config: []string{"merge.conflictStyle", "diff3", "diff.algorithm", "histogram"}, diff3: true},
		"checkout diff3":                     {rewrite: []string{"checkout", "--conflict=diff3", "--", "a.txt"}, diff3: true},
		"checkout zdiff3 under diff3 config": {config: []string{"merge.conflictStyle", "diff3"}, rewrite: []string{"checkout", "--conflict=zdiff3", "--", "a.txt"}},
	}
	for style, setup := range styles {
		t.Run(style, func(t *testing.T) {
			t.Parallel()
			r := rand.New(rand.NewSource(7))
			wanted, atLeast := 8, 4
			checked := 0
			for n := 0; checked < wanted && n < 400; n++ {
				var base []string
				for i := r.Intn(6) + 2; i > 0; i-- {
					base = append(base, string(rune('a'+r.Intn(5))))
				}
				// Half the merges share a change on both sides, which git may leave outside the markers.
				shared := base
				if r.Intn(2) == 0 {
					shared = mutateLines(r, base)
				}
				current, incoming := mutateLines(r, shared), mutateLines(r, shared)
				if joinLines(current) == joinLines(base) || joinLines(incoming) == joinLines(base) || joinLines(current) == joinLines(incoming) {
					continue
				}
				dir := divergedRepo(t, "a.txt", version(joinLines(base)), version(joinLines(current)), version(joinLines(incoming)))
				for i := 0; i < len(setup.config); i += 2 {
					gittest.Run(t, dir, "config", setup.config[i], setup.config[i+1])
				}
				tryGit(t, dir, "merge", "side")
				if len(tryGit(t, dir, "ls-files", "-u", "--", "a.txt")) == 0 {
					continue
				}
				if setup.rewrite != nil {
					gittest.Run(t, dir, setup.rewrite...)
				}
				c, wt := openWorktree(t, dir)
				got, err := c.Conflict(t.Context(), wt, "a.txt")
				if err != nil || got.Kind != core.ConflictText || len(conflictRegionsOf(got)) == 0 {
					continue
				}
				checked++
				assertBaseInBounds(t, got)
				assertRegionsAligned(t, got)
				if setup.diff3 {
					assertBaseIsMarkerSection(t, got)
				}
				for _, taken := range []struct {
					name, strategy string
					apply          bool
					take           regionTake
					side           string
				}{
					{"left", "ours", false, takeCurrent, got.Current.Content},
					{"right", "theirs", false, takeIncoming, got.Incoming.Content},
					{"apply, left", "ours", true, applyingNonConflicting(takeCurrent), ""},
					{"apply, right", "theirs", true, applyingNonConflicting(takeIncoming), ""},
				} {
					want := taken.side
					if taken.apply {
						if setup.rewrite == nil {
							want = mergedPreferring(t, dir, taken.strategy)
						} else {
							want = mergeFilePreferring(t, dir, taken.strategy, got)
						}
					}
					if have := resolvedWith(t, got, taken.take); have != want {
						t.Errorf("%s: base %q current %q incoming %q: %s gives %q, want %q",
							style, joinLines(base), joinLines(current), joinLines(incoming), taken.name, have, want)
					}
				}
			}
			if checked < atLeast {
				t.Fatalf("only %d conflicting merges generated", checked)
			}
		})
	}
}

// mergedPreferring is a.txt as merging the side branch with -X strategy leaves it.
func mergedPreferring(t *testing.T, dir, strategy string) string {
	t.Helper()
	tryGit(t, dir, "merge", "--abort")
	tryGit(t, dir, "merge", "--no-commit", "-X", strategy, "side")
	merged, err := os.ReadFile(filepath.Join(dir, "a.txt"))
	if err != nil {
		t.Fatal(err)
	}
	return string(merged)
}

// assertRegionsAligned checks that every region of a change only one side made covers the same
// change in all four texts: the Result lines are the changed side's, and the unchanged side's
// lines are the base's.
func assertRegionsAligned(t *testing.T, got core.ConflictFile) {
	t.Helper()
	for i, r := range got.Regions {
		changed, unchanged, changedText := r.Incoming, r.Current, got.Incoming.Content
		unchangedText := got.Current.Content
		switch r.Kind {
		case core.RegionCurrent:
			changed, unchanged, changedText, unchangedText = r.Current, r.Incoming, got.Current.Content, got.Incoming.Content
		case core.RegionConflict:
			continue
		}
		if have, want := selected(changedText, changed), selected(got.Result, r.Result); have != want {
			t.Errorf("base %q current %q incoming %q: region %d (%+v): changed side holds %q, Result %q",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, r, have, want)
		}
		if have, want := selected(unchangedText, unchanged), selected(got.Base.Content, r.Base); have != want {
			t.Errorf("base %q current %q incoming %q: region %d (%+v): unchanged side holds %q, base %q",
				got.Base.Content, got.Current.Content, got.Incoming.Content, i, r, have, want)
		}
	}
}

func TestChangeRegionsCoverTheSameChangeInAllFourTexts(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name                    string
		style                   string
		base, current, incoming string
	}{
		{"diff3", "diff3", "a\na\na\ne\n", "a\na\nD\nB\n", "a\nB\na\na\nD\nB\ne\n"},
		{"zdiff3", "zdiff3", "b\nb\na\na\n", "b\nb\nC\na\nD\n", "C\nb\na\n"},
		{"merge", "merge", "b\nb\nb\nd\nd\nd\nc\na\n", "B\nb\nB\nb\nd\nC\nd\nC\nc\na\n", "B\nb\nd\nd\nC\nd\nC\nc\na\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			dir := divergedRepo(t, "a.txt", version(tc.base), version(tc.current), version(tc.incoming))
			gittest.Run(t, dir, "config", "merge.conflictStyle", tc.style)
			tryGit(t, dir, "merge", "side")
			if len(tryGit(t, dir, "ls-files", "-u", "--", "a.txt")) == 0 {
				t.Skip("merges cleanly")
			}
			c, wt := openWorktree(t, dir)
			got := readConflict(t, c, wt, "a.txt")

			assertRegionsAligned(t, got)
			if have := resolvedWith(t, got, takeCurrent); have != tc.current {
				t.Errorf("taking left gives %q, want the current file %q", have, tc.current)
			}
			if have := resolvedWith(t, got, takeIncoming); have != tc.incoming {
				t.Errorf("taking right gives %q, want the incoming file %q", have, tc.incoming)
			}
		})
	}
}

// mergeFilePreferring is the three versions of got merged again by git merge-file with the
// strategy's side preferred. A file that checkout --conflict rewrote was merged that way, not by
// the merge machinery that -X steers.
func mergeFilePreferring(t *testing.T, dir, strategy string, got core.ConflictFile) string {
	t.Helper()
	tmp := t.TempDir()
	var paths []string
	for name, content := range map[string]string{"1current": got.Current.Content, "2base": got.Base.Content, "3incoming": got.Incoming.Content} {
		path := filepath.Join(tmp, name)
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, path)
	}
	slices.Sort(paths)
	flag := map[string]string{"ours": "--ours", "theirs": "--theirs"}[strategy]
	return string(tryGit(t, dir, append([]string{"merge-file", "-p", flag}, paths...)...))
}
