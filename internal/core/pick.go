package core

// PickOptions are CherryPick's and Revert's options; the zero value is git's
// defaults. RecordOrigin applies to cherry-pick only.
type PickOptions struct {
	NoCommit     bool `json:"noCommit"`     // -n: apply without committing
	RecordOrigin bool `json:"recordOrigin"` // -x: append "(cherry picked from commit <sha>)"
}
