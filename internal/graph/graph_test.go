// Requirements: FR-LOG-02

package graph

import (
	"maps"
	"math/rand/v2"
	"slices"
	"strconv"
	"testing"
)

func TestLinearHistoryDrawsOneLaneIntoAndOutOfEachDot(t *testing.T) {
	l := NewLayout()

	c := l.Next(Commit{SHA: "C", Parents: []string{"B"}})
	b := l.Next(Commit{SHA: "B", Parents: []string{"A"}})
	a := l.Next(Commit{SHA: "A"})

	want := [][]Edge{
		{{Lane: 0, Kind: Out}},
		{{Lane: 0, Kind: In}, {Lane: 0, Kind: Out}},
		{{Lane: 0, Kind: In}},
	}
	for i, row := range []Row{c, b, a} {
		if row.Lane != 0 || !slices.Equal(row.Edges, want[i]) {
			t.Errorf("row %d = lane %d %+v, want lane 0 %+v", i, row.Lane, row.Edges, want[i])
		}
	}
}

// B and C, both off A, merge into M. Topo order (newest first) feeds M, B, C, A.
func TestMergeDrawsALineToEachParentsLane(t *testing.T) {
	l := NewLayout()

	m := l.Next(Commit{SHA: "M", Parents: []string{"B", "C"}})
	b := l.Next(Commit{SHA: "B", Parents: []string{"A"}})
	c := l.Next(Commit{SHA: "C", Parents: []string{"A"}})

	wantM := []Edge{{Lane: 0, Kind: Out, Colour: 0}, {Lane: 1, Kind: Out, Colour: 1}}
	if !slices.Equal(m.Edges, wantM) {
		t.Errorf("M edges = %+v, want %+v", m.Edges, wantM)
	}
	if b.Lane != 0 || c.Lane != 1 || c.Colour != 1 {
		t.Errorf("B lane %d, C lane %d colour %d; want B on 0, C on 1 in colour 1", b.Lane, c.Lane, c.Colour)
	}
	if !slices.Contains(b.Edges, Edge{Lane: 1, Kind: Through, Colour: 1}) {
		t.Errorf("B edges = %+v, want C's lane 1 passing through", b.Edges)
	}
}

// The same history: A is where B's and C's lanes meet.
func TestForkDrawsEachBranchIntoTheSharedCommit(t *testing.T) {
	l := NewLayout()
	l.Next(Commit{SHA: "M", Parents: []string{"B", "C"}})
	l.Next(Commit{SHA: "B", Parents: []string{"A"}})
	l.Next(Commit{SHA: "C", Parents: []string{"A"}})

	a := l.Next(Commit{SHA: "A"})

	want := []Edge{{Lane: 0, Kind: In, Colour: 0}, {Lane: 1, Kind: In, Colour: 1}}
	if a.Lane != 0 || !slices.Equal(a.Edges, want) {
		t.Errorf("A = lane %d %+v, want lane 0 %+v", a.Lane, a.Edges, want)
	}
}

func TestOctopusMergeDrawsALineToEveryParent(t *testing.T) {
	l := NewLayout()

	o := l.Next(Commit{SHA: "O", Parents: []string{"P1", "P2", "P3"}})

	want := []Edge{{Lane: 0, Kind: Out, Colour: 0}, {Lane: 1, Kind: Out, Colour: 1}, {Lane: 2, Kind: Out, Colour: 2}}
	if !slices.Equal(o.Edges, want) {
		t.Errorf("O edges = %+v, want %+v", o.Edges, want)
	}
}

// On a random history, the lanes leaving a row at its bottom (through, out)
// are exactly the lanes entering the next row at its top (through, in),
// each in the same colour.
func TestLinesMeetAtRowBoundaries(t *testing.T) {
	l := NewLayout()
	var above map[int]int
	for i, c := range randomHistory(300) {
		row := l.Next(c)
		top, bottom := rowEnds(row)
		if i > 0 && !maps.Equal(above, top) {
			t.Fatalf("row %d (%s) top lanes %v, want the row above's bottom lanes %v", i, c.SHA, top, above)
		}
		above = bottom
	}
	if len(above) != 0 {
		t.Errorf("lanes left open below the last row: %v", above)
	}
}

func TestFreedLaneIsReusedBelowTheFork(t *testing.T) {
	l := NewLayout()
	l.Next(Commit{SHA: "X", Parents: []string{"R"}})
	l.Next(Commit{SHA: "Y", Parents: []string{"R"}})
	r := l.Next(Commit{SHA: "R", Parents: []string{"Q"}})
	z := l.Next(Commit{SHA: "Z", Parents: []string{"Q"}})

	if !slices.Contains(r.Edges, Edge{Lane: 1, Kind: In, Colour: 1}) {
		t.Fatalf("R edges = %+v, want Y's lane 1 ending in R", r.Edges)
	}
	if z.Lane != 1 || slices.ContainsFunc(z.Edges, func(e Edge) bool { return e.Kind == In }) {
		t.Errorf("Z = lane %d %+v, want the freed lane 1 with no line from above", z.Lane, z.Edges)
	}
}

// 40 branches share one root, past the 32-lane cap. The 8 tips past the cap draw only their dot.
func TestWideForkOverflowsPastCap(t *testing.T) {
	l := NewLayout()

	const branches = 40
	for i := range branches {
		row := l.Next(Commit{SHA: "b" + strconv.Itoa(i), Parents: []string{"R"}})
		if row.Lane >= laneCap {
			t.Errorf("b%d Lane = %d, want < %d", i, row.Lane, laneCap)
		}
		if i >= laneCap && slices.ContainsFunc(row.Edges, func(e Edge) bool { return e.Kind != Through }) {
			t.Errorf("b%d past the cap has lines into or out of its dot: %+v", i, row.Edges)
		}
	}
	root := l.Next(Commit{SHA: "R"})
	if root.Overflow != branches-laneCap {
		t.Errorf("root Overflow = %d, want %d", root.Overflow, branches-laneCap)
	}
}

// randomHistory returns n commits, newest first, each with 0 to 3 parents
// among the eight commits before it: branch tips, forks, merges and roots
// in one topologically ordered stream.
func randomHistory(n int) []Commit {
	rng := rand.New(rand.NewPCG(1, 2))
	commits := make([]Commit, n) // oldest first
	for i := range commits {
		c := Commit{SHA: strconv.Itoa(i)}
		for range []int{0, 1, 1, 1, 2, 3}[rng.IntN(6)] {
			if i == 0 {
				break
			}
			p := strconv.Itoa(i - 1 - rng.IntN(min(i, 8)))
			if !slices.Contains(c.Parents, p) {
				c.Parents = append(c.Parents, p)
			}
		}
		commits[i] = c
	}
	slices.Reverse(commits)
	return commits
}

// rowEnds maps the lanes a row's lines cross its top and bottom edges on
// to their colours.
func rowEnds(row Row) (top, bottom map[int]int) {
	top, bottom = map[int]int{}, map[int]int{}
	for _, e := range row.Edges {
		if e.Kind != Out {
			top[e.Lane] = e.Colour
		}
		if e.Kind != In {
			bottom[e.Lane] = e.Colour
		}
	}
	return top, bottom
}
