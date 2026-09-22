package app

import (
	"context"

	"github.com/brogli/nebegit/internal/settings"
)

// SettingsService is the shell-only Wails service backing SettingsPanel:
// settings are a shell concern, not part of core.API.
type SettingsService struct {
	store *settings.Store
	reg   *Registry
}

// NewSettingsService binds a SettingsService to store, and reg so a change
// can re-push editor profiles to every core already using one.
func NewSettingsService(store *settings.Store, reg *Registry) *SettingsService {
	return &SettingsService{store: store, reg: reg}
}

// Get returns the current settings.
func (s *SettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return s.store.Get(), nil
}

// Set replaces the persisted settings with next, then re-pushes each
// environment's editor profile, so an edit takes effect at once.
func (s *SettingsService) Set(ctx context.Context, next settings.Settings) error {
	if err := s.store.Update(func(cur *settings.Settings) { *cur = next }); err != nil {
		return err
	}
	s.reg.PushEditorProfiles(ctx)
	return nil
}
