//go:build server

package app

import (
	"context"
	"net/http"

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

// EmitEvent does nothing before the page's websocket connects: the page
// would not get the event anyway.
func (p browserPage) EmitEvent(name string, data ...any) bool {
	w := application.GetBrowserWindow(p.clientID)
	if w == nil {
		return false
	}
	ev := &application.CustomEvent{Name: name, Sender: w.Name()}
	if len(data) == 1 {
		ev.Data = data[0]
	}
	return application.Get().Event.EmitEvent(ev)
}

// A page has no OS window to focus, title or watch.

func (browserPage) Focus() {}

func (browserPage) SetTitle(string) application.Window { return nil }

func (browserPage) OnWindowEvent(events.WindowEventType, func(*application.WindowEvent)) func() {
	return func() {}
}

func (browserPage) ID() uint { return 0 }
