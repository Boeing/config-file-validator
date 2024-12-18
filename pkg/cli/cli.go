package cli

import (
	"fmt"
	"os"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// GroupOutput is a global variable that is used to
// store the group by options that the user specifies
var (
	GroupOutput []string
	Quiet       bool
	errorFound  bool
)

type CLI struct {
	// FileFinder interface to search for the files
	// in the SearchPath
	Finder finder.FileFinder
	// Reporter interface for outputting the results of
	// the CLI run
	Reporters []reporter.Reporter
}

// Implement the go options pattern to be able to
// set options to the CLI struct using functional
// programming
type Option func(*CLI)

// Set the CLI Finder
func WithFinder(f finder.FileFinder) Option {
	return func(c *CLI) {
		c.Finder = f
	}
}

// Set the reporter types
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

// Initialize the CLI object
func Init(opts ...Option) *CLI {
	defaultFsFinder := finder.FileSystemFinderInit()
	defaultReporter := reporter.NewStdoutReporter("")

	cli := &CLI{
		defaultFsFinder,
		[]reporter.Reporter{defaultReporter},
	}

	for _, opt := range opts {
		opt(cli)
	}

	return cli
}

// The Run method performs the following actions:
// - Finds the calls the Find method from the Finder interface to
// return a list of files
// - Reads each file that was found
// - Calls the Validate method from the Validator interface to validate the file
// - Outputs the results using the Reporters
func (c CLI) Run() (int, error) {
	errorFound = false
	var reports []reporter.Report
	foundFiles, err := c.Finder.Find()
	if err != nil {
		return 1, fmt.Errorf("Unable to find files: %w", err)
	}

	for _, fileToValidate := range foundFiles {
		// read it
		fileContent, err := os.ReadFile(fileToValidate.Path)
		if err != nil {
			return 1, fmt.Errorf("unable to read file: %w", err)
		}

		isValid, err := fileToValidate.FileType.Validator.Validate(fileContent)
		if !isValid {
			errorFound = true
		}
		report := reporter.Report{
			FileName:        fileToValidate.Name,
			FilePath:        fileToValidate.Path,
			IsValid:         isValid,
			ValidationError: err,
			IsQuiet:         Quiet,
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

// printReports prints the reports based on the specified grouping and reporter type.
// It returns any error encountered during the printing process.
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

	// Check reporter type to determine how to print
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

	// Check reporter type to determine how to print
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
