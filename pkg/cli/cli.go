package cli

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/bmatcuk/doublestar/v4"

	"github.com/Boeing/config-file-validator/v2/pkg/finder"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
	"github.com/Boeing/config-file-validator/v2/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v2/pkg/validator"
)

var (
	GroupOutput   []string
	Quiet         bool
	RequireSchema bool
	NoSchema      bool
	SchemaMap     map[string]string
	SchemaStore   *schemastore.Store
	errorFound    bool
)

type CLI struct {
	Finder    finder.FileFinder
	Reporters []reporter.Reporter
}

type Option func(*CLI)

func WithFinder(f finder.FileFinder) Option {
	return func(c *CLI) {
		c.Finder = f
	}
}

func WithReporters(r ...reporter.Reporter) Option {
	return func(c *CLI) {
		c.Reporters = r
	}
}

func WithGroupOutput(groupOutput []string) Option {
	return func(_ *CLI) {
		GroupOutput = groupOutput
	}
}

func WithQuiet(quiet bool) Option {
	return func(_ *CLI) {
		Quiet = quiet
	}
}

func WithRequireSchema(require bool) Option {
	return func(_ *CLI) {
		RequireSchema = require
	}
}

func WithNoSchema(noSchema bool) Option {
	return func(_ *CLI) {
		NoSchema = noSchema
	}
}

func WithSchemaMap(m map[string]string) Option {
	return func(_ *CLI) {
		SchemaMap = m
	}
}

func WithSchemaStore(s *schemastore.Store) Option {
	return func(_ *CLI) {
		SchemaStore = s
	}
}

func Init(opts ...Option) *CLI {
	defaultFsFinder := finder.FileSystemFinderInit()
	defaultReporter := reporter.NewStdoutReporter("")

	GroupOutput = nil
	Quiet = false
	RequireSchema = false
	NoSchema = false
	SchemaMap = nil
	SchemaStore = nil

	cli := &CLI{
		defaultFsFinder,
		[]reporter.Reporter{defaultReporter},
	}

	for _, opt := range opts {
		opt(cli)
	}

	return cli
}

func (c CLI) Run() (int, error) {
	errorFound = false

	var reports []reporter.Report
	foundFiles, err := c.Finder.Find()
	if err != nil {
		return 1, fmt.Errorf("unable to find files: %w", err)
	}

	for _, fileToValidate := range foundFiles {
		fileContent, err := os.ReadFile(fileToValidate.Path)
		if err != nil {
			return 1, fmt.Errorf("unable to read file: %w", err)
		}

		isValid, syntaxErr := fileToValidate.FileType.Validator.ValidateSyntax(fileContent)

		var schemaErr error
		if isValid {
			isValid, schemaErr = validateSchema(fileToValidate.FileType.Validator, fileContent, fileToValidate.Path)
		}

		err = syntaxErr
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

		var validationErrors []string
		var se *validator.SchemaErrors
		if errors.As(err, &se) {
			for _, e := range se.Errors() {
				validationErrors = append(validationErrors, "schema: "+e)
			}
		} else if err != nil {
			validationErrors = []string{"syntax: " + err.Error()}
		}

		report := reporter.Report{
			FileName:         fileToValidate.Name,
			FilePath:         fileToValidate.Path,
			IsValid:          isValid,
			ValidationError:  err,
			ValidationErrors: validationErrors,
			ErrorType:        errorType,
			IsQuiet:          Quiet,
			StartLine:        line,
			StartColumn:      col,
		}
		if !isValid {
			errorFound = true
		}
		reports = append(reports, report)
	}

	err = c.printReports(reports)
	if err != nil {
		return 1, err
	}

	if errorFound {
		return 1, nil
	}

	return 0, nil
}

func validateSchema(v validator.Validator, content []byte, filePath string) (bool, error) {
	if NoSchema {
		return true, nil
	}

	// 1. Try document-declared schema
	sv, hasSV := v.(validator.SchemaValidator)
	if hasSV {
		valid, err := sv.ValidateSchema(content, filePath)
		if !errors.Is(err, validator.ErrNoSchema) {
			return valid, err
		}
	}

	// 2. Try --schema-map
	if schemaPath, ok := lookupSchemaMap(filePath); ok {
		return validateWithExternal(v, content, schemaPath)
	}

	// 3. Try --schemastore
	if SchemaStore != nil {
		if schemaPath, ok := SchemaStore.Lookup(filePath); ok {
			return validateWithExternal(v, content, schemaPath)
		}
	}

	// 4. No schema found
	if hasSV && RequireSchema {
		return false, validator.ErrNoSchema
	}
	return true, nil
}

func lookupSchemaMap(filePath string) (string, bool) {
	if len(SchemaMap) == 0 {
		return "", false
	}
	baseName := filepath.Base(filePath)
	for pattern, schemaPath := range SchemaMap {
		if !isGlobPattern(pattern) {
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

func isGlobPattern(s string) bool {
	for _, c := range s {
		if c == '*' || c == '?' || c == '[' {
			return true
		}
	}
	return false
}

func validateWithExternal(v validator.Validator, content []byte, schemaPath string) (bool, error) {
	// XML uses XSD validation, not JSON Schema
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

	absSchema, err := filepath.Abs(schemaPath)
	if err != nil {
		return false, fmt.Errorf("resolving schema path: %w", err)
	}

	docJSON, err := jm.MarshalToJSON(content)
	if err != nil {
		return false, err
	}

	return validator.JSONSchemaValidate("file://"+absSchema, docJSON)
}

func (c CLI) printReports(reports []reporter.Report) error {
	if len(GroupOutput) == 1 && GroupOutput[0] != "" {
		return c.printGroupSingle(reports)
	} else if len(GroupOutput) == 2 {
		return c.printGroupDouble(reports)
	} else if len(GroupOutput) == 3 {
		return c.printGroupTriple(reports)
	}

	for _, reporterObj := range c.Reporters {
		err := reporterObj.Print(reports)
		if err != nil {
			fmt.Println("failed to report:", err)
			errorFound = true
		}
	}

	return nil
}

func (c CLI) printGroupSingle(reports []reporter.Report) error {
	reportGroup, err := GroupBySingle(reports, GroupOutput[0])
	if err != nil {
		return fmt.Errorf("unable to group by single value: %w", err)
	}

	for _, reporterObj := range c.Reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintSingleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintSingleGroupStdout(reportGroup)
}

func (c CLI) printGroupDouble(reports []reporter.Report) error {
	reportGroup, err := GroupByDouble(reports, GroupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by double value: %w", err)
	}

	for _, reporterObj := range c.Reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintDoubleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintDoubleGroupStdout(reportGroup)
}

func (c CLI) printGroupTriple(reports []reporter.Report) error {
	reportGroup, err := GroupByTriple(reports, GroupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by triple value: %w", err)
	}

	for _, reporterObj := range c.Reporters {
		if _, ok := reporterObj.(*reporter.JSONReporter); ok {
			return reporter.PrintTripleGroupJSON(reportGroup)
		}
	}

	return reporter.PrintTripleGroupStdout(reportGroup)
}
