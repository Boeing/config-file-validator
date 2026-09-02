package main

import (
	"errors"
	"flag"
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
)

// parseCheckFlags registers and parses all flags for the check subcommand.
func parseCheckFlags(args []string) (cfvConfig, error) {
	fs := flag.NewFlagSet("cfv check", flag.ContinueOnError)
	fs.Usage = printCheckUsage

	reporterConfigFlags := repeatableFlag{}
	typeMapConfigFlags := repeatableFlag{}
	schemaMapConfigFlags := repeatableFlag{}
	mergeSarifConfigFlags := repeatableFlag{}
	ignoreFileConfigFlags := repeatableFlag{}

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
				"Supported types: YAML (yaml-language-server comment),\n"+
				"XML (xsi:noNamespaceSchemaLocation). JSON/JSONC/TOML/TOON require --schema-map or --schemastore.\n"+
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
			"Disable all config file discovery (.cfv.toml, .prettierrc, taplo.toml, .yamlfmt, .editorconfig).")
		gitignorePtr = fs.Bool("gitignore", false,
			"Skip files and directories matched by .gitignore patterns.")
		mergeSarifDirPtr = fs.String("merge-sarif-dir", "",
			"Directory tree containing SARIF files to merge into SARIF output. Requires --reporter=sarif.")
		// Phase 1: --fix and --unsafe are reserved. No-op until Phase 4.
		fixPtr    = fs.Bool("fix", false, "Apply safe fixes automatically (trailing commas, schema coercion, formatting)")
		unsafePtr = fs.Bool("unsafe", false, "Apply unsafe fixes (requires --fix)")
		watchPtr  = fs.Bool("watch", false, "Watch search paths for file changes and re-run validation.")
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
		watch:            watchPtr,
	}, nil
}

