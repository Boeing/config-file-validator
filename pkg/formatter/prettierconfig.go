package formatter

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/pelletier/go-toml/v2"
	"github.com/tailscale/hujson"
	"gopkg.in/yaml.v3"
)

// prettierConfigNames lists supported .prettierrc file names, in discovery
// priority order within a single directory.
//
// .prettierrc.js, .prettierrc.cjs, .prettierrc.mjs, and prettier.config.*
// require JS evaluation and are intentionally not supported: they are
// skipped rather than treated as an error, and the search continues
// upward in case a supported file exists in a parent directory.
var prettierConfigNames = []string{
	".prettierrc",
	".prettierrc.json",
	".prettierrc.yaml",
	".prettierrc.yml",
	".prettierrc.toml",
}

// prettierRC holds the subset of prettier options cfv maps onto formatter.Options.
// Pointer fields distinguish "not set in the file" from "set to zero/false".
type prettierRC struct {
	TabWidth      *int    `json:"tabWidth"      yaml:"tabWidth"      toml:"tabWidth"`
	UseTabs       *bool   `json:"useTabs"       yaml:"useTabs"       toml:"useTabs"`
	PrintWidth    *int    `json:"printWidth"    yaml:"printWidth"    toml:"printWidth"`
	EndOfLine     *string `json:"endOfLine"     yaml:"endOfLine"     toml:"endOfLine"`
	TrailingComma *string `json:"trailingComma" yaml:"trailingComma" toml:"trailingComma"`
	SingleQuote   *bool   `json:"singleQuote"   yaml:"singleQuote"   toml:"singleQuote"`
}

// PrettierConfig resolves .prettierrc settings for individual files.
//
// Resolution is per-file: cfv walks up from the file's directory and uses
// the nearest directory containing a supported .prettierrc variant. Unlike
// .editorconfig, prettier configs are not merged across directory levels —
// the closest file found entirely determines the result.
//
// The zero value is not usable; use NewPrettierConfig.
//
// Safe for concurrent use.
type PrettierConfig struct {
	mu sync.Mutex
	// dirCache maps a starting directory to the resolved config found by
	// walking up from it (nil = none found).
	dirCache map[string]*prettierRC
	// fileCache maps a candidate config file's path to its parsed contents
	// (nil = file absent or malformed), so a config shared by many
	// directories in a walk is only parsed once.
	fileCache map[string]*prettierRC
	// warnings collects diagnostic messages about config issues (e.g.,
	// type mismatches in .prettierrc). Callers retrieve them via Warnings().
	warnings []string
}

// NewPrettierConfig returns a PrettierConfig backed by a cache, so a
// .prettierrc shared by many source files is only parsed once.
func NewPrettierConfig() *PrettierConfig {
	return &PrettierConfig{
		dirCache:  make(map[string]*prettierRC),
		fileCache: make(map[string]*prettierRC),
	}
}

// Warnings returns diagnostic messages collected during config resolution
// (e.g., type mismatches in .prettierrc fields). Each message is a
// human-readable string without a trailing newline. Safe for concurrent use.
func (p *PrettierConfig) Warnings() []string {
	p.mu.Lock()
	w := p.warnings
	p.warnings = nil
	p.mu.Unlock()
	return w
}

// Apply overlays the resolved .prettierrc properties for path onto opts.
// Properties that are absent, unset, or not representable as an Option are
// left alone. A missing, malformed, or unsupported (JS-based) config is
// ignored rather than reported.
func (p *PrettierConfig) Apply(opts *Options, path string) {
	p.mu.Lock()
	rc := p.resolve(filepath.Dir(path))
	p.mu.Unlock()
	if rc == nil {
		return
	}

	if rc.TabWidth != nil && *rc.TabWidth > 0 {
		opts.IndentWidth = *rc.TabWidth
	}
	if rc.UseTabs != nil {
		if *rc.UseTabs {
			opts.IndentStyle = IndentTabs
		} else {
			opts.IndentStyle = IndentSpaces
		}
	}
	if rc.PrintWidth != nil {
		opts.MaxLineWidth = *rc.PrintWidth
	}
	if rc.EndOfLine != nil {
		switch *rc.EndOfLine {
		case "lf":
			opts.LineEnding = LineEndingLF
		case "crlf":
			opts.LineEnding = LineEndingCRLF
		default:
			// "auto" (preserve existing) and "cr" have no cfv equivalent.
		}
	}
	if rc.TrailingComma != nil {
		switch *rc.TrailingComma {
		case "all", "es5":
			// "es5" adds trailing commas wherever ES5 allows them. For
			// JSON/JSONC (the only formats cfv applies this to), every
			// array/object position is valid ES5, so es5 ≡ all.
			opts.TrailingCommas = TrailingCommasAll
		case "none":
			opts.TrailingCommas = TrailingCommasNone
		default:
			opts.TrailingCommas = TrailingCommasPreserve
		}
	}
	if rc.SingleQuote != nil {
		if *rc.SingleQuote {
			opts.QuoteStyle = QuoteSingle
		} else {
			opts.QuoteStyle = QuoteDouble
		}
	}
}

// resolve returns the parsed .prettierrc that applies to files in dir,
// walking up the directory tree and caching results per starting directory.
// Not safe for concurrent use; callers must hold p.mu.
func (p *PrettierConfig) resolve(dir string) *prettierRC {
	if rc, ok := p.dirCache[dir]; ok {
		return rc
	}

	rc := p.discover(dir)
	p.dirCache[dir] = rc
	return rc
}

