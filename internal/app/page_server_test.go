//go:build server

package app

import (
	"sync"
	"testing"
	"time"
)

type sentEvent struct{ sender, name string }

type fakeBrowser struct {
	mu        sync.Mutex
	connected bool
	sent      []sentEvent
}

func (f *fakeBrowser) connect() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.connected = true
}

func (f *fakeBrowser) lookup(string) (string, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return "window-1", f.connected
}

func (f *fakeBrowser) emit(sender, name string, _ []any) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.sent = append(f.sent, sentEvent{sender, name})
}

func (f *fakeBrowser) senders() []sentEvent {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]sentEvent(nil), f.sent...)
}

func (f *fakeBrowser) names() []string {
	var names []string
	for _, e := range f.senders() {
		names = append(names, e.name)
	}
	return names
}

func newTestHold(f *fakeBrowser) *eventHold {
	return &eventHold{
		lookup: f.lookup, emit: f.emit,
		poll: time.Millisecond, giveUp: time.Second,
		pending: map[string][]heldEvent{},
	}
}

func (h *eventHold) idle() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return len(h.pending) == 0
}

func waitFor(t *testing.T, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for !cond() {
		if time.Now().After(deadline) {
			t.Fatal("condition not met in time")
		}
		time.Sleep(time.Millisecond)
	}
}

func TestTR_31_ServerPageHoldsEventsUntilItsWebsocketConnects(t *testing.T) {
	f := &fakeBrowser{}
	hold := newTestHold(f)

	hold.send("p", "a", nil)
	hold.send("p", "b", nil)
	if got := f.names(); len(got) != 0 {
		t.Fatalf("sent before connect: %v", got)
	}

	f.connect()
	hold.send("p", "c", nil)
	waitFor(t, func() bool { return len(f.names()) == 3 })

	for i, want := range []string{"a", "b", "c"} {
		if got := f.names()[i]; got != want {
			t.Fatalf("event %d = %q, want %q (all: %v)", i, got, want, f.names())
		}
	}
	waitFor(t, func() bool { return hold.idle() })
	hold.send("p", "d", nil)
	if got := f.names(); len(got) != 4 || got[3] != "d" {
		t.Fatalf("direct send after drain: %v", got)
	}
	if got := f.senders()[0].sender; got != "window-1" {
		t.Fatalf("sender = %q", got)
	}
}

func TestTR_31_ServerPageDropsHeldEventsOfAGonePage(t *testing.T) {
	f := &fakeBrowser{}
	hold := newTestHold(f)
	hold.giveUp = 20 * time.Millisecond

	hold.send("p", "a", nil)
	waitFor(t, func() bool { return hold.idle() })
	if got := f.names(); len(got) != 0 {
		t.Fatalf("sent for a gone page: %v", got)
	}
}
