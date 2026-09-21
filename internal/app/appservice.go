package app

import (
	"context"

	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// folderPicker shows the native folder picker; satisfied by wailsPicker in
// production, a fake in tests.
type folderPicker interface{ Pick() (path string, ok bool) }

// wailsPicker wraps the native "choose a folder" dialog (FR-REPO-01).
type wailsPicker struct{ app *application.App }

func (p wailsPicker) Pick() (path string, ok bool) {
	selection, err := p.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseDirectories: true,
	}).PromptForSingleSelection()
	if err != nil || selection == "" {
		return "", false
	}
	return selection, true
}

// AppService is a shell-only Wails service (components.md): dialogs and
// recents are window-manager concerns no core.API method needs.
type AppService struct {
	shell
	picker folderPicker
}

// PickAndOpen shows the native folder picker (FR-REPO-01); cancelling
// returns nil, not an error.
func (s *AppService) PickAndOpen(ctx context.Context) error {
	path, ok := s.picker.Pick()
	if !ok {
		return nil
	}
	s.open(ctx, path)
	return nil
}

// OpenRecent re-opens an already-remembered path (FR-REPO-02) through the
// same openWorktreePath flow.
func (s *AppService) OpenRecent(ctx context.Context, path string) error {
	s.open(ctx, path)
	return nil
}

// RecentRepos lists the remembered repos, most-recently-opened first.
func (s *AppService) RecentRepos(ctx context.Context) ([]settings.RecentRepo, error) {
	return s.store.Get().RecentRepos, nil
}
