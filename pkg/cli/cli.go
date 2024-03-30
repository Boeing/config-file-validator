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
	// Reporter interface for outputting the results of the
	// the CLI run
	Reporter reporter.Reporter
}

// Implement the go options pattern to be able to
// set options to the CLI struct using functional
// programming
type CLIOption func(*CLI)

// Set the CLI Finder
func WithFinder(finder finder.FileFinder) CLIOption {
	return func(c *CLI) {
		c.Finder = finder
	}
}

// Set the reporter type
func WithReporter(reporter reporter.Reporter) CLIOption {
	return func(c *CLI) {
		c.Reporter = reporter
	}
}

func WithGroupOutput(groupOutput []string) CLIOption {
	return func(c *CLI) {
		GroupOutput = groupOutput
	}
}

func WithQuiet(quiet bool) CLIOption {
	return func(c *CLI) {
		Quiet = quiet
	}
}

// Initialize the CLI object
func Init(opts ...CLIOption) *CLI {
	defaultFsFinder := finder.FileSystemFinderInit()
	defaultReporter := reporter.StdoutReporter{}

	cli := &CLI{
		defaultFsFinder,
		defaultReporter,
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
// - Outputs the results using the Reporter
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
	} else {
		return 0, nil
	}
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
	} else {
		err := c.Reporter.Print(reports)
		if err != nil {
			fmt.Println("failed to report:", err)
			errorFound = true
		}

		return nil
	}
}

func (c CLI) printGroupSingle(reports []reporter.Report) error {
	reportGroup, err := GroupBySingle(reports, GroupOutput[0])
	if err != nil {
		return fmt.Errorf("unable to group by single value: %w", err)
	}

	// Check reporter type to determine how to print
	if _, ok := c.Reporter.(reporter.JsonReporter); ok {
		reporter.PrintSingleGroupJson(reportGroup)
	} else {
		reporter.PrintSingleGroupStdout(reportGroup)
	}

	return nil
}

func (c CLI) printGroupDouble(reports []reporter.Report) error {
	reportGroup, err := GroupByDouble(reports, GroupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by double value: %w", err)
	}

	// Check reporter type to determine how to print
	if _, ok := c.Reporter.(reporter.JsonReporter); ok {
		reporter.PrintDoubleGroupJson(reportGroup)
	} else {
		reporter.PrintDoubleGroupStdout(reportGroup)
	}

	return nil
}

func (c CLI) printGroupTriple(reports []reporter.Report) error {
	reportGroup, err := GroupByTriple(reports, GroupOutput)
	if err != nil {
		return fmt.Errorf("unable to group by triple value: %w", err)
	}

	if _, ok := c.Reporter.(reporter.JsonReporter); ok {
		reporter.PrintTripleGroupJson(reportGroup)
	} else {
		reporter.PrintTripleGroupStdout(reportGroup)
	}

	return nil
}
