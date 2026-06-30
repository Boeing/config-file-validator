package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v2/internal/testhelper"
	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/finder"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
)

type recordingReporter struct {
	mu    sync.Mutex
	calls [][]reporter.Report
}

func (r *recordingReporter) Print(reports []reporter.Report) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	copied := append([]reporter.Report(nil), reports...)
	r.calls = append(r.calls, copied)
	return nil
}

func (r *recordingReporter) callCount() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return len(r.calls)
}

func (r *recordingReporter) call(index int) []reporter.Report {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]reporter.Report(nil), r.calls[index]...)
}

type fakeWatcher struct {
	mu     sync.Mutex
	events chan fsnotify.Event
	errors chan error
	added  []string
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan fsnotify.Event, 4),
		errors: make(chan error, 1),
	}
}

func (w *fakeWatcher) Add(path string) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	w.added = append(w.added, path)
	return nil
}

func (*fakeWatcher) Close() error {
	return nil
}

func (w *fakeWatcher) Events() <-chan fsnotify.Event {
	return w.events
}

func (w *fakeWatcher) Errors() <-chan error {
	return w.errors
}

func (w *fakeWatcher) addedPaths() []string {
	w.mu.Lock()
	defer w.mu.Unlock()
	return append([]string(nil), w.added...)
}

func TestRunWatchValidatesInitialPassThenChangedFile(t *testing.T) {
	dir := t.TempDir()
	jsonFile := testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
	yamlFile := testhelper.WriteFile(t, dir, "good.yaml", testhelper.ValidContent["yaml"])
	recorder := &recordingReporter{}
	watcher := newFakeWatcher()

	rc := &resolvedConfig{
		reporters:   []reporter.Reporter{recorder},
		groupOutput: []string{""},
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs(nil),
			finder.WithExcludeFileTypes(nil),
			finder.WithFileTypes(filetype.FileTypes),
		},
		searchPaths: []string{dir},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		runner := watchRunner{
			newWatcher: func() (fileWatcher, error) {
				return watcher, nil
			},
		}
		_, err := runner.run(ctx, rc)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return recorder.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	initial := recorder.call(0)
	require.Len(t, initial, 2)

	watcher.events <- fsnotify.Event{Name: jsonFile, Op: fsnotify.Write}

	require.Eventually(t, func() bool {
		return recorder.callCount() == 2
	}, time.Second, 10*time.Millisecond)

	changed := recorder.call(1)
	require.Len(t, changed, 1)
	require.Equal(t, filepath.Clean(jsonFile), filepath.Clean(changed[0].FilePath))

	cancel()
	require.NoError(t, <-done)
	require.FileExists(t, yamlFile)
}

func TestRunWatchAddsRealSearchPathDirectories(t *testing.T) {
	dir := t.TempDir()
	subdir := testhelper.CreateSubdir(t, dir, "nested")
	testhelper.WriteFile(t, subdir, "good.json", testhelper.ValidContent["json"])
	recorder := &recordingReporter{}
	watcher := newFakeWatcher()

	rc := &resolvedConfig{
		reporters:   []reporter.Reporter{recorder},
		groupOutput: []string{""},
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs(nil),
			finder.WithExcludeFileTypes(nil),
			finder.WithFileTypes(filetype.FileTypes),
		},
		searchPaths: []string{dir},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		runner := watchRunner{
			newWatcher: func() (fileWatcher, error) {
				return watcher, nil
			},
		}
		_, err := runner.run(ctx, rc)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return recorder.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	added := watcher.addedPaths()
	require.Contains(t, added, mustAbs(t, dir))
	require.Contains(t, added, mustAbs(t, subdir))

	cancel()
	require.NoError(t, <-done)
}

