package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log"
	"maps"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"

	"github.com/Boeing/config-file-validator/v3/pkg/finder"
)

// fileWatcher is an interface over fsnotify.Watcher so it can be replaced
// with a fake in tests.
type fileWatcher interface {
	Add(name string) error
	Close() error
	Events() <-chan fsnotify.Event
	Errors() <-chan error
}

type fsnotifyWatcher struct {
	*fsnotify.Watcher
}

func newFSNotifyWatcher() (fileWatcher, error) {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	return fsnotifyWatcher{Watcher: watcher}, nil
}

func (w fsnotifyWatcher) Events() <-chan fsnotify.Event {
	return w.Watcher.Events
}

func (w fsnotifyWatcher) Errors() <-chan error {
	return w.Watcher.Errors
}

// watchRunner holds injected dependencies for watch mode, allowing tests to
// substitute a fake watcher and control debounce timing.
type watchRunner struct {
	newWatcher    func() (fileWatcher, error)
	out           io.Writer
	debounceDelay time.Duration
}

const defaultWatchDebounceDelay = 75 * time.Millisecond

// runWatch is the entry point called from runCheck when --watch is set.
func runWatch(rc *resolvedConfig) (int, error) {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	runner := watchRunner{
		newWatcher: newFSNotifyWatcher,
		out:        os.Stdout,
	}
	return runner.run(ctx, rc)
}

func (r watchRunner) run(ctx context.Context, rc *resolvedConfig) (int, error) {
	if rc.isStdin {
		return 2, errors.New("--watch cannot be used with stdin")
	}

	newWatcher := r.newWatcher
	if newWatcher == nil {
		newWatcher = newFSNotifyWatcher
	}

	watcher, err := newWatcher()
	if err != nil {
		return 2, err
	}
	defer watcher.Close()

	watched := make(map[string]struct{})
	if err := addWatchPaths(watcher, rc.searchPaths, watched); err != nil {
		return 2, err
	}

	initialStatus, err := buildCLI(rc).Run()
	if err != nil || initialStatus == 2 {
		return initialStatus, err
	}

	delay := r.effectiveDebounceDelay()
	pendingChanges := make(map[string]struct{})
	var debounceTimer *time.Timer
	var debounceC <-chan time.Time

	for {
		select {
		case <-ctx.Done():
			stopWatchTimer(debounceTimer)
			return 0, nil
		case event, ok := <-watcher.Events():
			if !ok {
				stopWatchTimer(debounceTimer)
				return 0, nil
			}
			if !isValidationEvent(event) {
				continue
			}
			if err := addCreatedDirectoryWatchers(watcher, event.Name, watched); err != nil {
				log.Printf("watch: %v", err)
			}
			pendingChanges[filepath.Clean(event.Name)] = struct{}{}
			debounceTimer, debounceC = resetWatchTimer(debounceTimer, debounceC, delay)
		case err, ok := <-watcher.Errors():
			if !ok {
				stopWatchTimer(debounceTimer)
				return 0, nil
			}
			if err != nil {
				log.Printf("watch: %v", err)
			}
		case <-debounceC:
			debounceTimer = nil
			debounceC = nil
			for _, path := range drainPendingChanges(pendingChanges) {
				status, err := r.runChangedFile(rc, path)
				if err != nil || status == 2 {
					return status, err
				}
			}
		}
	}
}

func (r watchRunner) effectiveDebounceDelay() time.Duration {
	if r.debounceDelay > 0 {
		return r.debounceDelay
	}
	return defaultWatchDebounceDelay
}

func resetWatchTimer(timer *time.Timer, timerC <-chan time.Time, delay time.Duration) (*time.Timer, <-chan time.Time) {
	if timer == nil {
		timer = time.NewTimer(delay)
		return timer, timer.C
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
	timer.Reset(delay)
	return timer, timerC
}

func stopWatchTimer(timer *time.Timer) {
	if timer == nil {
		return
	}
	if !timer.Stop() {
		select {
		case <-timer.C:
		default:
		}
	}
}

func drainPendingChanges(pending map[string]struct{}) []string {
	paths := slices.Collect(maps.Keys(pending))
	slices.Sort(paths)
	clear(pending)
	return paths
}

func isValidationEvent(event fsnotify.Event) bool {
	return event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Rename) != 0
}

func (r watchRunner) runChangedFile(rc *resolvedConfig, path string) (int, error) {
	files, err := findChangedFile(rc, path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return 0, nil
		}
		return 2, err
	}
	if len(files) == 0 {
		return 0, nil
	}

	if !rc.quiet {
		fmt.Fprintf(r.output(), "\n--- change detected: %s ---\n", path)
	}

	return buildCLIWithFinder(rc, staticFileFinder{files: files}).Run()
}

func (r watchRunner) output() io.Writer {
	if r.out == nil {
		return io.Discard
	}
	return r.out
}

// staticFileFinder satisfies the finder.Finder interface with a pre-computed
// file list, used to validate only the changed file on each watch event.
type staticFileFinder struct {
	files []finder.FileMetadata
}

// Find returns the pre-computed list of files.
func (f staticFileFinder) Find() ([]finder.FileMetadata, error) {
	return f.files, nil
}

func findChangedFile(rc *resolvedConfig, path string) ([]finder.FileMetadata, error) {
	return finder.FileSystemFinderInit(rc.finderOpts...).MatchFile(path)
}

func addWatchPaths(watcher fileWatcher, paths []string, watched map[string]struct{}) error {
	for _, path := range paths {
		watchRoot := filepath.Clean(path)
		if watchRoot == "" {
			continue
		}

		//nolint:gosec // Watch mode intentionally inspects user-provided search paths.
		info, err := os.Stat(watchRoot)
		if err != nil {
			return err
		}
		if !info.IsDir() {
			watchRoot = filepath.Dir(watchRoot)
		}

		//nolint:gosec // Watch mode intentionally walks user-provided search paths.
		if err := filepath.WalkDir(watchRoot, func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !dirEntry.IsDir() {
				return nil
			}
			return addWatchDir(watcher, path, watched)
		}); err != nil {
			return err
		}
	}
	return nil
}

func addCreatedDirectoryWatchers(watcher fileWatcher, path string, watched map[string]struct{}) error {
	info, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil
		}
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return addWatchPaths(watcher, []string{path}, watched)
}

func addWatchDir(watcher fileWatcher, path string, watched map[string]struct{}) error {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if _, ok := watched[absPath]; ok {
		return nil
	}
	if err := watcher.Add(absPath); err != nil {
		return err
	}
	watched[absPath] = struct{}{}
	return nil
}
