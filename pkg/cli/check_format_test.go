package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/internal/testhelper"
	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
)

// defaultJSONOpts returns a FormatOptionsFunc that always returns JSON defaults.
func defaultJSONOpts() FormatOptionsFunc {
	return func(_, _ string) formatter.Options {
		return jsonfmt.DefaultOptions()
	}
}

// TestCheckReportsFormattingIssues verifies that cfv check reports a file as
// unformatted (StatusUnformatted with IssueTypeFormat) when the file is
// syntactically valid but not canonically formatted.
func TestCheckReportsFormattingIssues(t *testing.T) {
	dir := t.TempDir()
	// Unformatted JSON (missing bracket spacing)
	testhelper.WriteFile(t, dir, "app.json", `{"key":"value"}`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()),
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus, "exit 1 when file is unformatted")

	require.Len(t, capture.reports, 1)
	r := capture.reports[0]
	require.Equal(t, reporter.StatusUnformatted, r.Status)
	require.Len(t, r.Issues, 1)
	require.Equal(t, reporter.IssueTypeFormat, r.Issues[0].Type)
	require.Contains(t, r.Issues[0].Message, "not formatted")
}

// TestCheckFixAppliesFormatting verifies that cfv check --fix rewrites an
// unformatted file and reports it as pass.
func TestCheckFixAppliesFormatting(t *testing.T) {
	dir := t.TempDir()
	path := testhelper.WriteFile(t, dir, "app.json", `{"key":"value"}`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()),
		WithFix(true),
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus, "exit 0 after fix")

	require.Len(t, capture.reports, 1)
	r := capture.reports[0]
	require.Equal(t, reporter.StatusPass, r.Status)
	require.Contains(t, r.Notes, "fixed: formatting")

	// Verify file on disk is now formatted.
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{ \"key\": \"value\" }\n", string(data)) //nolint:testifylint // comparing formatted output, not JSON semantics
}

// TestCheckSkipsFormatOnSyntaxError verifies that format checking is not
// attempted on files that fail syntax validation.
func TestCheckSkipsFormatOnSyntaxError(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "bad.json", `{"key": }`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()),
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)

	require.Len(t, capture.reports, 1)
	r := capture.reports[0]
	require.Equal(t, reporter.StatusFail, r.Status)
	// Only syntax issue — no format issue.
	for _, issue := range r.Issues {
		require.NotEqual(t, reporter.IssueTypeFormat, issue.Type,
			"format issue should not be reported on syntax error")
	}
}

// TestCheckSkipsFormatWhenNoFormatter verifies that format checking is
// skipped for file types without a registered Formatter (e.g., CSV, CUE).
func TestCheckSkipsFormatWhenNoFormatter(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "data.csv", "a,b,c\n1,2,3\n")

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(
			finder.WithPathRoots(dir),
			finder.WithFileTypes([]filetype.FileType{filetype.CsvFileType}),
		)),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()), // opts func provided but CSV has no formatter
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus, "exit 0 — CSV has no formatter, no format check")

	require.Len(t, capture.reports, 1)
	require.Equal(t, reporter.StatusPass, capture.reports[0].Status)
}

// TestCheckFormatAfterFixerRules verifies that when --fix is enabled, the
// fixer resolves syntax errors first, then format checking runs on the
// fixed content and formats it.
func TestCheckFormatAfterFixerRules(t *testing.T) {
	dir := t.TempDir()
	// Trailing comma (fixer removes it) AND wrong indent/spacing (formatter fixes)
	path := testhelper.WriteFile(t, dir, "app.json", `{"key": "value",}`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()),
		WithFix(true),
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus, "exit 0 after fix + format")

	require.Len(t, capture.reports, 1)
	r := capture.reports[0]
	require.Equal(t, reporter.StatusPass, r.Status)

	// File should be fully clean (no trailing comma + formatted).
	data, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, "{ \"key\": \"value\" }\n", string(data)) //nolint:testifylint // comparing formatted output, not JSON semantics
}

// TestCheckFormatOptsNilSkipsFormatting verifies backward compatibility:
// when formatOptsFunc is nil (not provided), format checking is not performed.
func TestCheckFormatOptsNilSkipsFormatting(t *testing.T) {
	dir := t.TempDir()
	// Unformatted — but no format opts provided, so should pass.
	testhelper.WriteFile(t, dir, "app.json", `{"key":"value"}`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		// No WithFormatOptions — formatOptsFunc is nil
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus, "exit 0 — format checking disabled")

	require.Len(t, capture.reports, 1)
	require.Equal(t, reporter.StatusPass, capture.reports[0].Status)
}

// TestCheckSkipsFormatOnSchemaError verifies that format checking is not
// attempted on files that fail schema validation. The user should fix the
// schema error first — format issues are secondary.
func TestCheckSkipsFormatOnSchemaError(t *testing.T) {
	dir := t.TempDir()
	schemaDir := t.TempDir()
	// Valid JSON syntax, will fail schema validation (wrong type for "name").
	testhelper.WriteFile(t, dir, "app.json", `{"name": 123}`)
	// Schema that requires "name" to be a string (in separate dir so finder doesn't pick it up).
	schemaPath := testhelper.WriteFile(t, schemaDir, "schema.json",
		`{"type":"object","properties":{"name":{"type":"string"}},"required":["name"]}`)

	capture := &captureReporter{}
	c := Init(
		WithFinder(finder.FileSystemFinderInit(finder.WithPathRoots(dir))),
		WithReporters(capture),
		WithFormatOptions(defaultJSONOpts()),
		WithSchemaMap(map[string]string{"app.json": schemaPath}),
	)

	exitStatus, err := c.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)

	require.Len(t, capture.reports, 1)
	r := capture.reports[0]
	require.Equal(t, reporter.StatusFail, r.Status)
	// Only schema issue — no format issue.
	for _, issue := range r.Issues {
		require.NotEqual(t, reporter.IssueTypeFormat, issue.Type,
			"format issue should not be reported when schema errors exist")
	}
}
