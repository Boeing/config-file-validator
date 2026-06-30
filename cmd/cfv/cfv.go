/*
cfv validates, formats, and fixes configuration files across 18 formats.

Usage: cfv [global-flags] [subcommand] [subcommand-flags] [<search_path>...]

Subcommands:

	check    Validate syntax and schema (default when no subcommand given)
	format   Report formatting issues (use --fix to rewrite files)
	version  Print version and exit
	help     Print help; "cfv help <subcommand>" for subcommand help

positional arguments:

	search_path: Filesystem path to search for configuration files.
	             Defaults to the current working directory.
	             Multiple paths can be provided separated by spaces.
	             Use "-" to read from stdin (requires --file-types).

Schema validation runs automatically when a file declares a schema:

	JSON:  {"$schema": "schema.json", ...}
	YAML:  # yaml-language-server: $schema=schema.json
	TOML:  "$schema" = "schema.json"
	TOON:  "$schema": schema.json
	XML:   xsi:noNamespaceSchemaLocation="schema.xsd"
	XML:   <!DOCTYPE> with inline DTD (validated during syntax check)

Global flags apply to all subcommands and must precede the subcommand name.
Run "cfv help check" or "cfv help format" for subcommand-specific flags.
*/
package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/bmatcuk/doublestar/v4"

	configfilevalidator "github.com/Boeing/config-file-validator/v3"
	"github.com/Boeing/config-file-validator/v3/pkg/cli"
	"github.com/Boeing/config-file-validator/v3/pkg/configfile"
	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
	"github.com/Boeing/config-file-validator/v3/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v3/pkg/tools"
	"github.com/Boeing/config-file-validator/v3/pkg/validator"
)

// cfvConfig holds all resolved flag values for the check subcommand.
type cfvConfig struct {
	// fs is the FlagSet used to parse this config. Kept here so isFlagSet
	// and cleanString can be methods on cfvConfig rather than using a
	// package-level var (which would break when multiple subcommands run).
	fs               *flag.FlagSet
	searchPaths      []string
	excludeDirs      *string
	excludeFileTypes *string
	fileTypes        *string
	reportType       []reporterConfig
	depth            *int
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
	mergeSarif       sarifMergeFlags
	mergeSarifDir    *string
	ignoreFiles      ignoreFileFlags
	// Phase 1: --fix and --unsafe are reserved (no-op) until Phase 4.
	fix    *bool
	unsafe *bool
}

// reporterConfig pairs a reporter format name with an optional output path.
type reporterConfig struct {
	reportType string
	outputDest string
}

// resolvedConfig is the final merged configuration passed to the CLI engine.
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

// --- Repeatable flag types ---

// reporterFlags is a repeatable --reporter flag.
type reporterFlags []string

func (rf *reporterFlags) String() string { return fmt.Sprint(*rf) }
func (rf *reporterFlags) Set(value string) error {
	*rf = append(*rf, value)
	return nil
}

// typeMapFlags is a repeatable --type-map flag.
type typeMapFlags []string

func (tf *typeMapFlags) String() string { return fmt.Sprint(*tf) }
func (tf *typeMapFlags) Set(value string) error {
	*tf = append(*tf, value)
	return nil
}

// schemaMapFlags is a repeatable --schema-map flag.
type schemaMapFlags []string

func (sf *schemaMapFlags) String() string { return fmt.Sprint(*sf) }
func (sf *schemaMapFlags) Set(value string) error {
	*sf = append(*sf, value)
	return nil
}

// sarifMergeFlags is a repeatable --merge-sarif flag.
type sarifMergeFlags []string

func (smf *sarifMergeFlags) String() string { return fmt.Sprint(*smf) }
func (smf *sarifMergeFlags) Set(value string) error {
	*smf = append(*smf, value)
	return nil
}

// ignoreFileFlags is a repeatable --ignore-file flag.
type ignoreFileFlags []string

func (iff *ignoreFileFlags) String() string { return fmt.Sprint(*iff) }
func (iff *ignoreFileFlags) Set(value string) error {
	*iff = append(*iff, value)
	return nil
}

