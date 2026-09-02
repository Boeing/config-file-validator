package configfile

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/pelletier/go-toml/v2"
	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

//go:embed schema.json
var configSchema []byte

const FileName = ".cfv.toml"

// Config represents the parsed .cfv.toml configuration file.
type Config struct {
	ExcludeDirs      []string          `toml:"exclude-dirs"`
	ExcludeFileTypes []string          `toml:"exclude-file-types"`
	IgnoreFiles      []string          `toml:"ignore-files"`
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
	Gitignore        *bool             `toml:"gitignore"`
	Editorconfig     *bool             `toml:"editorconfig"`
	SchemaMap        map[string]string `toml:"schema-map"`
	TypeMap          map[string]string `toml:"type-map"`
	Validators       ValidatorOptions  `toml:"validators"`
	Format           FormatConfig      `toml:"format"`
}

// FormatConfig holds the [format] section and per-format overrides.
type FormatConfig struct {
	FormatOptions

	// Per-format overrides. Keys are format names: "json", "yaml", "hcl", etc.
	JSON       *FormatOptions `toml:"json"`
	JSONC      *FormatOptions `toml:"jsonc"`
	YAML       *FormatOptions `toml:"yaml"`
	HCL        *FormatOptions `toml:"hcl"`
	TOML       *FormatOptions `toml:"toml"`
	XML        *FormatOptions `toml:"xml"`
	INI        *FormatOptions `toml:"ini"`
	ENV        *FormatOptions `toml:"env"`
	Properties *FormatOptions `toml:"properties"`
}

// FormatOptions holds formatting configuration keys.
// All fields are pointers so we can distinguish "not set" from "set to zero/false".
// This allows correct cascade resolution: CLI > per-format > global > defaults.
type FormatOptions struct {
	Indent          *int    `toml:"indent"`
	UseTabs         *bool   `toml:"use-tabs"`
	SortKeys        *bool   `toml:"sort-keys"`
	TrailingNewline *bool   `toml:"trailing-newline"`
	LineEnding      *string `toml:"line-ending"`
	MaxLineWidth    *int    `toml:"max-line-width"`
	QuoteStyle      *string `toml:"quote-style"`
	TrailingCommas  *string `toml:"trailing-commas"`
	IndentSequences *bool   `toml:"indent-sequences"`
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
	schemaDoc, err := jsonschema.UnmarshalJSON(bytes.NewReader(configSchema))
	if err != nil {
		return nil, fmt.Errorf("config file %s: schema error: %w", path, err)
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(docJSON))
	if err != nil {
		return nil, fmt.Errorf("config file %s: schema error: %w", path, err)
	}
	compiler := jsonschema.NewCompiler()
	compiler.DefaultDraft(jsonschema.Draft7)
	if err := compiler.AddResource("cfv-config-schema.json", schemaDoc); err != nil {
		return nil, fmt.Errorf("config file %s: schema error: %w", path, err)
	}
	sch, err := compiler.Compile("cfv-config-schema.json")
	if err != nil {
		return nil, fmt.Errorf("config file %s: schema error: %w", path, err)
	}
	if err := sch.Validate(doc); err != nil {
		var verr *jsonschema.ValidationError
		if errors.As(err, &verr) {
			var errs []string
			basic := verr.BasicOutput()
			for _, unit := range basic.Errors {
				if unit.Error != nil {
					errs = append(errs, unit.Error.String())
				}
			}
			if len(errs) > 0 {
				return nil, fmt.Errorf("config file %s: schema validation failed: %s", path, joinErrors(errs))
			}
		}
		return nil, fmt.Errorf("config file %s: schema validation error: %w", path, err)
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
