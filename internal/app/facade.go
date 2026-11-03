package app

import (
	"context"

	"github.com/brogli/nebegit/internal/core"
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

// Facade is the Wails service windows call: the Forwarder's WindowAPI
// methods only, since Wails binds every method of a bound type.
type Facade struct{ core.WindowAPI }