// fileTypeFamilies groups file types that should be treated as a single family
// for --exclude-file-types and --file-types. Excluding one member excludes all.
var fileTypeFamilies = [][]string{
	{"json", "jsonc"},
}

// =============================================================================
// Subcommand router
// =============================================================================

// mainInit is the testable entry point. Returns an exit code.
func mainInit() int {
	args := os.Args[1:]

	// Phase 1: parse global flags. Only --version and --help live here.
	// All other flags belong to the subcommand FlagSet.
	globalFS := flag.NewFlagSet("cfv", flag.ContinueOnError)
	globalFS.Usage = printUsage
	versionFlag := globalFS.Bool("version", false, "Print the version and exit.")
	// Suppress the default error output — we handle it below.
	globalFS.SetOutput(io.Discard)

	// Parse only until the first non-flag argument (the subcommand or a path).
	// flag.ContinueOnError means unknown flags return an error rather than exiting,
	// which lets us forward unrecognised flags to the subcommand FlagSet.
	_ = globalFS.Parse(args)
	remaining := globalFS.Args()

	if *versionFlag {
		fmt.Println(configfilevalidator.GetVersion())
		return 0
	}

	// No arguments at all: run check on current directory.
	if len(args) == 0 {
		return runCheck(args)
	}

	// Phase 2: detect subcommand from the first non-flag token.
	// If global flag parsing consumed everything, remaining is empty —
	// treat that as a bare check too.
	subArgs := remaining
	if len(remaining) > 0 {
		switch remaining[0] {
		case "check":
			return runCheck(remaining[1:])
		case "format":
			return runFormat(remaining[1:])
		case "version":
			fmt.Println(configfilevalidator.GetVersion())
			return 0
		case "help":
			if len(remaining) > 1 {
				switch remaining[1] {
				case "check":
					printCheckUsage()
					return 0
				case "format":
					printFormatUsage()
					return 0
				}
			}
			printUsage()
			return 0
		}
		// Not a known subcommand — treat the full original args as a bare
		// check invocation so flags like --reporter still work.
		subArgs = args
	}

	// Bare invocation: cfv [flags] [paths] with no subcommand keyword.
	return runCheck(subArgs)
}

func main() {
	os.Exit(mainInit())
}

// =============================================================================
// Usage
// =============================================================================

func printUsage() {
	fmt.Println("Usage: cfv [global-flags] <subcommand> [subcommand-flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Subcommands:")
	fmt.Println("  check    Validate syntax and schema (equivalent to v2 'validator')")
	fmt.Println("  format   Report formatting issues; use --fix to rewrite files [Phase 2]")
	fmt.Println("  version  Print version and exit")
	fmt.Println("  help     Print this help; 'cfv help <subcommand>' for details")
	fmt.Println()
	fmt.Println("Running 'cfv [flags] [paths]' without a subcommand runs check.")
	fmt.Println()
	fmt.Println("Run 'cfv help check' for the full flag reference.")
}

func printCheckUsage() {
	fmt.Println("Usage: cfv check [flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Validate configuration files for syntax and schema errors.")
	fmt.Println("Equivalent to the v2 'validator' command.")
	fmt.Println()
	fmt.Println("positional arguments:")
	fmt.Println("  search_path  Path to search. Defaults to '.'. Use '-' for stdin.")
	fmt.Println()
	fmt.Println("Schema validation runs automatically when a file declares a schema:")
	fmt.Println("  JSON:  {\"$schema\": \"schema.json\", ...}")
	fmt.Println("  YAML:  # yaml-language-server: $schema=schema.json")
	fmt.Println("  TOML:  \"$schema\" = \"schema.json\"")
	fmt.Println("  TOON:  \"$schema\": schema.json")
	fmt.Println("  XML:   xsi:noNamespaceSchemaLocation=\"schema.xsd\"")
	fmt.Println("  XML:   <!DOCTYPE> with inline DTD (validated during syntax check)")
	fmt.Println()
	fmt.Println("flags:")
	// Flag defaults are printed by the subcommand's FlagSet after parsing.
	// When called from --help during flag parsing, the FlagSet prints defaults
	// to its own output automatically; this branch handles the cfv help check case.
}

