package core

// PullMode is the per-pull choice, preselected in the UI from the last one
// used. Seeded from the defaultPullMode setting and kept only as long as
// the window is open — entirely a frontend concern, with no backend
// surface of its own.
type PullMode string

const (
	PullMerge  PullMode = "merge"
	PullRebase PullMode = "rebase"
)
