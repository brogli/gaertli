// Package app wires the Wails application. It is the only package that
// imports Wails.
package app

import (
	"context"
	"errors"
	"io/fs"
	"net/url"
	"path/filepath"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/brogli/nebegit/internal/remote"
	"github.com/brogli/nebegit/internal/services"
	"github.com/brogli/nebegit/internal/settings"
	"github.com/wailsapp/wails/v3/pkg/application"
)

const singleInstanceID = "nebegit" // stable per machine; no reverse-domain owner to use

// pathArg reads the optional repo path from an argv (os.Args and
// SecondInstanceData.Args both start with the program name): none opens
// the placeholder window, one is the path to open or focus, more is a
// usage error.
func pathArg(args []string) (path string, ok bool) {
	switch len(args) {
	case 1:
		return "", true
	case 2:
		return args[1], true
	default:
		return "", false
	}
}

// resolveFrom makes a relative path argument absolute against dir, the
// launching process's working directory: a relayed "." means the second
// instance's directory, not this process's. "" (no path) stays "".
func resolveFrom(dir, path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(dir, path)
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
	api    core.API // the Facade: routing happens in Router.CoreFor
	reg    *Registry
	store  *settings.Store
	notify errorNotifier
}

// open validates path, opens or focuses its window, and records it in
// settings. On failure it shows a message and does nothing else. It
// reports whether the window opened.
func (sh shell) open(ctx context.Context, path string) bool {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		sh.notify.Show("nebegit", openFailedMessage(err))
		return false
	}
	distro, _, _ := remote.ClassifyWSLPath(path)
	sh.reg.OpenOrFocus(wt.ID, wt.Path, distro)
	sh.reg.SetRepo(wt.ID, wt.RepoKey)
	_ = sh.store.Update(func(s *settings.Settings) {
		s.AddRecent(settings.RecentRepo{Path: wt.Path, Distro: distro, RepoKey: wt.RepoKey}, time.Now())
		s.OpenWorktrees = sh.reg.OpenWorktreePaths()
	})
	return true
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

// launch handles a launch's resolved path argument, first or relayed. It
// reports false when a given path failed to open.
func (sh shell) launch(ctx context.Context, path string) bool {
	if path == "" {
		sh.restore(ctx)
		return true
	}
	return sh.open(ctx, path)
}

// callingWindow is the part of the ctx-carried calling window
// (application.WindowKey) this package needs. application.Window itself
// has unexported methods, so no test fake could implement it.
type callingWindow interface {
	ID() uint
	SetURL(s string) application.Window
	SetTitle(title string) application.Window
}

// switchTo moves w's selection to path: validates it like open, navigates
// the window (SetURL reloads it), records settings, and releases the
// worktree just left. Unlike open, a failure is returned rather than shown:
// the panel that asked has an ErrorBanner.
func (sh shell) switchTo(ctx context.Context, w callingWindow, path string) error {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		return err
	}
	distro, _, _ := remote.ClassifyWSLPath(path)
	previous, previousDistro, _ := sh.reg.SelectInWindow(w.ID(), wt.ID, wt.Path, distro)
	sh.reg.SetRepo(wt.ID, wt.RepoKey)
	w.SetURL(selectionURL(wt.Path, distro))
	sh.reg.Release(previous, previousDistro)
	_ = sh.store.Update(func(s *settings.Settings) { s.OpenWorktrees = sh.reg.OpenWorktreePaths() })
	return nil
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

// openFailedMessage is the user-facing text for a validation failure.
func openFailedMessage(err error) string {
	if ce, ok := errors.AsType[*core.Error](err); ok {
		return ce.Message
	}
	return err.Error()
}

// options builds the Wails application options: the Facade as the one
// service, single instance relaying its path through onSecondInstance.
// Builds no window and starts nothing, so it is testable without a display.
func options(assets fs.FS, reg *Registry, onSecondInstance func(path string)) application.Options {
	return application.Options{
		Name:        "nebegit",
		Description: "The git tool that sits beside your editor",
		Assets: application.AssetOptions{
			Handler: application.AssetFileServerFS(assets),
		},
		Mac: application.MacOptions{
			ApplicationShouldTerminateAfterLastWindowClosed: true,
		},
		Services: []application.Service{
			application.NewService(NewFacade(reg)),
		},
		OnShutdown: reg.Quitting,
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if path, ok := pathArg(data.Args); ok {
					onSecondInstance(resolveFrom(data.WorkingDir, path))
				}
			},
		},
	}
}

// Run starts the shell. args is os.Args (argv[0] included, see pathArg).
func Run(assets fs.FS, args []string) error {
	if !gitPresent() {
		showFatal("nebegit", missingGitMessage)
		return errors.New("git not found on PATH")
	}

	path, ok := pathArg(args)
	if !ok {
		showFatal("nebegit", "usage: nebegit [path]")
		return errors.New("bad arguments")
	}

	settingsPath, err := settings.Path()
	if err != nil {
		showFatal("nebegit", "could not determine the settings location: "+err.Error())
		return err
	}
	store, err := settings.Open(settingsPath)
	if err != nil {
		showFatal("nebegit", "could not load settings: "+err.Error())
		return err
	}

	reg := NewRegistry()
	reg.SetStore(store)
	reg.SetLocal(services.New(reg.Dispatch))
	reg.PushEditorProfiles(context.Background())

	var sh shell
	onPath := func(path string) { sh.launch(context.Background(), path) }
	a := application.New(options(assets, reg, onPath))
	reg.Bind(a)
	reg.SetConsent(wailsConsent{app: a})
	sh = shell{api: NewFacade(reg), reg: reg, store: store, notify: wailsNotifier{app: a}}
	a.RegisterService(application.NewService(&AppService{shell: sh, picker: wailsPicker{app: a}}))
	a.RegisterService(application.NewService(NewSettingsService(store, reg)))
	startup := sh
	startup.notify = startupNotifier{}
	if !startup.launch(context.Background(), path) {
		return errors.New("could not open " + path)
	}

	if err := a.Run(); err != nil {
		showFatal("nebegit", startupFailedMessage(err))
		return err
	}
	return nil
}
