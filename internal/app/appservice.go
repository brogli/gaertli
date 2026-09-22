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
// platform default); satisfied by wailsPicker in production, a fake in
// tests. dir lets "Open WSL folder" (FR-REPO-07) seed the picker at a
// distro's UNC root.
type folderPicker interface {
	Pick(dir string) (path string, ok bool)
}

// wailsPicker wraps the native "choose a folder" dialog (FR-REPO-01).
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
// Consent's blocking bool (QA-PORT-02): the shell asks before the core is
// copied into a distro, naming exactly where. Not unit-tested (needs a
// real Wails window; folded into case 30's manual check), same as
// wailsNotifier/wailsPicker above.
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

// AppService is a shell-only Wails service (components.md): dialogs and
// recents are window-manager concerns no core.API method needs.
type AppService struct {
	shell
	picker folderPicker
}

// pickAndOpen shows the native folder picker seeded at dir ("" = the
// platform default); cancelling returns nil, not an error. Shared by
// PickAndOpen (FR-REPO-01) and PickAndOpenWSL (FR-REPO-07's "browse").
func (s *AppService) pickAndOpen(ctx context.Context, dir string) error {
	path, ok := s.picker.Pick(dir)
	if !ok {
		return nil
	}
	s.open(ctx, path)
	return nil
}

// PickAndOpen shows the native folder picker (FR-REPO-01); cancelling
// returns nil, not an error.
func (s *AppService) PickAndOpen(ctx context.Context) error {
	return s.pickAndOpen(ctx, "")
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

// ListWSLDistros lists installed distros for the "Open WSL folder" picker
// (FR-REPO-07). Errors (WslNotFound) are shown the same generic way any
// other core.Error is (ErrorBanner, FR-UX-01) — no bespoke handling.
func (s *AppService) ListWSLDistros(ctx context.Context) ([]remote.Distro, error) {
	return remote.ListDistros(ctx)
}

// PickAndOpenWSL shows the native folder picker seeded at distro's UNC root
// (FR-REPO-07's "browse", Research used: OpenFileDialogOptions.Directory);
// cancelling returns nil, not an error, same as PickAndOpen.
func (s *AppService) PickAndOpenWSL(ctx context.Context, distro string) error {
	return s.pickAndOpen(ctx, `\\wsl.localhost\`+distro)
}

// OpenPath opens path the same way PickAndOpen/OpenRecent do. The one new
// caller is "Open WSL folder"'s typed-path field, which builds the
// \\wsl.localhost\<distro>\<typed> string client-side before calling this.
func (s *AppService) OpenPath(ctx context.Context, path string) error {
	s.open(ctx, path)
	return nil
}

// OpenCompareDiff opens or focuses the compare diff window for wt's a..b
// comparison, starting on path/oldPath (FR-CMP-01's menu entry and
// FR-CMP-02's double-click both call this).
func (s *AppService) OpenCompareDiff(ctx context.Context, wt core.WorktreeID, a, b, path, oldPath string) error {
	s.reg.OpenOrFocusCompareDiff(wt, a, b, path, oldPath)
	return nil
}

// InstallWSLLauncher makes `nebegit <path>` work inside distro's own shell
// (FR-REPO-06's WSL half). This running process's own path is exactly what
// the installed launcher will hand off to.
func (s *AppService) InstallWSLLauncher(ctx context.Context, distro string) error {
	guiExe, err := os.Executable()
	if err != nil {
		return core.Errorf(core.Internal, "could not determine nebegit's own path: %v", err)
	}
	return s.reg.distroCore(distro).InstallLauncher(ctx, guiExe)
}
