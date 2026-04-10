package configfile

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	"github.com/xeipuuv/gojsonschema"
)

//go:embed schema.json
var configSchema []byte

const FileName = ".cfv.toml"

// Config represents the parsed .cfv.toml configuration file.
type Config struct {
	ExcludeDirs      []string          `toml:"exclude-dirs"`
	ExcludeFileTypes []string          `toml:"exclude-file-types"`
	FileTypes        []string          `toml:"file-types"`
	Depth            *int              `toml:"depth"`
	Reporter         []string          `toml:"reporter"`
	GroupBy          []string          `toml:"groupby"`
	Quiet            *bool             `toml:"quiet"`
	RequireSchema    *bool             `toml:"require-schema"`
	NoSchema         *bool             `toml:"no-schema"`
	SchemaStore      *bool             `toml:"schemastore"`
	SchemaStorePath  *string           `toml:"schemastore-path"`
	Globbing         *bool             `toml:"globbing"`
	SchemaMap        map[string]string `toml:"schema-map"`
	TypeMap          map[string]string `toml:"type-map"`
	Validators       ValidatorOptions  `toml:"validators"`
}

// ValidatorOptions holds per-validator configuration.
type ValidatorOptions struct {
	CSV  *CSVOptions  `toml:"csv"`
	JSON *JSONOptions `toml:"json"`
	INI  *INIOptions  `toml:"ini"`
}

// CSVOptions configures the CSV validator.
type CSVOptions struct {
	Delimiter  *string `toml:"delimiter"`
	Comment    *string `toml:"comment"`
	LazyQuotes *bool   `toml:"lazy-quotes"`
}

// JSONOptions configures the JSON validator.
type JSONOptions struct {
	ForbidDuplicateKeys *bool `toml:"forbid-duplicate-keys"`
}

// INIOptions configures the INI validator.
type INIOptions struct {
	ForbidDuplicateKeys *bool `toml:"forbid-duplicate-keys"`
}

// Load reads and validates a .cfv.toml file at the given path.
// It validates TOML syntax first, then validates against the embedded schema.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config file: %w", err)
	}

	// Validate TOML syntax
	var raw any
	if err := toml.Unmarshal(data, &raw); err != nil {
		return nil, fmt.Errorf("config file %s: invalid TOML syntax: %w", path, err)
	}

	// Convert to JSON for schema validation
	docJSON, err := json.Marshal(raw)
	if err != nil {
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}

	// Validate against embedded schema
	result, err := gojsonschema.Validate(
		gojsonschema.NewBytesLoader(configSchema),
		gojsonschema.NewBytesLoader(docJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("config file %s: schema validation error: %w", path, err)
	}
	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return nil, fmt.Errorf("config file %s: schema validation failed: %s", path, joinErrors(errs))
	}

	// Parse into Config struct
	var cfg Config
	if err := toml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("config file %s: %w", path, err)
	}

	return &cfg, nil
}

// Discover walks up from startDir looking for a .cfv.toml file.
// Returns the path if found, or empty string if not found.
func Discover(startDir string) string {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return ""
	}
	for {
		path := filepath.Join(dir, FileName)
		if _, err := os.Stat(path); err == nil {
			return path
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return ""
		}
		dir = parent
	}
}

func joinErrors(errs []string) string {
	if len(errs) == 1 {
		return errs[0]
	}
	result := errs[0]
	for _, e := range errs[1:] {
		result += "; " + e
	}
	return result
}
