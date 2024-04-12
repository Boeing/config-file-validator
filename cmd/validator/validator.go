/*
Validator recursively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently Apple PList XML, CSV, HCL, HOCON, INI, JSON, Properties, TOML, XML, and YAML.
configuration file types are supported.

Cross Platform tool to validate configuration files

Usage:
  validator [flags]
  validator [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  version     Version prints the release version of validator

Flags:
      --depth int                   Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal.
      --exclude-dirs string         Subdirectories to exclude when searching for configuration files
      --exclude-file-types string   A comma separated list of file types to ignore
      --groupby string              Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports
  -h, --help                        help for validator
      --output string               Destination to a file to output results
      --quiet                       If quiet flag is set. It doesn't print any output to stdout.
      --reporter string             Format of the printed report. Options are standard and json (default "standard")

Use "validator [command] --help" for more information about a command.
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
	Quiet            bool
}

var Flags ValidatorConfig

// isFlagSet verifies if a given flag has been set or not
func isFlagSet(flagName string, cmd *cobra.Command) bool {
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
func getReporter(reportType, outputDest string) reporter.Reporter {
	switch reportType {
	case "junit":
		return reporter.NewJunitReporter(outputDest)
	case "json":
		return reporter.NewJSONReporter(outputDest)
	default:
		return reporter.StdoutReporter{}
	}
}

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func getFlags(cmd *cobra.Command, args []string) (ValidatorConfig, error) {
	depth := Flags.Depth
	excludeDirs := Flags.ExcludeDirs
	excludeFileTypes := Flags.ExcludeFileTypes
	output := Flags.Output
	reportType := Flags.ReportType
	groupby := Flags.GroupOutput
	quiet := Flags.Quiet

	searchPaths := make([]string, 0)

	// If search path arg is empty, default is cwd (".")
	// if not, set it to the arg. Supports N number of
	// paths
	if len(args) == 0 {
		searchPaths = append(searchPaths, ".")
	} else {
		searchPaths = append(searchPaths, args...)
	}

	if reportType != "standard" && reportType != "json" && reportType != "junit" {
		fmt.Println("Wrong parameter value for reporter, only supports standard, json or junit")
		_ = cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard, json or junit")
	}

	if reportType == "junit" && groupby != "" {
		fmt.Println("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
		_ = cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
	}

	if isFlagSet("depth", cmd) && depth < 0 {
		fmt.Println("Wrong parameter value for depth, value cannot be negative.")
		_ = cmd.Usage()
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
				_ = cmd.Usage()
				return ValidatorConfig{}, errors.New(
					"Wrong parameter value for groupby, only supports filetype, directory, pass-fail",
				)
			}
			if _, ok := seenValues[groupBy]; ok {
				fmt.Println("Wrong parameter value for groupby, duplicate values are not allowed")
				_ = cmd.Usage()
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
		Quiet:            quiet,
	}

	return config, nil
}

// ExecRoot control all the flow of the program and call cli.Run() that process everything.
func ExecRoot(cmd *cobra.Command, args []string) int {
	validatorConfig, err := getFlags(cmd, args)
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

	if isFlagSet("depth", cmd) {
		fsOpts = append(fsOpts, finder.WithDepth(validatorConfig.Depth))
	}

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(fsOpts...)

	choosenReporter := getReporter(validatorConfig.ReportType, validatorConfig.Output)
	groupby := strings.Split(validatorConfig.GroupOutput, ",")

	// Initialize the CLI
	c := cli.Init(
		cli.WithReporter(choosenReporter),
		cli.WithFinder(fileSystemFinder),
		cli.WithGroupOutput(groupby),
		cli.WithQuiet(validatorConfig.Quiet),
	)

	// Run the config file validation
	exitStatus, err := c.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}

	return exitStatus
}
