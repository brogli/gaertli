package app

import (
	"maps"
	"slices"
	"testing"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/settings"
)

// fakeEmitter records every event emitted to all windows, after running
// onEmit, if set.
type fakeEmitter struct {
	emitted []emittedEvent
	onEmit  func()
}

func (e *fakeEmitter) Emit(name string, data ...any) bool {
	if e.onEmit != nil {
		e.onEmit()
	}
	e.emitted = append(e.emitted, newEmittedEvent(name, data))
	return false
}

func TestSettingsService_SetThenGetRoundTrips(t *testing.T) {
	reg, _ := newTestRegistry()
	s := NewSettingsService(newTestStore(t), reg, &fakeEmitter{})
	want := settings.Settings{
		EditorProfiles:  map[string]core.EditorProfile{"": {Name: "the editor", FolderCmd: "editor {path}"}},
		DefaultPullMode: "rebase",
		DiffMode:        "unified",
	}

	if err := s.Set(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !maps.Equal(got.EditorProfiles, want.EditorProfiles) || got.DefaultPullMode != want.DefaultPullMode {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

// An edit takes effect immediately wherever the profile is already in use,
// not just on the next connect.
func TestTR_13_SettingsServiceSetPushesTheNewProfileToTheLocalCore(t *testing.T) {
	reg, _ := newTestRegistry()
	var pushed []core.EditorProfile
	reg.SetLocal(fakeAPI{editorProfiles: &pushed})
	store := newTestStore(t)
	reg.SetStore(store)
	s := NewSettingsService(store, reg, &fakeEmitter{})

	next := settings.Settings{EditorProfiles: map[string]core.EditorProfile{"": {Name: "another editor", FolderCmd: "other {path}"}}}
	if err := s.Set(t.Context(), next); err != nil {
		t.Fatal(err)
	}

	if len(pushed) != 1 || pushed[0].Name != "another editor" {
		t.Errorf("pushed = %+v, want exactly one push of another editor", pushed)
	}
}

// Every window reloads on the event, so the new profile must already be
// pushed when it goes out.
func TestFR_ED_05_SettingsServiceSetEmitsSettingsChangedAfterThePush(t *testing.T) {
	reg, _ := newTestRegistry()
	var pushed []core.EditorProfile
	reg.SetLocal(fakeAPI{editorProfiles: &pushed})
	store := newTestStore(t)
	reg.SetStore(store)
	pushesAtEmit := -1
	events := &fakeEmitter{onEmit: func() { pushesAtEmit = len(pushed) }}
	s := NewSettingsService(store, reg, events)

	if err := s.Set(t.Context(), settings.Settings{}); err != nil {
		t.Fatal(err)
	}

	if want := []emittedEvent{{name: "settings:changed"}}; !slices.Equal(events.emitted, want) {
		t.Errorf("emitted = %+v, want %+v", events.emitted, want)
	}
	if pushesAtEmit != 1 {
		t.Errorf("pushes before the event = %d, want 1", pushesAtEmit)
	}
	assertRegisteredType(t, "settings:changed", nil)
}
