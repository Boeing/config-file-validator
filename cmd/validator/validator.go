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
  -file-types string
    	A comma separated list of file types to validate. Mutually exclusive with -exclude-file-types
  -output
     	Destination of a file to outputting results
  -reporter string
    	Format of the printed report. Options are standard, json, junit and sarif (default "standard")
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
	"sort"
	"strings"

	configfilevalidator "github.com/Boeing/config-file-validator"
	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/misc"
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
	quiet            *bool
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

// Assemble pretty formatted list of file types
func getFileTypes() []string {
	options := make([]string, 0, len(filetype.FileTypes))
	for _, typ := range filetype.FileTypes {
		options = append(options, typ.Name)
	}
	sort.Strings(options)
	return options
}

// Given a slice of strings, validates each is a valid file type
func validateFileTypeList(input []string) bool {
	types := getFileTypes()
	for _, t := range input {
		if len(t) == 0 {
			continue
		}
		if !slices.Contains(types, t) {
			return false
		}
	}
	return true
}

func buildSearchPath(n int) []string {
	searchPaths := make([]string, 0)
	// If search path arg is empty, set it to the cwd
	// if not, set it to the arg. Supports n number of
	// paths
	if n == 0 {
		searchPaths = append(searchPaths, ".")
	} else {
		searchPaths = append(searchPaths, flag.Args()...)
	}
	return searchPaths
}

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func getFlags() (validatorConfig, error) {
	flag.Usage = validatorUsage
	var (
		depthPtr            = flag.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
		excludeDirsPtr      = flag.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
		excludeFileTypesPtr = flag.String("exclude-file-types", "", "A comma separated list of file types to ignore.\nValid options: "+strings.Join(getFileTypes(), ", "))
		fileTypesPtr        = flag.String("file-types", "", "A comma separated list of file types to validate. Mutually exclusive with --exclude-file-types")
		outputPtr           = flag.String("output", "", "Destination to a file to output results")
		reportTypePtr       = flag.String("reporter", "standard", "Format of the printed report. Options are standard, json, junit and sarif")
		versionPtr          = flag.Bool("version", false, "Version prints the release version of validator")
		groupOutputPtr      = flag.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
		quietPtr            = flag.Bool("quiet", false, "If quiet flag is set. It doesn't print any output to stdout.")
	)

	flagsEnvMap := map[string]string{
		"depth":              "CFV_DEPTH",
		"exclude-dirs":       "CFV_EXCLUDE_DIRS",
		"exclude-file-types": "CFV_EXCLUDE_FILE_TYPES",
		"file-types":         "CFV_FILE_TYPES",
		"output":             "CFV_OUTPUT",
		"reporter":           "CFV_REPORTER",
		"groupby":            "CFV_GROUPBY",
		"quiet":              "CFV_QUIET",
	}

	if err := setFlagFromEnvIfNotSet(flagsEnvMap); err != nil {
		return validatorConfig{}, err
	}

	flag.Parse()

	searchPaths := buildSearchPath(flag.NArg())

	acceptedReportTypes := map[string]bool{"standard": true, "json": true, "junit": true, "sarif": true}

	if !acceptedReportTypes[*reportTypePtr] {
		return validatorConfig{}, errors.New("Wrong parameter value for reporter, only supports standard, json, junit or sarif")
	}

	groupOutputReportTypes := map[string]bool{"standard": true, "json": true}

	if !groupOutputReportTypes[*reportTypePtr] && *groupOutputPtr != "" {
		return validatorConfig{}, errors.New("Wrong parameter value for reporter, groupby is only supported for standard and JSON reports")
	}

	if depthPtr != nil && isFlagSet("depth") && *depthPtr < 0 {
		return validatorConfig{}, errors.New("Wrong parameter value for depth, value cannot be negative")
	}

	if *excludeFileTypesPtr != "" {
		if !validateFileTypeList(strings.Split(*excludeFileTypesPtr, ",")) {
			return validatorConfig{}, errors.New("Invalid exclude file type")
		}
	}

	if *excludeFileTypesPtr != "" && *fileTypesPtr != "" {
		return validatorConfig{}, errors.New("Cannot use --exclude-file-types and --file-types together")
	}

	if *fileTypesPtr != "" {
		if err := flag.Set("exclude-file-types", getExcludeFileTypesFromFileTypes(fileTypesPtr)); err != nil {
			return validatorConfig{}, err
		}
	}

	if err := checkValidGroupByValues(groupOutputPtr); err != nil {
		return validatorConfig{}, err
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
		quietPtr,
	}

	return config, nil
}

func checkValidGroupByValues(groupOutputPtr *string) error {
	groupByCleanString := cleanString("groupby")
	groupByUserInput := strings.Split(groupByCleanString, ",")
	groupByAllowedValues := []string{"filetype", "directory", "pass-fail"}
	seenValues := make(map[string]bool)

	// Check that the groupby values are valid and not duplicates
	if groupOutputPtr != nil && isFlagSet("groupby") {
		for _, groupBy := range groupByUserInput {
			if !slices.Contains(groupByAllowedValues, groupBy) {
				return errors.New("Wrong parameter value for groupby, only supports filetype, directory, pass-fail")
			}
			if _, ok := seenValues[groupBy]; ok {
				return errors.New("Wrong parameter value for groupby, duplicate values are not allowed")
			}
			seenValues[groupBy] = true
		}
	}
	return nil
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

func setFlagFromEnvIfNotSet(flagsEnvMap map[string]string) error {
	for flagName, envVar := range flagsEnvMap {
		if isFlagSet(flagName) {
			return nil
		}

		if envVarValue, ok := os.LookupEnv(envVar); ok {
			if err := flag.Set(flagName, envVarValue); err != nil {
				return err
			}
		}
	}

	return nil
}

// Build exclude-file-type list from file-type values
func getExcludeFileTypesFromFileTypes(fileTypesPtr *string) string {
	validTypes := make([]string, 0, len(filetype.FileTypes))

	for _, t := range filetype.FileTypes {
		validTypes = append(validTypes, t.Name)
	}

	validExcludeTypes := misc.ArrToMap(validTypes...)

	for _, t := range strings.Split(*fileTypesPtr, ",") {
		delete(validExcludeTypes, t)
	}

	excludeFileTypes := make([]string, 0, len(validExcludeTypes))

	for fileType := range validExcludeTypes {
		excludeFileTypes = append(excludeFileTypes, fileType)
	}

	return strings.Join(excludeFileTypes, ",")
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
		fmt.Println(err.Error())
		flag.Usage()
		return 1
	}

	if *validatorConfig.versionQuery {
		fmt.Println(configfilevalidator.GetVersion())
		return 0
	}

	// since the exclude dirs are a comma separated string
	// it needs to be split into a slice of strings
	excludeDirs := strings.Split(*validatorConfig.excludeDirs, ",")
	choosenReporter := getReporter(validatorConfig.reportType, validatorConfig.output)
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
		cli.WithReporter(choosenReporter),
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
