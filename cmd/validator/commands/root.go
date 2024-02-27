package cmd

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"os"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"github.com/spf13/cobra"
)

//type ValidatorConfig struct {
//	searchPaths      []string
//	excludeDirs      *string
//	excludeFileTypes *string
//	reportType       *string
//	depth            *int
//	versionQuery     *bool
//	output           *string
//	groupby      *string
//}

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
func getFlags(cmd *cobra.Command) (ValidatorConfig, error) {
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

	if isFlagSet("depth", cmd) && depth < 0 {
		fmt.Println("Wrong parameter value for depth, value cannot be negative.")
		cmd.Usage()
		return ValidatorConfig{}, errors.New("Wrong parameter value for depth, value cannot be negative")
	}

	if groupby != "" {
		groupByCleanString := cleanString(groupby)
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

// isFlagSet verifies if a given flag has been set or not
func isFlagSet(flagName string, cmd *cobra.Command) bool {
	return cmd.Flags().Lookup(flagName).Changed
}

// Return the reporter associated with the
// reportType string
func getReporter(reportType, outputDest string) reporter.Reporter {
	switch reportType {
	case "junit":
		return reporter.NewJunitReporter(outputDest)
	case "json":
		return reporter.NewJsonReporter(outputDest)
	default:
		return reporter.StdoutReporter{}
	}
}

// cleanString takes a command string and a split string
// and returns a cleaned string
func cleanString(str string) string {
	str = strings.ToLower(str)
	str = strings.TrimSpace(str)
	return str
}

func execRoot(cmd *cobra.Command) int {
	validatorConfig, err := getFlags(cmd)
	if err != nil {
		return 1
	}

	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(validatorConfig.ExcludeDirs, ",")
	reporter := getReporter(validatorConfig.ReportType, validatorConfig.Output)
	excludeFileTypes := strings.Split(validatorConfig.ExcludeFileTypes, ",")
	groupby := strings.Split(validatorConfig.GroupOutput, ",")
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

var rootCmd = &cobra.Command{
	Use:   "validator",
	Short: "Cross Platform tool to validate configuration files",
	Run: func(cmd *cobra.Command, args []string) {
		os.Exit(execRoot(cmd))
	},
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
