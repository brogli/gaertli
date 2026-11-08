//go:build race

// Requirements: QA-REL-04

package app

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// syncWindow is a shellWindow safe for the concurrent use Wails makes of a
// real one.
type syncWindow struct {
	id       uint
	mu       sync.Mutex
	handlers map[events.WindowEventType]func(*application.WindowEvent)
}

func (w *syncWindow) ID() uint                           { return w.id }
func (w *syncWindow) Focus()                             {}
func (w *syncWindow) SetTitle(string) application.Window { return nil }
func (w *syncWindow) EmitEvent(string, ...any) bool      { return true }

func (w *syncWindow) OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func() {
	w.mu.Lock()
	defer w.mu.Unlock()
	w.handlers[eventType] = callback
	return func() {}
}

func (w *syncWindow) fire(eventType events.WindowEventType) {
	w.mu.Lock()
	handler := w.handlers[eventType]
	w.mu.Unlock()
	if handler != nil {
		handler(&application.WindowEvent{})
	}
}

type syncOpener struct {
	next    atomic.Uint64
	mu      sync.Mutex
	windows map[string]*syncWindow // by window URL
}

func (o *syncOpener) NewWithOptions(opts application.WebviewWindowOptions) shellWindow {
	w := &syncWindow{id: uint(o.next.Add(1)), handlers: map[events.WindowEventType]func(*application.WindowEvent){}}
	o.mu.Lock()
	defer o.mu.Unlock()
	o.windows[opts.URL] = w
	return w
}

func (o *syncOpener) windowAt(url string) *syncWindow {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.windows[url]
}

func TestRegistryKeepsItsInvariantsUnderConcurrentWindowFocusEventsGitWarningsAndQuit(t *testing.T) {
	const workers = 16
	distros := []string{"", "Ubuntu", "Debian"}

	r, _ := newTestRegistry()
	opener := &syncOpener{windows: map[string]*syncWindow{}}
	r.opener = opener
	r.SetLocal(fakeAPI{version: core.Version{Git: "2.40.0", GitOutdated: true}})

	var taken struct {
		sync.Mutex
		warnings []GitWarning
	}
	var closed atomic.Int64
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := range workers {
		wg.Go(func() {
			path := fmt.Sprintf("/repo/%d", i)
			id := core.WorktreeID(path)
			<-start
			r.OpenOrFocus(id, path, "")
			w := opener.windowAt(selectionURL(path, ""))
			w.fire(events.Common.WindowFocus)
			r.Dispatch(worktreeEvent(id))
			r.noteGit(r.ctx, distros[i%len(distros)], r.local)
			r.OpenOrFocus(id, path, "")
			w.fire(events.Common.WindowLostFocus)
			if i == 0 {
				r.Quitting()
			}
			if i%2 == 1 {
				w.fire(events.Common.WindowClosing)
				closed.Add(1)
			}
			got := r.TakeGitWarnings()
			taken.Lock()
			taken.warnings = append(taken.warnings, got...)
			taken.Unlock()
		})
	}
	close(start)
	wg.Wait()
	taken.Lock()
	taken.warnings = append(taken.warnings, r.TakeGitWarnings()...)
	taken.Unlock()

	if got, want := r.windowCount(), workers-int(closed.Load()); got != want {
		t.Errorf("window count = %d, want %d", got, want)
	}
	var gotDistros []string
	for _, w := range taken.warnings {
		gotDistros = append(gotDistros, w.Distro)
	}
	slices.Sort(gotDistros)
	wantDistros := slices.Clone(distros)
	slices.Sort(wantDistros)
	if !slices.Equal(gotDistros, wantDistros) {
		t.Errorf("warned distros = %q, want each of %q once", gotDistros, wantDistros)
	}
}
