package app

import "github.com/brogli/nebegit/internal/core"

// Router picks the core for a call (ADR-3): the local one, the one owning a
// worktree, or the one for a path. Implemented by the window registry.
type Router interface {
	Local() core.API
	CoreOf(core.WorktreeID) (core.API, error)
	CoreFor(core.RepoPath) (core.API, error)
}

// Facade is the one Wails service; its methods are generated and forward to
// the core the Router names.
type Facade struct{ router Router }

// NewFacade binds a Facade to r.
func NewFacade(r Router) *Facade { return &Facade{router: r} }
