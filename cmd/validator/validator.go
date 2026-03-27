/*
Validator recursively scans a directory to search for configuration files and
validates them using the go package for each configuration type.

Currently Apple PList XML, CSV, HCL, HOCON, INI, JSON, Properties, Sarif, TOML, TOON, XML, and YAML.
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
    	A comma separated list of file types to validate
  -globbing bool
    	Set globbing to true to enable pattern matching for search paths
  -reporter string
		A string representing report format and optional output file path separated by colon if present.
		Usage: --reporter <format>:<optional_file_path>
		Multiple reporters can be specified: --reporter json:file_path.json --reporter junit:another_file_path.xml
		Omit the file path to output to stdout: --reporter json or explicitly specify stdout using "-": --reporter json:-
		Supported formats: standard, json, junit, and sarif (default: "standard")
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

	"github.com/bmatcuk/doublestar/v4"

	configfilevalidator "github.com/Boeing/config-file-validator"
	"github.com/Boeing/config-file-validator/pkg/cli"
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/finder"
	"github.com/Boeing/config-file-validator/pkg/reporter"
	"github.com/Boeing/config-file-validator/pkg/tools"
)

var flagSet *flag.FlagSet

type validatorConfig struct {
	searchPaths      []string
	excludeDirs      *string
	excludeFileTypes *string
	fileTypes        *string
	reportType       map[string]string
	depth            *int
	versionQuery     *bool
	groupOutput      *string
	quiet            *bool
	globbing         *bool
	format           *string
	schema           *string
}

type reporterFlags []string

func (rf *reporterFlags) String() string {
	return fmt.Sprint(*rf)
}

func (rf *reporterFlags) Set(value string) error {
	*rf = append(*rf, value)
	return nil
}

// Custom Usage function to cover. Uses the current flagSet when available.
func validatorUsage() {
	fmt.Println("Usage: validator [OPTIONS] [<search_path>...]")
	fmt.Println()
	fmt.Println("positional arguments:")
	fmt.Printf(
		"    search_path: The search path on the filesystem for configuration files. " +
			"Defaults to the current working directory if no search_path provided\n\n")
	fmt.Println("optional flags:")
	if flagSet != nil {
		flagSet.PrintDefaults()
	} else {
		flag.PrintDefaults()
	}
}

// Assemble pretty formatted list of file types
func getFileTypes() []string {
	options := make([]string, 0, len(filetype.FileTypes))
	for _, typ := range filetype.FileTypes {
		for extName := range typ.Extensions {
			options = append(options, extName)
		}
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

// Parses, validates, and returns the flags
// flag.String returns a pointer
// If a required parameter is missing the help
// output will be displayed and the function
// will return with exit = 1
func getFlags() (validatorConfig, error) {
	// construct a dedicated FlagSet rather than using the package-level
	// default. this satisfies revive's deep-exit rule and makes the
	// function easier to test.
	flagSet = flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	flagSet.Usage = validatorUsage
	reporterConfigFlags := reporterFlags{}

	var (
		depthPtr            = flagSet.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
		excludeDirsPtr      = flagSet.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
		excludeFileTypesPtr = flagSet.String("exclude-file-types", "", "A comma separated list of file types to ignore")
		fileTypesPtr        = flagSet.String("file-types", "", "A comma separated list of file types to validate")
		versionPtr          = flagSet.Bool("version", false, "Version prints the release version of validator")
		groupOutputPtr      = flagSet.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
		quietPtr            = flagSet.Bool("quiet", false, "If quiet flag is set. It doesn't print any output to stdout.")
		globbingPrt         = flagSet.Bool("globbing", false, "If globbing flag is set, check for glob patterns in the arguments.")
		formatPtr           = flagSet.String("check-format", "", "Comma separated list of file types to check formatting. Only json is supported currently.")
		schemaPtr           = flagSet.String("schema", "", "Comma separated list of file types to validate against their schema. Only sarif is supported currently.")
	)
	flagSet.Var(
		&reporterConfigFlags,
		"reporter",
		"Report format and optional output path. Format: <type>:<path> Supported: standard, json, junit, sarif (default: standard)",
	)

	if err := flagSet.Parse(os.Args[1:]); err != nil {
		return validatorConfig{}, err
	}

	if err := applyDefaultFlagsFromEnv(); err != nil {
		return validatorConfig{}, err
	}

	reporterConf, err := parseReporterFlags(reporterConfigFlags)
	if err != nil {
		return validatorConfig{}, err
	}

	if err := validateGlobbing(globbingPrt); err != nil {
		return validatorConfig{}, err
	}

	searchPaths, err := parseSearchPath(globbingPrt)
	if err != nil {
		return validatorConfig{}, err
	}

	if err := validateFlagValues(formatPtr, schemaPtr, excludeFileTypesPtr, fileTypesPtr, depthPtr, reporterConf, groupOutputPtr); err != nil {
		return validatorConfig{}, err
	}

	config := validatorConfig{
		searchPaths,
		excludeDirsPtr,
		excludeFileTypesPtr,
		fileTypesPtr,
		reporterConf,
		depthPtr,
		versionPtr,
		groupOutputPtr,
		quietPtr,
		globbingPrt,
		formatPtr,
		schemaPtr,
	}

	return config, nil
}

func validateFlagValues(formatPtr, schemaPtr, excludeFileTypesPtr, fileTypesPtr *string, depthPtr *int, reporterConf map[string]string, groupOutputPtr *string) error {
	if err := validateFormatFlag(formatPtr); err != nil {
		return err
	}

	if err := validateSchemaFlag(schemaPtr); err != nil {
		return err
	}

	if err := validateReporterConf(reporterConf, groupOutputPtr); err != nil {
		return err
	}

	if depthPtr != nil && isFlagSet("depth") && *depthPtr < 0 {
		return errors.New("wrong parameter value for depth, value cannot be negative")
	}

	if err := validateFileTypeFlags(excludeFileTypesPtr, fileTypesPtr); err != nil {
		return err
	}

	return validateGroupByConf(groupOutputPtr)
}

func validateFormatFlag(formatPtr *string) error {
	if *formatPtr == "" {
		return nil
	}
	formatFileTypes := strings.Split(strings.ToLower(*formatPtr), ",")
	if !slices.Contains(formatFileTypes, "all") && !validateFileTypeList(formatFileTypes) {
		return errors.New("invalid check format file type")
	}
	return nil
}

func validateSchemaFlag(schemaPtr *string) error {
	if *schemaPtr == "" {
		return nil
	}
	schemaFileTypes := strings.Split(strings.ToLower(*schemaPtr), ",")
	if !validateFileTypeList(schemaFileTypes) {
		return errors.New("invalid schema file type")
	}
	return nil
}

func validateFileTypeFlags(excludeFileTypesPtr, fileTypesPtr *string) error {
	if *excludeFileTypesPtr != "" {
		*excludeFileTypesPtr = strings.ToLower(*excludeFileTypesPtr)
		if !validateFileTypeList(strings.Split(*excludeFileTypesPtr, ",")) {
			return errors.New("invalid exclude file type")
		}
	}
	if *fileTypesPtr != "" && *excludeFileTypesPtr != "" {
		return errors.New("--file-types and --exclude-file-types cannot be used together")
	}
	if *fileTypesPtr != "" {
		*fileTypesPtr = strings.ToLower(*fileTypesPtr)
		if !validateFileTypeList(strings.Split(*fileTypesPtr, ",")) {
			return errors.New("invalid file type")
		}
	}
	return nil
}

func validateReporterConf(conf map[string]string, groupBy *string) error {
	acceptedReportTypes := map[string]bool{"standard": true, "json": true, "junit": true, "sarif": true}
	groupOutputReportTypes := map[string]bool{"standard": true, "json": true}

	for reportType := range conf {
		_, ok := acceptedReportTypes[reportType]
		if !ok {
			return errors.New("wrong parameter value for reporter, only supports standard, json, junit, or sarif")
		}

		if !groupOutputReportTypes[reportType] && groupBy != nil && *groupBy != "" {
			return errors.New("wrong parameter value for reporter, groupby is only supported for standard and JSON reports")
		}
	}

	return nil
}

func validateGroupByConf(groupBy *string) error {
	groupByCleanString := cleanString("groupby")
	groupByUserInput := strings.Split(groupByCleanString, ",")
	groupByAllowedValues := []string{"filetype", "directory", "pass-fail"}
	seenValues := make(map[string]bool)

	// Check that the groupby values are valid and not duplicates
	if groupBy != nil && isFlagSet("groupby") {
		for _, groupBy := range groupByUserInput {
			if !slices.Contains(groupByAllowedValues, groupBy) {
				return errors.New("wrong parameter value for groupby, only supports filetype, directory, pass-fail")
			}
			if _, ok := seenValues[groupBy]; ok {
				return errors.New("wrong parameter value for groupby, duplicate values are not allowed")
			}
			seenValues[groupBy] = true
		}
	}

	return nil
}

func validateGlobbing(globbingPrt *bool) error {
	if *globbingPrt && (isFlagSet("exclude-dirs") || isFlagSet("exclude-file-types") || isFlagSet("file-types")) {
		return errors.New("the -globbing flag cannot be used with --exclude-dirs, --exclude-file-types, or --file-types")
	}
	return nil
}

func parseSearchPath(globbingPrt *bool) ([]string, error) {
	searchPaths := make([]string, 0)

	if flagSet.NArg() == 0 {
		searchPaths = append(searchPaths, ".")
	} else if *globbingPrt {
		return handleGlobbing(searchPaths)
	} else {
		searchPaths = append(searchPaths, flagSet.Args()...)
	}

	return searchPaths, nil
}

func handleGlobbing(searchPaths []string) ([]string, error) {
	for _, flagArg := range flagSet.Args() {
		if isGlobPattern(flagArg) {
			matches, err := doublestar.Glob(os.DirFS("."), flagArg)
			if err != nil {
				return nil, errors.New("glob matching error")
			}
			searchPaths = append(searchPaths, matches...)
		} else {
			searchPaths = append(searchPaths, flagArg)
		}
	}
	return searchPaths, nil
}

func parseReporterFlags(flags reporterFlags) (map[string]string, error) {
	conf := make(map[string]string)
	for _, reportFlag := range flags {
		parts := strings.Split(reportFlag, ":")
		switch len(parts) {
		case 1:
			conf[parts[0]] = ""
		case 2:
			if parts[1] == "-" {
				conf[parts[0]] = ""
			} else {
				conf[parts[0]] = parts[1]
			}
		default:
			return nil, errors.New("wrong parameter value format for reporter, expected format is `report_type:optional_file_path`")
		}
	}

	if len(conf) == 0 {
		conf["standard"] = ""
	}

	return conf, nil
}

// isFlagSet verifies if a given flag has been set or not
func isFlagSet(flagName string) bool {
	var isSet bool

	flagSet.Visit(func(f *flag.Flag) {
		if f.Name == flagName {
			isSet = true
		}
	})

	return isSet
}

func applyDefaultFlagsFromEnv() error {
	flagsEnvMap := map[string]string{
		"depth":              "CFV_DEPTH",
		"exclude-dirs":       "CFV_EXCLUDE_DIRS",
		"exclude-file-types": "CFV_EXCLUDE_FILE_TYPES",
		"file-types":         "CFV_FILE_TYPES",
		"reporter":           "CFV_REPORTER",
		"groupby":            "CFV_GROUPBY",
		"quiet":              "CFV_QUIET",
		"format":             "CFV_FORMAT",
		"globbing":           "CFV_GLOBBING",
		"schema":             "CFV_SCHEMA",
	}

	for flagName, envVar := range flagsEnvMap {
		if err := setFlagFromEnvIfNotSet(flagName, envVar); err != nil {
			return err
		}
	}

	return nil
}

func setFlagFromEnvIfNotSet(flagName string, envVar string) error {
	if isFlagSet(flagName) {
		return nil
	}

	if envVarValue, ok := os.LookupEnv(envVar); ok {
		if err := flagSet.Set(flagName, envVarValue); err != nil {
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
	cleanedString := flagSet.Lookup(command).Value.String()
	cleanedString = strings.ToLower(cleanedString)
	cleanedString = strings.TrimSpace(cleanedString)

	return cleanedString
}

// Function to check if a string is a glob pattern
func isGlobPattern(s string) bool {
	return strings.ContainsAny(s, "*?[]")
}

func mainInit() int {
	validatorConfig, err := getFlags()
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Println(err.Error())
		if flagSet != nil {
			flagSet.Usage()
		} else {
			flag.Usage()
		}
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

	excludeFileTypes := getExcludeFileTypes(*validatorConfig.excludeFileTypes)
	var fileTypeFilter []filetype.FileType
	if *validatorConfig.fileTypes != "" {
		includeTypes := tools.ArrToMap(strings.Split(strings.ToLower(*validatorConfig.fileTypes), ",")...)
		for _, ft := range filetype.FileTypes {
			for ext := range ft.Extensions {
				if _, ok := includeTypes[ext]; ok {
					fileTypeFilter = append(fileTypeFilter, ft)
					break
				}
			}
		}
	}
	groupOutput := strings.Split(*validatorConfig.groupOutput, ",")
	fsOpts := []finder.FSFinderOptions{
		finder.WithPathRoots(validatorConfig.searchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes),
	}
	if len(fileTypeFilter) > 0 {
		fsOpts = append(fsOpts, finder.WithFileTypes(fileTypeFilter))
	}
	quiet := *validatorConfig.quiet

	if validatorConfig.depth != nil && isFlagSet("depth") {
		fsOpts = append(fsOpts, finder.WithDepth(*validatorConfig.depth))
	}

	formatFileTypes := getFormatFileTypes(*validatorConfig.format)
	schemaFileTypes := getSchemaFileTypes(*validatorConfig.schema)

	// Initialize a file system finder
	fileSystemFinder := finder.FileSystemFinderInit(fsOpts...)

	// Initialize the CLI
	c := cli.Init(
		cli.WithReporters(chosenReporters...),
		cli.WithFinder(fileSystemFinder),
		cli.WithGroupOutput(groupOutput),
		cli.WithQuiet(quiet),
		cli.WithFormatCheckTypes(formatFileTypes),
		cli.WithSchemaCheckTypes(schemaFileTypes),
	)

	// Run the config file validation
	exitStatus, err := c.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}

	return exitStatus
}

func getExcludeFileTypes(configExcludeFileTypes string) []string {
	excludeFileTypes := strings.Split(strings.ToLower(configExcludeFileTypes), ",")
	uniqueFileTypes := tools.ArrToMap(excludeFileTypes...)

	for _, ft := range filetype.FileTypes {
		for ext := range ft.Extensions {
			if _, ok := uniqueFileTypes[ext]; !ok {
				continue
			}

			for ext := range ft.Extensions {
				uniqueFileTypes[ext] = struct{}{}
			}
			break
		}
	}

	excludeFileTypes = make([]string, 0, len(uniqueFileTypes))
	for ft := range uniqueFileTypes {
		excludeFileTypes = append(excludeFileTypes, ft)
	}

	return excludeFileTypes
}

func getFormatFileTypes(formatFlag string) []string {
	if formatFlag == "" {
		return nil
	}

	typesToFormat := strings.Split(strings.ToLower(formatFlag), ",")
	typesToFormatSet := tools.ArrToMap(typesToFormat...)
	fileTypesToFormat := make(map[string]struct{})

	formatAll := false
	if _, ok := typesToFormatSet["all"]; ok {
		formatAll = true
	}
	for _, ft := range filetype.FileTypes {
		for ext := range ft.Extensions {
			if _, ok := typesToFormatSet[ext]; formatAll || ok {
				fileTypesToFormat[ft.Name] = struct{}{}
			}
		}
	}
	types := make([]string, 0, len(fileTypesToFormat))
	for ft := range fileTypesToFormat {
		types = append(types, ft)
	}
	return types
}

func getSchemaFileTypes(schemaFlag string) []string {
	if schemaFlag == "" {
		return nil
	}

	typesToValidate := strings.Split(strings.ToLower(schemaFlag), ",")
	typesToValidateSet := tools.ArrToMap(typesToValidate...)
	fileTypesToValidate := make(map[string]struct{})

	for _, ft := range filetype.FileTypes {
		for ext := range ft.Extensions {
			if _, ok := typesToValidateSet[ext]; ok {
				fileTypesToValidate[ft.Name] = struct{}{}
			}
		}
	}
	types := make([]string, 0, len(fileTypesToValidate))
	for ft := range fileTypesToValidate {
		types = append(types, ft)
	}
	return types
}

func main() {
	os.Exit(mainInit())
}
