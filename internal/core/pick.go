package core

// PickOptions are CherryPickWith's and RevertWith's options. RecordOrigin and
// Mainline apply to cherry-pick only.
type PickOptions struct {
	NoCommit     bool `json:"noCommit"`     // -n: apply without committing
	RecordOrigin bool `json:"recordOrigin"` // -x: append "(cherry picked from commit <sha>)"
	Mainline     int  `json:"mainline"`     // -m <n>; 0 refuses merge commits
}
