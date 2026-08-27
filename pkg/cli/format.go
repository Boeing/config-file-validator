package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"sync"

	"github.com/pmezard/go-difflib/difflib"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
)

// FormatOptionsFunc returns resolved format options for a given format name
// and file path. The path matters because .editorconfig settings are resolved
// per file.
type FormatOptionsFunc func(formatName, path string) formatter.Options

// Format runs the formatting pipeline.
//
// For each discovered file whose FileType has a registered Formatter:
//   - If the file is already formatted: Report{Status: StatusPass}
//   - If the file needs formatting: Report{Status: StatusUnformatted}
//   - If --fix is set: rewrite the file atomically, report as StatusPass
//   - If the file cannot be parsed by the formatter: skipped (not reported)
//   - If the file cannot be read (non-symlink): skipped (not reported)
//
// Files whose FileType has no Formatter are silently skipped.
// Broken symlinks are reported as StatusFail.
//
// Returns 0 if all files are formatted (or all were fixed), 1 if any file
// needs formatting and --fix was not set, 2 on tool error.
func (c *CLI) Format(optsFunc FormatOptionsFunc) (int, error) {
	// Stdin mode: format the piped content and write to stdout.
	if c.stdinData != nil {
		return c.formatStdin(optsFunc)
	}

	foundFiles, err := c.finder.Find()
	if err != nil {
		return 2, fmt.Errorf("unable to find files: %w", err)
	}

	type job struct {
		path       string
		name       string
		formatName string
		fmter      formatter.Formatter
	}

	var jobs []job
	for _, f := range foundFiles {
		if f.FileType.Formatter == nil {
			continue
		}
		// Skip files that are format-ignored by external tool config.
		if c.formatIgnores != nil {
			absPath, err := filepath.Abs(f.Path)
			if err == nil && c.formatIgnores.ShouldSkipFormat(absPath, f.FileType.Name) {
				continue
			}
		}
		jobs = append(jobs, job{
			path:       f.Path,
			name:       f.Name,
			formatName: f.FileType.Name,
			fmter:      f.FileType.Formatter,
		})
	}

	if len(jobs) == 0 {
		return 0, nil
	}

	// Process files in parallel using a bounded worker pool.
	// results is pre-allocated with one slot per job, indexed by job position,
	// so each goroutine writes to a unique index — no mutex needed.
	// A nil pointer means the file was skipped (parse error or unreadable).
	results := make([]*reporter.Report, len(jobs))
	jobCh := make(chan int, len(jobs))
	for i := range jobs {
		jobCh <- i
	}
	close(jobCh)

	workers := runtime.NumCPU()
	if workers > len(jobs) {
		workers = len(jobs)
	}

	var wg sync.WaitGroup
	for range workers {
		wg.Go(func() {
			for idx := range jobCh {
				j := jobs[idx]
				opts := optsFunc(j.formatName, j.path)
				results[idx] = c.formatFile(j.path, j.name, j.fmter, opts)
			}
		})
	}
	wg.Wait()

	// Collect non-nil reports in original (sorted) order for deterministic output.
	var reports []reporter.Report
	issueFound := false
	for _, r := range results {
		if r == nil {
			// File was skipped (unparseable or unreadable).
			continue
		}
		reports = append(reports, *r)
		if r.Status == reporter.StatusUnformatted || r.Status == reporter.StatusFail {
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

// formatStdin formats stdin data and writes the result to stdout.
// With --diff: prints unified diff between stdin and formatted output.
// With --fix: ignored (cannot write back to stdin); behaves like plain format.
// Returns 0 if content was already formatted, 1 if it needed formatting.
func (c *CLI) formatStdin(optsFunc FormatOptionsFunc) (int, error) {
	fmter := c.stdinFileType.Formatter
	if fmter == nil {
		return 2, fmt.Errorf("file type %q has no formatter", c.stdinFileType.Name)
	}

	hadBOM := hasBOM(c.stdinData)
	content := stripBOM(c.stdinData)
	opts := optsFunc(c.stdinFileType.Name, "stdin")

	formatted, err := fmter.Format(content, opts)
	if err != nil {
		return 2, fmt.Errorf("formatting stdin: %w", err)
	}

	if bytes.Equal(content, formatted) {
		// Already formatted — write original to stdout unchanged (preserves BOM).
		if _, err := os.Stdout.Write(c.stdinData); err != nil {
			return 2, fmt.Errorf("writing to stdout: %w", err)
		}
		return 0, nil
	}

	if c.diff {
		diff := unifiedDiff("stdin", content, formatted)
		fmt.Print(diff)
		return 1, nil
	}

	// Restore BOM in output if the original input had one.
	if hadBOM {
		formatted = append([]byte{0xef, 0xbb, 0xbf}, formatted...)
	}

	if _, err := os.Stdout.Write(formatted); err != nil {
		return 2, fmt.Errorf("writing to stdout: %w", err)
	}
	return 1, nil
}

// formatFile formats a single file and returns its report.
// Returns nil when the file should be skipped entirely (unreadable or unparseable).
// Broken symlinks are never skipped — they return a StatusFail report.
func (c *CLI) formatFile(path, name string, fmter formatter.Formatter, opts formatter.Options) *reporter.Report {
	// Skip the config file — a tool should not format its own config.
	if c.configFilePath != "" {
		absPath, err := filepath.Abs(path)
		if err == nil && absPath == c.configFilePath {
			return &reporter.Report{
				FileName: name,
				FilePath: path,
				Status:   reporter.StatusPass,
				IsQuiet:  c.quiet || c.diff,
			}
		}
	}

	content, err := os.ReadFile(path)
	if err != nil {
		if isBrokenSymlink(path) {
			return &reporter.Report{
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
		// Read error — not a format issue. Skip so the file doesn't inflate
		// pass counts. cfv check handles read errors separately.
		return nil
	}

	hadBOM := hasBOM(content)
	content = stripBOM(content)

	formatted, err := fmter.Format(content, opts)
	if err != nil {
		// Check if the formatter skipped this file (e.g., mixed content XML).
		// Report it to the user as a pass with the skip reason.
		var skipped *formatter.ErrSkipped
		if errors.As(err, &skipped) {
			return &reporter.Report{
				FileName: name,
				FilePath: path,
				Status:   reporter.StatusPass,
				IsQuiet:  c.quiet,
				Issues: []reporter.Issue{
					{Message: skipped.Error(), Type: reporter.IssueTypeFormat},
				},
			}
		}
		// Formatter could not parse the file — it's a syntax error, not a
		// formatting issue. cfv check would catch it. Skip here.
		return nil
	}

	if bytes.Equal(content, formatted) {
		return &reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet || c.diff}
	}

	// Restore BOM in output if the original file had one.
	if hadBOM {
		formatted = append([]byte{0xef, 0xbb, 0xbf}, formatted...)
	}

	// Diff mode: print unified diff to stdout, report as unformatted.
	if c.diff {
		diff := unifiedDiff(path, content, formatted)
		fmt.Print(diff)
		return &reporter.Report{
			FileName: name,
			FilePath: path,
			Status:   reporter.StatusUnformatted,
			IsQuiet:  true, // suppress reporter output — diff is the output
		}
	}

	// File needs formatting.
	if c.fix {
		if err := writeFileAtomic(path, formatted); err != nil {
			return &reporter.Report{
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
		return &reporter.Report{FileName: name, FilePath: path, Status: reporter.StatusPass, IsQuiet: c.quiet}
	}

	return &reporter.Report{
		FileName: name,
		FilePath: path,
		Status:   reporter.StatusUnformatted,
		IsQuiet:  c.quiet,
	}
}

// unifiedDiff computes a unified diff between original and formatted content.
func unifiedDiff(path string, original, formatted []byte) string {
	diff, _ := difflib.GetUnifiedDiffString(difflib.UnifiedDiff{
		A:        difflib.SplitLines(string(original)),
		B:        difflib.SplitLines(string(formatted)),
		FromFile: path,
		ToFile:   path + " (formatted)",
		Context:  3,
	})
	return diff
}

// FileSystem abstracts file operations for testability.
type FileSystem interface {
	Stat(path string) (fs.FileInfo, error)
	CreateTemp(dir, pattern string) (File, error)
	Chmod(path string, mode fs.FileMode) error
	Rename(oldpath, newpath string) error
	Remove(path string) error
}

// File abstracts file write operations.
type File interface {
	Name() string
	Write(b []byte) (int, error)
	Close() error
}

// osFS is the real filesystem implementation.
type osFS struct{}

func (osFS) Stat(path string) (fs.FileInfo, error)        { return os.Stat(path) }
func (osFS) CreateTemp(dir, pattern string) (File, error) { return os.CreateTemp(dir, pattern) }
func (osFS) Chmod(path string, mode fs.FileMode) error    { return os.Chmod(path, mode) }
func (osFS) Rename(oldpath, newpath string) error         { return os.Rename(oldpath, newpath) }
func (osFS) Remove(path string) error                     { return os.Remove(path) }

// defaultFS is the filesystem used in production.
var defaultFS FileSystem = osFS{}

// writeFileAtomic writes data to path using a temp file + rename.
// This is atomic on POSIX (same filesystem). Preserves original permissions.
// Symlinks are replaced with regular files (standard behavior, same as gofmt).
func writeFileAtomic(path string, data []byte) error {
	return writeFileAtomicWith(defaultFS, path, data)
}

func writeFileAtomicWith(fsys FileSystem, path string, data []byte) error {
	var perm fs.FileMode = 0o600 //nolint:mnd // sensible default if stat fails
	if info, err := fsys.Stat(path); err == nil {
		perm = info.Mode().Perm()
	}

	dir := filepath.Dir(path)
	tmp, err := fsys.CreateTemp(dir, ".cfv-fmt-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmp.Name()

	// Clean up temp file on failure. Intentionally ignore the remove error —
	// it's best-effort cleanup and the original error matters more.
	success := false
	defer func() {
		if !success {
			_ = fsys.Remove(tmpPath)
		}
	}()

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("writing temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("closing temp file: %w", err)
	}
	if err := fsys.Chmod(tmpPath, perm); err != nil {
		return fmt.Errorf("setting permissions: %w", err)
	}
	if err := fsys.Rename(tmpPath, path); err != nil {
		return fmt.Errorf("renaming temp file: %w", err)
	}

	success = true
	return nil
}
