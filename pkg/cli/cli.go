package cli

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/fixer"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
	"github.com/Boeing/config-file-validator/v3/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v3/pkg/tools"
	"github.com/Boeing/config-file-validator/v3/pkg/validator"
)

// CLI is the main entry point for running config file validation and formatting.
// Use Init with Option functions to configure, then call Run (check) or Format.
// SchemaMapping is a single glob-pattern → schema-path association.
// Ordered slice preserves user-specified priority (first match wins).
type SchemaMapping struct {
	Pattern    string
	SchemaPath string
}

type CLI struct {
	finder         finder.FileFinder
	reporters      []reporter.Reporter
	groupOutput    []string
	quiet          bool
	requireSchema  bool
	noSchema       bool
	schemaMap      []SchemaMapping
	schemaStore    *schemastore.Store
	configFilePath string // excluded from format checking
	stdinData      []byte
	stdinFileType  filetype.FileType
	errorFound     bool
	// fix enables writing formatted output back to disk.
	// When false, Format reports issues but does not write.
	fix bool
	// diff enables unified diff output mode.
	// When true, Format prints diffs to stdout instead of the normal report.
	// Mutually exclusive with fix.
	diff bool
	// formatOptsFunc resolves format options per file during check.
	// When non-nil, Run() checks formatting after syntax+schema validation.
	// When nil, format checking is skipped (backward compatibility).
	formatOptsFunc FormatOptionsFunc
	// formatIgnores holds per-format ignore patterns from external tool configs.
	// When non-nil, files matching ignore patterns are skipped from format checking.
	formatIgnores *formatter.FormatIgnores
}

// Option configures a CLI instance.
type Option func(*CLI)

func WithFinder(f finder.FileFinder) Option {
	return func(c *CLI) {
		c.finder = f
	}
}

func WithReporters(r ...reporter.Reporter) Option {
	return func(c *CLI) {
		c.reporters = r
	}
}

func WithGroupOutput(groupOutput []string) Option {
	return func(c *CLI) {
		c.groupOutput = groupOutput
	}
}

func WithQuiet(quiet bool) Option {
	return func(c *CLI) {
		c.quiet = quiet
	}
}

func WithRequireSchema(require bool) Option {
	return func(c *CLI) {
		c.requireSchema = require
	}
}

func WithNoSchema(noSchema bool) Option {
	return func(c *CLI) {
		c.noSchema = noSchema
	}
}

func WithSchemaMap(m []SchemaMapping) Option {
	return func(c *CLI) {
		c.schemaMap = m
	}
}

// WithConfigFile sets the path to the resolved config file so it can be
// excluded from format checking (a tool should not format its own config).
func WithConfigFile(path string) Option {
	return func(c *CLI) {
		c.configFilePath = path
	}
}

func WithSchemaStore(s *schemastore.Store) Option {
	return func(c *CLI) {
		c.schemaStore = s
	}
}

func WithStdinData(data []byte, ft filetype.FileType) Option {
	return func(c *CLI) {
		c.stdinData = data
		c.stdinFileType = ft
	}
}

// WithFix enables writing formatted output back to disk when calling Format.
// When false (the default), Format reports issues but does not modify files.
func WithFix(fix bool) Option {
	return func(c *CLI) {
		c.fix = fix
	}
}

// WithDiff enables unified diff output mode for Format.
// When true, Format prints diffs instead of the normal pass/fail report.
func WithDiff(diff bool) Option {
	return func(c *CLI) {
		c.diff = diff
	}
}

// WithFormatOptions enables format checking in the check pipeline.
// When set, Run() checks formatting after syntax+schema validation passes.
// Files that are valid but not canonically formatted are reported as
// StatusUnformatted. When --fix is also enabled, the formatted output is
// written back to disk.
func WithFormatOptions(f FormatOptionsFunc) Option {
	return func(c *CLI) {
		c.formatOptsFunc = f
	}
}

