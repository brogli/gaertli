package app

import (
	"context"

	"github.com/brogli/gaertli/internal/core"
)

// Router picks the core for a call: the local one, the one owning a
// worktree, or the one for a path. Implemented by the window registry.
type Router interface {
	Local() core.API
	// CoreOf returns the core owning a worktree and the id that core knows
	// it by. ctx is the call's.
	CoreOf(context.Context, core.WorktreeID) (core.API, core.WorktreeID, error)
	CoreFor(core.RepoPath) (core.API, error)
}

// Forwarder implements core.API by forwarding each call to the core the
// Router names; the shell's api. Its methods are generated.
type Forwarder struct{ router Router }

// NewForwarder binds a Forwarder to r.
func NewForwarder(r Router) *Forwarder { return &Forwarder{router: r} }

// validateEach returns the first error of vs' Validate. The Forwarder checks
// every value with a rule (a SHA, a full ref name) here, so no core is
// reached with a malformed one.
func validateEach[V interface{ Validate() error }](vs ...V) error {
	for _, v := range vs {
		if err := v.Validate(); err != nil {
			return err
		}
	}
	return nil
}

// Facade is the Wails service windows call: the Forwarder's WindowAPI
// methods only, since Wails binds every method of a bound type.
type Facade struct{ core.WindowAPI }
