package app

import (
	"log/slog"
	"maps"
	"slices"
	"testing"

	"github.com/brogli/nebegit/internal/applog"
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

func TestFR_UX_04_SettingsServiceSetThenGetRoundTrips(t *testing.T) {
	reg, _ := newTestRegistry()
	s := NewSettingsService(newTestStore(t), reg, &fakeEmitter{}, &slog.LevelVar{})
	want := settings.Preferences{
		EditorProfiles:  map[string]core.EditorProfile{"": {Name: "Alpha", FolderCmd: "alpha {path}"}},
		DefaultPullMode: core.PullRebase,
		DiffMode:        settings.UnifiedDiff,
	}

	if err := s.Set(t.Context(), want); err != nil {
		t.Fatal(err)
	}
	got := s.Get()
	if !maps.Equal(got.EditorProfiles, want.EditorProfiles) || got.DefaultPullMode != want.DefaultPullMode {
		t.Errorf("Get() = %+v, want %+v", got, want)
	}
}

// Set replaces the preferences only: a repo opened or a window closed
// while the dialog was open survives its Save.
func TestFR_UX_04_SettingsServiceSetKeepsTheRecentReposAndOpenWorktrees(t *testing.T) {
	reg, _ := newTestRegistry()
	store := newTestStore(t)
	recent := []settings.RecentRepo{{Path: "/opened/meanwhile"}}
	open := []string{"/still/open"}
	if err := store.Update(func(s *settings.Settings) { s.RecentRepos, s.OpenWorktrees = recent, open }); err != nil {
		t.Fatal(err)
	}
	s := NewSettingsService(store, reg, &fakeEmitter{}, &slog.LevelVar{})

	if err := s.Set(t.Context(), settings.Preferences{DiffMode: settings.SideBySideDiff}); err != nil {
		t.Fatal(err)
	}

	got := store.Get()
	if !slices.Equal(got.RecentRepos, recent) || !slices.Equal(got.OpenWorktrees, open) {
		t.Errorf("recent, open = %v, %v; want %v, %v kept", got.RecentRepos, got.OpenWorktrees, recent, open)
	}
	if got.DiffMode != settings.SideBySideDiff {
		t.Errorf("DiffMode = %q, want the saved side-by-side", got.DiffMode)
	}
}

// Set replaces the preferences only: the diff window's remembered size
// (outside Preferences, like RecentRepos and OpenWorktrees) survives it too.
func TestFR_UX_04_SettingsServiceSetKeepsTheDiffWindowSize(t *testing.T) {
	reg, _ := newTestRegistry()
	store := newTestStore(t)
	want := &settings.WindowSize{Width: 900, Height: 600}
	if err := store.Update(func(s *settings.Settings) { s.DiffWindowSize = want }); err != nil {
		t.Fatal(err)
	}
	s := NewSettingsService(store, reg, &fakeEmitter{}, &slog.LevelVar{})

	if err := s.Set(t.Context(), settings.Preferences{}); err != nil {
		t.Fatal(err)
	}

	if got := store.Get().DiffWindowSize; got == nil || *got != *want {
		t.Errorf("DiffWindowSize = %+v, want %+v kept", got, want)
	}
}

func TestFR_UX_19_SetSwitchesTheLogLevelAtOnce(t *testing.T) {
	reg, _ := newTestRegistry()
	store := newTestStore(t)
	level := &slog.LevelVar{} // zero value is Info
	s := NewSettingsService(store, reg, &fakeEmitter{}, level)

	if err := s.Set(t.Context(), settings.Preferences{DebugMode: true}); err != nil {
		t.Fatal(err)
	}
	if level.Level() != slog.LevelDebug {
		t.Errorf("level after DebugMode true = %v, want Debug", level.Level())
	}

	if err := s.Set(t.Context(), settings.Preferences{DebugMode: false}); err != nil {
		t.Fatal(err)
	}
	if level.Level() != slog.LevelInfo {
		t.Errorf("level after DebugMode false = %v, want Info", level.Level())
	}
}

func TestFR_UX_19_SetKeepsDebugWhenNebegitLogForcesIt(t *testing.T) {
	t.Setenv("NEBEGIT_LOG", "debug")
	reg, _ := newTestRegistry()
	store := newTestStore(t)
	level := &slog.LevelVar{}
	applog.SetLevel(level, false) // as applog.Open sets it at start
	s := NewSettingsService(store, reg, &fakeEmitter{}, level)

	if err := s.Set(t.Context(), settings.Preferences{DebugMode: false}); err != nil {
		t.Fatal(err)
	}

	if level.Level() != slog.LevelDebug {
		t.Errorf("level = %v, want Debug (forced by NEBEGIT_LOG)", level.Level())
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
	s := NewSettingsService(store, reg, &fakeEmitter{}, &slog.LevelVar{})

	next := settings.Preferences{EditorProfiles: map[string]core.EditorProfile{"": {Name: "Beta", FolderCmd: "beta {path}"}}}
	if err := s.Set(t.Context(), next); err != nil {
		t.Fatal(err)
	}

	if len(pushed) != 1 || pushed[0].Name != "Beta" {
		t.Errorf("pushed = %+v, want exactly one push of Beta", pushed)
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
	s := NewSettingsService(store, reg, events, &slog.LevelVar{})

	if err := s.Set(t.Context(), settings.Preferences{}); err != nil {
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
