package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"time"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"

	"github.com/brogli/nebegit/internal/core"
)

// eventDelay is how long after start the pushed "event" notification fires.
const eventDelay = 500 * time.Millisecond

// serve runs the spike server on r/w until r closes (Design): "ping" and
// "rows" methods, plus one pushed "event" notification carrying a
// core.Event so the measured JSON shape matches the real wire format.
func serve(ctx context.Context, r io.Reader, w io.Writer) error {
	srv := jrpc2.NewServer(handler.Map{
		"ping": handler.New(func(context.Context) (string, error) { return "pong", nil }),
		"rows": handler.NewPos(func(_ context.Context, n int) ([]spikeRow, error) { return spikeRows(n), nil }, "n"),
	}, &jrpc2.ServerOptions{AllowPush: true})
	srv.Start(channel.Line(r, nopCloser{w}))

	go pushEvent(ctx, srv)

	if err := srv.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

func pushEvent(ctx context.Context, srv *jrpc2.Server) {
	time.Sleep(eventDelay)
	ev := core.Event{Name: "spike:watch", Payload: json.RawMessage(`{"seq":1}`)}
	_ = srv.Notify(ctx, "event", ev)
}

// nopCloser lets a plain writer serve as the channel's WriteCloser; the
// caller owns stdout.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
