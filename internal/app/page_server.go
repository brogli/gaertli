//go:build server

package app

import (
	"context"
	"net/http"
	"sync"
	"time"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

func init() { pageMiddleware = tagPage }

// tagPage names the calling page by its runtime's client id, which every
// runtime request carries. Wails knows a page as a window (BrowserWindow)
// only once its event websocket connects, after its first calls.
func tagPage(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if id := r.Header.Get("x-wails-client-id"); id != "" {
			r = r.WithContext(context.WithValue(r.Context(), pageKey{}, shellWindow(browserPage{clientID: id})))
		}
		next.ServeHTTP(w, r)
	})
}

// browserPage is a page as the registry's window. Wails sends every event
// to every page; BrowserWindow.EmitEvent names no sender, so a page names
// its BrowserWindow itself, the name the page's Window.Name() returns.
type browserPage struct{ clientID string }

// EmitEvent holds the event until the page's websocket connects; it never
// reports the event cancelled.
func (p browserPage) EmitEvent(name string, data ...any) bool {
	pageEvents.send(p.clientID, name, data)
	return false
}

// pageEvents holds each page's events, because a page's first calls run
// before its websocket connects and Wails drops events for a page it does
// not know as a window yet.
var pageEvents = &eventHold{
	lookup: func(clientID string) (string, bool) {
		w := application.GetBrowserWindow(clientID)
		if w == nil {
			return "", false
		}
		return w.Name(), true
	},
	emit: func(sender, name string, data []any) {
		ev := &application.CustomEvent{Name: name, Sender: sender}
		if len(data) == 1 {
			ev.Data = data[0]
		}
		application.Get().Event.EmitEvent(ev)
	},
	poll:    20 * time.Millisecond,
	giveUp:  10 * time.Second,
	pending: map[string][]heldEvent{},
}

type heldEvent struct {
	name string
	data []any
}

// eventHold queues a page's events from its first one until lookup finds
// the page, then emits them in order. Events sent while the queue exists
// join it, which keeps the order. A page not found within giveUp is gone
// and its queue is dropped.
type eventHold struct {
	lookup  func(clientID string) (sender string, ok bool)
	emit    func(sender, name string, data []any)
	poll    time.Duration
	giveUp  time.Duration
	mu      sync.Mutex
	pending map[string][]heldEvent
}

func (h *eventHold) send(clientID, name string, data []any) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if queue, held := h.pending[clientID]; held {
		h.pending[clientID] = append(queue, heldEvent{name, data})
		return
	}
	sender, ok := h.lookup(clientID)
	if !ok {
		h.pending[clientID] = []heldEvent{{name, data}}
		go h.drainWhenFound(clientID)
		return
	}
	h.emit(sender, name, data)
}

func (h *eventHold) drainWhenFound(clientID string) {
	deadline := time.Now().Add(h.giveUp)
	for {
		time.Sleep(h.poll)
		h.mu.Lock()
		sender, found := h.lookup(clientID)
		if found {
			for _, ev := range h.pending[clientID] {
				h.emit(sender, ev.name, ev.data)
			}
		}
		if found || time.Now().After(deadline) {
			delete(h.pending, clientID)
			h.mu.Unlock()
			return
		}
		h.mu.Unlock()
	}
}

// A page has no OS window to focus, title or watch.

func (browserPage) Focus() {}

func (browserPage) SetTitle(string) application.Window { return nil }

func (browserPage) OnWindowEvent(events.WindowEventType, func(*application.WindowEvent)) func() {
	return func() {}
}

func (browserPage) ID() uint { return 0 }