func printFormatUsage() {
	fmt.Println("Usage: cfv format [--fix] [flags] [<search_path>...]")
	fmt.Println()
	fmt.Println("Report formatting issues. Use --fix to rewrite files.")
	fmt.Println()
	fmt.Println("NOTE: cfv format is not yet implemented (Phase 2).")
}

// =============================================================================
// check subcommand
// =============================================================================

// runCheck implements "cfv check [flags] [paths]" and the bare "cfv [flags] [paths]".
// Behavior is identical to the v2 validator binary.
func runCheck(args []string) int {
	cfg, err := parseCheckFlags(args)
	if err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		fmt.Fprintln(os.Stderr, err.Error())
		printCheckUsage()
		return 2
	}

	resolved, err := resolveConfig(&cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
		return 2
	}

	c := buildCLI(resolved)
	exitStatus, err := c.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "cfv: %v\n", err)
	}
	return exitStatus
}

// parseCheckFlags registers and parses all flags for the check subcommand.
func parseCheckFlags(args []string) (cfvConfig, error) {
	fs := flag.NewFlagSet("cfv check", flag.ContinueOnError)
	fs.Usage = printCheckUsage

	reporterConfigFlags := reporterFlags{}
	typeMapConfigFlags := typeMapFlags{}
	schemaMapConfigFlags := schemaMapFlags{}
	mergeSarifConfigFlags := sarifMergeFlags{}
	ignoreFileConfigFlags := ignoreFileFlags{}

	var (
		depthPtr         = fs.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
		excludeDirsPtr   = fs.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
		excludeTypesPtr  = fs.String("exclude-file-types", "", "A comma separated list of file types to ignore")
		fileTypesPtr     = fs.String("file-types", "", "A comma separated list of file types to validate")
		groupOutputPtr   = fs.String("groupby", "", "Group output by filetype, directory, pass-fail, error-type. Supported for Standard and JSON reports")
		quietPtr         = fs.Bool("quiet", false, "If quiet flag is set, no output is printed to stdout")
		globbingPtr      = fs.Bool("globbing", false, "Enable glob pattern matching for search paths")
		requireSchemaPtr = fs.Bool("require-schema", false,
			"Fail validation if a file supports schema validation but does not declare a schema.\n"+
				"Supported types: JSON ($schema property), YAML (yaml-language-server comment),\n"+
				"TOML ($schema key), TOON (\"$schema\" key), XML (xsi:noNamespaceSchemaLocation).\n"+
				"Cannot be used with --no-schema.")
		noSchemaPtr = fs.Bool("no-schema", false,
			"Disable all schema validation. Only syntax is checked.\n"+
				"Cannot be used with --require-schema, --schema-map, or --schemastore.")
		schemaStorePtr = fs.Bool("schemastore", false,
			"Enable automatic schema lookup by filename using the SchemaStore catalog.")
		schemaStorePathPtr = fs.String("schemastore-path", "",
			"Path to a local SchemaStore clone. Implies --schemastore.")
		configPathPtr = fs.String("config", "",
			"Path to a .cfv.toml configuration file.\n"+
				"If not specified, searches for .cfv.toml in the current and parent directories.")
		noConfigPtr = fs.Bool("no-config", false,
			"Disable automatic discovery of .cfv.toml configuration files.")
		gitignorePtr = fs.Bool("gitignore", false,
			"Skip files and directories matched by .gitignore patterns.")
		mergeSarifDirPtr = fs.String("merge-sarif-dir", "",
			"Directory tree containing SARIF files to merge into SARIF output. Requires --reporter=sarif.")
		// Phase 1: --fix and --unsafe are reserved. No-op until Phase 4.
		fixPtr    = fs.Bool("fix", false, "Apply safe fixes automatically [not yet implemented]")
		unsafePtr = fs.Bool("unsafe", false, "Apply unsafe fixes (requires --fix) [not yet implemented]")
	)

	fs.Var(&reporterConfigFlags, "reporter",
		"Report format and optional output path.\n"+
			"Format: <type>:<path>  Example: --reporter json:results.json\n"+
			"Supported: standard, json, junit, sarif, github (default: standard)\n"+
			"Multiple reporters can be specified.")
	fs.Var(&typeMapConfigFlags, "type-map",
		"Map a glob pattern to a file type.\n"+
			"Format: <pattern>:<type>  Example: --type-map=\"**/inventory:ini\"")
	fs.Var(&schemaMapConfigFlags, "schema-map",
		"Map a glob pattern to a schema file.\n"+
			"Format: <pattern>:<schema_path>\n"+
			"Use JSON Schema (.json) for JSON/YAML/TOML/TOON. Use XSD (.xsd) for XML.")
	fs.Var(&mergeSarifConfigFlags, "merge-sarif",
		"External SARIF file to merge into SARIF output. Requires --reporter=sarif.")
	fs.Var(&ignoreFileConfigFlags, "ignore-file",
		"Path to a gitignore-style ignore file. Can be specified multiple times.")

	if err := fs.Parse(args); err != nil {
		return cfvConfig{}, err
	}

	if err := applyDefaultFlagsFromEnv(fs); err != nil {
		return cfvConfig{}, err
	}
	setIgnoreFilesFromEnvIfNotSet(fs, &ignoreFileConfigFlags)

	reporterConf, err := parseReporterFlags(reporterConfigFlags)
	if err != nil {
		return cfvConfig{}, err
	}

	if err := validateGlobbing(fs, globbingPtr); err != nil {
		return cfvConfig{}, err
	}

	searchPaths, err := parseSearchPaths(fs, globbingPtr)
	if err != nil {
		return cfvConfig{}, err
	}

	if err := validateFlagValues(fs, excludeTypesPtr, fileTypesPtr, depthPtr, reporterConf, groupOutputPtr, mergeSarifConfigFlags, mergeSarifDirPtr); err != nil {
		return cfvConfig{}, err
	}

	return cfvConfig{
		fs:               fs,
		searchPaths:      searchPaths,
		excludeDirs:      excludeDirsPtr,
		excludeFileTypes: excludeTypesPtr,
		fileTypes:        fileTypesPtr,
		reportType:       reporterConf,
		depth:            depthPtr,
		groupOutput:      groupOutputPtr,
		quiet:            quietPtr,
		globbing:         globbingPtr,
		requireSchema:    requireSchemaPtr,
		noSchema:         noSchemaPtr,
		typeMap:          typeMapConfigFlags,
		schemaMap:        schemaMapConfigFlags,
		schemaStore:      schemaStorePtr,
		schemaStorePath:  schemaStorePathPtr,
		configPath:       configPathPtr,
		noConfig:         noConfigPtr,
		gitignore:        gitignorePtr,
		mergeSarif:       mergeSarifConfigFlags,
		mergeSarifDir:    mergeSarifDirPtr,
		ignoreFiles:      ignoreFileConfigFlags,
		fix:              fixPtr,
		unsafe:           unsafePtr,
	}, nil
}

