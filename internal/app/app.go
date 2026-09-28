// Package app wires the Wails application. It is the only package that
// imports Wails.
package app

import (
	"context"
	"errors"
	"io/fs"
	"log/slog"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"github.com/brogli/nebegit/internal/applog"
	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/git"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/brogli/nebegit/internal/userpath"
	"github.com/wailsapp/wails/v3/pkg/application"
	wailslog "github.com/wailsapp/wails/v3/pkg/services/log"
)

const singleInstanceID = "nebegit" // stable per machine; no reverse-domain owner to use

// Run starts the shell. args is os.Args (argv[0] included, see launchPath).
func Run(assets fs.FS, args []string) error {
	if action := pathActionFor(args); action != nil {
		return runPathAction(action)
	}

	if ok, msg := checkGit(context.Background(), git.Runner{}); !ok {
		showFatal(text(appName, nil), msg)
		return errors.New("git prerequisite not met")
	}

	wd, _ := os.Getwd()
	path, ok := launchPath(args, wd)
	if !ok {
		showFatal(text(appName, nil), text(usage, nil))
		return errors.New("bad arguments")
	}

	store, err := openSettings()
	if err != nil {
		return err
	}

	// SettingsService.Set moves the level on a later DebugMode change.
	logger, logPath, logCloser, logLevel, logErr := applog.Open(store.Get().DebugMode)
	slog.SetDefault(logger)
	if logCloser != nil {
		defer func() { _ = logCloser.Close() }()
	}

	reg := NewRegistry()
	reg.SetStore(store)
	reg.SetLocal(services.New(reg.Dispatch))
	reg.PushEditorProfiles(context.Background())
	reg.SetConsent(nativeConsent{ask: askYesNo})

	var sh shell
	onPath := func(path string) { sh.launch(context.Background(), path) }
	a := application.New(options(assets, reg, onPath, logger))
	reg.Bind(a)
	sh = shell{api: NewForwarder(reg), reg: reg, store: store, notify: wailsNotifier{app: a}, logPath: logPath, logErr: errString(logErr)}
	a.RegisterService(application.NewService(&AppService{shell: sh, picker: wailsPicker{app: a}}))
	a.RegisterService(application.NewService(NewSettingsService(store, reg, a.Event, logLevel)))
	// The frontend's uncaught errors reach the same app log, at Error, via
	// Wails' own bound log service.
	a.RegisterService(application.NewService(wailslog.NewWithConfig(&wailslog.Config{Logger: logger})))
	startup := sh
	startup.notify = startupNotifier{}
	if !startup.launch(context.Background(), path) {
		return errors.New("could not open " + path)
	}

	if err := a.Run(); err != nil {
		showFatal(text(appName, nil), text(startupFailed, map[string]string{"error": err.Error()}))
		return err
	}
	return nil
}

// errString is err.Error(), "" for a nil err.
func errString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// openSettings opens the settings file, showing why it could not.
func openSettings() (*settings.Store, error) {
	path, err := settings.Path()
	if err != nil {
		showFatal(text(appName, nil), text(settingsPathErr, map[string]string{"error": err.Error()}))
		return nil, err
	}
	store, err := settings.Open(path)
	if err != nil {
		showFatal(text(appName, nil), text(settingsLoadErr, map[string]string{"error": err.Error()}))
		return nil, err
	}
	return store, nil
}

// options builds the Wails application options: the Facade service, single
// instance relaying its path through onSecondInstance, and logger for
// Wails' own system messages, so they land in the app log too.
// Builds no window and starts nothing, so it is testable without a display.
func options(assets fs.FS, reg *Registry, onSecondInstance func(path string), logger *slog.Logger) application.Options {
	return application.Options{
		Name:        "nebegit",
		Description: "The git tool that sits beside your editor",
		Logger:      logger,
		Assets: application.AssetOptions{
			Handler:    application.AssetFileServerFS(assets),
			Middleware: pageMiddleware,
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(&Facade{NewForwarder(reg)}),
		},
		OnShutdown: reg.Quitting,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if path, ok := launchPath(data.Args, data.WorkingDir); ok {
					onSecondInstance(path)
				}
			},
		},
	}
}

// launchPath reads the optional repo path from an argv (os.Args and
// SecondInstanceData.Args both start with the program name) and resolves
// it against wd, the launching process's working directory: a relayed "."
// means the second instance's directory, not this process's. None opens
// the placeholder window (path "", ok true); one argument is the path to
// open or focus; more is a usage error (ok false).
func launchPath(args []string, wd string) (path string, ok bool) {
	switch len(args) {
	case 1:
		return "", true
	case 2:
		return resolveFrom(wd, args[1]), true
	default:
		return "", false
	}
}

// resolveFrom makes a relative path argument absolute against dir, the
// launching process's working directory. "" (no path) stays "".
func resolveFrom(dir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
}

// pathActions maps each installer flag argv[1] can be to the action it
// runs on the exe's bin folder.
var pathActions = map[string]func(dir string) error{
	"--add-to-path":      userpath.Add,
	"--remove-from-path": userpath.Remove,
}

// pathActionFor is the action argv selects, nil for none: argv[0] plus one
// of the two installer flags selects it; any other argv, including a repo
// path, selects none.
func pathActionFor(args []string) func(string) error {
	if len(args) != 2 {
		return nil
	}
	return pathActions[args[1]]
}

// runPathAction adds or removes the exe's own bin folder in the user PATH;
// a failure's reason also goes to the app log.
func runPathAction(action func(string) error) error {
	dir, err := exeBinDir()
	if err == nil {
		err = action(dir)
	}
	if err != nil {
		logPathActionFailure(err)
	}
	return err
}