func (p *PrettierConfig) discover(dir string) *prettierRC {
	for {
		if rc := p.parseDir(dir); rc != nil {
			return rc
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// parseDir looks for a supported .prettierrc variant directly in dir and
// parses the first one found, in priority order. It does not recurse.
func (p *PrettierConfig) parseDir(dir string) *prettierRC {
	for _, name := range prettierConfigNames {
		path := filepath.Join(dir, name)

		if rc, ok := p.fileCache[path]; ok {
			if rc != nil {
				return rc
			}
			continue
		}

		src, err := os.ReadFile(path)
		if err != nil {
			p.fileCache[path] = nil
			continue
		}

		rc := parsePrettierRC(name, src)
		p.fileCache[path] = rc
		if rc != nil {
			p.collectTypeMismatches(path, name, src, rc)
			return rc
		}
	}
	return nil
}

// parsePrettierRC parses src according to name's format. A malformed file
// returns nil rather than an error, matching the "ignore, don't fail the
// run" behavior used for .editorconfig.
//
// For JSON configs, type mismatches (e.g., "tabWidth": "4") are tolerated:
// the correctly-typed fields are used and the mismatched ones fall through
// to defaults. The caller should invoke warnPrettierTypeMismatches to emit
// user-facing diagnostics.
func parsePrettierRC(name string, src []byte) *prettierRC {
	var rc prettierRC

	switch name {
	case ".prettierrc.yaml", ".prettierrc.yml":
		if err := yaml.Unmarshal(src, &rc); err != nil {
			return nil
		}
	case ".prettierrc.toml":
		if err := toml.Unmarshal(src, &rc); err != nil {
			return nil
		}
	default:
		// .prettierrc and .prettierrc.json: auto-detect by trying JSON
		// first (tolerating comments/trailing commas via hujson), then
		// falling back to YAML since a bare .prettierrc may be either.
		std, err := hujson.Standardize(src)
		if err == nil {
			if err := json.Unmarshal(std, &rc); err == nil {
				return &rc
			}
			// Type mismatches: json.Unmarshal partially populates the
			// struct but returns an error. Parse via raw map to extract
			// only correctly-typed values.
			var raw map[string]any
			if json.Unmarshal(std, &raw) == nil && len(raw) > 0 {
				return parsePrettierRCFromRaw(raw)
			}
		}
		if name == ".prettierrc" {
			if err := yaml.Unmarshal(src, &rc); err == nil {
				return &rc
			}
		}
		return nil
	}

	return &rc
}

// parsePrettierRCFromRaw builds a prettierRC from a raw JSON map, extracting
// only values with correct types. This is the fallback path when
// json.Unmarshal into the typed struct fails due to type mismatches.
func parsePrettierRCFromRaw(raw map[string]any) *prettierRC {
	rc := &prettierRC{}
	if v, ok := raw["tabWidth"].(float64); ok {
		i := int(v)
		rc.TabWidth = &i
	}
	if v, ok := raw["printWidth"].(float64); ok {
		i := int(v)
		rc.PrintWidth = &i
	}
	if v, ok := raw["useTabs"].(bool); ok {
		rc.UseTabs = &v
	}
	if v, ok := raw["endOfLine"].(string); ok {
		rc.EndOfLine = &v
	}
	if v, ok := raw["trailingComma"].(string); ok {
		rc.TrailingComma = &v
	}
	if v, ok := raw["singleQuote"].(bool); ok {
		rc.SingleQuote = &v
	}
	return rc
}

// collectTypeMismatches appends warnings to p.warnings when a .prettierrc
// contains keys with wrong types (e.g., "tabWidth": "4" instead of
// "tabWidth": 4). Only JSON-format configs are checked since YAML and TOML
// parsers report type errors during unmarshal rather than silently dropping.
//
// Must be called under p.mu.
func (p *PrettierConfig) collectTypeMismatches(path, name string, src []byte, rc *prettierRC) {
	// Only JSON-parsed configs have the silent-nil problem. YAML/TOML
	// parsers return errors on type mismatch rather than silently skipping.
	if name == ".prettierrc.yaml" || name == ".prettierrc.yml" || name == ".prettierrc.toml" {
		return
	}

	std, err := hujson.Standardize(src)
	if err != nil {
		return
	}
	var raw map[string]any
	if json.Unmarshal(std, &raw) != nil {
		return
	}

	if v, ok := raw["tabWidth"]; ok && rc.TabWidth == nil {
		p.warnings = append(p.warnings, fmt.Sprintf("%s: tabWidth: expected number, got %T; using default", path, v))
	}
	if v, ok := raw["printWidth"]; ok && rc.PrintWidth == nil {
		p.warnings = append(p.warnings, fmt.Sprintf("%s: printWidth: expected number, got %T; using default", path, v))
	}
	if v, ok := raw["useTabs"]; ok && rc.UseTabs == nil {
		p.warnings = append(p.warnings, fmt.Sprintf("%s: useTabs: expected boolean, got %T; using default", path, v))
	}
	if v, ok := raw["singleQuote"]; ok && rc.SingleQuote == nil {
		p.warnings = append(p.warnings, fmt.Sprintf("%s: singleQuote: expected boolean, got %T; using default", path, v))
	}
}
