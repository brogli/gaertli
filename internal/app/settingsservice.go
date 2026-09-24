package app

import (
	"context"

	"github.com/brogli/nebegit/internal/settings"
)

// emitter sends an event to every window; satisfied by
// *application.EventManager.
type emitter interface {
	Emit(name string, data ...any) bool
}

// SettingsService is the shell-only Wails service backing SettingsPanel:
// settings are a shell concern, not part of core.API.
type SettingsService struct {
	store  *settings.Store
	reg    *Registry
	events emitter
}

// NewSettingsService binds a SettingsService to store, reg so a change can
// re-push editor profiles to every core already using one, and events to
// tell every window about the change.
func NewSettingsService(store *settings.Store, reg *Registry, events emitter) *SettingsService {
	return &SettingsService{store: store, reg: reg, events: events}
}

// Get returns the current settings.
func (s *SettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return s.store.Get(), nil
}

// Set replaces the persisted settings with next, re-pushes each
// environment's editor profile, then tells every window to reload, so an
// edit takes effect at once everywhere.
func (s *SettingsService) Set(ctx context.Context, next settings.Settings) error {
	if err := s.store.Update(func(cur *settings.Settings) { *cur = next }); err != nil {
		return err
	}
	s.reg.PushEditorProfiles(ctx)
	s.events.Emit("settings:changed")
	return nil
}
