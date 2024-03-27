/*
Validator recusively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently Apple PList XML, CSV, HCL, HOCON, INI, JSON, Properties, TOML, XML, and YAML.
configuration file types are supported.

Usage: [OPTIONS] [<search_path>...]

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

package cmd

import (
	"errors"
	"fmt"
	"log"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"github.com/spf13/cobra"
)

// ValidatorConfig holds all flag possible to be setted
type ValidatorConfig struct {
	SearchPaths      []string
	Depth            int
	ExcludeDirs      string
	ExcludeFileTypes string
	Output           string
	ReportType       string
	GroupOutput      string
	SearchPath       string
}

var Flags ValidatorConfig

// IsFlagSet verifies if a given flag has been set or not
func IsFlagSet(flagName string, cmd *cobra.Command) bool {
	return cmd.Flags().Lookup(flagName).Changed
}

// CleanString takes a command string and a split string
// and returns a cleaned string
func CleanString(str string) string {
	str = strings.ToLower(str)
	str = strings.TrimSpace(str)
	return str
}

// Return the reporter associated with the
// reportType string
func GetReporter(reportType, outputDest string) reporter.Reporter {
	switch reportType {
	case "junit":
		return reporter.NewJunitReporter(outputDest)
	case "json":
		return reporter.NewJsonReporter(outputDest)
	default:
		return reporter.StdoutReporter{}
	}
}

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func GetFlags(cmd *cobra.Command) (ValidatorConfig, error) {
	depth := Flags.Depth
	excludeDirs := Flags.ExcludeDirs
	excludeFileTypes := Flags.ExcludeFileTypes
	output := Flags.Output
	reportType := Flags.ReportType
	groupby := Flags.GroupOutput

	searchPaths := make([]string, 0)

	// If search path arg is empty, default is cwd (".")
	// if not, set it to the arg. Supports N number of paths
	searchPaths = append(searchPaths, Flags.SearchPath)

	if reportType != "standard" && reportType != "json" && reportType != "junit" {
		fmt.Println("Wrong parameter value for reporter, only supports standard, json or junit")
		cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard, json or junit")
	}

	if reportType == "junit" && groupby != "" {
		fmt.Println("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
		cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
	}

	if IsFlagSet("depth", cmd) && depth < 0 {
		fmt.Println("Wrong parameter value for depth, value cannot be negative.")
		cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for depth, value cannot be negative")
	}

	if groupby != "" {
		groupByCleanString := CleanString(groupby)
		groupByUserInput := strings.Split(groupByCleanString, ",")
		groupByAllowedValues := []string{"filetype", "directory", "pass-fail"}
		seenValues := make(map[string]bool)

		// Check that the groupby values are valid and not duplicates
		for _, groupBy := range groupByUserInput {
			if !slices.Contains(groupByAllowedValues, groupBy) {
				fmt.Println("Wrong parameter value for groupby, only supports filetype, directory, pass-fail")
				cmd.Usage()
				return ValidatorConfig{}, errors.New(
					"Wrong parameter value for groupby, only supports filetype, directory, pass-fail",
				)
			}
			if _, ok := seenValues[groupBy]; ok {
				fmt.Println("Wrong parameter value for groupby, duplicate values are not allowed")
				cmd.Usage()
				return ValidatorConfig{}, errors.New("Wrong parameter value for groupby, duplicate values are not allowed")
			}
			seenValues[groupBy] = true
		}
	}

	config := ValidatorConfig{
		SearchPaths:      searchPaths,
		ExcludeDirs:      excludeDirs,
		ExcludeFileTypes: excludeFileTypes,
		ReportType:       reportType,
		Depth:            depth,
		Output:           output,
		GroupOutput:      groupby,
	}

	return config, nil
}

// ExecRoot control all the flow of the program and call cli.Run() that process everything.
func ExecRoot(cmd *cobra.Command) int {
	validatorConfig, err := GetFlags(cmd)
	if err != nil {
		return 1
	}

	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(validatorConfig.ExcludeDirs, ",")
	excludeFileTypes := strings.Split(validatorConfig.ExcludeFileTypes, ",")

	fsOpts := []finder.FSFinderOptions{
		finder.WithPathRoots(validatorConfig.SearchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes),
	}

	if IsFlagSet("depth", cmd) {
		fsOpts = append(fsOpts, finder.WithDepth(validatorConfig.Depth))
	}

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(fsOpts...)

	reporter := GetReporter(validatorConfig.ReportType, validatorConfig.Output)
	groupby := strings.Split(validatorConfig.GroupOutput, ",")

	// Initialize the CLI
	cli := cli.Init(
		cli.WithReporter(reporter),
		cli.WithFinder(fileSystemFinder),
		cli.WithGroupOutput(groupby),
	)

	// Run the config file validation
	exitStatus, err := cli.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}

	return exitStatus
}