func TestRunWatchSkipsFilesFilteredByFinder(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
	yamlFile := testhelper.WriteFile(t, dir, "good.yaml", testhelper.ValidContent["yaml"])
	recorder := &recordingReporter{}
	watcher := newFakeWatcher()

	rc := &resolvedConfig{
		reporters:   []reporter.Reporter{recorder},
		groupOutput: []string{""},
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs(nil),
			finder.WithExcludeFileTypes([]string{"yaml", "yml"}),
			finder.WithFileTypes(filetype.FileTypes),
		},
		searchPaths: []string{dir},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		runner := watchRunner{
			newWatcher: func() (fileWatcher, error) {
				return watcher, nil
			},
		}
		_, err := runner.run(ctx, rc)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return recorder.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	watcher.events <- fsnotify.Event{Name: yamlFile, Op: fsnotify.Write}

	require.Never(t, func() bool {
		return recorder.callCount() > 1
	}, 100*time.Millisecond, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}

func TestRunWatchCoalescesRapidEvents(t *testing.T) {
	dir := t.TempDir()
	jsonFile := testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
	recorder := &recordingReporter{}
	watcher := newFakeWatcher()

	rc := &resolvedConfig{
		reporters:   []reporter.Reporter{recorder},
		groupOutput: []string{""},
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs(nil),
			finder.WithExcludeFileTypes(nil),
			finder.WithFileTypes(filetype.FileTypes),
		},
		searchPaths: []string{dir},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		runner := watchRunner{
			newWatcher: func() (fileWatcher, error) {
				return watcher, nil
			},
			debounceDelay: 10 * time.Millisecond,
		}
		_, err := runner.run(ctx, rc)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return recorder.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	watcher.events <- fsnotify.Event{Name: jsonFile, Op: fsnotify.Write}
	watcher.events <- fsnotify.Event{Name: jsonFile, Op: fsnotify.Write}

	require.Eventually(t, func() bool {
		return recorder.callCount() == 2
	}, time.Second, 10*time.Millisecond)
	require.Never(t, func() bool {
		return recorder.callCount() > 2
	}, 50*time.Millisecond, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}

func TestRunWatchContinuesAfterWatcherError(t *testing.T) {
	dir := t.TempDir()
	jsonFile := testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
	recorder := &recordingReporter{}
	watcher := newFakeWatcher()

	rc := &resolvedConfig{
		reporters:   []reporter.Reporter{recorder},
		groupOutput: []string{""},
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs(nil),
			finder.WithExcludeFileTypes(nil),
			finder.WithFileTypes(filetype.FileTypes),
		},
		searchPaths: []string{dir},
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	done := make(chan error, 1)
	go func() {
		runner := watchRunner{
			newWatcher: func() (fileWatcher, error) {
				return watcher, nil
			},
			debounceDelay: 10 * time.Millisecond,
		}
		_, err := runner.run(ctx, rc)
		done <- err
	}()

	require.Eventually(t, func() bool {
		return recorder.callCount() == 1
	}, time.Second, 10*time.Millisecond)

	watcher.errors <- errors.New("temporary watcher error")
	watcher.events <- fsnotify.Event{Name: jsonFile, Op: fsnotify.Write}

	require.Eventually(t, func() bool {
		return recorder.callCount() == 2
	}, time.Second, 10*time.Millisecond)

	cancel()
	require.NoError(t, <-done)
}

func TestFindChangedFileHonorsDirectoryFilters(t *testing.T) {
	dir := t.TempDir()
	excludedDir := filepath.Join(dir, "vendor")
	require.NoError(t, os.Mkdir(excludedDir, 0o755))
	jsonFile := testhelper.WriteFile(t, excludedDir, "good.json", testhelper.ValidContent["json"])

	rc := &resolvedConfig{
		finderOpts: []finder.FSFinderOptions{
			finder.WithPathRoots(dir),
			finder.WithExcludeDirs([]string{"vendor"}),
			finder.WithExcludeFileTypes(nil),
			finder.WithFileTypes(filetype.FileTypes),
		},
	}

	files, err := findChangedFile(rc, jsonFile)
	require.NoError(t, err)
	require.Empty(t, files)
}

func TestAddWatchPathsUsesFileParentAndSkipsDuplicates(t *testing.T) {
	dir := t.TempDir()
	jsonFile := testhelper.WriteFile(t, dir, "good.json", testhelper.ValidContent["json"])
	watcher := newFakeWatcher()

	watched := make(map[string]struct{})
	require.NoError(t, addWatchPaths(watcher, []string{jsonFile, jsonFile}, watched))

	require.Equal(t, []string{mustAbs(t, dir)}, watcher.addedPaths())
}

func mustAbs(t *testing.T, path string) string {
	t.Helper()

	absPath, err := filepath.Abs(path)
	require.NoError(t, err)
	return absPath
}
