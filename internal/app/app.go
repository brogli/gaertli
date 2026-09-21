// Package app wires the Wails application. It is the only package that
// imports Wails (TR-07).
package app

import (
	"context"
	"errors"
	"io/fs"
	"time"

	"github.com/brogli/nebegit/internal/core"
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

// errorNotifier shows a message outside any specific window — the
// FR-REPO-05 "say so, do nothing else" failure path.
type errorNotifier interface{ Show(title, msg string) }

// wailsNotifier shows a native error dialog.
type wailsNotifier struct{ app *application.App }

func (n wailsNotifier) Show(title, msg string) {
	n.app.Dialog.Error().SetTitle(title).SetMessage(msg).Show()
}

// shell is what every "open this worktree" trigger shares: CLI launch,
// second-instance relay, folder picker, recent-repo click.
type shell struct {
	api    core.API // the Facade: OpenWorktree routes through Router.CoreFor, where spec 30 hooks WSL in
	reg    *Registry
	store  *settings.Store
	notify errorNotifier
}

// open validates path, opens or focuses its window, and records it in
// settings. On failure it shows a message and does nothing else (FR-REPO-05).
func (sh shell) open(ctx context.Context, path string) {
	wt, err := sh.api.OpenWorktree(ctx, core.RepoPath(path))
	if err != nil {
		sh.notify.Show("nebegit", openFailedMessage(err))
		return
	}
	sh.reg.OpenOrFocus(wt.ID, wt.Path)
	sh.reg.SetRepo(wt.ID, wt.RepoKey)
	_ = sh.store.Update(func(s *settings.Settings) {
		s.AddRecent(wt.Path, time.Now())
		s.SetOpenWorktrees(sh.reg.OpenWorktreePaths())
	})
}

// restoreOrHome is the no-path launch (FR-REPO-02): a window per worktree
// open at last quit, or the home window if there were none.
func (sh shell) restoreOrHome(ctx context.Context) {
	open := sh.store.Get().OpenWorktrees
	if len(open) == 0 {
		sh.reg.OpenHome()
		return
	}
	for _, p := range open {
		sh.open(ctx, p)
	}
}

// launch handles a launch's resolved path argument, first or relayed.
func (sh shell) launch(ctx context.Context, path string) {
	if path == "" {
		sh.restoreOrHome(ctx)
		return
	}
	sh.open(ctx, path)
}

// openFailedMessage is FR-REPO-05's user-facing text for a validation
// failure.
func openFailedMessage(err error) string {
	var ce *core.Error
	if errors.As(err, &ce) {
		return ce.Message
	}
	return err.Error()
}

// options builds the Wails application options: the Facade as the one
// service (ADR-3), single instance relaying its path through
// onSecondInstance. Builds no window and starts nothing, so it is
// unit-testable without a display.
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
		SingleInstance: &application.SingleInstanceOptions{
			UniqueID: singleInstanceID,
			OnSecondInstanceLaunch: func(data application.SecondInstanceData) {
				if path, ok := pathArg(data.Args); ok {
					onSecondInstance(path)
				}
			},
		},
	}
}

// Run starts the shell. args is os.Args (argv[0] included, see pathArg).
func Run(assets fs.FS, args []string) error {
	if !gitPresent() {
		showFatal("nebegit", missingGitMessage())
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
	reg.SetLocal(services.New(reg.Dispatch))

	var sh shell
	onPath := func(path string) { sh.launch(context.Background(), path) }
	a := application.New(options(assets, reg, onPath))
	reg.Bind(a)
	sh = shell{api: NewFacade(reg), reg: reg, store: store, notify: wailsNotifier{app: a}}
	a.RegisterService(application.NewService(&AppService{shell: sh, picker: wailsPicker{app: a}}))
	a.RegisterService(application.NewService(NewSettingsService(store)))
	sh.launch(context.Background(), path)

	if err := a.Run(); err != nil {
		showFatal("nebegit", startupFailedMessage(err))
		return err
	}
	return nil
}
