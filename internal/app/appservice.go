package app

import (
	"context"
	"errors"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// folderPicker shows the native folder picker, seeded at dir ("" = the
// platform default). On Windows it reaches a WSL distro's folders through
// the \\wsl.localhost share.
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

// nativeConsent asks before installing the shell's helper in a distro,
// through ask rather than a Wails dialog: a distro deploy can happen while
// restoring windows at startup, before Wails' own App.Run creates its
// platform layer, and a Wails dialog needs that layer.
type nativeConsent struct {
	ask func(title, msg string) bool
}

func (c nativeConsent) Ask(distro string) bool {
	return c.ask(text(appName, nil), consentMessage(distro))
}

// consentMessage is the consent dialog's text.
func consentMessage(distro string) string {
	return text(deployConsent, map[string]string{"distro": distro})
}

// AppService is the shell's own Wails service: windows, native dialogs,
// recent repos, the installed WSL distros and the app log's path, which no
// core.API method covers.
type AppService struct {
	shell
	picker folderPicker
}

// PickFolder shows the native folder picker seeded at dir, with no "open it
// as a repo" side effect: CreateWorktreeDialog's target field must not be
// validated as an existing repo.
func (s *AppService) PickFolder(dir string) (path string, ok bool) {
	return s.picker.Pick(dir)
}

// callingWindowFrom extracts ctx's calling window
// (application.WindowKey), or false for a windowless/background call.
func callingWindowFrom(ctx context.Context) (callingWindow, bool) {
	w, ok := ctx.Value(application.WindowKey).(callingWindow)
	return w, ok
}

// requireCallingWindow is callingWindowFrom for a call that acts on its
// window: a windowless call is Code Internal.
func requireCallingWindow(ctx context.Context) (callingWindow, error) {
	if w, ok := callingWindowFrom(ctx); ok {
		return w, nil
	}
	return nil, core.Errorf(core.Internal, "no calling window for this call")
}

// SelectWorktree switches the calling window's selection. "This window"
// comes from ctx; the frontend never passes a window handle.
func (s *AppService) SelectWorktree(ctx context.Context, path string) error {
	w, err := requireCallingWindow(ctx)
	if err != nil {
		return err
	}
	return s.switchTo(ctx, w, path)
}

// SelectRecentRepo switches the calling window to repo's remembered
// worktree, with resolveRecentRepo's fallback. A second failure is
// returned as-is; the repo picker shows it.
func (s *AppService) SelectRecentRepo(ctx context.Context, repo settings.RecentRepo) error {
	w, err := requireCallingWindow(ctx)
	if err != nil {
		return err
	}
	return s.resolveRecentRepo(ctx, repo, func(path string) error {
		return s.switchTo(ctx, w, path)
	})
}

// OpenRecentRepo opens repo in a window of its own, with resolveRecentRepo's
// fallback, or brings forward the window already showing that worktree. A
// second failure is returned as-is; the repo picker shows it.
func (s *AppService) OpenRecentRepo(ctx context.Context, repo settings.RecentRepo) error {
	return s.resolveRecentRepo(ctx, repo, func(path string) error {
		return s.openOrFocus(ctx, path)
	})
}

// resolveRecentRepo runs action on repo's remembered worktree path; if that
// fails (Code NotARepo) and repo names its common git dir, it retries action
// at the repo's main worktree, found there directly with no worktree of its
// own needed.
func (s *AppService) resolveRecentRepo(ctx context.Context, repo settings.RecentRepo, action func(path string) error) error {
	err := action(shellPath(repo.Path, repo.Distro))
	ce, isCoreError := errors.AsType[*core.Error](err)
	if !isCoreError || ce.Code != core.NotARepo || repo.RepoKey == "" {
		return err
	}
	main, err := s.api.MainWorktree(ctx, core.RepoPath(shellPath(repo.RepoKey, repo.Distro)))
	if err != nil {
		return err
	}
	return action(shellPath(main.Path, repo.Distro))
}

// UpdateWindowTitle sets the calling window's OS title from its worktree's
// path and branch. The frontend calls it on every Summary change, so the
// title tracks branch changes.
func (s *AppService) UpdateWindowTitle(ctx context.Context, path, branch string) {
	if w, ok := callingWindowFrom(ctx); ok { // a windowless/background call has nothing to title
		w.SetTitle(windowTitle(path, branch))
	}
}

// PickAndOpen shows the native folder picker and switches the calling
// window to the choice, or brings forward another window already showing
// it, leaving this one unchanged; cancelling does nothing.
func (s *AppService) PickAndOpen(ctx context.Context) error {
	w, err := requireCallingWindow(ctx)
	if err != nil {
		return err
	}
	path, ok := s.PickFolder("")
	if !ok {
		return nil
	}
	return s.switchOrFocus(ctx, w, path)
}

// NewWindow opens a main window with no worktree selected: the repo
// picker's "New window" and Ctrl+Shift+N.
func (s *AppService) NewWindow() {
	s.reg.NewWindow()
}

// RecentRepos lists the remembered repos, most-recently-opened first.
func (s *AppService) RecentRepos() []settings.RecentRepo {
	return s.store.Get().RecentRepos
}

// RemoveRecentRepo takes repo off the recent repos; OpenWorktrees and every
// open window are left untouched.
func (s *AppService) RemoveRecentRepo(repo settings.RecentRepo) error {
	return s.store.Update(func(cur *settings.Settings) { cur.RemoveRecent(repo) })
}

// ListWSLDistros names the installed distros for Settings' per-distro
// editor profiles. A host without WSL has none; that is an answer, not an
// error.
func (s *AppService) ListWSLDistros(ctx context.Context) ([]string, error) {
	distros, err := remote.ListDistros(ctx)
	if ce, ok := errors.AsType[*core.Error](err); ok && ce.Code == core.WslNotFound {
		return nil, nil
	}
	names := make([]string, len(distros))
	for i, d := range distros {
		names[i] = d.Name
	}
	return names, err
}

// OpenPath opens or focuses path's window.
func (s *AppService) OpenPath(ctx context.Context, path string) {
	s.open(ctx, path)
}

// OpenDiff opens the calling window's diff window on wt's target, or
// focuses it and shows the target there. A call from no window uses wt's
// diff window.
func (s *AppService) OpenDiff(ctx context.Context, wt core.WorktreeID, target DiffTarget) {
	key := diffWindowKey{wt: wt}
	if w, ok := callingWindowFrom(ctx); ok {
		key = diffWindowKey{mainWindow: w.ID()}
	}
	s.reg.OpenOrFocusDiff(key, wt, target)
}

// SetUnsavedWork tells the shell whether the calling diff window's page
// holds work a close would lose; the window then asks its page before it
// closes.
func (s *AppService) SetUnsavedWork(ctx context.Context, unsaved bool) {
	if w, ok := callingWindowFrom(ctx); ok {
		s.reg.SetUnsavedWork(w.ID(), unsaved)
	}
}

// LogPathInfo is LogPath's result: where the app log is, or why it could
// not be opened.
type LogPathInfo struct {
	Path  string `json:"path"`
	Error string `json:"error"` // "" if the log opened
}

// LogPath is Settings' read-only "App log: <path>" line.
func (s *AppService) LogPath() LogPathInfo {
	return LogPathInfo{Path: s.logPath, Error: s.logErr}
}
