package cli

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
)

// Format runs the formatting pipeline.
//
// For each discovered file whose FileType has a registered Formatter:
//   - If the file is already formatted: Report{Status: StatusPass}
//   - If the file needs formatting: Report{Status: StatusUnformatted}
//   - If --fix is set: rewrite the file atomically, report as StatusPass
//   - If the file cannot be parsed by the formatter: skip silently
//
// Files whose FileType has no Formatter are silently skipped.
//
// Returns 0 if all files are formatted (or all were fixed), 1 if any file
// needs formatting and --fix was not set, 2 on tool error.
func (c *CLI) Format(opts formatter.Options) (int, error) {
	foundFiles, err := c.finder.Find()
	if err != nil {
		return 2, fmt.Errorf("unable to find files: %w", err)
	}

	type job struct {
		path  string
		name  string
		fmter formatter.Formatter
	}

	var jobs []job
	for _, f := range foundFiles {
		if f.FileType.Formatter == nil {
			continue
		}
		jobs = append(jobs, job{
			path:  f.Path,
			name:  f.Name,
			fmter: f.FileType.Formatter,
		})
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	// Process files in parallel using a bounded worker pool.
	type result struct {
		idx    int
		report reporter.Report
	}

	results := make([]result, len(jobs))
	jobCh := make(chan int, len(jobs))
	for i := range jobs {
		jobCh <- i
	}
	close(jobCh)

	workers := runtime.NumCPU()
	if workers > len(jobs) {
		workers = len(jobs)
	}

	var mu sync.Mutex
	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for idx := range jobCh {
				j := jobs[idx]
				r := result{idx: idx, report: c.formatFile(j.path, j.name, j.fmter, opts)}
				mu.Lock()
				results[idx] = r
				mu.Unlock()
			}
		})
	}
	wg.Wait()

	// Collect reports in original (sorted) order for deterministic output.
	reports := make([]reporter.Report, len(results))
	issueFound := false
	for _, r := range results {
		reports[r.idx] = r.report
		if r.report.Status == reporter.StatusUnformatted {
			issueFound = true
		}
	}

	if err := c.printReports(reports); err != nil {
		return 2, err
	}

	if issueFound {
		return 1, nil
	}
	return 0, nil
}

// formatFile formats a single file and returns its report.
func (c *CLI) formatFile(path, name string, fmter formatter.Formatter, opts formatter.Options) reporter.Report {
	content, err := os.ReadFile(path)
	if err != nil {
		if isBrokenSymlink(path) {
			return reporter.Report{
				FileName: name,
				FilePath: path,
				Status:   reporter.StatusFail,
				Issues: []reporter.Issue{{
					Type:    reporter.IssueTypeSyntax,
					Message: "broken symlink",
				}},
				IsQuiet: c.quiet,
			}
		}
		// Read error — not a format issue, skip silently.
		return reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet}
	}

	formatted, err := fmter.Format(content, opts)
	if err != nil {
		// Formatter could not parse the file — it's a syntax error, not a
		// format issue. cfv check would catch it. Skip silently here.
		return reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet}
	}

	if bytes.Equal(content, formatted) {
		return reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet}
	}

	// File needs formatting.
	if c.fix {
		if err := writeFileAtomic(path, formatted); err != nil {
			return reporter.Report{
				FileName: name,
				FilePath: path,
				Status:   reporter.StatusFail,
				Issues: []reporter.Issue{{
					Type:    reporter.IssueTypeSyntax,
					Message: fmt.Sprintf("failed to write formatted file: %v", err),
				}},
				IsQuiet: c.quiet,
			}
		}
		// Successfully fixed — report as pass.
		return reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet}
	}

	return reporter.Report{
		FileName: name,
		FilePath: path,
		Status:   reporter.StatusUnformatted,
		Issues:   []reporter.Issue{{Type: reporter.IssueTypeFormat, Message: "needs formatting (run cfv format --fix to rewrite)"}},
		IsQuiet:  c.quiet,
	}
}

// writeFileAtomic writes data to path using a temp file + rename.
// This is atomic on POSIX (same filesystem). Preserves original permissions.
func writeFileAtomic(path string, data []byte) error {
	var perm fs.FileMode = 0o600 //nolint:mnd // sensible default if stat fails
	if info, err := os.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".cfv-fmt-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on failure. Intentionally ignore the remove error —
	// it's best-effort cleanup and the original error matters more.
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := os.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return nil
}