// WithFormatIgnores sets the format-ignore matcher for the CLI.
// Files matching ignore patterns are skipped from format checking and formatting.
func WithFormatIgnores(fi *formatter.FormatIgnores) Option {
	return func(c *CLI) {
		c.formatIgnores = fi
	}
}

func Init(opts ...Option) *CLI {
	c := &CLI{
		finder:    finder.FileSystemFinderInit(),
		reporters: []reporter.Reporter{reporter.NewStdoutReporter("")},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *CLI) Run() (int, error) {
	c.errorFound = false

	if c.stdinData != nil {
		return c.runSingle(c.stdinData, c.stdinFileType, "stdin")
	}

	foundFiles, err := c.finder.Find()
	if err != nil {
		return 2, fmt.Errorf("unable to find files: %w", err)
	}

	var reports []reporter.Report
	for _, f := range foundFiles {
		content, err := os.ReadFile(f.Path)
		if err != nil {
			if isBrokenSymlink(f.Path) {
				report := reporter.Report{
					FileName: f.Name,
					FilePath: f.Path,
					Status:   reporter.StatusFail,
					Issues: []reporter.Issue{{
						Type:    reporter.IssueTypeSyntax,
						Message: "broken symlink",
					}},
				}
				c.errorFound = true
				reports = append(reports, report)
				continue
			}
			return 2, fmt.Errorf("unable to read file: %w", err)
		}

		// Strip UTF-8 BOM if present. Many editors (especially on Windows)
		// add BOM to UTF-8 files. Parsers for JSON, TOML, and JSONC don't
		// handle it, so we strip it centrally before validation.
		content = stripBOM(content)

		report := c.validate(content, f.FileType, f.Name, f.Path)

		// When --fix is enabled and there are errors, attempt to fix.
		if c.fix && report.HasErrors() {
			fixedReport := c.attemptFix(content, f.FileType, f.Name, f.Path)
			if fixedReport != nil {
				report = *fixedReport
			}
		}

		// Format checking: only on files that pass all validation (syntax + schema).
		// If a file has errors, the user should fix those first — format issues
		// are secondary noise. With --fix, the fixer may resolve errors AND then
		// formatting is checked on the fixed content.
		if !report.HasErrors() && c.formatOptsFunc != nil && f.FileType.Formatter != nil {
			report = c.checkFormatting(report, content, f.FileType, f.Path)
		}

		if report.HasErrors() || report.Status == reporter.StatusUnformatted {
			c.errorFound = true
		}
		reports = append(reports, report)
	}

	if err := c.printReports(reports); err != nil {
		return 2, err
	}

	if c.errorFound {
		return 1, nil
	}
	return 0, nil
}

// validate runs syntax and schema validation on content and returns a Report.
func (c *CLI) validate(content []byte, ft filetype.FileType, name, path string) reporter.Report {
	isValid, syntaxErr := ft.Validator.ValidateSyntax(content)

	var schemaErr error
	var schemaWarnings []string
	if isValid {
		isValid, schemaWarnings, schemaErr = c.validateSchema(ft.Validator, content, path)
	}

	notes := checkJSONCFallback(syntaxErr, ft, content, name)
	// Schema warnings become notes (they don't fail the file).
	notes = append(notes, schemaWarnings...)

	report := reporter.Report{
		FileName: name,
		FilePath: path,
		IsQuiet:  c.quiet,
		Notes:    notes,
	}

	if isValid {
		report.Status = reporter.StatusPass
	} else {
		report.Status = reporter.StatusFail
	}

	// Convert syntax error to issues.
	if syntaxErr != nil {
		report.Issues = append(report.Issues, buildIssues(syntaxErr, reporter.IssueTypeSyntax)...)
	}

	// Convert schema error to issues.
	if schemaErr != nil {
		report.Issues = append(report.Issues, buildIssues(schemaErr, reporter.IssueTypeSchema)...)
	}

	return report
}

// buildIssues converts a validation error into one or more Issue structs.
func buildIssues(err error, issueType reporter.IssueType) []reporter.Issue {
	var se *validator.SchemaErrors
	if errors.As(err, &se) {
		issues := make([]reporter.Issue, 0, len(se.Errors()))
		for i, msg := range se.Errors() {
			issue := reporter.Issue{
				Type:    issueType,
				Message: msg,
			}
			if i < len(se.Positions) {
				issue.Line = se.Positions[i].Line
				issue.Column = se.Positions[i].Column
			}
			issues = append(issues, issue)
		}
		return issues
	}

	issue := reporter.Issue{
		Type:    issueType,
		Message: err.Error(),
	}
	var ve *validator.ValidationError
	if errors.As(err, &ve) {
		issue.Message = ve.Err.Error()
		issue.Line = ve.Line
		issue.Column = ve.Column
	}
	return []reporter.Issue{issue}
}

// runSingle validates a single piece of content (used for stdin mode).
func (c *CLI) runSingle(content []byte, ft filetype.FileType, name string) (int, error) {
	report := c.validate(content, ft, name, name)

	if err := c.printReports([]reporter.Report{report}); err != nil {
		return 2, err
	}

	if report.HasErrors() {
		return 1, nil
	}
	return 0, nil
}

// checkJSONCFallback checks if a failed JSON file is valid JSONC and returns a note if so.
func checkJSONCFallback(syntaxErr error, ft filetype.FileType, content []byte, name string) []string {
	if syntaxErr == nil {
		return nil
	}
	if _, isJSON := ft.Validator.(validator.JSONValidator); !isJSON {
		return nil
	}
	jsoncValidator := validator.JSONCValidator{}
	if valid, _ := jsoncValidator.ValidateSyntax(content); valid {
		return []string{
			`this file is valid JSONC (JSON with comments/trailing commas). To validate as JSONC, use --type-map="**/` +
				name + `:jsonc"`,
		}
	}
	return nil
}

func (c *CLI) validateSchema(v validator.Validator, content []byte, filePath string) (bool, []string, error) {
	if c.noSchema {
		return true, nil, nil
	}

	sv, hasSV := v.(validator.SchemaValidator)
	if hasSV {
		valid, err := sv.ValidateSchema(content, filePath)
		if !errors.Is(err, validator.ErrNoSchema) {
			return valid, nil, err
		}
	}

	if schemaPath, ok := c.lookupSchemaMap(filePath); ok {
		valid, skipped, err := validateWithExternal(v, content, schemaPath)
		if skipped {
			if c.requireSchema {
				return false, nil, &validator.SchemaErrors{
					Items: []string{schemaMapUnsupportedError(schemaPath)},
				}
			}
			return valid, []string{schemaMapUnsupportedWarning(schemaPath)}, nil
		}
		return valid, nil, err
	}

	if c.schemaStore != nil {
		if schemaPath, ok := c.schemaStore.Resolve(filePath); ok {
			valid, _, err := validateWithExternal(v, content, schemaPath)
			return valid, nil, err
		}
	}

	if hasSV && c.requireSchema {
		return false, nil, validator.ErrNoSchema
	}
	if !hasSV && c.requireSchema {
		if _, hasJM := v.(validator.JSONMarshaler); hasJM {
			return false, nil, validator.ErrNoSchema
		}
	}
	return true, nil, nil
}

func (c *CLI) lookupSchemaMap(filePath string) (string, bool) {
	if len(c.schemaMap) == 0 {
		return "", false
	}
	baseName := filepath.Base(filePath)
	for _, m := range c.schemaMap {
		if !tools.IsGlobPattern(m.Pattern) {
			if m.Pattern == baseName {
				return m.SchemaPath, true
			}
			continue
		}
		// If pattern has no path separator, match against basename only.
		// "*.json" matches any JSON file regardless of directory depth.
		target := filePath
		if !strings.Contains(m.Pattern, "/") {
			target = baseName
		}
		matched, err := doublestar.PathMatch(m.Pattern, target)
		if err == nil && matched {
			return m.SchemaPath, true
		}
	}
	return "", false
}

func validateWithExternal(v validator.Validator, content []byte, schemaPath string) (valid bool, skipped bool, err error) {
	if _, ok := v.(validator.XMLSchemaValidator); ok {
		absSchema, err := filepath.Abs(schemaPath)
		if err != nil {
			return false, false, fmt.Errorf("resolving schema path: %w", err)
		}
		valid, err := validator.ValidateXSD(content, absSchema)
		return valid, false, err
	}

	jm, ok := v.(validator.JSONMarshaler)
	if !ok {
		return true, true, nil
	}

	schemaURL, err := toSchemaURL(schemaPath)
	if err != nil {
		return false, false, err
	}

	docJSON, err := jm.MarshalToJSON(content)
	if err != nil {
		return false, false, err
	}

	valid, err = validator.JSONSchemaValidate(schemaURL, docJSON)
	return valid, false, err
}

func schemaMapUnsupportedWarning(schemaPath string) string {
	return fmt.Sprintf("--schema-map matched this file, but its validator does not support schema validation; skipping schema %q", schemaPath)
}

func schemaMapUnsupportedError(schemaPath string) string {
	return fmt.Sprintf("--schema-map matched this file, but its validator does not support schema validation for schema %q", schemaPath)
}

func toSchemaURL(schemaPath string) (string, error) {
	if strings.HasPrefix(schemaPath, "https://") || strings.HasPrefix(schemaPath, "http://") {
		return schemaPath, nil
	}
	absSchema, err := filepath.Abs(schemaPath)
	if err != nil {
		return "", fmt.Errorf("resolving schema path: %w", err)
	}
	return tools.FileURL(absSchema), nil
}

func (c *CLI) printReports(reports []reporter.Report) error {
	if len(c.groupOutput) > 0 && c.groupOutput[0] != "" {
		return c.printGroup(reports)
	}

	var errs []error
	for _, reporterObj := range c.reporters {
		if err := reporterObj.Print(reports); err != nil {
			errs = append(errs, fmt.Errorf("reporter: %w", err))
			c.errorFound = true
		}
	}

	return errors.Join(errs...)
}

func (c *CLI) printGroup(reports []reporter.Report) error {
	reportGroup, err := GroupBy(reports, c.groupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by value: %w", err)
	}

	for _, reporterObj := range c.reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintGroupJSON(reportGroup)
		}
	}

	return reporter.PrintGroupStdout(reportGroup)
}

