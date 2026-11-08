//go:build race && server

// Requirements: QA-REL-04

package app

import (
	"fmt"
	"slices"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type heldDelivery struct {
	sender string
	seq    int
}

func TestEventHoldDeliversEverySendOnceAndInOrderPerSenderWhileTheDrainRuns(t *testing.T) {
	const senders, perSender = 8, 50

	var found atomic.Bool
	var delivered []heldDelivery // guarded by the hold's mutex: emit runs under it
	hold := &eventHold{
		lookup: func(string) (string, bool) { return "page", found.Load() },
		emit: func(_, name string, data []any) {
			delivered = append(delivered, heldDelivery{name, data[0].(int)})
		},
		poll:    time.Millisecond,
		giveUp:  time.Minute,
		pending: map[string][]heldEvent{},
	}

	var wg sync.WaitGroup
	for s := range senders {
		wg.Go(func() {
			for seq := range perSender {
				hold.send("client", fmt.Sprint(s), []any{seq})
				if seq == perSender/2 && s == 0 {
					found.Store(true)
				}
			}
		})
	}
	wg.Wait()

	deadline := time.Now().Add(time.Second)
	for {
		hold.mu.Lock()
		drained := len(hold.pending) == 0
		got := slices.Clone(delivered)
		hold.mu.Unlock()
		if drained {
			assertDeliveredOncePerSenderInOrder(t, got, senders, perSender)
			return
		}
		if time.Now().After(deadline) {
			t.Fatal("held events were not drained within 1s")
		}
		time.Sleep(time.Millisecond)
	}
}

func assertDeliveredOncePerSenderInOrder(t *testing.T, got []heldDelivery, senders, perSender int) {
	t.Helper()
	if len(got) != senders*perSender {
		t.Fatalf("delivered %d events, want %d", len(got), senders*perSender)
	}
	next := map[string]int{}
	for _, d := range got {
		if d.seq != next[d.sender] {
			t.Fatalf("sender %s delivered seq %d, want %d", d.sender, d.seq, next[d.sender])
		}
		next[d.sender]++
	}
}
