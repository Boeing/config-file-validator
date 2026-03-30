package cli

import (
	"errors"
	"fmt"
	"os"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

var (
	GroupOutput   []string
	Quiet         bool
	RequireSchema bool
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

func Init(opts ...Option) *CLI {
	defaultFsFinder := finder.FileSystemFinderInit()
	defaultReporter := reporter.NewStdoutReporter("")

	GroupOutput = nil
	Quiet = false
	RequireSchema = false

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

		isValid, err := fileToValidate.FileType.Validator.ValidateSyntax(fileContent)

		if isValid {
			isValid, err = validateSchema(fileToValidate.FileType.Validator, fileContent, fileToValidate.Path)
		}

		report := reporter.Report{
			FileName:        fileToValidate.Name,
			FilePath:        fileToValidate.Path,
			IsValid:         isValid,
			ValidationError: err,
			IsQuiet:         Quiet,
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
	sv, ok := v.(validator.SchemaValidator)
	if !ok {
		return true, nil
	}

	valid, err := sv.ValidateSchema(content, filePath)
	if errors.Is(err, validator.ErrNoSchema) {
		if RequireSchema {
			return false, err
		}
		return true, nil
	}

	return valid, err
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
