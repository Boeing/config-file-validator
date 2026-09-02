package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/cli"
	"github.com/Boeing/config-file-validator/v3/pkg/configfile"
	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
	"github.com/Boeing/config-file-validator/v3/pkg/schemastore"
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
	typeMap          repeatableFlag
	schemaMap        repeatableFlag
	schemaStore      *bool
	schemaStorePath  *string
	configPath       *string
	noConfig         *bool
	gitignore        *bool
	mergeSarif       repeatableFlag
	mergeSarifDir    *string
	ignoreFiles      repeatableFlag
	// Phase 1: --fix and --unsafe are reserved (no-op) until Phase 4.
	fix    *bool
	unsafe *bool
	watch  *bool
	// Format option flags (cfv format only).
	fmtIndent         *int
	fmtUseTabs        *bool
	fmtSortKeys       *bool
	fmtNoSortKeys     *bool
	fmtLineEnding     *string
	fmtMaxLineWidth   *int
	fmtQuoteStyle     *string
	fmtDiff           *bool
	fmtNoEditorConfig *bool
}

// reporterConfig pairs a reporter format name with an optional output path.
type reporterConfig struct {
	reportType string
	outputDest string
}

// resolvedConfig is the final merged configuration passed to the CLI engine.
type resolvedConfig struct {
	reporters      []reporter.Reporter
	groupOutput    []string
	quiet          bool
	requireSchema  bool
	noSchema       bool
	schemaMap      []cli.SchemaMapping
	store          *schemastore.Store
	finderOpts     []finder.FSFinderOptions
	configFilePath string
	stdinData      []byte
	stdinFileType  filetype.FileType
	isStdin        bool
	fix            bool
	diff           bool
	formatCfg      *configfile.FormatConfig
	watch          bool
	searchPaths    []string
}

// --- Repeatable flag types ---

// repeatableFlag is a string slice that implements flag.Value for repeatable flags.
// Used for --reporter, --type-map, --schema-map, --merge-sarif, --ignore-file.
type repeatableFlag []string

func (rf *repeatableFlag) String() string { return fmt.Sprint(*rf) }
func (rf *repeatableFlag) Set(value string) error {
	*rf = append(*rf, value)
	return nil
}

// =============================================================================
// Config resolution
// =============================================================================

// reporterIsQuiet returns whether reporters should suppress stdout output.
// True when --quiet is set or in --diff mode (diff output replaces reporter output).
func (c *cfvConfig) reporterIsQuiet() bool {
	return *c.quiet || (c.fmtDiff != nil && *c.fmtDiff)
}

// resolveBaseConfig handles configuration shared by all subcommands:
// config file loading, reporters, groupOutput, quiet, fix, diff, stdin,
// and finder options. It does not touch schema-specific fields.
func resolveBaseConfig(cfg *cfvConfig, sarifMerge reporter.SARIFMergeConfig) (*resolvedConfig, *configfile.ValidatorOptions, error) {
	cfgFilePath, validatorOpts, formatCfg, err := applyConfigFile(cfg)
	if err != nil {
		return nil, nil, fmt.Errorf("loading config file: %w", err)
	}

	reporters, err := buildReporters(cfg.reportType, sarifMerge, cfg.reporterIsQuiet())
	if err != nil {
		return nil, nil, err
	}

	groupOutput := strings.Split(*cfg.groupOutput, ",")

	resolved := &resolvedConfig{
		reporters:   reporters,
		groupOutput: groupOutput,
		quiet:       *cfg.quiet,
		fix:         cfg.fix != nil && *cfg.fix,
		diff:        cfg.fmtDiff != nil && *cfg.fmtDiff,
		formatCfg:   formatCfg,
		watch:       cfg.watch != nil && *cfg.watch,
		searchPaths: cfg.searchPaths,
	}

	// Handle stdin mode: single path of "-"
	stdinCount := 0
	for _, p := range cfg.searchPaths {
		if p == "-" {
			stdinCount++
		}
	}
	if stdinCount > 1 {
		return nil, nil, errors.New("stdin (-) can only be specified once")
	}
	if stdinCount == 1 && len(cfg.searchPaths) > 1 {
		return nil, nil, errors.New("stdin (-) cannot be combined with other search paths")
	}

	if len(cfg.searchPaths) == 1 && cfg.searchPaths[0] == "-" {
		ft, data, err := readStdin(*cfg.fileTypes)
		if err != nil {
			return nil, nil, err
		}
		resolved.isStdin = true
		resolved.stdinData = data
		resolved.stdinFileType = ft
		return resolved, validatorOpts, nil
	}

	excludeFileTypes := getExcludeFileTypes(*cfg.excludeFileTypes)
	configuredTypes := applyValidatorOptions(validatorOpts)
	fsOpts, err := buildFinderOpts(*cfg, excludeFileTypes, configuredTypes)
	if err != nil {
		return nil, nil, err
	}
	resolved.finderOpts = fsOpts

	if cfgFilePath != "" {
		abs, err := filepath.Abs(cfgFilePath)
		if err == nil {
			resolved.configFilePath = abs
		}
	}

	return resolved, validatorOpts, nil
}

