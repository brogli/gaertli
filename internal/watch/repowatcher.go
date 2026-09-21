package watch

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
)

// RepoWatcher watches one repo's common git dir (ADR-9: repo-session-
// level; FR-REPO-04's git-state half). fsnotify is the primary signal
// (near-zero idle cost — no busy loop, QA-PERF-06); a low-frequency
// directory-mtime poll is an insurance backstop for what fsnotify can
// miss: a dropped inotify event, an exhausted per-user watch count, or a
// filesystem fsnotify doesn't support at all.
//
// Bursts (git's own lockfile-then-rename per write, several refs from one
// fetch) are debounced ~300ms; onChange fires once per distinct Scope
// touched in the window, not once per fs event.
type RepoWatcher struct {
	commonDir string
	onChange  func(Scope)
	watcher   *fsnotify.Watcher // nil if fsnotify failed to start

	debounce sync.Mutex
	pending  map[Scope]bool
	timer    *time.Timer

	snapshots sync.Mutex
	mtimes    map[string]time.Time

	fallback   *time.Ticker
	fallbackFn func() // CheckNow; a test seam for the ticker (repowatcher_test.go)
	done       chan struct{}
	closed     atomic.Bool
}

// NewRepoWatcher starts watching commonDir. If fsnotify itself fails to
// start, the error is not fatal to the caller: the returned watcher still
// runs on the fallback poll alone, degraded, not broken.
func NewRepoWatcher(commonDir string, onChange func(Scope)) (*RepoWatcher, error) {
	w := &RepoWatcher{
		commonDir: commonDir,
		onChange:  onChange,
		pending:   map[Scope]bool{},
		done:      make(chan struct{}),
	}
	w.mtimes = w.walk() // baseline, so the first CheckNow does not report every file as changed

	fw, err := fsnotify.NewWatcher()
	if err == nil {
		w.watcher = fw
		if addErr := w.addWatches(); addErr != nil {
			err = addErr
			_ = fw.Close()
			w.watcher = nil
		} else {
			go w.watchLoop()
		}
	}

	w.fallback = time.NewTicker(FallbackPollInterval)
	w.fallbackFn = w.CheckNow
	go w.fallbackLoop()
	return w, err
}

func (w *RepoWatcher) addWatches() error {
	if err := w.watcher.Add(w.commonDir); err != nil {
		return err
	}
	_ = w.addTree(filepath.Join(w.commonDir, "refs"))
	_ = w.addChildren(filepath.Join(w.commonDir, "worktrees"))
	return nil
}

// addTree adds root and every directory beneath it: fsnotify has no
// recursive watch (Research used).
func (w *RepoWatcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		return w.watcher.Add(path)
	})
}

func (w *RepoWatcher) addChildren(dir string) error {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			_ = w.watcher.Add(filepath.Join(dir, e.Name()))
		}
	}
	return nil
}

func (w *RepoWatcher) watchLoop() {
	for {
		select {
		case ev, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(ev)
		case _, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
		}
	}
}

// handleEvent adds a watch for a newly created directory under refs/ or
// worktrees/ before classifying it, so nested branch names and worktrees
// added while the app is running are covered without a restart.
func (w *RepoWatcher) handleEvent(ev fsnotify.Event) {
	if ev.Op&fsnotify.Create != 0 {
		w.watchIfNewDir(ev.Name)
	}
	if scope, ok := classify(w.commonDir, ev.Name); ok {
		w.notify(scope)
	}
}

func (w *RepoWatcher) watchIfNewDir(path string) {
	rel, err := filepath.Rel(w.commonDir, path)
	if err != nil {
		return
	}
	rel = filepath.ToSlash(rel)
	if rel != "refs" && !strings.HasPrefix(rel, "refs/") && !strings.HasPrefix(rel, "worktrees/") {
		return
	}
	if info, err := os.Stat(path); err == nil && info.IsDir() {
		_ = w.watcher.Add(path)
	}
}

func (w *RepoWatcher) fallbackLoop() {
	for {
		select {
		case <-w.fallback.C:
			w.fallbackFn()
		case <-w.done:
			return
		}
	}
}

// CheckNow runs one immediate fallback-walk pass, independent of its own
// timer. Used by the fallback ticker itself and by Refresh (services.go).
func (w *RepoWatcher) CheckNow() {
	next := w.walk()

	w.snapshots.Lock()
	prev := w.mtimes
	w.mtimes = next
	w.snapshots.Unlock()

	for path, mtime := range next {
		if old, ok := prev[path]; ok && old.Equal(mtime) {
			continue
		}
		if scope, ok := classify(w.commonDir, path); ok {
			w.notify(scope)
		}
	}
}

// walk records the ModTime of every regular file directly in commonDir,
// under commonDir/refs (recursive), and under commonDir/worktrees/*
// (non-recursive) — never objects/.
func (w *RepoWatcher) walk() map[string]time.Time {
	mtimes := map[string]time.Time{}
	addDir(mtimes, w.commonDir)

	_ = filepath.WalkDir(filepath.Join(w.commonDir, "refs"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			addFile(mtimes, path)
		}
		return nil
	})

	worktrees := filepath.Join(w.commonDir, "worktrees")
	entries, _ := os.ReadDir(worktrees)
	for _, e := range entries {
		if e.IsDir() {
			addDir(mtimes, filepath.Join(worktrees, e.Name()))
		}
	}
	return mtimes
}

func addDir(mtimes map[string]time.Time, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			addFile(mtimes, filepath.Join(dir, e.Name()))
		}
	}
}

func addFile(mtimes map[string]time.Time, path string) {
	info, err := os.Stat(path)
	if err == nil {
		mtimes[path] = info.ModTime()
	}
}

// notify debounces scope with every other scope reported within
// DebounceInterval, then calls onChange once per distinct scope.
func (w *RepoWatcher) notify(scope Scope) {
	if w.closed.Load() {
		return
	}
	w.debounce.Lock()
	defer w.debounce.Unlock()
	w.pending[scope] = true
	if w.timer == nil {
		w.timer = time.AfterFunc(DebounceInterval, w.flush)
	}
}

func (w *RepoWatcher) flush() {
	if w.closed.Load() {
		return
	}
	w.debounce.Lock()
	pending := w.pending
	w.pending = map[Scope]bool{}
	w.timer = nil
	w.debounce.Unlock()

	for scope := range pending {
		w.onChange(scope)
	}
}

// Close stops the fsnotify watcher and the fallback ticker.
func (w *RepoWatcher) Close() error {
	if !w.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(w.done)
	w.fallback.Stop()
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}
