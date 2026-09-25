package core

// SquashCheck says whether Squash and Fixup can fold a selection of
// commits, and what the user is told before they run.
type SquashCheck struct {
	// Blocker is the error Squash and Fixup would refuse with; nil if they
	// can run.
	Blocker *Error `json:"blocker,omitempty"`
	// Pushed: a selected commit is already on the branch's upstream, so the
	// branch needs a force push afterwards.
	Pushed bool `json:"pushed"`
	// Message is the selected commits' messages, oldest first, separated by
	// a blank line: what Squash proposes for the folded commit.
	Message string `json:"message"`
}

// SquashResult is Squash's and Fixup's result: the branch's commit before,
// so the user can go back.
type SquashResult struct {
	PreviousHead string `json:"previousHead"`
}