// parseFormatFlags registers and parses flags for the format subcommand.
// Format reuses most of the same flags as check (same finder infrastructure)
// but drops schema-specific flags.
func parseFormatFlags(args []string) (cfvConfig, error) {
	fs := flag.NewFlagSet("cfv format", flag.ContinueOnError)
	fs.Usage = printFormatUsage

	reporterConfigFlags := repeatableFlag{}
	typeMapConfigFlags := repeatableFlag{}
	ignoreFileConfigFlags := repeatableFlag{}

	var (
		depthPtr        = fs.Int("depth", 0, "Depth of recursion for the provided search paths. Set depth to 0 to disable recursive path traversal")
		excludeDirsPtr  = fs.String("exclude-dirs", "", "Subdirectories to exclude when searching for configuration files")
		excludeTypesPtr = fs.String("exclude-file-types", "", "A comma separated list of file types to ignore")
		fileTypesPtr    = fs.String("file-types", "", "A comma separated list of file types to format")
		groupOutputPtr  = fs.String("groupby", "", "Group output by filetype, directory, pass-fail. Supported for Standard and JSON reports")
		quietPtr        = fs.Bool("quiet", false, "If quiet flag is set, no output is printed to stdout")
		globbingPtr     = fs.Bool("globbing", false, "Enable glob pattern matching for search paths")
		configPathPtr   = fs.String("config", "",
			"Path to a .cfv.toml configuration file.")
		noConfigPtr  = fs.Bool("no-config", false, "Disable all config file discovery (.cfv.toml, .prettierrc, taplo.toml, .yamlfmt, .editorconfig)")
		gitignorePtr = fs.Bool("gitignore", false, "Skip files matched by .gitignore patterns.")
		fixPtr       = fs.Bool("fix", false, "Rewrite files to canonical style")
		unsafePtr    = fs.Bool("unsafe", false, "Apply unsafe formatting fixes (requires --fix) [not yet implemented]")
		// Format option flags.
		fmtIndentPtr         = fs.Int("indent", 0, "Override indent width (1-16). 0 = use config/default.")
		fmtUseTabsPtr        = fs.Bool("use-tabs", false, "Use tabs for indentation")
		fmtSortKeysPtr       = fs.Bool("sort-keys", false, "Sort object/mapping keys alphabetically")
		fmtNoSortKeysPtr     = fs.Bool("no-sort-keys", false, "Disable key sorting (overrides config)")
		fmtLineEndingPtr     = fs.String("line-ending", "", "Line ending: lf, crlf")
		fmtMaxLineWidthPtr   = fs.Int("max-line-width", 0, "Max line width hint (0 = unlimited)")
		fmtQuoteStylePtr     = fs.String("quote-style", "", "Quote style: double, single, preserve")
		fmtNoEditorConfigPtr = fs.Bool("no-editorconfig", false, "Ignore .editorconfig files when resolving format options")
		fmtDiffPtr           = fs.Bool("diff", false, "Show unified diff instead of rewriting (implies no --fix)")
	)

	fs.Var(&reporterConfigFlags, "reporter",
		"Report format and optional output path.\n"+
			"Format: <type>:<path>  Supported: standard, json, junit, sarif, github")
	fs.Var(&typeMapConfigFlags, "type-map",
		"Map a glob pattern to a file type. Format: <pattern>:<type>")
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

	// Validate flag values (no schema flags for format).
	if err := validateReporterConf(reporterConf, groupOutputPtr); err != nil {
		return cfvConfig{}, err
	}
	if depthPtr != nil && isFlagSet(fs, "depth") && *depthPtr < 0 {
		return cfvConfig{}, errors.New("wrong parameter value for depth, value cannot be negative")
	}
	if err := validateFileTypeFlags(excludeTypesPtr, fileTypesPtr); err != nil {
		return cfvConfig{}, err
	}
	if err := validateGroupByConf(fs, groupOutputPtr); err != nil {
		return cfvConfig{}, err
	}
	if err := validateFormatFlags(fs, fmtIndentPtr, fmtLineEndingPtr, fmtQuoteStylePtr); err != nil {
		return cfvConfig{}, err
	}

	// Schema fields are nil for format — resolveFormatConfig does not use them.

	return cfvConfig{
		fs:                fs,
		searchPaths:       searchPaths,
		excludeDirs:       excludeDirsPtr,
		excludeFileTypes:  excludeTypesPtr,
		fileTypes:         fileTypesPtr,
		reportType:        reporterConf,
		depth:             depthPtr,
		groupOutput:       groupOutputPtr,
		quiet:             quietPtr,
		globbing:          globbingPtr,
		configPath:        configPathPtr,
		noConfig:          noConfigPtr,
		gitignore:         gitignorePtr,
		ignoreFiles:       ignoreFileConfigFlags,
		fix:               fixPtr,
		unsafe:            unsafePtr,
		fmtIndent:         fmtIndentPtr,
		fmtUseTabs:        fmtUseTabsPtr,
		fmtSortKeys:       fmtSortKeysPtr,
		fmtNoSortKeys:     fmtNoSortKeysPtr,
		fmtLineEnding:     fmtLineEndingPtr,
		fmtMaxLineWidth:   fmtMaxLineWidthPtr,
		fmtQuoteStyle:     fmtQuoteStylePtr,
		fmtDiff:           fmtDiffPtr,
		fmtNoEditorConfig: fmtNoEditorConfigPtr,
	}, nil
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

// validateFormatFlags checks format-specific CLI flags for mutual exclusion,
// range violations, and invalid enum values.
func validateFormatFlags(fs *flag.FlagSet, indent *int, lineEnding, quoteStyle *string) error {
	if isFlagSet(fs, "sort-keys") && isFlagSet(fs, "no-sort-keys") {
		return errors.New("--sort-keys and --no-sort-keys cannot be used together")
	}
	if isFlagSet(fs, "fix") && isFlagSet(fs, "diff") {
		return errors.New("--fix and --diff cannot be used together")
	}
	if isFlagSet(fs, "indent") && (*indent < 1 || *indent > 16) {
		return errors.New("--indent must be between 1 and 16")
	}
	if isFlagSet(fs, "line-ending") {
		switch *lineEnding {
		case "lf", "crlf":
			// valid
		default:
			return fmt.Errorf("--line-ending must be \"lf\" or \"crlf\", got %q", *lineEnding)
		}
	}
	if isFlagSet(fs, "quote-style") {
		switch *quoteStyle {
		case "double", "single", "preserve":
			// valid
		default:
			return fmt.Errorf("--quote-style must be \"double\", \"single\", or \"preserve\", got %q", *quoteStyle)
		}
	}
	return nil
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
