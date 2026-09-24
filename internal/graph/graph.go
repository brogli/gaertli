// Package graph assigns lanes to a streamed commit sequence for the commit
// graph. Standard library only.
package graph

import "cmp"

// Commit is the minimal input one layout step needs.
type Commit struct {
	SHA     string
	Parents []string // full SHAs, first parent first
}

type EdgeKind string

const (
	Start    EdgeKind = "start"    // this lane's line begins at this row
	Continue EdgeKind = "continue" // this lane passes through this row unchanged
	End      EdgeKind = "end"      // this lane's line, coming from above, terminates at this row
)

// Edge is one lane's state on a row.
type Edge struct {
	Lane int
	Kind EdgeKind
}

// Row is one commit's lane data.
type Row struct {
	Lane     int    // the lane this row's own commit sits in
	Colour   int    // 0-based colour number, stable for Lane's lifetime, never reused
	Edges    []Edge // every lane active on this row, including Lane's own
	Overflow int    // lanes beyond Layout's cap collapsed into a "+k" marker at this row; 0 = none
}

// DefaultCap is Layout's lane cap when Cap is zero.
const DefaultCap = 32

// openLane is one lane's state: the SHA it is waiting for next, and its
// colour, stable for as long as the lane stays open.
type openLane struct {
	sha    string
	colour int
}

// Layout assigns lanes to a streamed commit sequence, ported from
// lazygit's getNextPipes (MIT; its notice is in
// tools/licensecheck/adapted/lazygit.LICENSE). Stateful, one per repo load,
// not for concurrent use.
type Layout struct {
	Cap int // lane cap before overflow; 0 uses DefaultCap

	lanes    []*openLane    // index = lane number; nil = free slot
	overflow map[string]int // sha -> count of hidden waiters for it
	colours  int            // next colour to assign
}

// NewLayout returns a Layout ready for the first commit of a load.
func NewLayout() *Layout {
	return &Layout{}
}

// Next consumes only l's own state from previous calls and c — never the
// whole history. For c's SHA: if an open lane is waiting for it, that lane
// closes (Edge Kind End) and c takes that lane (or, if two lanes were both
// waiting for the same SHA — a fork — both close and c takes the first of
// them). If no lane was waiting, c opens a new lane at the lowest free
// index (Edge Kind Start), real if one remains under Cap, otherwise a
// cosmetic-only slot folded into Row.Overflow at the row where its target
// is eventually reached. c's first parent re-arms c's own
// lane to wait for it next; each further parent (a merge) opens a fresh
// lane waiting for that parent (Edge Kind Start on this row), subject to
// the same cap. Every other open lane not touched by c gets Edge Kind
// Continue. A colour is assigned when its lane opens and never reused.
// Lane indices are not compacted once assigned.
func (l *Layout) Next(c Commit) Row {
	if l.overflow == nil {
		l.overflow = map[string]int{}
	}

	closing := l.closingLanes(c.SHA)
	row := Row{Overflow: l.overflow[c.SHA]}
	delete(l.overflow, c.SHA)
	for _, i := range closing {
		row.Edges = append(row.Edges, Edge{Lane: i, Kind: End})
	}

	own, colour, ownReal := l.ownLane(closing)
	row.Lane, row.Colour = own, colour
	if ownReal && len(closing) == 0 {
		row.Edges = append(row.Edges, Edge{Lane: own, Kind: Start})
	}

	touched := toSet(closing)
	if ownReal {
		touched[own] = true
	}
	for _, i := range l.armParents(own, colour, ownReal, c.Parents, &row) {
		touched[i] = true
	}

	for i, ln := range l.lanes {
		if ln == nil || touched[i] {
			continue
		}
		row.Edges = append(row.Edges, Edge{Lane: i, Kind: Continue})
	}
	return row
}

// closingLanes returns, ascending, every open lane index waiting for sha.
func (l *Layout) closingLanes(sha string) []int {
	var idx []int
	for i, ln := range l.lanes {
		if ln != nil && ln.sha == sha {
			idx = append(idx, i)
		}
	}
	return idx
}

// ownLane returns the lane index c's own dot sits in: the first closing
// lane (freeing the rest), or a freshly opened one — real if a slot
// remains under Cap, cosmetic-only otherwise.
func (l *Layout) ownLane(closing []int) (idx, colour int, ownReal bool) {
	if len(closing) > 0 {
		idx = closing[0]
		colour = l.lanes[idx].colour
		for _, i := range closing[1:] {
			l.free(i)
		}
		return idx, colour, true
	}
	if idx, ok := l.freeIndex(); ok {
		colour = l.newColour()
		l.lanes[idx] = &openLane{colour: colour}
		return idx, colour, true
	}
	return l.cap() - 1, l.newColour(), false
}

// armParents re-arms own for c's first parent (or frees own, a root), and
// opens a lane per further parent (a merge). A target with no room under
// Cap becomes an overflow wait instead of a lane. Returns every real lane
// index touched, for Next's Continue-edge exclusion.
func (l *Layout) armParents(own, colour int, ownReal bool, parents []string, row *Row) []int {
	if len(parents) == 0 {
		if ownReal {
			l.free(own)
		}
		return nil
	}

	var touched []int
	if ownReal {
		l.lanes[own] = &openLane{sha: parents[0], colour: colour}
		touched = append(touched, own)
	} else {
		l.overflow[parents[0]]++
	}

	for _, p := range parents[1:] {
		if idx, ok := l.freeIndex(); ok {
			l.lanes[idx] = &openLane{sha: p, colour: l.newColour()}
			row.Edges = append(row.Edges, Edge{Lane: idx, Kind: Start})
			touched = append(touched, idx)
		} else {
			l.overflow[p]++
		}
	}
	return touched
}

// freeIndex returns the lowest free lane index under Cap, growing lanes by
// one nil slot if every existing index is taken. ok is false once Cap is
// reached.
func (l *Layout) freeIndex() (int, bool) {
	for i, ln := range l.lanes {
		if ln == nil {
			return i, true
		}
	}
	if len(l.lanes) < l.cap() {
		l.lanes = append(l.lanes, nil)
		return len(l.lanes) - 1, true
	}
	return 0, false
}

func (l *Layout) free(i int) { l.lanes[i] = nil }

func (l *Layout) newColour() int {
	c := l.colours
	l.colours++
	return c
}

func (l *Layout) cap() int { return cmp.Or(l.Cap, DefaultCap) }

func toSet(idx []int) map[int]bool {
	set := make(map[int]bool, len(idx))
	for _, i := range idx {
		set[i] = true
	}
	return set
}
