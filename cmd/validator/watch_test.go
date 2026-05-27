package main

import (
	"context"
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
	events chan fsnotify.Event
	errors chan error
}

func newFakeWatcher() *fakeWatcher {
	return &fakeWatcher{
		events: make(chan fsnotify.Event, 4),
		errors: make(chan error, 1),
	}
}

func (*fakeWatcher) Add(string) error {
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
