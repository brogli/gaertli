package app

import (
	"context"
	"fmt"
	"os"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// folderPicker shows the native folder picker, seeded at dir ("" = the
// platform default).
type folderPicker interface {
	Pick(dir string) (path string, ok bool)
}

// wailsPicker wraps the native "choose a folder" dialog.
type wailsPicker struct{ app *application.App }

func (p wailsPicker) Pick(dir string) (path string, ok bool) {
	selection, err := p.app.Dialog.OpenFileWithOptions(&application.OpenFileDialogOptions{
		CanChooseDirectories: true,
		Directory:            dir,
	}).PromptForSingleSelection()
	if err != nil || selection == "" {
		return "", false
	}
	return selection, true
}

// wailsConsent bridges DialogManager's callback-based Question dialog to
// Consent's blocking bool: the shell asks before the core is copied into a
// distro, naming exactly where.
type wailsConsent struct{ app *application.App }

func (c wailsConsent) Ask(distro, targetPath string) bool {
	result := make(chan bool, 1)
	yes := (&application.Button{Label: "Copy"}).OnClick(func() { result <- true })
	no := (&application.Button{Label: "Cancel", IsCancel: true}).OnClick(func() { result <- false })
	c.app.Dialog.Question().
		SetTitle("nebegit").
		SetMessage(fmt.Sprintf("nebegit needs to place its helper program inside %s, at %s. Copy it there?", distro, targetPath)).
		AddButtons([]*application.Button{yes, no}).
		SetDefaultButton(yes).SetCancelButton(no).
		Show()
	return <-result
}

// AppService is a shell-only Wails service: dialogs and recents are
// window-manager concerns no core.API method needs.
type AppService struct {
	shell
	picker folderPicker
}

// pickAndOpen shows the native folder picker seeded at dir ("" = the
// platform default) and opens the choice; cancelling returns nil.
func (s *AppService) pickAndOpen(ctx context.Context, dir string) error {
	path, ok := s.PickFolder(ctx, dir)
	if !ok {
		return nil
	}
	s.open(ctx, path)
	return nil
}

// PickFolder shows the native folder picker seeded at dir, with no "open it
// as a repo" side effect: CreateWorktreeDialog's target field must not be
// validated as an existing repo.
func (s *AppService) PickFolder(ctx context.Context, dir string) (path string, ok bool) {
	return s.picker.Pick(dir)
}

// callingWindowFrom extracts ctx's calling window
// (application.WindowKey), or false for a windowless/background call.
func callingWindowFrom(ctx context.Context) (callingWindow, bool) {
	w, ok := ctx.Value(application.WindowKey).(callingWindow)
	return w, ok
}

// SelectWorktree switches the calling window's selection. "This window"
// comes from ctx; the frontend never passes a window handle.
func (s *AppService) SelectWorktree(ctx context.Context, path string) error {
	w, ok := callingWindowFrom(ctx)
	if !ok {
		return core.Errorf(core.Internal, "no calling window for this call")
	}
	return s.switchTo(ctx, w, path)
}

// UpdateWindowTitle sets the calling window's OS title. The frontend calls
// it on every Summary change, so the title tracks branch changes.
func (s *AppService) UpdateWindowTitle(ctx context.Context, name, branch string) error {
	w, ok := callingWindowFrom(ctx)
	if !ok {
		return nil // a windowless/background call: nothing to title, not an error
	}
	w.SetTitle(windowTitle(name, branch))
	return nil
}

// PickAndOpen shows the native folder picker and opens the choice;
// cancelling returns nil.
func (s *AppService) PickAndOpen(ctx context.Context) error {
	return s.pickAndOpen(ctx, "")
}

// OpenRecent opens or focuses path's window.
func (s *AppService) OpenRecent(ctx context.Context, path string) error {
	s.open(ctx, path)
	return nil
}

// RecentRepos lists the remembered repos, most-recently-opened first.
func (s *AppService) RecentRepos(ctx context.Context) ([]settings.RecentRepo, error) {
	return s.store.Get().RecentRepos, nil
}

// ListWSLDistros lists installed distros for the "Open WSL folder" picker.
func (s *AppService) ListWSLDistros(ctx context.Context) ([]remote.Distro, error) {
	return remote.ListDistros(ctx)
}

// PickAndOpenWSL is PickAndOpen seeded at distro's UNC root.
func (s *AppService) PickAndOpenWSL(ctx context.Context, distro string) error {
	return s.pickAndOpen(ctx, `\\wsl.localhost\`+distro)
}

// OpenPath opens path like OpenRecent, for "Open WSL folder"'s typed path.
func (s *AppService) OpenPath(ctx context.Context, path string) error {
	s.open(ctx, path)
	return nil
}

// OpenCompareDiff opens or focuses the compare diff window for wt's a..b
// comparison, starting on path/oldPath.
func (s *AppService) OpenCompareDiff(ctx context.Context, wt core.WorktreeID, a, b, path, oldPath string) error {
	s.reg.OpenOrFocusCompareDiff(wt, a, b, path, oldPath)
	return nil
}

// InstallWSLLauncher makes `nebegit <path>` work inside distro's shell; the
// launcher hands off to this process's executable.
func (s *AppService) InstallWSLLauncher(ctx context.Context, distro string) error {
	guiExe, err := os.Executable()
	if err != nil {
		return core.Errorf(core.Internal, "could not determine nebegit's own path: %v", err)
	}
	return s.reg.distroCore(distro).InstallLauncher(ctx, guiExe)
}

// TestEditorProfile tries profile on env's core: "" is the local core, else
// a WSL distro's, connected (and deployed, with consent) on demand.
func (s *AppService) TestEditorProfile(ctx context.Context, env string, profile core.EditorProfile) error {
	if env == "" {
		return s.reg.Local().TestEditorProfile(ctx, profile)
	}
	client, _, err := s.reg.distroCore(env).Ensure(ctx)
	if err != nil {
		return err
	}
	return client.TestEditorProfile(ctx, profile)
}
