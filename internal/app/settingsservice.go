package app

import (
	"context"

	"github.com/brogli/nebegit/internal/settings"
)

// SettingsService is the shell-only Wails service backing SettingsPanel
// (FR-UX-04): settings are a shell concern, not part of core.API.
type SettingsService struct{ store *settings.Store }

// NewSettingsService binds a SettingsService to store.
func NewSettingsService(store *settings.Store) *SettingsService {
	return &SettingsService{store: store}
}

// Get returns the current settings.
func (s *SettingsService) Get(ctx context.Context) (settings.Settings, error) {
	return s.store.Get(), nil
}

// Set replaces the persisted settings with next.
func (s *SettingsService) Set(ctx context.Context, next settings.Settings) error {
	return s.store.Update(func(cur *settings.Settings) { *cur = next })
}
