package cli

import (
	"fmt"
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"io/ioutil"
)

type CLI struct {
	// FileFinder interface to search for the files
	// in the SearchPath
	Finder finder.FileFinder
	// The root directory to begin searching for files
	SearchPath string
	// An array of subdirectories to exclude when searching
	ExcludeDir []string
	// An array of file types that are supported by the validator
	FileTypes []filetype.FileType
	// Reporter interface for outputting the results of the
	// the CLI run
	Reporter reporter.Reporter
}

// Returns the list of supported file types, i.e.
// file types that have a validator
func getFileTypes() []filetype.FileType {
	return filetype.FileTypes
}

// Initialize the CLI object with default values
func Init(searchPath string, excludeDirs []string) CLI {
	// future releases could add support for searchPath to be a url
	// that would require creating a URLFileFinder that implements
	// the Finder interface and passing that as the finder argument
	// when instantiating the CLI
	fsFinder := finder.FileSystemFinder{}

	fileTypes := getFileTypes()

	stdoutReporter := reporter.StdoutReporter{}
	return CLI{
		fsFinder,
		searchPath,
		excludeDirs,
		fileTypes,
		stdoutReporter,
	}
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
	foundFiles, err := c.Finder.Find(c.SearchPath, c.FileTypes, c.ExcludeDir)

	if err != nil {
		return 1, fmt.Errorf("Unable to find files: %v", err)
	}

	for _, fileToValidate := range foundFiles {
		// read it
		fileContent, err := ioutil.ReadFile(fileToValidate.Path)
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

	c.Reporter.Print(reports)

	if errorFound {
		return 1, nil
	} else {
		return 0, nil
	}
}
