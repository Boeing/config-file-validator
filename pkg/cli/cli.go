package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

var GroupOutput string

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

func WithGroupOutput(groupOutput string) CLIOption {
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

	switch {
	case GroupOutput == "filetype":
		reports = GroupByFile(reports)
		c.Reporter.Print(reports)
	case GroupOutput == "pass/fail":
		reports = GroupByPassFail(reports)
		c.Reporter.Print(reports)
	case GroupOutput == "directory":
		reports = GroupByDirectory(reports)
		c.Reporter.Print(reports)
	default:
		c.Reporter.Print(reports)
	}

	if errorFound {
		return 1, nil
	} else {
		return 0, nil
	}
}

// Group Files by File Type
func GroupByFile(reports []reporter.Report) []reporter.Report {

    mapFiles := make(map[string][]reporter.Report)
    reportByFile := []reporter.Report{}

	for _, report := range reports {
        fileType := strings.Split(report.FileName, ".")[1]
        if mapFiles[fileType] == nil {
            mapFiles[fileType] = []reporter.Report{report}
        } else {
            mapFiles[fileType] = append(mapFiles[fileType], report)
        }
	}

    for _, reports := range mapFiles {
        reportByFile = append(reportByFile, reports...)
    }

	return reportByFile
}

func GroupByPassFail(reports []reporter.Report) []reporter.Report {
    mapFiles := make(map[string][]reporter.Report)
    reportByPassOrFail := []reporter.Report{}

	for _, report := range reports {
        if report.IsValid {
            if mapFiles["pass"] == nil {
                mapFiles["pass"] = []reporter.Report{report}
            } else {
                mapFiles["pass"] = append(mapFiles["pass"], report)
            }
        } else {
            if mapFiles["fail"] == nil {
                mapFiles["fail"] = []reporter.Report{report}
            } else {
                mapFiles["fail"] = append(mapFiles["fail"], report)
            }
        }
    }

    for _, reports := range mapFiles {
        reportByPassOrFail = append(reportByPassOrFail, reports...)
    }

	return reportByPassOrFail
}

func GroupByDirectory(reports []reporter.Report) []reporter.Report {
    mapFiles := make(map[string][]reporter.Report)
    reportByDirectory := []reporter.Report{}

    for _, report := range reports {
        directory := strings.Split(report.FilePath, "/")[1]
        if mapFiles[directory] == nil {
            mapFiles[directory] = []reporter.Report{report}
        } else {
            mapFiles[directory] = append(mapFiles[directory], report)
        }
    }

    for _, reports := range mapFiles {
        reportByDirectory = append(reportByDirectory, reports...)
    }

    return reportByDirectory
}