// =============================================================================
// format subcommand (Phase 2 stub)
// =============================================================================

// runFormat is the Phase 2 stub. Prints a clear not-yet-implemented message
// rather than silently doing nothing or panicking.
func runFormat(args []string) int {
	// Register the flagset so --help works and the flag parser doesn't error
	// on valid flags that will be wired up in Phase 2.
	fs := flag.NewFlagSet("cfv format", flag.ContinueOnError)
	fs.Usage = printFormatUsage
	_ = fs.Bool("fix", false, "Rewrite files to canonical style [not yet implemented]")
	_ = fs.Bool("unsafe", false, "Apply unsafe formatting fixes (requires --fix) [not yet implemented]")

	if err := fs.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return 0
		}
		return 2
	}

	fmt.Fprintln(os.Stderr, "cfv format: not yet implemented (coming in Phase 2)")
	fmt.Fprintln(os.Stderr, "Use 'cfv check' for syntax and schema validation.")
	return 2
}

// =============================================================================
// Flag validation helpers
// =============================================================================

func validateFlagValues(fs *flag.FlagSet, excludeFileTypesPtr, fileTypesPtr *string, depthPtr *int, reporterConf []reporterConfig, groupOutputPtr *string, mergeSarif []string, mergeSarifDir *string) error {
	if err := validateReporterConf(reporterConf, groupOutputPtr); err != nil {
		return err
	}
	if depthPtr != nil && isFlagSet(fs, "depth") && *depthPtr < 0 {
		return errors.New("wrong parameter value for depth, value cannot be negative")
	}
	if err := validateFileTypeFlags(excludeFileTypesPtr, fileTypesPtr); err != nil {
		return err
	}
	if err := validateGroupByConf(fs, groupOutputPtr); err != nil {
		return err
	}
	return validateSARIFMergeConf(fs, reporterConf, mergeSarif, mergeSarifDir)
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

func validateReporterConf(conf []reporterConfig, groupBy *string) error {
	acceptedReportTypes := map[string]bool{"standard": true, "json": true, "junit": true, "sarif": true, "github": true}
	groupOutputReportTypes := map[string]bool{"standard": true, "json": true}

	for _, rc := range conf {
		if !acceptedReportTypes[rc.reportType] {
			return errors.New("wrong parameter value for reporter, only supports standard, json, junit, sarif, or github")
		}
		if !groupOutputReportTypes[rc.reportType] && groupBy != nil && *groupBy != "" {
			return errors.New("wrong parameter value for reporter, groupby is only supported for standard and JSON reports")
		}
	}
	return nil
}

func validateSARIFMergeConf(fs *flag.FlagSet, conf []reporterConfig, mergeSarif []string, mergeSarifDir *string) error {
	for _, path := range mergeSarif {
		if strings.TrimSpace(path) == "" {
			return errors.New("--merge-sarif requires a file path")
		}
	}
	if mergeSarifDir != nil && isFlagSet(fs, "merge-sarif-dir") && strings.TrimSpace(*mergeSarifDir) == "" {
		return errors.New("--merge-sarif-dir requires a directory path")
	}
	if isFlagSet(fs, "reporter") {
		return validateSARIFMergeReporters(conf, mergeSarif, mergeSarifDir)
	}
	return nil
}

func validateSARIFMergeReporters(conf []reporterConfig, mergeSarif []string, mergeSarifDir *string) error {
	if !sarifMergeRequested(mergeSarif, mergeSarifDir) {
		return nil
	}
	for _, rc := range conf {
		if rc.reportType == "sarif" {
			return nil
		}
	}
	return errors.New("--merge-sarif and --merge-sarif-dir require --reporter=sarif")
}

func sarifMergeRequested(mergeSarif []string, mergeSarifDir *string) bool {
	// mergeSarifDir is always non-nil (registered as a flag with default "").
	// It counts as requested only if it was explicitly set to a non-empty value.
	dirRequested := mergeSarifDir != nil && *mergeSarifDir != ""
	return len(mergeSarif) > 0 || dirRequested
}

func mergeSarifDirectoryValue(mergeSarifDir *string) string {
	if mergeSarifDir == nil {
		return ""
	}
	return *mergeSarifDir
}

func validateGlobbing(fs *flag.FlagSet, globbingPtr *bool) error {
	if *globbingPtr && (isFlagSet(fs, "exclude-dirs") || isFlagSet(fs, "exclude-file-types") || isFlagSet(fs, "file-types")) {
		return errors.New("the -globbing flag cannot be used with --exclude-dirs, --exclude-file-types, or --file-types")
	}
	return nil
}

func validateGroupByConf(fs *flag.FlagSet, groupBy *string) error {
	if groupBy == nil || !isFlagSet(fs, "groupby") {
		return nil
	}
	groupByCleanString := cleanString(fs, "groupby")
	groupByAllowedValues := []string{"filetype", "directory", "pass-fail", "error-type"}
	seenValues := make(map[string]bool)

	for _, val := range strings.Split(groupByCleanString, ",") {
		if !slices.Contains(groupByAllowedValues, val) {
			return errors.New("wrong parameter value for groupby, only supports filetype, directory, pass-fail, error-type")
		}
		if seenValues[val] {
			return errors.New("wrong parameter value for groupby, duplicate values are not allowed")
		}
		seenValues[val] = true
	}
	return nil
}

// getFileTypes returns all registered file type extension strings.
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

// validateFileTypeList returns true if every entry in input is a known file type.
func validateFileTypeList(input []string) bool {
	types := getFileTypes()
	for _, t := range input {
		if t == "" {
			continue
		}
		if !slices.Contains(types, t) {
			return false
		}
	}
	return true
}

// isFlagSet reports whether flagName was explicitly set by the user on fs.
func isFlagSet(fs *flag.FlagSet, flagName string) bool {
	if fs == nil {
		return false
	}
	var isSet bool
	fs.Visit(func(f *flag.Flag) {
		if f.Name == flagName {
			isSet = true
		}
	})
	return isSet
}

// cleanString returns the lowercased, trimmed value of the named flag on fs.
func cleanString(fs *flag.FlagSet, name string) string {
	s := fs.Lookup(name).Value.String()
	return strings.TrimSpace(strings.ToLower(s))
}

// isFlagSet reports whether flagName was explicitly set by the user.
func (c *cfvConfig) isFlagSet(flagName string) bool {
	return isFlagSet(c.fs, flagName)
}

// isGlobPattern reports whether s contains glob metacharacters.
func isGlobPattern(s string) bool {
	return tools.IsGlobPattern(s)
}

// =============================================================================
// Search path and reporter parsing
// =============================================================================

func parseSearchPaths(fs *flag.FlagSet, globbingPtr *bool) ([]string, error) {
	if fs.NArg() == 0 {
		return []string{"."}, nil
	}
	if *globbingPtr {
		return handleGlobbing(fs)
	}
	return fs.Args(), nil
}

func handleGlobbing(fs *flag.FlagSet) ([]string, error) {
	var searchPaths []string
	for _, arg := range fs.Args() {
		if isGlobPattern(arg) {
			matches, err := doublestar.Glob(os.DirFS("."), arg)
			if err != nil {
				return nil, errors.New("glob matching error")
			}
			searchPaths = append(searchPaths, matches...)
		} else {
			searchPaths = append(searchPaths, arg)
		}
	}
	return searchPaths, nil
}

func parseReporterFlags(flags reporterFlags) ([]reporterConfig, error) {
	conf := make([]reporterConfig, 0, len(flags))
	for _, reportFlag := range flags {
		parts := strings.SplitN(reportFlag, ":", 2)
		switch len(parts) {
		case 1:
			conf = append(conf, reporterConfig{reportType: parts[0]})
		case 2:
			if parts[1] == "-" {
				conf = append(conf, reporterConfig{reportType: parts[0]})
			} else {
				conf = append(conf, reporterConfig{reportType: parts[0], outputDest: parts[1]})
			}
		default:
			return nil, errors.New("wrong parameter value format for reporter, expected format is `report_type:optional_file_path`")
		}
	}
	if len(conf) == 0 {
		conf = append(conf, reporterConfig{reportType: "standard"})
	}
	return conf, validateUniqueReporterOutputDestinations(conf)
}

func validateUniqueReporterOutputDestinations(conf []reporterConfig) error {
	seen := make(map[string]struct{}, len(conf))
	for _, rc := range conf {
		if rc.outputDest == "" {
			continue
		}
		dest := filepath.Clean(rc.outputDest)
		if _, ok := seen[dest]; ok {
			return fmt.Errorf("multiple reporters target the same output file: %s", dest)
		}
		seen[dest] = struct{}{}
	}
	return nil
}

// getReporter constructs the reporter for the given type and output destination.
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

// =============================================================================
// Config resolution
// =============================================================================

func resolveConfig(cfg *cfvConfig) (*resolvedConfig, error) {
	validatorOpts, err := applyConfigFile(cfg)
	if err != nil {
		return nil, fmt.Errorf("loading config file: %w", err)
	}

	noSchema := *cfg.noSchema
	requireSchema := *cfg.requireSchema
	useSchemaStore := *cfg.schemaStore || *cfg.schemaStorePath != ""

	if noSchema && (requireSchema || len(cfg.schemaMap) > 0 || useSchemaStore) {
		return nil, errors.New("--no-schema cannot be used with --require-schema, --schema-map, or --schemastore")
	}

	if err := validateSARIFMergeReporters(cfg.reportType, cfg.mergeSarif, cfg.mergeSarifDir); err != nil {
		return nil, err
	}

	sarifMergeCfg := reporter.SARIFMergeConfig{
		Files:     []string(cfg.mergeSarif),
		Directory: mergeSarifDirectoryValue(cfg.mergeSarifDir),
	}
	reporters, err := buildReporters(cfg.reportType, sarifMergeCfg)
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
		quiet:         *cfg.quiet,
		requireSchema: requireSchema,
		noSchema:      noSchema,
		schemaMap:     schemaMap,
		store:         store,
	}

	// Handle stdin mode: single path of "-"
	stdinCount := 0
	for _, p := range cfg.searchPaths {
		if p == "-" {
			stdinCount++
		}
	}
	if stdinCount > 1 {
		return nil, errors.New("stdin (-) can only be specified once")
	}
	if stdinCount == 1 && len(cfg.searchPaths) > 1 {
		return nil, errors.New("stdin (-) cannot be combined with other search paths")
	}

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

func buildReporters(reporterConfigs []reporterConfig, sarifMergeCfg reporter.SARIFMergeConfig) ([]reporter.Reporter, error) {
	reporters := make([]reporter.Reporter, 0, len(reporterConfigs))
	for _, rc := range reporterConfigs {
		if rc.reportType == "sarif" {
			reporters = append(reporters, reporter.NewSARIFReporterWithMerge(rc.outputDest, sarifMergeCfg))
			continue
		}
		reporters = append(reporters, getReporter(rc.reportType, rc.outputDest))
	}
	return reporters, nil
}

func openSchemaStore(cfg *cfvConfig) (*schemastore.Store, error) {
	if *cfg.schemaStorePath != "" {
		store, err := schemastore.Open(*cfg.schemaStorePath)
		if err != nil {
			return nil, fmt.Errorf("opening schemastore: %w", err)
		}
		return store, nil
	}
	if *cfg.schemaStore {
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

// =============================================================================
// Finder options
// =============================================================================

func buildFinderOpts(cfg cfvConfig, excludeFileTypes []string, fileTypes []filetype.FileType) ([]finder.FSFinderOptions, error) {
	excludeDirs := strings.Split(*cfg.excludeDirs, ",")
	fsOpts := []finder.FSFinderOptions{
		finder.WithPathRoots(cfg.searchPaths...),
		finder.WithExcludeDirs(excludeDirs),
		finder.WithExcludeFileTypes(excludeFileTypes),
		finder.WithFileTypes(fileTypes),
	}

	if *cfg.fileTypes != "" {
		includeTypes := tools.ArrToMap(strings.Split(strings.ToLower(*cfg.fileTypes), ",")...)
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

	if cfg.depth != nil && cfg.isFlagSet("depth") {
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
	if len(cfg.ignoreFiles) > 0 {
		fsOpts = append(fsOpts, finder.WithIgnoreFiles([]string(cfg.ignoreFiles)))
	}

	return fsOpts, nil
}

func getExcludeFileTypes(configExcludeFileTypes string) []string {
	if configExcludeFileTypes == "" {
		return nil
	}
	excludeFileTypes := strings.Split(strings.ToLower(configExcludeFileTypes), ",")
	uniqueFileTypes := tools.ArrToMap(excludeFileTypes...)

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

	return slices.Collect(maps.Keys(uniqueFileTypes))
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
		ft, ok := fileTypesByName[strings.ToLower(parts[1])]
		if !ok {
			return nil, fmt.Errorf("unknown file type %q in type-map", parts[1])
		}
		overrides = append(overrides, finder.TypeOverride{Pattern: parts[0], FileType: ft})
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

// =============================================================================
// Environment variable defaults
// =============================================================================

func applyDefaultFlagsFromEnv(fs *flag.FlagSet) error {
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
		if err := setFlagFromEnvIfNotSet(fs, flagName, envVar); err != nil {
			return err
		}
	}
	return nil
}

func setFlagFromEnvIfNotSet(fs *flag.FlagSet, flagName, envVar string) error {
	if isFlagSet(fs, flagName) {
		return nil
	}
	if v, ok := os.LookupEnv(envVar); ok && v != "" {
		if err := fs.Set(flagName, v); err != nil {
			return err
		}
	}
	return nil
}

func setIgnoreFilesFromEnvIfNotSet(fs *flag.FlagSet, flags *ignoreFileFlags) {
	if isFlagSet(fs, "ignore-file") {
		return
	}
	v, ok := os.LookupEnv("CFV_IGNORE_FILES")
	if !ok || v == "" {
		return
	}
	for _, f := range strings.Split(v, ",") {
		f = strings.TrimSpace(f)
		if f != "" {
			*flags = append(*flags, f)
		}
	}
}

// =============================================================================
// Config file (.cfv.toml) handling
// =============================================================================

func applyConfigFile(cfg *cfvConfig) (*configfile.ValidatorOptions, error) {
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

	// CLI flag > env var (already applied to flagSet) > config file.
	if !cfg.isFlagSet("exclude-dirs") && len(fileCfg.ExcludeDirs) > 0 {
		v := strings.Join(fileCfg.ExcludeDirs, ",")
		cfg.excludeDirs = &v
	}
	if !cfg.isFlagSet("exclude-file-types") && len(fileCfg.ExcludeFileTypes) > 0 {
		v := strings.Join(fileCfg.ExcludeFileTypes, ",")
		cfg.excludeFileTypes = &v
	}
	if !cfg.isFlagSet("file-types") && len(fileCfg.FileTypes) > 0 {
		v := strings.Join(fileCfg.FileTypes, ",")
		cfg.fileTypes = &v
	}
	if !cfg.isFlagSet("depth") && fileCfg.Depth != nil {
		if err := cfg.fs.Set("depth", fmt.Sprintf("%d", *fileCfg.Depth)); err != nil {
			return nil, fmt.Errorf("config file depth: %w", err)
		}
		cfg.depth = fileCfg.Depth
	}
	if !cfg.isFlagSet("reporter") && len(fileCfg.Reporter) > 0 {
		conf, err := parseReporterFlags(reporterFlags(fileCfg.Reporter))
		if err != nil {
			return nil, fmt.Errorf("config file reporter: %w", err)
		}
		cfg.reportType = conf
	}
	if !cfg.isFlagSet("groupby") && len(fileCfg.GroupBy) > 0 {
		v := strings.Join(fileCfg.GroupBy, ",")
		cfg.groupOutput = &v
	}
	if !cfg.isFlagSet("quiet") && fileCfg.Quiet != nil {
		cfg.quiet = fileCfg.Quiet
	}
	if !cfg.isFlagSet("require-schema") && fileCfg.RequireSchema != nil {
		cfg.requireSchema = fileCfg.RequireSchema
	}
	if !cfg.isFlagSet("no-schema") && fileCfg.NoSchema != nil {
		cfg.noSchema = fileCfg.NoSchema
	}
	if !cfg.isFlagSet("schemastore") && fileCfg.SchemaStore != nil {
		cfg.schemaStore = fileCfg.SchemaStore
	}
	if !cfg.isFlagSet("schemastore-path") && fileCfg.SchemaStorePath != nil {
		cfg.schemaStorePath = fileCfg.SchemaStorePath
	}
	if !cfg.isFlagSet("globbing") && fileCfg.Globbing != nil {
		cfg.globbing = fileCfg.Globbing
	}
	if !cfg.isFlagSet("gitignore") && fileCfg.Gitignore != nil {
		cfg.gitignore = fileCfg.Gitignore
	}
	if !cfg.isFlagSet("ignore-file") && len(fileCfg.IgnoreFiles) > 0 {
		cfg.ignoreFiles = ignoreFileFlags(fileCfg.IgnoreFiles)
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

// =============================================================================
// Validator option application (per-format config from .cfv.toml)
// =============================================================================

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
			// no per-format validator options for this type
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