// isBrokenSymlink reports whether path is a symlink whose target does not exist.
func isBrokenSymlink(path string) bool {
	fi, err := os.Lstat(path)
	if err != nil {
		return false
	}
	if fi.Mode()&fs.ModeSymlink == 0 {
		return false
	}
	_, err = os.Stat(path)
	return os.IsNotExist(err)
}

// hasBOM returns true if b starts with a UTF-8 byte order mark (0xEF 0xBB 0xBF).
func hasBOM(b []byte) bool {
	return len(b) >= 3 && b[0] == 0xef && b[1] == 0xbb && b[2] == 0xbf
}

// stripBOM removes a UTF-8 BOM prefix if present.
func stripBOM(b []byte) []byte {
	if hasBOM(b) {
		return b[3:]
	}
	return b
}

// defaultFixRules returns the standard set of safe fix rules.
func defaultFixRules() []fixer.Rule {
	return []fixer.Rule{
		fixer.JSONTrailingComma{},
		fixer.JSONStringToInt{},
		fixer.JSONStringToBool{},
	}
}

// attemptFix runs the fixer on content and writes the result if fixes were applied.
// Returns a new report reflecting the post-fix state, or nil if no fixes could be applied.
func (c *CLI) attemptFix(content []byte, ft filetype.FileType, name, path string) *reporter.Report {
	// Resolve schema bytes for this file (nil if no schema).
	schemaBytes := c.resolveSchemaBytes(ft.Validator, path)

	f := fixer.New(defaultFixRules()...)
	result := f.Fix(content, schemaBytes, ft.Name)

	if len(result.Applied) == 0 {
		return nil // no fixes available
	}

	// Verify the fixed output is valid before writing.
	verifyReport := c.validate(result.Fixed, ft, name, path)
	if verifyReport.HasErrors() {
		// Fix didn't fully resolve the issue — don't write a partial fix.
		// Return nil to keep the original error report.
		return nil
	}

	// Write the fixed file atomically.
	if err := writeFileAtomic(path, result.Fixed); err != nil {
		// Write failed — report as the original error.
		return nil
	}

	// Build a pass report with notes about what was fixed.
	report := reporter.Report{
		FileName: name,
		FilePath: path,
		Status:   reporter.StatusPass,
		IsQuiet:  c.quiet,
	}
	for _, fix := range result.Applied {
		report.Notes = append(report.Notes, "fixed: "+fix.Message)
	}

	return &report
}

