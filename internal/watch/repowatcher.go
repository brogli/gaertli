package watch

import (
	"io/fs"
	"iter"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/brogli/nebegit/internal/core"
	"github.com/fsnotify/fsnotify"
)

// RepoWatcher watches one repo's common git dir. fsnotify is the primary
// signal (near-zero idle cost); a low-frequency mtime poll backs it up
// for what fsnotify can miss: a dropped event, an exhausted watch
// count, or an unsupported filesystem.
//
// Bursts (git's lockfile-then-rename per write, several refs from one
// fetch) are debounced; onChange fires once per distinct scope touched in
// the window, not once per fs event.
type RepoWatcher struct {
	commonDir string
	onChange  func(core.ChangeScope)
	watcher   *fsnotify.Watcher // nil if fsnotify failed to start

	debounce sync.Mutex
	pending  map[core.ChangeScope]bool
	timer    *time.Timer

	snapshots sync.Mutex
	mtimes    map[string]time.Time

	done   chan struct{}
	closed atomic.Bool
}

// NewRepoWatcher starts watching commonDir. If fsnotify fails to start, it
// returns that error with a watcher that runs on the fallback poll alone.
func NewRepoWatcher(commonDir string, onChange func(core.ChangeScope)) (*RepoWatcher, error) {
	w := &RepoWatcher{
		commonDir: commonDir,
		onChange:  onChange,
		pending:   map[core.ChangeScope]bool{},
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

	go w.fallbackLoop(FallbackPollInterval, w.CheckNow)
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
// recursive watch.
func (w *RepoWatcher) addTree(root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil //nolint:nilerr // an unreadable directory is skipped, not fatal: the rest of the tree still gets watched
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

// fallbackLoop calls check every interval until Close.
func (w *RepoWatcher) fallbackLoop(interval time.Duration, check func()) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			check()
		case <-w.done:
			return
		}
	}
}

// CheckNow runs one immediate fallback-walk pass, independent of its own
// timer. Used by the fallback ticker and by Refresh.
func (w *RepoWatcher) CheckNow() {
	next := w.walk()

	w.snapshots.Lock()
	prev := w.mtimes
	w.mtimes = next
	w.snapshots.Unlock()

	for path := range changedPaths(prev, next) {
		if scope, ok := classify(w.commonDir, path); ok {
			w.notify(scope)
		}
	}
}

// changedPaths yields every path whose ModTime differs between the two
// snapshots, one that appeared or disappeared included.
func changedPaths(prev, next map[string]time.Time) iter.Seq[string] {
	return func(yield func(string) bool) {
		for path, mtime := range next {
			if old, ok := prev[path]; (!ok || !old.Equal(mtime)) && !yield(path) {
				return
			}
		}
		for path := range prev {
			if _, ok := next[path]; !ok && !yield(path) {
				return
			}
		}
	}
}

// walk records the ModTime of every entry directly in commonDir, of every
// file under commonDir/refs (recursive), and of every entry of
// commonDir/worktrees/* and those directories themselves — never what is
// in objects/. A directory counts, so a state directory (rebase-merge/) or
// a worktree's appearing or going away is seen.
func (w *RepoWatcher) walk() map[string]time.Time {
	mtimes := map[string]time.Time{}
	addDir(mtimes, w.commonDir)

	_ = filepath.WalkDir(filepath.Join(w.commonDir, "refs"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() {
			record(mtimes, path)
		}
		return nil
	})

	worktrees := filepath.Join(w.commonDir, "worktrees")
	addDir(mtimes, worktrees)
	entries, _ := os.ReadDir(worktrees)
	for _, e := range entries {
		if e.IsDir() {
			addDir(mtimes, filepath.Join(worktrees, e.Name()))
		}
	}
	return mtimes
}

// addDir records every entry of dir, files and directories alike.
func addDir(mtimes map[string]time.Time, dir string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		record(mtimes, filepath.Join(dir, e.Name()))
	}
}

// record records path's ModTime, if it still exists.
func record(mtimes map[string]time.Time, path string) {
	info, err := os.Stat(path)
	if err == nil {
		mtimes[path] = info.ModTime()
	}
}

// notify debounces scope with every other scope reported within
// DebounceInterval, then calls onChange once per distinct scope.
func (w *RepoWatcher) notify(scope core.ChangeScope) {
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
	w.pending = map[core.ChangeScope]bool{}
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
	if w.watcher != nil {
		return w.watcher.Close()
	}
	return nil
}
