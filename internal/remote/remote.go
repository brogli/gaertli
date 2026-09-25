// Package remote carries core.API over JSON-RPC 2.0 on stdio: Serve
// runs a core behind a dispatcher, Client is the generated core.API that
// talks to it. Messages are LF-delimited JSON (channel.Line).
package remote

import (
	"context"
	"encoding/json"
	"errors"
	"io"

	"github.com/creachadair/jrpc2"
	"github.com/creachadair/jrpc2/channel"
	"github.com/creachadair/jrpc2/handler"

	"github.com/brogli/nebegit/internal/core"
)

// eventMethod is the notification that carries a core.Event to the client.
const eventMethod = "event"

// coreErrorCode marks a JSON-RPC error whose data is a core.Error.
const coreErrorCode jrpc2.Code = 1

// Client is core.API over a JSON-RPC connection; methods are generated.
type Client struct {
	c         *jrpc2.Client
	closeRead func() // closes the reader NewClient got, if it is an io.Closer
}

// NewClient speaks to a core served on r/w. Events the core emits arrive on
// sink, which may be nil. Once the client stops reading (closed, r ended,
// or a line that is no message), it closes r if r is an io.Closer, so a
// writer feeding r is not left blocked.
func NewClient(r io.Reader, w io.Writer, sink core.EventSink) *Client {
	closeRead := func() {}
	if closer, ok := r.(io.Closer); ok {
		closeRead = func() { _ = closer.Close() }
	}
	opts := &jrpc2.ClientOptions{OnStop: func(*jrpc2.Client, error) { closeRead() }}
	if sink != nil {
		opts.OnNotify = func(req *jrpc2.Request) {
			var ev core.Event
			if req.Method() == eventMethod && req.UnmarshalParams(&ev) == nil {
				sink(ev)
			}
		}
	}
	return &Client{c: jrpc2.NewClient(channel.Line(r, nopCloser{w}), opts), closeRead: closeRead}
}

// Close ends the connection. It closes the reader first, if it can: jrpc2
// waits for its read loop to end, which it would not while the core runs.
// The connection's end is not reported as an error.
func (c *Client) Close() error {
	c.closeRead()
	_ = c.c.Close() // the reader's closed error
	return nil
}

// call is the wire of every generated method: params in, result out, and
// the server's core.Error restored from the error data.
func (c *Client) call(ctx context.Context, method string, params, result any) error {
	if result == nil {
		_, err := c.c.Call(ctx, method, params)
		return toCoreError(err)
	}
	return toCoreError(c.c.CallResult(ctx, method, params, result))
}

func toCoreError(err error) error {
	if err == nil {
		return nil
	}
	if rpcErr, ok := errors.AsType[*jrpc2.Error](err); ok && rpcErr.Code == coreErrorCode {
		var ce core.Error
		if json.Unmarshal(rpcErr.Data, &ce) == nil && ce.Code != "" {
			return &ce
		}
	}
	return core.Errorf(core.Internal, "%v", err)
}

// Serve runs the core built by newCore on r/w until r closes or ctx ends.
// The sink handed to newCore forwards every event to the client.
func Serve(ctx context.Context, r io.Reader, w io.Writer, newCore func(core.EventSink) core.API) error {
	var srv *jrpc2.Server
	sink := func(ev core.Event) { _ = srv.Notify(ctx, eventMethod, ev) }
	srv = jrpc2.NewServer(guarded(handlers(newCore(sink))), &jrpc2.ServerOptions{AllowPush: true})
	srv.Start(channel.Line(r, nopCloser{w}))
	defer context.AfterFunc(ctx, srv.Stop)()
	if err := srv.Wait(); err != nil && !errors.Is(err, io.EOF) {
		return err
	}
	return nil
}

// guarded wraps every handler so a panic becomes an INTERNAL error and a
// core.Error travels as error data.
func guarded(m handler.Map) handler.Map {
	out := make(handler.Map, len(m))
	for name, h := range m {
		out[name] = guard(h)
	}
	return out
}

func guard(h jrpc2.Handler) jrpc2.Handler {
	return func(ctx context.Context, req *jrpc2.Request) (result any, err error) {
		defer func() {
			if p := recover(); p != nil {
				err = core.Errorf(core.Internal, "%s: %v", req.Method(), p)
			}
			if err != nil {
				err = toRPCError(err)
			}
		}()
		return h(ctx, req)
	}
}

func toRPCError(err error) *jrpc2.Error {
	ce := core.ErrorOf(err)
	return jrpc2.Errorf(coreErrorCode, "%s", ce.Error()).WithData(ce)
}

// nopCloser lets a plain writer serve as the channel's WriteCloser; the
// caller owns stdout and pipes.
type nopCloser struct{ io.Writer }

func (nopCloser) Close() error { return nil }
