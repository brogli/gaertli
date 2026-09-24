// Package services implements core.API in-process: it validates input, runs
// git, and returns DTOs. One file per feature area.
package services

import (
	"context"
	"sync/atomic"

	"github.com/brogli/nebegit/internal/buildinfo"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/session"
)

// Core is the in-process core.
type Core struct {
	sink     core.EventSink
	sessions *session.Manager
	discover git.Runner // no Log, no session: ListWorktrees/ListBranches's stateless reads

	// editorProfile is the profile pushed by SetEditorProfile, set from a
	// different goroutine than the ones reading it.
	editorProfile atomic.Value
}

// New builds a core that reports events to sink.
func New(sink core.EventSink) *Core {
	return &Core{sink: sink, sessions: session.NewManager(sink)}
}

// Version implements core.API.
func (c *Core) Version(context.Context) (core.Version, error) {
	return core.Version{App: buildinfo.App, Protocol: core.Protocol}, nil
}
