/*
Validator recusively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently Apple PList XML, CSV, HCL, HOCON, INI, JSON, Properties, TOML, XML, and YAML.
configuration file types are supported.

Usage: validator [OPTIONS] [<search_path>...]

positional arguments:
    search_path: The search path on the filesystem for configuration files. Defaults to the current working directory if no search_path provided. Multiple search paths can be declared separated by a space.

optional flags:
  -depth int
    	Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal
  -exclude-dirs string
    	Subdirectories to exclude when searching for configuration files
  -exclude-file-types string
    	A comma separated list of file types to ignore
  -output
     	Destination of a file to outputting results
  -reporter string
    	Format of the printed report. Options are standard and json (default "standard")
  -version
    	Version prints the release version of validator
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	configfilevalidator "github.com/Boeing/config-file-validator"
	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

type validatorConfig struct {
	searchPaths      []string
	excludeDirs      *string
	excludeFileTypes *string
	reportType       *string
	depth            *int
	versionQuery     *bool
	output           *string
	groupOutput      *string
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
	depthPtr := flag.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
	excludeDirsPtr := flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	excludeFileTypesPtr := flag.String("exclude-file-types", "", "A comma separated list of file types to ignore")
	outputPtr := flag.String("output", "", "Destination to a file to output results")
	reportTypePtr := flag.String("reporter", "standard", "Format of the printed report. Options are standard and json")
	versionPtr := flag.Bool("version", false, "Version prints the release version of validator")
	groupOutputPtr := flag.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
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

	if *reportTypePtr != "standard" && *reportTypePtr != "json" && *reportTypePtr != "junit" {
		fmt.Println("Wrong parameter value for reporter, only supports standard, json or junit")
		flag.Usage()
		return validatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard, json or junit")
	}

	if *reportTypePtr == "junit" && *groupOutputPtr != "" {
		fmt.Println("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
		flag.Usage()
		return validatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
	}

	if depthPtr != nil && isFlagSet("depth") && *depthPtr < 0 {
		fmt.Println("Wrong parameter value for depth, value cannot be negative.")
		flag.Usage()
		return validatorConfig{}, errors.New("Wrong parameter value for depth, value cannot be negative")
	}

	groupByCleanString := cleanString("groupby")
	groupByUserInput := strings.Split(groupByCleanString, ",")
	groupByAllowedValues := []string{"filetype", "directory", "pass-fail"}
	seenValues := make(map[string]bool)

	// Check that the groupby values are valid and not duplicates
	if groupOutputPtr != nil && isFlagSet("groupby") {
		for _, groupBy := range groupByUserInput {
			if !slices.Contains(groupByAllowedValues, groupBy) {
				fmt.Println("Wrong parameter value for groupby, only supports filetype, directory, pass-fail")
				flag.Usage()
				return validatorConfig{}, errors.New("Wrong parameter value for groupby, only supports filetype, directory, pass-fail")
			}
			if _, ok := seenValues[groupBy]; ok {
				fmt.Println("Wrong parameter value for groupby, duplicate values are not allowed")
				flag.Usage()
				return validatorConfig{}, errors.New("Wrong parameter value for groupby, duplicate values are not allowed")
			}
			seenValues[groupBy] = true
		}
	}

	config := validatorConfig{
		searchPaths,
		excludeDirsPtr,
		excludeFileTypesPtr,
		reportTypePtr,
		depthPtr,
		versionPtr,
		outputPtr,
		groupOutputPtr,
	}

	return config, nil
}

// isFlagSet verifies if a given flag has been set or not
func isFlagSet(flagName string) bool {
	var isSet bool

	flag.Visit(func(f *flag.Flag) {
		if f.Name == flagName {
			isSet = true
		}
	})

	return isSet
}

// Return the reporter associated with the
// reportType string
func getReporter(reportType, outputDest *string) reporter.Reporter {
	switch *reportType {
	case "junit":
		return reporter.NewJunitReporter(*outputDest)
	case "json":
		return reporter.NewJsonReporter(*outputDest)
	case "sarif":
		return reporter.NewSarifReporter(*outputDest)
	default:
		return reporter.StdoutReporter{}
	}
}

// cleanString takes a command string and a split string
// and returns a cleaned string
func cleanString(command string) string {
	cleanedString := flag.Lookup(command).Value.String()
	cleanedString = strings.ToLower(cleanedString)
	cleanedString = strings.TrimSpace(cleanedString)

	return cleanedString
}

func mainInit() int {
	validatorConfig, err := getFlags()
	if err != nil {
		return 1
	}

	if *validatorConfig.versionQuery {
		fmt.Println(configfilevalidator.GetVersion())
		return 0
	}

	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(*validatorConfig.excludeDirs, ",")
	reporter := getReporter(validatorConfig.reportType, validatorConfig.output)
	excludeFileTypes := strings.Split(*validatorConfig.excludeFileTypes, ",")
	groupOutput := strings.Split(*validatorConfig.groupOutput, ",")
	fsOpts := []finder.FSFinderOptions{finder.WithPathRoots(validatorConfig.searchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes)}

	if validatorConfig.depth != nil && isFlagSet("depth") {
		fsOpts = append(fsOpts, finder.WithDepth(*validatorConfig.depth))
	}

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(fsOpts...)

	// Initialize the CLI
	cli := cli.Init(
		cli.WithReporter(reporter),
		cli.WithFinder(fileSystemFinder),
		cli.WithGroupOutput(groupOutput),
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
