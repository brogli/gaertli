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

// Facade is the Wails service for core.API; its methods are generated and
// forward to the core the Router names.
type Facade struct{ router Router }

// NewFacade binds a Facade to r.
func NewFacade(r Router) *Facade { return &Facade{router: r} }
