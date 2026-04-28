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
		Supported formats: standard, json, junit, sarif, and github (default: "standard")
  -version
    	Version prints the release version of validator
*/

package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	configfilevalidator "github.com/Boeing/config-file-validator/v2"
	"github.com/Boeing/config-file-validator/v2/pkg/cli"
	"github.com/Boeing/config-file-validator/v2/pkg/configfile"
	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/finder"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
	"github.com/Boeing/config-file-validator/v2/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v2/pkg/tools"
	"github.com/Boeing/config-file-validator/v2/pkg/validator"
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
	requireSchema    *bool
	noSchema         *bool
	typeMap          typeMapFlags
	schemaMap        schemaMapFlags
	schemaStore      *bool
	schemaStorePath  *string
	configPath       *string
	noConfig         *bool
	gitignore        *bool
}

type reporterFlags []string

func (rf *reporterFlags) String() string {
	return fmt.Sprint(*rf)
}

func (rf *reporterFlags) Set(value string) error {
	*rf = append(*rf, value)
	return nil
}

type typeMapFlags []string

func (tf *typeMapFlags) String() string {
	return fmt.Sprint(*tf)
}

func (tf *typeMapFlags) Set(value string) error {
	*tf = append(*tf, value)
	return nil
}

type schemaMapFlags []string

func (sf *schemaMapFlags) String() string {
	return fmt.Sprint(*sf)
}