// resolveSchemaBytes returns the raw JSON Schema bytes for a file, or nil
// if no schema is available. This is used by the fixer to understand what
// types fields should have.
func (c *CLI) resolveSchemaBytes(v validator.Validator, filePath string) []byte {
	if c.noSchema {
		return nil
	}

	// Check schema-map first.
	if schemaPath, ok := c.lookupSchemaMap(filePath); ok {
		data, err := os.ReadFile(schemaPath)
		if err == nil {
			return data
		}
	}

	// Check SchemaStore.
	if c.schemaStore != nil {
		if schemaPath, ok := c.schemaStore.Resolve(filePath); ok {
			data, err := os.ReadFile(schemaPath)
			if err == nil {
				return data
			}
		}
	}

	// Check inline schema declaration (JSON $schema property).
	_ = v // validator might declare schema inline — but we'd need to fetch it.
	// For now, only support explicitly mapped schemas.

	return nil
}

// checkFormatting checks whether content is canonically formatted and updates
// the report accordingly. If --fix is enabled and the file is unformatted,
// the formatted output is written to disk.
//
// When the fixer has already written a new version of the file, content will
// be stale — we re-read from disk in that case (detected by fix notes on the report).
func (c *CLI) checkFormatting(report reporter.Report, content []byte, ft filetype.FileType, path string) reporter.Report {
	// Skip the config file itself — a tool should not format-check its own config.
	if c.configFilePath != "" {
		absPath, err := filepath.Abs(path)
		if err == nil && absPath == c.configFilePath {
			return report
		}
	}

	// Skip if file is format-ignored by external tool config.
	if c.formatIgnores != nil {
		absPath, err := filepath.Abs(path)
		if err == nil && c.formatIgnores.ShouldSkipFormat(absPath, ft.Name) {
			return report
		}
	}

	// If the fixer applied changes and wrote a new file, re-read for formatting.
	if len(report.Notes) > 0 {
		updated, err := os.ReadFile(path)
		if err == nil {
			content = updated
		}
	}

	opts := c.formatOptsFunc(ft.Name, path)
	formatted, err := formatContent(ft.Formatter, content, opts)
	if err != nil {
		// Formatter can't parse the file — shouldn't happen since syntax
		// validation passed, but skip format checking gracefully.
		return report
	}

	if bytes.Equal(content, formatted) {
		// File is already formatted — no change to report.
		return report
	}

	// File is valid but not formatted.
	if c.fix {
		if err := writeFileAtomic(path, formatted); err == nil {
			// Successfully formatted — keep report as pass, add a note.
			report.Notes = append(report.Notes, "fixed: formatting")
			return report
		}
		// Write failed — fall through to report as unformatted.
	}

	report.Status = reporter.StatusUnformatted
	report.Issues = append(report.Issues, reporter.Issue{
		Type:    reporter.IssueTypeFormat,
		Message: "file is not formatted",
	})
	return report
}

// formatContent invokes the formatter, handling ErrSkipped gracefully.
func formatContent(fmter formatter.Formatter, content []byte, opts formatter.Options) ([]byte, error) {
	formatted, err := fmter.Format(content, opts)
	if err != nil {
		var skipped *formatter.ErrSkipped
		if errors.As(err, &skipped) {
			// Formatter explicitly skipped this file — treat as already
			// formatted (no issue to report).
			return content, nil
		}
		return nil, err
	}
	return formatted, nil
}
