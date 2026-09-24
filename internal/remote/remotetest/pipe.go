// Package remotetest serves a core.API over an in-memory pipe so tests
// exercise the real generated client and dispatcher without a subprocess.
package remotetest

import (
	"context"
	"io"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
)

// NewPipeClient serves the API newAPI builds (given the server-side event
// sink) and returns a client on it; sink receives the client-side events.
func NewPipeClient(tb testing.TB, newAPI func(core.EventSink) core.API, sink core.EventSink) *remote.Client {
	tb.Helper()
	clientIn, serverOut := io.Pipe()
	serverIn, clientOut := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	go func() { _ = remote.Serve(ctx, serverIn, serverOut, newAPI) }()
	tb.Cleanup(func() {
		cancel()
		_ = clientOut.Close()
		_ = serverOut.Close()
	})
	return remote.NewClient(clientIn, clientOut, sink)
}
