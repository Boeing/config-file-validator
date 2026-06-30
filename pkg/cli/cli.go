package cli

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
	"github.com/Boeing/config-file-validator/v3/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v3/pkg/tools"
	"github.com/Boeing/config-file-validator/v3/pkg/validator"
)

// CLI is the main entry point for running config file validation and formatting.
// Use Init with Option functions to configure, then call Run (check) or Format.
type CLI struct {
	finder        finder.FileFinder
	reporters     []reporter.Reporter
	groupOutput   []string
	quiet         bool
	requireSchema bool
	noSchema      bool
	schemaMap     map[string]string
	schemaStore   *schemastore.Store
	stdinData     []byte
	stdinFileType filetype.FileType
	errorFound    bool
	// fix enables writing formatted output back to disk.
	// When false, Format reports issues but does not write.
	fix bool
	// diff enables unified diff output mode.
	// When true, Format prints diffs to stdout instead of the normal report.
	// Mutually exclusive with fix.
	diff bool
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

func WithSchemaMap(m map[string]string) Option {
	return func(c *CLI) {
		c.schemaMap = m
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

		report := c.validate(content, f.FileType, f.Name, f.Path)
		if report.HasErrors() {
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
	return true, nil, nil
}

func (c *CLI) lookupSchemaMap(filePath string) (string, bool) {
	if len(c.schemaMap) == 0 {
		return "", false
	}
	baseName := filepath.Base(filePath)
	for pattern, schemaPath := range c.schemaMap {
		if !tools.IsGlobPattern(pattern) {
			if pattern == baseName {
				return schemaPath, true
			}
			continue
		}
		matched, err := doublestar.PathMatch(pattern, filePath)
		if err == nil && matched {
			return schemaPath, true
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
	return "file://" + absSchema, nil
}

func (c *CLI) printReports(reports []reporter.Report) error {
	if len(c.groupOutput) > 0 && c.groupOutput[0] != "" {
		return c.printGroup(reports)
	}

	for _, reporterObj := range c.reporters {
		err := reporterObj.Print(reports)
		if err != nil {
			fmt.Println("failed to report:", err)
			c.errorFound = true
		}
	}

	return nil
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
