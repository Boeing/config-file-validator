package cli

import (
	"fmt"
	"os"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// GroupOutput is a global variable that is used to
// store the group by options that the user specifies
var GroupOutput []string

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
	errorFound := false
	var reports []reporter.Report
	foundFiles, err := c.Finder.Find()

	if err != nil {
		return 1, fmt.Errorf("Unable to find files: %v", err)
	}

	for _, fileToValidate := range foundFiles {
		// read it
		fileContent, err := os.ReadFile(fileToValidate.Path)
		if err != nil {
			return 1, fmt.Errorf("unable to read file: %v", err)
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
		}
		reports = append(reports, report)
	}

	// Group the output if the user specified a group by option
	// Length is equal to one when empty as it contains an empty string
	if len(GroupOutput) == 1 && GroupOutput[0] != "" {
		reportGroup, err := GroupBySingle(reports, GroupOutput[0])
		if err != nil {
			return 1, fmt.Errorf("unable to group by single value: %v", err)
		}
		c.Reporter.PrintSingleGroup(reportGroup, GroupOutput[0])
	} else if len(GroupOutput) == 2 {
		reportGroup, err := GroupByDouble(reports, GroupOutput)
		if err != nil {
			return 1, fmt.Errorf("unable to group by double value: %v", err)
		}
		c.Reporter.PrintDoubleGroup(reportGroup, GroupOutput)
	} else if len(GroupOutput) == 3 {
		reportGroup, err := GroupByTriple(reports, GroupOutput)
		if err != nil {
			return 1, fmt.Errorf("unable to group by triple value: %v", err)
		}
		c.Reporter.PrintTripleGroup(reportGroup, GroupOutput)
	} else {
		c.Reporter.Print(reports)
	}

	if errorFound {
		return 1, nil
	} else {
		return 0, nil
	}
}
