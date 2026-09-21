package app

import (
	"slices"
	"sort"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// fakeWindow is a shellWindow that records what was called on it, and lets
// a test simulate the OS closing it.
type fakeWindow struct {
	focusCount int
	emitCount  int
	closing    func(*application.WindowEvent)
}

func (w *fakeWindow) Focus() { w.focusCount++ }

func (w *fakeWindow) EmitEvent(string, ...any) bool {
	w.emitCount++
	return true
}

func (w *fakeWindow) OnWindowEvent(eventType events.WindowEventType, callback func(*application.WindowEvent)) func() {
	if eventType == events.Common.WindowClosing {
		w.closing = callback
	}
	return func() {}
}

// close simulates the OS destroying the window.
func (w *fakeWindow) close() {
	if w.closing != nil {
		w.closing(&application.WindowEvent{})
	}
}

// fakeOpener is a windowOpener that hands out fakeWindows and records them
// in creation order.
type fakeOpener struct {
	windows []*fakeWindow
}

func (o *fakeOpener) NewWithOptions(application.WebviewWindowOptions) shellWindow {
	w := &fakeWindow{}
	o.windows = append(o.windows, w)
	return w
}

func newTestRegistry() (*Registry, *fakeOpener) {
	opener := &fakeOpener{}
	r := NewRegistry()
	r.opener = opener
	r.SetLocal(fakeAPI{})
	return r, opener
}

func TestFR_WT_05_OpenOrFocusOpensADistinctWindowPerPath(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("/a", "/a")
	r.OpenOrFocus("/b", "/b")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
	if opener.windows[0].focusCount != 0 {
		t.Errorf("/a's window focused %d times, want 0", opener.windows[0].focusCount)
	}
}

func TestFR_WT_08_OpenOrFocusFocusesAnExistingWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("/a", "/a")
	r.OpenOrFocus("/a", "/a")

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if got := opener.windows[0].focusCount; got != 1 {
		t.Errorf("focus count = %d, want 1", got)
	}
}

func TestADR_9_DispatchTargetsTheWindowsAnEventConcerns(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("wtA", "wtA")
	r.OpenOrFocus("wtB", "wtB")
	r.OpenOrFocus("wtC", "wtC")
	r.SetRepo("wtA", "R")
	r.SetRepo("wtB", "R")
	r.SetRepo("wtC", "S")
	wtA, wtB, wtC := opener.windows[0], opener.windows[1], opener.windows[2]

	r.Dispatch(core.Event{Worktree: "wtA"})

	if wtA.emitCount != 1 {
		t.Errorf("wtA emitCount = %d, want 1", wtA.emitCount)
	}
	if wtB.emitCount != 0 || wtC.emitCount != 0 {
		t.Errorf("wtB, wtC emitCount = %d, %d, want 0, 0", wtB.emitCount, wtC.emitCount)
	}

	r.Dispatch(core.Event{Repo: "R"})

	if wtA.emitCount != 2 || wtB.emitCount != 1 {
		t.Errorf("wtA, wtB emitCount = %d, %d, want 2, 1", wtA.emitCount, wtB.emitCount)
	}
	if wtC.emitCount != 0 {
		t.Errorf("wtC emitCount = %d, want 0", wtC.emitCount)
	}
}

func TestFR_REPO_02_ClosingAWindowLetsOpenOrFocusCreateANewOne(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("X", "/x")
	opener.windows[0].close()
	r.OpenOrFocus("X", "/x")

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
	if opener.windows[0].focusCount != 0 {
		t.Errorf("closed window focused %d times, want 0", opener.windows[0].focusCount)
	}
}

func TestOpenHomeOpensAtMostOneWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenHome()
	r.OpenHome()

	if len(opener.windows) != 1 {
		t.Fatalf("windows created = %d, want 1", len(opener.windows))
	}
	if opener.windows[0].focusCount != 1 {
		t.Errorf("focus count = %d, want 1", opener.windows[0].focusCount)
	}
}

func TestOpenHomeReopensAfterItIsClosed(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenHome()
	opener.windows[0].close()
	r.OpenHome()

	if len(opener.windows) != 2 {
		t.Fatalf("windows created = %d, want 2", len(opener.windows))
	}
}

func TestOpenWorktreePathsListsOpenWindows(t *testing.T) {
	r, _ := newTestRegistry()

	r.OpenOrFocus("a", "/a")
	r.OpenOrFocus("b", "/b")

	got := r.OpenWorktreePaths()
	sort.Strings(got)
	if want := []string{"/a", "/b"}; !slices.Equal(got, want) {
		t.Errorf("OpenWorktreePaths() = %v, want %v", got, want)
	}
}

func TestQA_PERF_06_ClosingAWindowCallsCloseWorktreeOnTheLocalCore(t *testing.T) {
	r, opener := newTestRegistry()
	var closed []core.WorktreeID
	r.SetLocal(fakeAPI{closed: &closed})

	r.OpenOrFocus("X", "/x")
	opener.windows[0].close()

	if len(closed) != 1 || closed[0] != "X" {
		t.Errorf("CloseWorktree calls = %v, want [\"X\"]", closed)
	}
}

func TestOpenWorktreePathsDropsAClosedWindow(t *testing.T) {
	r, opener := newTestRegistry()

	r.OpenOrFocus("a", "/a")
	opener.windows[0].close()

	if got := r.OpenWorktreePaths(); len(got) != 0 {
		t.Errorf("OpenWorktreePaths() = %v, want empty", got)
	}
}