func (sf *schemaMapFlags) Set(value string) error {
	*sf = append(*sf, value)
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
	fmt.Println("Schema validation runs automatically when a file declares a schema:")
	fmt.Println("  JSON:  {\"$schema\": \"schema.json\", ...}")
	fmt.Println("  YAML:  # yaml-language-server: $schema=schema.json")
	fmt.Println("  TOML:  \"$schema\" = \"schema.json\"")
	fmt.Println("  TOON:  \"$schema\": schema.json")
	fmt.Println("  XML:   xsi:noNamespaceSchemaLocation=\"schema.xsd\"")
	fmt.Println("  XML:   <!DOCTYPE> with inline DTD (validated during syntax check)")
	fmt.Println()
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
	slices.Sort(options)
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
func getFlags(args []string) (validatorConfig, error) {
	flagSet = flag.NewFlagSet("validator", flag.ContinueOnError)
	flagSet.Usage = validatorUsage
	reporterConfigFlags := reporterFlags{}

	var (
		depthPtr            = flagSet.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
		excludeDirsPtr      = flagSet.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
		excludeFileTypesPtr = flagSet.String("exclude-file-types", "", "A comma separated list of file types to ignore")
		fileTypesPtr        = flagSet.String("file-types", "", "A comma separated list of file types to validate")
		versionPtr          = flagSet.Bool("version", false, "Version prints the release version of validator")
		groupOutputPtr      = flagSet.String("groupby", "", "Group output by filetype, directory, pass-fail, error-type. Supported for Standard and JSON reports")
		quietPtr            = flagSet.Bool("quiet", false, "If quiet flag is set. It doesn't print any output to stdout.")
		globbingPrt         = flagSet.Bool("globbing", false, "If globbing flag is set, check for glob patterns in the arguments.")
		requireSchemaPtr    = flagSet.Bool("require-schema", false,
			"Fail validation if a file supports schema validation but does not declare a schema.\n"+
				"Supported types: JSON ($schema property), YAML (yaml-language-server comment),\n"+
				"TOML ($schema key), TOON (\"$schema\" key), XML (xsi:noNamespaceSchemaLocation).\n"+
				"Other file types (INI, CSV, ENV, HCL, HOCON, Properties, PList, EditorConfig) are not affected.\n"+
				"Cannot be used with --no-schema.")
		noSchemaPtr = flagSet.Bool("no-schema", false,
			"Disable all schema validation. Only syntax is checked.\n"+
				"Cannot be used with --require-schema, --schema-map, or --schemastore.")
		schemaStorePtr = flagSet.Bool("schemastore", false,
			"Enable automatic schema lookup by filename using the SchemaStore catalog.\n"+
				"Schemas are fetched remotely and cached locally.\n"+
				"Document-declared schemas and --schema-map take priority over SchemaStore.")
		schemaStorePathPtr = flagSet.String("schemastore-path", "",
			"Path to a local SchemaStore clone for automatic schema lookup by filename.\n"+
				"Implies --schemastore. Use for air-gapped environments.\n"+
				"Download with: git clone --depth=1 https://github.com/SchemaStore/schemastore.git")
		configPathPtr = flagSet.String("config", "",
			"Path to a .cfv.toml configuration file.\n"+
				"If not specified, searches for .cfv.toml in the current and parent directories.")
		noConfigPtr = flagSet.Bool("no-config", false,
			"Disable automatic discovery of .cfv.toml configuration files.")
		gitignorePtr = flagSet.Bool("gitignore", false,
			"Skip files and directories matched by .gitignore patterns.")
	)
	flagSet.Var(
		&reporterConfigFlags,
		"reporter",
		"Report format and optional output path. Format: <type>:<path> Supported: standard, json, junit, sarif, github (default: standard)",
	)

	typeMapConfigFlags := typeMapFlags{}
	flagSet.Var(
		&typeMapConfigFlags,
		"type-map",
		"Map a glob pattern to a file type. Format: <pattern>:<type> Example: --type-map=\"**/inventory:ini\"",
	)

	schemaMapConfigFlags := schemaMapFlags{}
	flagSet.Var(
		&schemaMapConfigFlags,
		"schema-map",
		"Map a glob pattern to a schema file for validation.\n"+
			"Format: <pattern>:<schema_path>\n"+
			"Use JSON Schema (.json) for JSON, YAML, TOML, and TOON files.\n"+
			"Use XSD (.xsd) for XML files. Paths are relative to the current directory.\n"+
			"Multiple mappings can be specified.\n"+
			"Examples:\n"+
			"  --schema-map=\"**/package.json:schemas/package.schema.json\"\n"+
			"  --schema-map=\"**/config.xml:schemas/config.xsd\"",
	)

	if err := flagSet.Parse(args); err != nil {
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

	if err := validateFlagValues(excludeFileTypesPtr, fileTypesPtr, depthPtr, reporterConf, groupOutputPtr); err != nil {
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
		requireSchemaPtr,
		noSchemaPtr,
		typeMapConfigFlags,
		schemaMapConfigFlags,
		schemaStorePtr,
		schemaStorePathPtr,
		configPathPtr,
		noConfigPtr,
		gitignorePtr,
	}

	return config, nil
}

func validateFlagValues(excludeFileTypesPtr, fileTypesPtr *string, depthPtr *int, reporterConf map[string]string, groupOutputPtr *string) error {
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
	acceptedReportTypes := map[string]bool{"standard": true, "json": true, "junit": true, "sarif": true, "github": true}
	groupOutputReportTypes := map[string]bool{"standard": true, "json": true}

	for reportType := range conf {
		_, ok := acceptedReportTypes[reportType]
		if !ok {
			return errors.New("wrong parameter value for reporter, only supports standard, json, junit, sarif, or github")
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
	groupByAllowedValues := []string{"filetype", "directory", "pass-fail", "error-type"}
	seenValues := make(map[string]bool)

	// Check that the groupby values are valid and not duplicates
	if groupBy != nil && isFlagSet("groupby") {
		for _, groupBy := range groupByUserInput {
			if !slices.Contains(groupByAllowedValues, groupBy) {
				return errors.New("wrong parameter value for groupby, only supports filetype, directory, pass-fail, error-type")
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
		"globbing":           "CFV_GLOBBING",
		"require-schema":     "CFV_REQUIRE_SCHEMA",
		"no-schema":          "CFV_NO_SCHEMA",
		"schemastore":        "CFV_SCHEMASTORE",
		"schemastore-path":   "CFV_SCHEMASTORE_PATH",
		"gitignore":          "CFV_GITIGNORE",
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

	if envVarValue, ok := os.LookupEnv(envVar); ok && envVarValue != "" {
		if err := flagSet.Set(flagName, envVarValue); err != nil {
			return err
		}
	}

	return nil
}

// Return the reporter associated with the
// reportType string
func getReporter(reportType, outputDest string) reporter.Reporter {
	switch reportType {
	case "junit":
		return reporter.NewJunitReporter(outputDest)
	case "json":
		return reporter.NewJSONReporter(outputDest)
	case "sarif":
		return reporter.NewSARIFReporter(outputDest)
	case "github":
		return reporter.NewGitHubReporter(outputDest)
	default:
		return reporter.NewStdoutReporter(outputDest)
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
	return tools.IsGlobPattern(s)
}

// resolvedConfig holds the final merged configuration from CLI flags, config file, and env vars.
type resolvedConfig struct {
	reporters     []reporter.Reporter
	groupOutput   []string
	quiet         bool
	requireSchema bool
	noSchema      bool
	schemaMap     map[string]string
	store         *schemastore.Store
	finderOpts    []finder.FSFinderOptions
	stdinData     []byte
	stdinFileType filetype.FileType
	isStdin       bool
}

func mainInit() int {
	validatorConfig, err := getFlags(os.Args[1:])
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
		return 2
	}

	if *validatorConfig.versionQuery {
		fmt.Println(configfilevalidator.GetVersion())
		return 0
	}

	resolved, err := resolveConfig(&validatorConfig)
	if err != nil {
		log.Printf("An error occurred: %v", err)
		return 2
	}

	c := buildCLI(resolved)

	exitStatus, err := c.Run()
	if err != nil {
		log.Printf("An error occurred during CLI execution: %v", err)
	}
	return exitStatus
}

func resolveConfig(cfg *validatorConfig) (*resolvedConfig, error) {
	validatorOpts, err := applyConfigFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("loading config file: %w", err)
	}

	quiet := *cfg.quiet
	requireSchema := *cfg.requireSchema
	noSchema := *cfg.noSchema
	useSchemaStore := *cfg.schemaStore || *cfg.schemaStorePath != ""

	if noSchema && (requireSchema || len(cfg.schemaMap) > 0 || useSchemaStore) {
		return nil, errors.New("--no-schema cannot be used with --require-schema, --schema-map, or --schemastore")
	}

	reporters, err := buildReporters(cfg.reportType)
	if err != nil {
		return nil, err
	}

	schemaMap, err := parseSchemaMapFlags(cfg.schemaMap)
	if err != nil {
		return nil, err
	}

	store, err := openSchemaStore(cfg)
	if err != nil {
		return nil, err
	}

	groupOutput := strings.Split(*cfg.groupOutput, ",")

	resolved := &resolvedConfig{
		reporters:     reporters,
		groupOutput:   groupOutput,
		quiet:         quiet,
		requireSchema: requireSchema,
		noSchema:      noSchema,
		schemaMap:     schemaMap,
		store:         store,
	}

	// Handle stdin mode
	if len(cfg.searchPaths) == 1 && cfg.searchPaths[0] == "-" {
		ft, data, err := readStdin(*cfg.fileTypes)
		if err != nil {
			return nil, err
		}
		resolved.isStdin = true
		resolved.stdinData = data
		resolved.stdinFileType = ft
		return resolved, nil
	}

	excludeFileTypes := getExcludeFileTypes(*cfg.excludeFileTypes)
	configuredTypes := applyValidatorOptions(validatorOpts)
	fsOpts, err := buildFinderOpts(*cfg, excludeFileTypes, configuredTypes)
	if err != nil {
		return nil, err
	}
	resolved.finderOpts = fsOpts

	return resolved, nil
}

func buildCLI(rc *resolvedConfig) *cli.CLI {
	opts := []cli.Option{
		cli.WithReporters(rc.reporters...),
		cli.WithGroupOutput(rc.groupOutput),
		cli.WithQuiet(rc.quiet),
		cli.WithRequireSchema(rc.requireSchema),
		cli.WithNoSchema(rc.noSchema),
		cli.WithSchemaMap(rc.schemaMap),
		cli.WithSchemaStore(rc.store),
	}

	if rc.isStdin {
		opts = append(opts, cli.WithStdinData(rc.stdinData, rc.stdinFileType))
	} else {
		opts = append(opts, cli.WithFinder(finder.FileSystemFinderInit(rc.finderOpts...)))
	}

	return cli.Init(opts...)
}

func buildReporters(reportType map[string]string) ([]reporter.Reporter, error) {
	reporters := make([]reporter.Reporter, 0, len(reportType))
	for rt, of := range reportType {
		reporters = append(reporters, getReporter(rt, of))
	}
	return reporters, nil
}

func openSchemaStore(cfg *validatorConfig) (*schemastore.Store, error) {
	if *cfg.schemaStorePath != "" {
		store, err := schemastore.Open(*cfg.schemaStorePath)
		if err != nil {
			return nil, fmt.Errorf("opening schemastore: %w", err)
		}
		return store, nil
	}
	if *cfg.schemaStore || *cfg.schemaStorePath != "" {
		store, err := schemastore.OpenEmbedded()
		if err != nil {
			return nil, fmt.Errorf("opening embedded schemastore: %w", err)
		}
		return store, nil
	}
	return nil, nil
}

func readStdin(fileTypesFlag string) (filetype.FileType, []byte, error) {
	if fileTypesFlag == "" {
		return filetype.FileType{}, nil, errors.New("reading from stdin requires --file-types to specify exactly one file type")
	}
	fileTypeName := strings.ToLower(fileTypesFlag)
	if strings.Contains(fileTypeName, ",") {
		return filetype.FileType{}, nil, errors.New("reading from stdin requires exactly one file type")
	}
	for _, ft := range filetype.FileTypes {
		if _, ok := ft.Extensions[fileTypeName]; ok {
			data, err := io.ReadAll(os.Stdin)
			if err != nil {
				return filetype.FileType{}, nil, fmt.Errorf("reading stdin: %w", err)
			}
			return ft, data, nil
		}
	}
	return filetype.FileType{}, nil, fmt.Errorf("unknown file type %q", fileTypeName)
}
func buildFinderOpts(cfg validatorConfig, excludeFileTypes []string, fileTypes []filetype.FileType) ([]finder.FSFinderOptions, error) {
	excludeDirs := strings.Split(*cfg.excludeDirs, ",")
	fsOpts := []finder.FSFinderOptions{
		finder.WithPathRoots(cfg.searchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes),
		finder.WithFileTypes(fileTypes),
	}

	if *cfg.fileTypes != "" {
		includeTypes := tools.ArrToMap(strings.Split(strings.ToLower(*cfg.fileTypes), ",")...)
		// Expand families: json ↔ jsonc
		for _, family := range fileTypeFamilies {
			for _, member := range family {
				if _, ok := includeTypes[member]; ok {
					for _, sibling := range family {
						includeTypes[sibling] = struct{}{}
					}
					break
				}
			}
		}
		var fileTypeFilter []filetype.FileType
		for _, ft := range fileTypes {
			for ext := range ft.Extensions {
				if _, ok := includeTypes[ext]; ok {
					fileTypeFilter = append(fileTypeFilter, ft)
					break
				}
			}
		}
		fsOpts = append(fsOpts, finder.WithFileTypes(fileTypeFilter))
	}

	if cfg.depth != nil && isFlagSet("depth") {
		fsOpts = append(fsOpts, finder.WithDepth(*cfg.depth))
	}

	typeOverrides, err := parseTypeMapFlags(cfg.typeMap)
	if err != nil {
		return nil, err
	}
	if len(typeOverrides) > 0 {
		fsOpts = append(fsOpts, finder.WithTypeOverrides(typeOverrides))
	}

	if *cfg.gitignore {
		fsOpts = append(fsOpts, finder.WithGitignore(true))
	}

	return fsOpts, nil
}

// fileTypeFamilies maps file type names that should be treated as a single
// family for --exclude-file-types and --file-types. Excluding one member
// of a family excludes all members.
var fileTypeFamilies = [][]string{
	{"json", "jsonc"},
}

func getExcludeFileTypes(configExcludeFileTypes string) []string {
	if configExcludeFileTypes == "" {
		return nil
	}
	excludeFileTypes := strings.Split(strings.ToLower(configExcludeFileTypes), ",")
	uniqueFileTypes := tools.ArrToMap(excludeFileTypes...)

	// Expand families: json ↔ jsonc
	for _, family := range fileTypeFamilies {
		for _, member := range family {
			if _, ok := uniqueFileTypes[member]; ok {
				for _, sibling := range family {
					uniqueFileTypes[sibling] = struct{}{}
				}
				break
			}
		}
	}

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

	excludeFileTypes = slices.Collect(maps.Keys(uniqueFileTypes))

	return excludeFileTypes
}

func parseTypeMapFlags(flags typeMapFlags) ([]finder.TypeOverride, error) {
	var overrides []finder.TypeOverride
	fileTypesByName := make(map[string]filetype.FileType)
	for _, ft := range filetype.FileTypes {
		fileTypesByName[ft.Name] = ft
	}

	for _, mapping := range flags {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid type-map format %q, expected pattern:type", mapping)
		}
		pattern := parts[0]
		typeName := strings.ToLower(parts[1])

		ft, ok := fileTypesByName[typeName]
		if !ok {
			return nil, fmt.Errorf("unknown file type %q in type-map", typeName)
		}
		overrides = append(overrides, finder.TypeOverride{Pattern: pattern, FileType: ft})
	}

	return overrides, nil
}

func parseSchemaMapFlags(flags schemaMapFlags) (map[string]string, error) {
	result := make(map[string]string)
	for _, mapping := range flags {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid schema-map format %q, expected pattern:schema_path", mapping)
		}
		result[parts[0]] = parts[1]
	}
	return result, nil
}

func applyConfigFile(cfg *validatorConfig) (*configfile.ValidatorOptions, error) {
	if *cfg.noConfig {
		return nil, nil
	}

	var cfgPath string
	if *cfg.configPath != "" {
		cfgPath = *cfg.configPath
	} else {
		cfgPath = configfile.Discover(".")
	}
	if cfgPath == "" {
		return nil, nil
	}

	fileCfg, err := configfile.Load(cfgPath)
	if err != nil {
		return nil, err
	}

	// Apply config values only when the CLI flag was not explicitly set.
	// CLI flags > config file > env vars (env vars already applied to flags).
	if !isFlagSet("exclude-dirs") && len(fileCfg.ExcludeDirs) > 0 {
		v := strings.Join(fileCfg.ExcludeDirs, ",")
		cfg.excludeDirs = &v
	}
	if !isFlagSet("exclude-file-types") && len(fileCfg.ExcludeFileTypes) > 0 {
		v := strings.Join(fileCfg.ExcludeFileTypes, ",")
		cfg.excludeFileTypes = &v
	}
	if !isFlagSet("file-types") && len(fileCfg.FileTypes) > 0 {
		v := strings.Join(fileCfg.FileTypes, ",")
		cfg.fileTypes = &v
	}
	if !isFlagSet("depth") && fileCfg.Depth != nil {
		if err := flagSet.Set("depth", fmt.Sprintf("%d", *fileCfg.Depth)); err != nil {
			return nil, fmt.Errorf("config file depth: %w", err)
		}
		cfg.depth = fileCfg.Depth
	}
	if !isFlagSet("reporter") && len(fileCfg.Reporter) > 0 {
		conf, err := parseReporterFlags(reporterFlags(fileCfg.Reporter))
		if err != nil {
			return nil, fmt.Errorf("config file reporter: %w", err)
		}
		cfg.reportType = conf
	}
	if !isFlagSet("groupby") && len(fileCfg.GroupBy) > 0 {
		v := strings.Join(fileCfg.GroupBy, ",")
		cfg.groupOutput = &v
	}
	if !isFlagSet("quiet") && fileCfg.Quiet != nil {
		cfg.quiet = fileCfg.Quiet
	}
	if !isFlagSet("require-schema") && fileCfg.RequireSchema != nil {
		cfg.requireSchema = fileCfg.RequireSchema
	}
	if !isFlagSet("no-schema") && fileCfg.NoSchema != nil {
		cfg.noSchema = fileCfg.NoSchema
	}
	if !isFlagSet("schemastore") && fileCfg.SchemaStore != nil {
		cfg.schemaStore = fileCfg.SchemaStore
	}
	if !isFlagSet("schemastore-path") && fileCfg.SchemaStorePath != nil {
		cfg.schemaStorePath = fileCfg.SchemaStorePath
	}
	if !isFlagSet("globbing") && fileCfg.Globbing != nil {
		cfg.globbing = fileCfg.Globbing
	}
	if !isFlagSet("gitignore") && fileCfg.Gitignore != nil {
		cfg.gitignore = fileCfg.Gitignore
	}
	if len(cfg.schemaMap) == 0 && len(fileCfg.SchemaMap) > 0 {
		for pattern, schema := range fileCfg.SchemaMap {
			cfg.schemaMap = append(cfg.schemaMap, pattern+":"+schema)
		}
	}
	if len(cfg.typeMap) == 0 && len(fileCfg.TypeMap) > 0 {
		for pattern, typeName := range fileCfg.TypeMap {
			cfg.typeMap = append(cfg.typeMap, pattern+":"+typeName)
		}
	}

	return &fileCfg.Validators, nil
}

func applyValidatorOptions(opts *configfile.ValidatorOptions) []filetype.FileType {
	types := make([]filetype.FileType, len(filetype.FileTypes))
	copy(types, filetype.FileTypes)

	if opts == nil {
		return types
	}

	for i, ft := range types {
		switch ft.Name {
		case "csv":
			if opts.CSV != nil {
				types[i].Validator = applyCSVOptions(opts.CSV)
			}
		case "json":
			if opts.JSON != nil {
				types[i].Validator = applyJSONOptions(opts.JSON)
			}
		case "ini":
			if opts.INI != nil {
				types[i].Validator = applyINIOptions(opts.INI)
			}
		default:
		}
	}

	return types
}

func applyCSVOptions(opts *configfile.CSVOptions) validator.CsvValidator {
	v := validator.CsvValidator{}
	if opts.Delimiter != nil {
		v.Delimiter = parseDelimiter(*opts.Delimiter)
	}
	if opts.Comment != nil {
		r := []rune(*opts.Comment)
		if len(r) == 1 {
			v.Comment = r[0]
		}
	}
	if opts.LazyQuotes != nil {
		v.LazyQuotes = *opts.LazyQuotes
	}
	return v
}

func applyJSONOptions(opts *configfile.JSONOptions) validator.JSONValidator {
	v := validator.JSONValidator{}
	if opts.ForbidDuplicateKeys != nil {
		v.ForbidDuplicateKeys = *opts.ForbidDuplicateKeys
	}
	return v
}

func applyINIOptions(opts *configfile.INIOptions) validator.IniValidator {
	v := validator.IniValidator{}
	if opts.ForbidDuplicateKeys != nil {
		v.ForbidDuplicateKeys = *opts.ForbidDuplicateKeys
	}
	return v
}

func parseDelimiter(s string) rune {
	if s == "\\t" || s == "\t" {
		return '\t'
	}
	r := []rune(s)
	if len(r) == 1 {
		return r[0]
	}
	return 0
}

func main() {
	os.Exit(mainInit())
}