// exeBinDir is the bin folder beside the running executable, the folder the
// installer adds to and removes from the user PATH.
func exeBinDir() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(exe), "bin"), nil
}

// logPathActionFailure writes why the PATH edit failed to the app log; the
// settings-driven debug level is not available yet this early in Run.
func logPathActionFailure(err error) {
	logger, _, closer, _, _ := applog.Open(false)
	if closer != nil {
		defer func() { _ = closer.Close() }()
	}
	logger.Error("could not update the user PATH", "error", err)
}

// errorNotifier shows a message outside any specific window.
type errorNotifier interface{ Show(title, msg string) }

// startupNotifier reports before the app runs, when a Wails dialog would
// panic: stderr on Linux and macOS, a message box on Windows.
type startupNotifier struct{}

func (startupNotifier) Show(title, msg string) { showFatal(title, msg) }

// wailsNotifier shows a native error dialog.
type wailsNotifier struct{ app *application.App }

func (n wailsNotifier) Show(title, msg string) {
	n.app.Dialog.Error().SetTitle(title).SetMessage(msg).Show()
}

// shell is what every "open this worktree" trigger shares: CLI launch,
// second-instance relay, folder picker, recent-repo click.
type shell struct {
	api    core.API // the Forwarder, which routes each call through the Router
	reg    *Registry
	store  *settings.Store
	notify errorNotifier

	logPath string // the app log's path, "" if it could not be opened
	logErr  string // why it could not be opened, "" on success
}

// launch handles a launch's resolved path argument, first or relayed. It
// reports false when a given path failed to open.
func (sh shell) launch(ctx context.Context, path string) bool {
	if path == "" {
		sh.restore(ctx)
		return true
	}
	return sh.open(ctx, path)
}

// open is openOrFocus, showing a failure in the native dialog instead of
// returning it. It reports whether the window opened.
func (sh shell) open(ctx context.Context, path string) bool {
	if err := sh.openOrFocus(ctx, path); err != nil {
		sh.notify.Show(text(appName, nil), core.ErrorOf(err).Message)
		return false
	}
	return true
}

// openOrFocus validates path, opens or focuses its window, and records it
// in settings.
func (sh shell) openOrFocus(ctx context.Context, path string) error {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		return err
	}
	distro, _, _ := remote.ClassifyWSLPath(path)
	sh.reg.OpenOrFocus(wt.ID, wt.Path, distro)
	sh.reg.SetRepo(wt.ID, wt.RepoKey)
	sh.remember(wt, distro)
	return nil
}

// remember records wt as its repo's recent entry and the open windows for
// the next launch's restore.
func (sh shell) remember(wt core.WorktreeInfo, distro string) {
	_ = sh.store.Update(func(s *settings.Settings) {
		s.AddRecent(settings.RecentRepo{Path: wt.Path, Distro: distro, RepoKey: wt.RepoKey}, time.Now())
		s.OpenWorktrees = sh.reg.OpenWorktreePaths()
	})
}

// restore is the no-path launch: a window per worktree selected at last
// quit, or one empty-selection window if none of them opens.
func (sh shell) restore(ctx context.Context) {
	opened := false
	for _, p := range sh.store.Get().OpenWorktrees {
		opened = sh.open(ctx, p) || opened
	}
	if !opened {
		sh.reg.OpenOrFocus("", "", "")
	}
}

// callingWindow is the part of the ctx-carried calling window
// (application.WindowKey) this package needs. application.Window itself
// has unexported methods, so no test fake could implement it.
type callingWindow interface {
	ID() uint
	SetURL(s string) application.Window
	SetTitle(title string) application.Window
}

// switchTo moves w's selection to path, validated like open. Unlike open, a
// failure is returned rather than shown: the picker that asked shows it.
func (sh shell) switchTo(ctx context.Context, w callingWindow, path string) error {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		return err
	}
	sh.selectInWindow(w, wt, path)
	return nil
}

// switchOrFocus is switchTo, except that a window already showing path's
// worktree is brought forward instead, leaving w unchanged.
func (sh shell) switchOrFocus(ctx context.Context, w callingWindow, path string) error {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		return err
	}
	if other := sh.reg.windowShowing(wt.ID, w.ID()); other != nil {
		other.Focus()
		return nil
	}
	sh.selectInWindow(w, wt, path)
	return nil
}

// selectInWindow moves w's selection to wt (already resolved from path):
// navigates the window (SetURL reloads it), records settings, and releases
// the worktree just left.
func (sh shell) selectInWindow(w callingWindow, wt core.WorktreeInfo, path string) {
	distro, _, _ := remote.ClassifyWSLPath(path)
	previous, _ := sh.reg.SelectInWindow(w.ID(), wt.ID, wt.Path, distro)
	sh.reg.SetRepo(wt.ID, wt.RepoKey)
	w.SetURL(selectionURL(wt.Path, distro))
	sh.reg.Release(previous)
	sh.remember(wt, distro)
}

// shellPath is path in the shell's id form: UNC for a WSL repo (distro
// set), unchanged otherwise.
func shellPath(path, distro string) string {
	if distro == "" {
		return path
	}
	return remote.BuildWSLPath(distro, path)
}

// selectionURL builds a worktree window's URL; "" (the empty selection)
// gets the placeholder "/". distro is "" for a local repo, else the WSL
// distro path is POSIX inside.
func selectionURL(path, distro string) string {
	if path == "" {
		return "/"
	}
	u := "/?path=" + url.QueryEscape(path)
	if distro != "" {
		u += "&distro=" + url.QueryEscape(distro)
	}
	return u
}