// resolveCheckConfig resolves configuration for the check subcommand.
// Adds schema validation, schema map, schema store, and SARIF merge
// on top of the base configuration.
func resolveCheckConfig(cfg *cfvConfig) (*resolvedConfig, error) {
	// SARIF merge config is built early so it can be passed to resolveBaseConfig,
	// which builds reporters once with both quiet and SARIF merge baked in.
	sarifMergeCfg := reporter.SARIFMergeConfig{
		Files:     []string(cfg.mergeSarif),
		Directory: mergeSarifDirectoryValue(cfg.mergeSarifDir),
	}

	resolved, _, err := resolveBaseConfig(cfg, sarifMergeCfg)
	if err != nil {
		return nil, err
	}

	// SARIF merge validation runs after resolveBaseConfig because the config
	// file (loaded inside resolveBaseConfig via applyConfigFile) may add SARIF
	// to the reporter list. Validating before would miss config-file reporters.
	if err := validateSARIFMergeReporters(cfg.reportType, cfg.mergeSarif, cfg.mergeSarifDir); err != nil {
		return nil, err
	}

	// Schema-specific resolution.
	noSchema := cfg.noSchema != nil && *cfg.noSchema
	requireSchema := cfg.requireSchema != nil && *cfg.requireSchema
	useSchemaStore := (cfg.schemaStore != nil && *cfg.schemaStore) ||
		(cfg.schemaStorePath != nil && *cfg.schemaStorePath != "")

	if noSchema && (requireSchema || len(cfg.schemaMap) > 0 || useSchemaStore) {
		return nil, errors.New("--no-schema cannot be used with --require-schema, --schema-map, or --schemastore")
	}

	resolved.requireSchema = requireSchema
	resolved.noSchema = noSchema

	schemaMap, err := parseSchemaMapFlags(cfg.schemaMap)
	if err != nil {
		return nil, err
	}
	resolved.schemaMap = schemaMap

	store, err := openSchemaStore(cfg)
	if err != nil {
		return nil, err
	}
	resolved.store = store

	return resolved, nil
}

// resolveFormatConfig resolves configuration for the format subcommand.
// No schema fields, no SARIF merge — just the base config.
func resolveFormatConfig(cfg *cfvConfig) (*resolvedConfig, error) {
	resolved, _, err := resolveBaseConfig(cfg, reporter.SARIFMergeConfig{})
	if err != nil {
		return nil, err
	}
	return resolved, nil
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
		"watch":              "CFV_WATCH",
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

func setIgnoreFilesFromEnvIfNotSet(fs *flag.FlagSet, flags *repeatableFlag) {
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

func applyConfigFile(cfg *cfvConfig) (string, *configfile.ValidatorOptions, *configfile.FormatConfig, error) {
	if *cfg.noConfig {
		if cfg.configPath != nil && *cfg.configPath != "" {
			return "", nil, nil, errors.New("--config cannot be used with --no-config")
		}
		return "", nil, nil, nil
	}

	var cfgPath string
	if *cfg.configPath != "" {
		cfgPath = *cfg.configPath
	} else {
		cfgPath = configfile.Discover(".")
	}
	if cfgPath == "" {
		return "", nil, nil, nil
	}

	fileCfg, err := configfile.Load(cfgPath)
	if err != nil {
		return "", nil, nil, err
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
			return "", nil, nil, fmt.Errorf("config file depth: %w", err)
		}
		cfg.depth = fileCfg.Depth
	}
	if !cfg.isFlagSet("reporter") && len(fileCfg.Reporter) > 0 {
		conf, err := parseReporterFlags(repeatableFlag(fileCfg.Reporter))
		if err != nil {
			return "", nil, nil, fmt.Errorf("config file reporter: %w", err)
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
		cfg.ignoreFiles = repeatableFlag(fileCfg.IgnoreFiles)
	}
	if len(cfg.schemaMap) == 0 && len(fileCfg.SchemaMap) > 0 {
		// Sort patterns for deterministic ordering (TOML maps are unordered).
		patterns := make([]string, 0, len(fileCfg.SchemaMap))
		for pattern := range fileCfg.SchemaMap {
			patterns = append(patterns, pattern)
		}
		slices.Sort(patterns)
		for _, pattern := range patterns {
			cfg.schemaMap = append(cfg.schemaMap, pattern+":"+fileCfg.SchemaMap[pattern])
		}
	}
	if len(cfg.typeMap) == 0 && len(fileCfg.TypeMap) > 0 {
		for pattern, typeName := range fileCfg.TypeMap {
			cfg.typeMap = append(cfg.typeMap, pattern+":"+typeName)
		}
	}

	return cfgPath, &fileCfg.Validators, &fileCfg.Format, nil
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
