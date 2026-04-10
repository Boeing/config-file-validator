package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/finder"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
	"github.com/Boeing/config-file-validator/v2/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v2/pkg/tools"
	"github.com/Boeing/config-file-validator/v2/pkg/validator"
)

// CLI is the main entry point for running config file validation.
// Use Init with Option functions to configure, then call Run.
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
			return 2, fmt.Errorf("unable to read file: %w", err)
		}

		report := c.validate(content, f.FileType, f.Name, f.Path)
		if !report.IsValid {
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
	if isValid {
		isValid, schemaErr = c.validateSchema(ft.Validator, content, path)
	}

	err := syntaxErr
	errorType := ""
	if syntaxErr != nil {
		errorType = "syntax"
	}
	if schemaErr != nil {
		err = schemaErr
		errorType = "schema"
	}

	var line, col int
	var ve *validator.ValidationError
	if errors.As(err, &ve) {
		line = ve.Line
		col = ve.Column
	}

	validationErrors := formatErrors(err)
	notes := checkJSONCFallback(syntaxErr, ft, content, name)

	return reporter.Report{
		FileName:         name,
		FilePath:         path,
		IsValid:          isValid,
		ValidationError:  err,
		ValidationErrors: validationErrors,
		Notes:            notes,
		ErrorType:        errorType,
		IsQuiet:          c.quiet,
		StartLine:        line,
		StartColumn:      col,
	}
}

// runSingle validates a single piece of content (used for stdin mode).
func (c *CLI) runSingle(content []byte, ft filetype.FileType, name string) (int, error) {
	report := c.validate(content, ft, name, name)

	if err := c.printReports([]reporter.Report{report}); err != nil {
		return 2, err
	}

	if !report.IsValid {
		return 1, nil
	}
	return 0, nil
}

func formatErrors(err error) []string {
	if err == nil {
		return nil
	}
	var se *validator.SchemaErrors
	if errors.As(err, &se) {
		var errs []string
		for _, e := range se.Errors() {
			errs = append(errs, "schema: "+e)
		}
		return errs
	}
	return []string{"syntax: " + err.Error()}
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

func (c *CLI) validateSchema(v validator.Validator, content []byte, filePath string) (bool, error) {
	if c.noSchema {
		return true, nil
	}

	sv, hasSV := v.(validator.SchemaValidator)
	if hasSV {
		valid, err := sv.ValidateSchema(content, filePath)
		if !errors.Is(err, validator.ErrNoSchema) {
			return valid, err
		}
	}

	if schemaPath, ok := c.lookupSchemaMap(filePath); ok {
		return validateWithExternal(v, content, schemaPath)
	}

	if c.schemaStore != nil {
		if schemaPath, ok := c.schemaStore.Resolve(filePath); ok {
			return validateWithExternal(v, content, schemaPath)
		}
	}

	if hasSV && c.requireSchema {
		return false, validator.ErrNoSchema
	}
	return true, nil
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

func validateWithExternal(v validator.Validator, content []byte, schemaPath string) (bool, error) {
	if _, ok := v.(validator.XMLSchemaValidator); ok {
		absSchema, err := filepath.Abs(schemaPath)
		if err != nil {
			return false, fmt.Errorf("resolving schema path: %w", err)
		}
		return validator.ValidateXSD(content, absSchema)
	}

	jm, ok := v.(validator.JSONMarshaler)
	if !ok {
		return true, nil
	}

	schemaURL, err := toSchemaURL(schemaPath)
	if err != nil {
		return false, err
	}

	docJSON, err := jm.MarshalToJSON(content)
	if err != nil {
		return false, err
	}

	return validator.JSONSchemaValidate(schemaURL, docJSON)
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
	if len(c.groupOutput) == 1 && c.groupOutput[0] != "" {
		return c.printGroupSingle(reports)
	} else if len(c.groupOutput) == 2 {
		return c.printGroupDouble(reports)
	} else if len(c.groupOutput) == 3 {
		return c.printGroupTriple(reports)
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

func (c *CLI) printGroupSingle(reports []reporter.Report) error {
	reportGroup, err := GroupBySingle(reports, c.groupOutput[0])
	if err != nil {
		return fmt.Errorf("unable to group by single value: %w", err)
	}

	for _, reporterObj := range c.reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintSingleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintSingleGroupStdout(reportGroup)
}

func (c *CLI) printGroupDouble(reports []reporter.Report) error {
	reportGroup, err := GroupByDouble(reports, c.groupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by double value: %w", err)
	}

	for _, reporterObj := range c.reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintDoubleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintDoubleGroupStdout(reportGroup)
}

func (c *CLI) printGroupTriple(reports []reporter.Report) error {
	reportGroup, err := GroupByTriple(reports, c.groupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by triple value: %w", err)
	}

	for _, reporterObj := range c.reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintTripleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintTripleGroupStdout(reportGroup)
}
