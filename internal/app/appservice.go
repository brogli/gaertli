package app

import (
	"context"
	"errors"
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
		SetTitle(text(appName, nil)).
		SetMessage(text(deployConsent, map[string]string{"distro": distro, "path": targetPath})).
		AddButtons([]*application.Button{yes, no}).
		SetDefaultButton(yes).SetCancelButton(no).
		Show()
	return <-result
}

// AppService is the shell's own Wails service: windows, native dialogs,
// recent repos and WSL setup, which no core.API method covers.
type AppService struct {
	shell
	picker folderPicker
}

// pickAndOpen shows the native folder picker seeded at dir ("" = the
// platform default) and opens the choice; cancelling returns nil.
func (s *AppService) pickAndOpen(ctx context.Context, dir string) error {
	if path, ok := s.PickFolder(ctx, dir); ok {
		s.open(ctx, path)
	}
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

// SelectRecentRepo switches the calling window to repo's remembered
// worktree; if that worktree is gone (Code NotARepo) and repo names its
// common git dir, it retries at the repo's main worktree, found there
// directly with no worktree of its own needed. A second failure is
// returned as-is; the repo picker shows it.
func (s *AppService) SelectRecentRepo(ctx context.Context, repo settings.RecentRepo) error {
	w, ok := callingWindowFrom(ctx)
	if !ok {
		return core.Errorf(core.Internal, "no calling window for this call")
	}
	err := s.switchTo(ctx, w, shellPath(repo.Path, repo.Distro))
	ce, isCoreError := errors.AsType[*core.Error](err)
	if !isCoreError || ce.Code != core.NotARepo || repo.RepoKey == "" {
		return err
	}
	main, err := s.api.MainWorktree(ctx, core.RepoPath(shellPath(repo.RepoKey, repo.Distro)))
	if err != nil {
		return err
	}
	return s.switchTo(ctx, w, shellPath(main.Path, repo.Distro))
}

// UpdateWindowTitle sets the calling window's OS title from its worktree's
// path and branch. The frontend calls it on every Summary change, so the
// title tracks branch changes.
func (s *AppService) UpdateWindowTitle(ctx context.Context, path, branch string) error {
	w, ok := callingWindowFrom(ctx)
	if !ok {
		return nil // a windowless/background call: nothing to title, not an error
	}
	w.SetTitle(windowTitle(path, branch))
	return nil
}

// PickAndOpen shows the native folder picker and opens the choice;
// cancelling returns nil.
func (s *AppService) PickAndOpen(ctx context.Context) error {
	return s.pickAndOpen(ctx, "")
}

// RecentRepos lists the remembered repos, most-recently-opened first.
func (s *AppService) RecentRepos(ctx context.Context) ([]settings.RecentRepo, error) {
	return s.store.Get().RecentRepos, nil
}

// ListWSLDistros lists installed distros for the "Open WSL folder" picker.
// A host without WSL has none; that is an answer, not an error.
func (s *AppService) ListWSLDistros(ctx context.Context) ([]remote.Distro, error) {
	distros, err := remote.ListDistros(ctx)
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Code == core.WslNotFound {
		return nil, nil
	}
	return distros, err
}

// PickAndOpenWSL is PickAndOpen seeded at distro's UNC root.
func (s *AppService) PickAndOpenWSL(ctx context.Context, distro string) error {
	return s.pickAndOpen(ctx, remote.BuildWSLPath(distro, ""))
}

// OpenPath opens or focuses path's window.
func (s *AppService) OpenPath(ctx context.Context, path string) error {
	s.open(ctx, path)
	return nil
}

// OpenDiff opens the calling window's diff window on wt's target, or
// focuses it and shows the target there. A call from no window uses wt's
// diff window.
func (s *AppService) OpenDiff(ctx context.Context, wt core.WorktreeID, target DiffTarget) error {
	key := diffWindowKey{wt: wt}
	if w, ok := callingWindowFrom(ctx); ok {
		key = diffWindowKey{mainWindow: w.ID()}
	}
	s.reg.OpenOrFocusDiff(key, wt, target)
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

// LogPathInfo is LogPath's result: where the app log is, or why it could
// not be opened.
type LogPathInfo struct {
	Path  string `json:"path"`
	Error string `json:"error"` // "" if the log opened
}

// LogPath is Settings' read-only "App log: <path>" line.
func (s *AppService) LogPath(ctx context.Context) (LogPathInfo, error) {
	return LogPathInfo{Path: s.logPath, Error: s.logErr}, nil
}
