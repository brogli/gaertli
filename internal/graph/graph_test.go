package graph

import (
	"strconv"
	"testing"
)

// TestFR_LOG_02_MergeOpensTwoLanesForItsParents: A is the common ancestor
// of B and C, which merge into M. Topo-order (newest first) feeds M, B, C,
// A.
func TestFR_LOG_02_MergeOpensTwoLanesForItsParents(t *testing.T) {
	l := NewLayout()

	m := l.Next(Commit{SHA: "M", Parents: []string{"B", "C"}})
	starts := countKind(m.Edges, Start)
	if starts != 2 {
		t.Fatalf("M starts = %d, want 2 (%+v)", starts, m.Edges)
	}

	b := l.Next(Commit{SHA: "B", Parents: []string{"A"}})
	c := l.Next(Commit{SHA: "C", Parents: []string{"A"}})
	if b.Lane == c.Lane {
		t.Errorf("B and C share lane %d, want distinct lanes", b.Lane)
	}

	a := l.Next(Commit{SHA: "A"})
	if ends := countKind(a.Edges, End); ends != 2 {
		t.Errorf("A ends = %d, want 2 (%+v)", ends, a.Edges)
	}
}

func TestFR_LOG_02_ForkClosesBothLanesAtSharedRoot(t *testing.T) {
	l := NewLayout()

	l.Next(Commit{SHA: "X", Parents: []string{"R"}})
	l.Next(Commit{SHA: "Y", Parents: []string{"R"}})
	r := l.Next(Commit{SHA: "R"})

	if ends := countKind(r.Edges, End); ends != 2 {
		t.Fatalf("R ends = %d, want 2 (%+v)", ends, r.Edges)
	}
	if !hasEdge(r.Edges, r.Lane, End) {
		t.Errorf("R's own lane %d has no End edge (%+v)", r.Lane, r.Edges)
	}
}

// TestFR_LOG_02_WideForkOverflowsPastCap: 40 branches share one root, Cap
// 32.
func TestFR_LOG_02_WideForkOverflowsPastCap(t *testing.T) {
	l := &Layout{Cap: 32}

	const branches = 40
	var rows []Row
	for i := 0; i < branches; i++ {
		rows = append(rows, l.Next(Commit{SHA: sha(i), Parents: []string{"R"}}))
	}
	root := l.Next(Commit{SHA: "R"})
	rows = append(rows, root)

	for _, row := range rows {
		if row.Lane >= 32 {
			t.Errorf("Lane = %d, want < 32", row.Lane)
		}
	}
	if root.Overflow != 8 {
		t.Errorf("root Overflow = %d, want 8", root.Overflow)
	}
}

func sha(i int) string {
	return "b" + strconv.Itoa(i)
}

func countKind(edges []Edge, kind EdgeKind) int {
	n := 0
	for _, e := range edges {
		if e.Kind == kind {
			n++
		}
	}
	return n
}

func hasEdge(edges []Edge, lane int, kind EdgeKind) bool {
	for _, e := range edges {
		if e.Lane == lane && e.Kind == kind {
			return true
		}
	}
	return false
}
