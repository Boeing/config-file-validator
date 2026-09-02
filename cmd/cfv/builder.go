package main

import (
	"errors"
	"flag"
	"fmt"
	"io"
	"maps"
	"os"
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

// fileTypeFamilies groups file types that should be treated as a single family
// for --exclude-file-types and --file-types. Excluding one member excludes all.
var fileTypeFamilies = [][]string{
	{"json", "jsonc"},
}

func buildCLI(rc *resolvedConfig, extra ...cli.Option) *cli.CLI {
	opts := []cli.Option{
		cli.WithReporters(rc.reporters...),
		cli.WithGroupOutput(rc.groupOutput),
		cli.WithQuiet(rc.quiet),
		cli.WithRequireSchema(rc.requireSchema),
		cli.WithNoSchema(rc.noSchema),
		cli.WithSchemaMap(rc.schemaMap),
		cli.WithSchemaStore(rc.store),
		cli.WithConfigFile(rc.configFilePath),
		cli.WithFix(rc.fix),
		cli.WithDiff(rc.diff),
	}
	if rc.isStdin {
		opts = append(opts, cli.WithStdinData(rc.stdinData, rc.stdinFileType))
	} else {
		opts = append(opts, cli.WithFinder(finder.FileSystemFinderInit(rc.finderOpts...)))
	}
	opts = append(opts, extra...)
	return cli.Init(opts...)
}

// buildCLIWithFinder builds a CLI using the given finder instead of constructing
// one from finderOpts. Used by watch mode to validate a single changed file.
func buildCLIWithFinder(rc *resolvedConfig, f finder.FileFinder) *cli.CLI {
	opts := []cli.Option{
		cli.WithReporters(rc.reporters...),
		cli.WithGroupOutput(rc.groupOutput),
		cli.WithQuiet(rc.quiet),
		cli.WithRequireSchema(rc.requireSchema),
		cli.WithNoSchema(rc.noSchema),
		cli.WithSchemaMap(rc.schemaMap),
		cli.WithSchemaStore(rc.store),
		cli.WithConfigFile(rc.configFilePath),
		cli.WithFix(rc.fix),
		cli.WithDiff(rc.diff),
		cli.WithFinder(f),
	}
	return cli.Init(opts...)
}

func buildReporters(reporterConfigs []reporterConfig, sarifMergeCfg reporter.SARIFMergeConfig, isQuiet bool) ([]reporter.Reporter, error) {
	reporters := make([]reporter.Reporter, 0, len(reporterConfigs))
	for _, rc := range reporterConfigs {
		if rc.reportType == "sarif" {
			reporters = append(reporters, reporter.NewSARIFReporterWithMerge(rc.outputDest, configfilevalidator.GetVersion().Version, isQuiet, sarifMergeCfg))
			continue
		}
		reporters = append(reporters, getReporter(rc.reportType, rc.outputDest, isQuiet))
	}
	return reporters, nil
}

// getReporter constructs the reporter for the given type and output destination.
func getReporter(reportType, outputDest string, isQuiet bool) reporter.Reporter {
	switch reportType {
	case "junit":
		return reporter.NewJunitReporter(outputDest, isQuiet)
	case "json":
		return reporter.NewJSONReporter(outputDest, isQuiet)
	case "sarif":
		return reporter.NewSARIFReporter(outputDest, configfilevalidator.GetVersion().Version, isQuiet)
	case "github":
		return reporter.NewGitHubReporter(outputDest, isQuiet)
	default:
		return reporter.NewStdoutReporter(outputDest, isQuiet)
	}
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

func parseTypeMapFlags(flags repeatableFlag) ([]finder.TypeOverride, error) {
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

func parseSchemaMapFlags(flags repeatableFlag) ([]cli.SchemaMapping, error) {
	result := make([]cli.SchemaMapping, 0, len(flags))
	for _, mapping := range flags {
		parts := strings.SplitN(mapping, ":", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil, fmt.Errorf("invalid schema-map format %q, expected pattern:schema_path", mapping)
		}
		// Validate glob pattern syntax — catch typos like "[invalid" early.
		if _, err := doublestar.PathMatch(parts[0], ""); err != nil {
			return nil, fmt.Errorf("invalid glob pattern in --schema-map %q: %w", parts[0], err)
		}
		result = append(result, cli.SchemaMapping{Pattern: parts[0], SchemaPath: parts[1]})
	}
	return result, nil
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

// isGlobPattern reports whether s contains glob metacharacters.
func isGlobPattern(s string) bool {
	return tools.IsGlobPattern(s)
}

func parseReporterFlags(flags repeatableFlag) ([]reporterConfig, error) {
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

func openSchemaStore(cfg *cfvConfig) (*schemastore.Store, error) {
	if cfg.schemaStorePath != nil && *cfg.schemaStorePath != "" {
		store, err := schemastore.Open(*cfg.schemaStorePath)
		if err != nil {
			return nil, fmt.Errorf("opening schemastore: %w", err)
		}
		return store, nil
	}
	if cfg.schemaStore != nil && *cfg.schemaStore {
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
