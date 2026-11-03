package app

import (
	"context"
	"log/slog"

	"github.com/brogli/nebegit/internal/applog"
	"github.com/brogli/nebegit/internal/settings"
)

// emitter sends an event to every window; satisfied by
// *application.EventManager.
type emitter interface {
	Emit(name string, data ...any) bool
}

// SettingsService is the shell-only Wails service behind the frontend's
// settings store: settings are a shell concern, not part of core.API.
type SettingsService struct {
	store  *settings.Store
	reg    *Registry
	events emitter
	level  *slog.LevelVar
}

// NewSettingsService binds a SettingsService to store, reg so a change can
// re-push editor profiles to every core already using one, events to tell
// every window about the change, and level, the app log's level.
func NewSettingsService(store *settings.Store, reg *Registry, events emitter, level *slog.LevelVar) *SettingsService {
	return &SettingsService{store: store, reg: reg, events: events, level: level}
}

// Get returns the current preferences.
func (s *SettingsService) Get() settings.Preferences {
	return s.store.Get().Preferences
}

// Set replaces the persisted preferences with next, sets the app log's
// level from its DebugMode, re-pushes each environment's editor profile,
// then tells every window to reload, so an edit takes effect at once
// everywhere.
func (s *SettingsService) Set(ctx context.Context, next settings.Preferences) error {
	if err := s.store.Update(func(cur *settings.Settings) { cur.Preferences = next }); err != nil {
		return err
	}
	applog.SetLevel(s.level, next.DebugMode)
	s.reg.PushEditorProfile(ctx)
	s.events.Emit("settings:changed")
	return nil
}
