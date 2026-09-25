// Package graph assigns lanes to a streamed commit sequence for the commit
// graph. Standard library only.
package graph

import "slices"

// Commit is the minimal input one layout step needs.
type Commit struct {
	SHA     string
	Parents []string // full SHAs, first parent first
}

// EdgeKind is the shape of one line inside a row. Lanes keep their index,
// so only a line that ends at or starts from the row's commit changes lane.
type EdgeKind string

const (
	Through EdgeKind = "through" // the lane passes the row, top to bottom
	In      EdgeKind = "in"      // from the lane at the row's top into the commit: from a child
	Out     EdgeKind = "out"     // from the commit to the lane at the row's bottom: to a parent
)

// Edge is one line on a row.
type Edge struct {
	Lane   int
	Kind   EdgeKind
	Colour int // the lane's own colour
}

// Row is one commit's lane data. The lanes at a row's bottom (Through and
// Out) are the lanes at the next row's top (Through and In).
type Row struct {
	Lane     int    // the lane this row's own commit sits in
	Colour   int    // 0-based colour number, stable for Lane's lifetime, never reused
	Edges    []Edge // Ins, then Outs, then Throughs
	Overflow int    // lanes beyond laneCap collapsed into a "+k" marker at this row; 0 = none
}

// laneCap is how many lanes a Layout draws before overflow.
const laneCap = 32

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
	lanes    []*openLane    // index = lane number; nil = free slot
	overflow map[string]int // sha -> count of hidden waiters for it
	colours  int            // next colour to assign
}

// NewLayout returns a Layout ready for the first commit of a load.
func NewLayout() *Layout {
	return &Layout{}
}

// Next consumes only l's own state from previous calls and c — never the
// whole history. Every open lane waiting for c's SHA ends in c (Kind In):
// c takes the first of them, and more than one is a fork. If no lane was
// waiting, c opens a new lane at the lowest free index, real if one
// remains under laneCap, otherwise a cosmetic-only slot folded into
// Row.Overflow at the row where its target is eventually reached. c's
// first parent re-arms c's own lane to wait for it next; each further
// parent (a merge) opens a fresh lane waiting for that parent, subject to
// the same cap; each is a line out of c (Kind Out). Every other open lane
// passes the row (Kind Through). A colour is assigned when its lane opens
// and never reused. Lane indices are not compacted once assigned.
func (l *Layout) Next(c Commit) Row {
	if l.overflow == nil {
		l.overflow = map[string]int{}
	}

	closing := l.closingLanes(c.SHA)
	row := Row{Overflow: l.overflow[c.SHA]}
	delete(l.overflow, c.SHA)
	for _, i := range closing {
		row.Edges = append(row.Edges, Edge{Lane: i, Kind: In, Colour: l.lanes[i].colour})
	}

	own, colour, ownReal := l.ownLane(closing)
	row.Lane, row.Colour = own, colour

	outs := l.armParents(own, colour, ownReal, c.Parents)
	for _, i := range outs {
		row.Edges = append(row.Edges, Edge{Lane: i, Kind: Out, Colour: l.lanes[i].colour})
	}
	for i, ln := range l.lanes {
		if ln != nil && !slices.Contains(outs, i) {
			row.Edges = append(row.Edges, Edge{Lane: i, Kind: Through, Colour: ln.colour})
		}
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
// remains under laneCap, cosmetic-only otherwise.
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
	return laneCap - 1, l.newColour(), false
}

// armParents re-arms own for c's first parent (or frees own, a root), and
// opens a lane per further parent (a merge). A target with no room under
// laneCap becomes an overflow wait instead of a lane. Returns the lanes
// armed, in parent order.
func (l *Layout) armParents(own, colour int, ownReal bool, parents []string) []int {
	if len(parents) == 0 {
		if ownReal {
			l.free(own)
		}
		return nil
	}

	var armed []int
	if ownReal {
		l.lanes[own] = &openLane{sha: parents[0], colour: colour}
		armed = append(armed, own)
	} else {
		l.overflow[parents[0]]++
	}

	for _, p := range parents[1:] {
		if idx, ok := l.freeIndex(); ok {
			l.lanes[idx] = &openLane{sha: p, colour: l.newColour()}
			armed = append(armed, idx)
		} else {
			l.overflow[p]++
		}
	}
	return armed
}

// freeIndex returns the lowest free lane index under laneCap, growing lanes
// by one nil slot if every existing index is taken. ok is false once
// laneCap is reached.
func (l *Layout) freeIndex() (int, bool) {
	for i, ln := range l.lanes {
		if ln == nil {
			return i, true
		}
	}
	if len(l.lanes) < laneCap {
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
