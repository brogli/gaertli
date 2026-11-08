// Requirements: QA-PORT-02

package app

import (
	"encoding/json"
	"log/slog"
	"maps"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/brogli/gaertli/internal/core"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// wailsEvents checks an event's data the way a running Wails app does
// before delivering it.
var wailsEvents = application.NewWailsEventProcessor(func(*application.CustomEvent) {})

// registered reports whether name is registered. Wails lets any data
// through for an unregistered name, so data no event is registered with
// tells the two apart.
func registered(name string) bool {
	type unregistered struct{}
	return wailsEvents.Emit(&application.CustomEvent{Name: name, Data: unregistered{}}) != nil
}

// assertRegisteredType fails unless name is registered and data has exactly
// its registered type.
func assertRegisteredType(t *testing.T, name string, data any) {
	t.Helper()
	if !registered(name) {
		t.Errorf("%q is not registered", name)
	}
	if err := wailsEvents.Emit(&application.CustomEvent{Name: name, Data: data}); err != nil {
		t.Error(err)
	}
}

func TestDispatchEmitsEachEventsRegisteredPayloadType(t *testing.T) {
	// "command" is not sampled here: dispatch logs it instead of emitting it
	// to a window (see applog_test.go).
	samples := map[string]any{
		"repo:changed": core.RepoChanged{Scope: core.RefsChanged},
		"log:progress": core.LogProgress{Loaded: 5, Done: true, Filtered: true, Generation: 2},
		"job:update": core.JobInfo{
			ID: "j1", Kind: core.FetchJob, State: core.JobFailed, Progress: -1, Message: "m",
			Error: &core.Error{Code: core.AuthFailed, Message: "no", GitStderr: "fatal", Command: "git fetch"},
		},
		"core:reconnected": nil,
	}
	for name, sample := range samples {
		t.Run(name, func(t *testing.T) {
			r, opener := newTestRegistry()
			r.OpenOrFocus("wt", "/wt", "")
			payload, err := json.Marshal(sample)
			if err != nil {
				t.Fatal(err)
			}

			r.Dispatch(core.Event{Name: name, Worktree: "wt", Payload: payload})

			emitted := opener.windows[0].emitted
			if len(emitted) != 1 {
				t.Fatalf("emitted %d events, want 1", len(emitted))
			}
			if got := emitted[0].payload; !reflect.DeepEqual(got, sample) {
				t.Errorf("payload = %#v, want %#v", got, sample)
			}
			assertRegisteredType(t, name, emitted[0].payload)
		})
	}
}

// emittedEventName matches an event name where the shell's own sources
// emit it: a core.Event literal or a direct Emit/EmitEvent call.
var emittedEventName = regexp.MustCompile(`Event\{Name: "([^"]+)"|Emit(?:Event)?\("([^"]+)"`)

// emittedEventNames collects the event names emitted by the non-test Go
// files in dirs, sorted and without duplicates.
func emittedEventNames(t *testing.T, dirs ...string) []string {
	t.Helper()
	var names []string
	for _, dir := range dirs {
		files, err := filepath.Glob(filepath.Join(dir, "*.go"))
		if err != nil {
			t.Fatal(err)
		}
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			src, err := os.ReadFile(file)
			if err != nil {
				t.Fatal(err)
			}
			for _, m := range emittedEventName.FindAllStringSubmatch(string(src), -1) {
				names = append(names, m[1]+m[2])
			}
		}
	}
	slices.Sort(names)
	return slices.Compact(names)
}

func TestEveryEmittedEventDecodesAndOnlyWindowEventsAreRegistered(t *testing.T) {
	emitted := emittedEventNames(t, ".", "../session", "../services")

	if decoded := slices.Sorted(maps.Keys(eventPayloads)); !slices.Equal(decoded, emitted) {
		t.Errorf("eventPayloads has %v, want the emitted events %v", decoded, emitted)
	}

	for _, name := range emitted {
		t.Run(name, func(t *testing.T) {
			r, opener := newTestRegistry()
			r.OpenOrFocus("wt", "/wt", "")

			r.Dispatch(core.Event{Name: name, Worktree: "wt", Payload: json.RawMessage("null")})

			received := opener.windows[0].emitted
			switch len(received) {
			case 1:
				assertRegisteredType(t, name, received[0].payload)
			case 0:
				if registered(name) {
					t.Errorf("%q is registered but reaches no window", name)
				}
			default:
				t.Fatalf("emitted %d events, want 0 or 1", len(received))
			}
		})
	}
}

func TestDispatchDropsAndLogsAPayloadThatDoesNotDecode(t *testing.T) {
	r, opener := newTestRegistry()
	var logged strings.Builder
	r.log = slog.New(slog.NewTextHandler(&logged, nil))
	r.OpenOrFocus("wt", "/wt", "")

	r.Dispatch(core.Event{Name: "repo:changed", Worktree: "wt", Payload: json.RawMessage(`{"scope":3}`)})

	if n := len(opener.windows[0].emitted); n != 0 {
		t.Errorf("emitted %d events, want 0", n)
	}
	if lines := strings.Count(logged.String(), "\n"); lines != 1 {
		t.Errorf("logged %d lines, want 1: %q", lines, logged.String())
	}
}

func TestTheDiffWindowIsRetargetedWithTheRegisteredPayloadType(t *testing.T) {
	r, opener := newTestRegistry()
	key := diffWindowKey{wt: "wt"}

	r.OpenOrFocusDiff(key, "wt", DiffTarget{Kind: CommitDiff, Sha: "a", Path: "f1"})
	r.OpenOrFocusDiff(key, "wt", DiffTarget{Kind: RangeDiff, From: "a", To: "b", Path: "f2", OldPath: "f0"})

	got := opener.windows[0].emitted[0]
	assertRegisteredType(t, got.name, got.payload)
}
