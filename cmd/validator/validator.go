/*
Validator recursively scans a directory to search for configuration files and
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
  -reporter string
		A colon-separated string of report formats with optional output file paths.
		Usage: --reporter <format>:<optional_file_path>
		Multiple reporters can be specified: --reporter json:file_path.json --reporter junit:another_file_path.xml
		Omit the file path to output to stdout: --reporter json or explicitly specify stdout using "-": --reporter json:-
		Supported formats: standard, json, junit (default: "standard")
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
	reportType       map[string]string
	depth            *int
	versionQuery     *bool
	groupOutput      *string
	quiet            *bool
}

type reporterFlags []string

func (rf *reporterFlags) String() string {
	return fmt.Sprintf("%v", *rf)
}

func (rf *reporterFlags) Set(value string) error {
	*rf = append(*rf, value)
	return nil
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
	reporterConfigFlags := reporterFlags{}

	depthPtr := flag.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
	excludeDirsPtr := flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
	excludeFileTypesPtr := flag.String("exclude-file-types", "", "A comma separated list of file types to ignore")
	versionPtr := flag.Bool("version", false, "Version prints the release version of validator")
	groupOutputPtr := flag.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
	quietPtr := flag.Bool("quiet", false, "If quiet flag is set. It doesn't print any output to stdout.")
	flag.Var(
		&reporterConfigFlags,
		"reporter",
		`A colon-separated string of report formats with optional output file paths.
Usage: --reporter <format>:<optional_file_path>
Multiple reporters can be specified: --reporter json:file_path.json --reporter junit:another_file_path.xml
Omit the file path to output to stdout: --reporter json or explicitly specify stdout using "-": --reporter json:-
Supported formats: standard, json, junit (default: "standard")`,
	)

	flag.Parse()

	flagsEnvMap := map[string]string{
		"depth":              "CFV_DEPTH",
		"exclude-dirs":       "CFV_EXCLUDE_DIRS",
		"exclude-file-types": "CFV_EXCLUDE_FILE_TYPES",
		"reporter":           "CFV_REPORTER",
		"groupby":            "CFV_GROUPBY",
		"quiet":              "CFV_QUIET",
	}
	for flagName, envVar := range flagsEnvMap {
		if err := setFlagFromEnvIfNotSet(flagName, envVar); err != nil {
			return validatorConfig{}, err
		}
	}

	reporterConf := make(map[string]string)
	for _, reportFlag := range reporterConfigFlags {
		parts := strings.Split(reportFlag, ":")
		switch len(parts) {
		case 1:
			reporterConf[parts[0]] = ""
		case 2:
			if parts[1] == "-" {
				reporterConf[parts[0]] = ""
			} else {
				reporterConf[parts[0]] = parts[1]
			}
		default:
			return validatorConfig{}, errors.New("Wrong parameter value format for reporter, expected format is `report_type:optional_file_path`")
		}
	}

	if len(reporterConf) == 0 {
		reporterConf["standard"] = ""
	}

	searchPaths := make([]string, 0)

	// If search path arg is empty, set it to the cwd
	// if not, set it to the arg. Supports n number of
	// paths
	if flag.NArg() == 0 {
		searchPaths = append(searchPaths, ".")
	} else {
		searchPaths = append(searchPaths, flag.Args()...)
	}

	acceptedReportTypes := map[string]bool{"standard": true, "json": true, "junit": true, "sarif": true}
	groupOutputReportTypes := map[string]bool{"standard": true, "json": true}
	for reportType := range reporterConf {
		_, ok := acceptedReportTypes[reportType]
		if !ok {
			fmt.Println("Wrong parameter value for reporter, only supports standard, json, junit, or sarif")
			flag.Usage()
			return validatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard, json, junit, or sarif")
		}

		if reportType == "junit" && *groupOutputPtr != "" {
			fmt.Println("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
			flag.Usage()
			return validatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is not supported for JUnit reports")
		}

		if !groupOutputReportTypes[reportType] && *groupOutputPtr != "" {
			fmt.Println("Wrong parameter value for reporter, groupby is only supported for standard and JSON reports")
			flag.Usage()
			return validatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is only supported for standard and JSON reports")
		}
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
		reporterConf,
		depthPtr,
		versionPtr,
		groupOutputPtr,
		quietPtr,
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

func setFlagFromEnvIfNotSet(flagName string, envVar string) error {
	if isFlagSet(flagName) {
		return nil
	}

	if envVarValue, ok := os.LookupEnv(envVar); ok {
		if err := flag.Set(flagName, envVarValue); err != nil {
			return err
		}
	}

	return nil
}

// Return the reporter associated with the
// reportType string
func getReporter(reportType, outputDest *string) reporter.Reporter {
	switch *reportType {
	case "junit":
		return reporter.NewJunitReporter(*outputDest)
	case "json":
		return reporter.NewJSONReporter(*outputDest)
	case "sarif":
		return reporter.NewSARIFReporter(*outputDest)
	default:
		return reporter.NewStdoutReporter(*outputDest)
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

	chosenReporters := make([]reporter.Reporter, 0)
	for reportType, outputFile := range validatorConfig.reportType {
		rt, of := reportType, outputFile // avoid "Implicit memory aliasing in for loop"
		chosenReporters = append(chosenReporters, getReporter(&rt, &of))
	}

	excludeFileTypes := strings.Split(*validatorConfig.excludeFileTypes, ",")
	groupOutput := strings.Split(*validatorConfig.groupOutput, ",")
	fsOpts := []finder.FSFinderOptions{
		finder.WithPathRoots(validatorConfig.searchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes),
	}
	quiet := *validatorConfig.quiet

	if validatorConfig.depth != nil && isFlagSet("depth") {
		fsOpts = append(fsOpts, finder.WithDepth(*validatorConfig.depth))
	}

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(fsOpts...)

	// Initialize the CLI
	c := cli.Init(
		cli.WithReporters(chosenReporters),
		cli.WithFinder(fileSystemFinder),
		cli.WithGroupOutput(groupOutput),
		cli.WithQuiet(quiet),
	)

	// Run the config file validation
	exitStatus, err := c.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}

	return exitStatus
}

func main() {
	os.Exit(mainInit())
}
