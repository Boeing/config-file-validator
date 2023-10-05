/*
Validator recusively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently json, yaml, toml, xml and ini configuration file types are supported.

Usage:

    validator [OPTIONS] [search_path]

The flags are:
    -exclude-dirs string
        Subdirectories to exclude when searching for configuration files.
    -reporter string
        Format of printed report. Currently supports standard, JSON.
    -exclude-file-types string
        A comma separated list of file types to ignore.
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

type validatorConfig struct {
	searchPaths      []string
	excludeDirs      *string
	excludeFileTypes *string
	reportType       *string
}

// Custom Usage function to cover
func validatorUsage() {
	fmt.Printf("Usage: validator [OPTIONS] [<search_path>...]\n\n")
	fmt.Printf("positional arguments:\n")
	fmt.Printf(
		"    search_path: The search path on the filesystem for configuration files. " +
			"Defaults to the current working directory if no search_path provided\n\n")
	fmt.Printf("optional flags:\n")
	flag.PrintDefaults()
}

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func getFlags() (validatorConfig, error) {
	flag.Usage = validatorUsage
	excludeDirsPtr := flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	reportTypePtr := flag.String("reporter", "standard", "Format of the printed report. Options are standard and json")
	excludeFileTypesPtr := flag.String("exclude-file-types", "", "A comma separated list of file types to ignore")
	flag.Parse()

	searchPaths := make([]string, 0)

	// If search path arg is empty, set it to the cwd
	// if not, set it to the arg. Supports n number of
	// paths
	if flag.NArg() == 0 {
		searchPaths = append(searchPaths, ".")
	} else {
		searchPaths = append(searchPaths, flag.Args()...)
	}

	if *reportTypePtr != "standard" && *reportTypePtr != "json" {
		fmt.Println("Wrong parameter value for reporter, only supports standard or json")
		flag.Usage()
		return validatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard or json")
	}

	config := validatorConfig{
		searchPaths,
		excludeDirsPtr,
		excludeFileTypesPtr,
		reportTypePtr,
	}

	return config, nil
}

// Return the reporter associated with the
// reportType string
func getReporter(reportType *string) reporter.Reporter {
	switch *reportType {
	case "json":
		return reporter.JsonReporter{}
	default:
		return reporter.StdoutReporter{}
	}
}

func mainInit() int {
	validatorConfig, err := getFlags()
	if err != nil {
		return 1
	}

	searchPaths := validatorConfig.searchPaths
	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(*validatorConfig.excludeDirs, ",")
	reporter := getReporter(validatorConfig.reportType)
	excludeFileTypes := strings.Split(*validatorConfig.excludeFileTypes, ",")

	// Initialize a file system finder for each searchPath
	finders := make([]finder.FileFinder, len(searchPaths))
	for idx, searchPath := range searchPaths {
		fileSystemFinder := finder.FileSystemFinderInit(
			finder.WithPathRoot(searchPath),
			finder.WithExcludeDirs(excludeDirs),
			finder.WithExcludeFileTypes(excludeFileTypes),
		)
		finders[idx] = fileSystemFinder
	}

	// Initialize a composite file finder with all the file system finders
	compositeFinder := finder.NewCompositeFileFinder(finders)

	// Initialize the CLI
	cli := cli.Init(
		cli.WithReporter(reporter),
		cli.WithFinder(compositeFinder),
	)

	// Run the config file validation
	exitStatus, err := cli.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}

	return exitStatus
}

func main() {
	os.Exit(mainInit())
}
