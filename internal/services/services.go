// Package services implements core.API in-process: it validates input, runs
// git, and returns DTOs. One service per feature area; this is the stub
// every spec adds to.
package services

import (
	"context"

	"github.com/brogli/nebegit/internal/buildinfo"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/session"
)

// Core is the in-process core.
type Core struct {
	sink     core.EventSink
	sessions *session.Manager
}

// New builds a core that reports events to sink.
func New(sink core.EventSink) *Core {
	return &Core{sink: sink, sessions: session.NewManager(sink)}
}

// Version implements core.API.
func (c *Core) Version(context.Context) (core.Version, error) {
	return core.Version{App: buildinfo.App, Protocol: core.Protocol}, nil
}
