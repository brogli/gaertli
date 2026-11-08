//go:build server

// Requirements: QA-MNT-02

package app

import (
	"slices"
	"sync"
	"testing"
	"testing/synctest"
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
	return slices.Clone(f.sent)
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

func TestServerPageHoldsEventsUntilItsWebsocketConnects(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeBrowser{}
		hold := newTestHold(f)

		hold.send("p", "a", nil)
		hold.send("p", "b", nil)
		if got := f.names(); len(got) != 0 {
			t.Fatalf("sent before connect: %v", got)
		}

		f.connect()
		hold.send("p", "c", nil)
		time.Sleep(hold.poll)
		synctest.Wait()

		if got, want := f.names(), []string{"a", "b", "c"}; !slices.Equal(got, want) {
			t.Fatalf("events = %v, want %v", got, want)
		}
		if !hold.idle() {
			t.Fatal("hold still pending after the drain")
		}
		hold.send("p", "d", nil)
		if got := f.names(); len(got) != 4 || got[3] != "d" {
			t.Fatalf("direct send after drain: %v", got)
		}
		if got := f.senders()[0].sender; got != "window-1" {
			t.Fatalf("sender = %q", got)
		}
	})
}

func TestServerPageDropsHeldEventsOfAGonePage(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		f := &fakeBrowser{}
		hold := newTestHold(f)
		hold.giveUp = 20 * time.Millisecond

		hold.send("p", "a", nil)
		time.Sleep(hold.giveUp + hold.poll)
		synctest.Wait()
		if !hold.idle() {
			t.Fatal("hold still pending after giving up")
		}
		if got := f.names(); len(got) != 0 {
			t.Fatalf("sent for a gone page: %v", got)
		}
	})
}
