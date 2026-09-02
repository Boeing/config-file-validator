package main

import (
	"fmt"
	"os"

	"github.com/Boeing/config-file-validator/v3/pkg/cli"
	"github.com/Boeing/config-file-validator/v3/pkg/configfile"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/envfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/inifmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/propfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/tomlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

// =============================================================================
// Format options resolution
// =============================================================================

// buildFormatOptionsResolver builds a function that resolves format options
// using the two-tier model:
//
// Tier 1 (.cfv.toml exists): .cfv.toml is sole authority. External tool configs NOT read.
//
//	defaults → .cfv.toml [format] → .cfv.toml [format.<type>] → CLI flags
//
// Tier 2 (no .cfv.toml): per-format tool config ownership, no stacking.
//
//	defaults → .editorconfig → tool config (ONE per format) → CLI flags
//
// Tool config ownership per format:
//
//	JSON/JSONC: .prettierrc
//	YAML: .yamlfmt (if found), else .prettierrc
//	TOML: taplo.toml
//	Others: .editorconfig only
//
// --no-config disables ALL config (pure defaults + CLI flags).
// --no-editorconfig disables .editorconfig only (Tier 2 tool configs still apply).
func buildFormatOptionsResolver(cfg *cfvConfig, rc *resolvedConfig) (cli.FormatOptionsFunc, *formatter.PrettierConfig) {
	// --no-config: pure defaults + CLI flags only. Skip everything.
	if cfg.noConfig != nil && *cfg.noConfig {
		return func(formatName, _ string) formatter.Options {
			opts := formatDefaults(formatName)
			applyCLIFormatFlags(&opts, cfg)
			return opts
		}, nil
	}

	// Tier 1: .cfv.toml exists — sole authority.
	if rc.formatCfg != nil {
		globalCfg := &rc.formatCfg.FormatOptions
		perFormatCfg := map[string]*configfile.FormatOptions{
			"json":       rc.formatCfg.JSON,
			"jsonc":      rc.formatCfg.JSONC,
			"yaml":       rc.formatCfg.YAML,
			"hcl":        rc.formatCfg.HCL,
			"toml":       rc.formatCfg.TOML,
			"xml":        rc.formatCfg.XML,
			"ini":        rc.formatCfg.INI,
			"env":        rc.formatCfg.ENV,
			"properties": rc.formatCfg.Properties,
		}

		return func(formatName, _ string) formatter.Options {
			opts := formatDefaults(formatName)

			// .cfv.toml [format] (global section)
			applyFormatOptions(&opts, globalCfg)

			// .cfv.toml [format.<type>] (per-format section)
			if perFmt := perFormatCfg[formatName]; perFmt != nil {
				applyFormatOptions(&opts, perFmt)
			}

			// CLI flags (highest priority)
			applyCLIFormatFlags(&opts, cfg)
			return opts
		}, nil
	}

	// Tier 2: no .cfv.toml — per-format tool config ownership.
	var editorCfg *formatter.EditorConfig
	if cfg.fmtNoEditorConfig == nil || !*cfg.fmtNoEditorConfig {
		editorCfg = formatter.NewEditorConfig()
	}

	// Load tool configs once (shared across all file resolutions).
	prettierCfg := formatter.NewPrettierConfig()
	taploCfg := formatter.LoadTaplo(".")
	yamlfmtCfg := formatter.LoadYamlfmt(".")

	return func(formatName, path string) formatter.Options {
		opts := formatDefaults(formatName)

		// A zero default width means the format is not indented at all
		// (TOML keys under a table header). Neither editorconfig nor tool
		// configs may reintroduce indentation. CLI flags still can.
		unindented := opts.IndentWidth == 0

		// Base layer: .editorconfig (per-file glob matching)
		if editorCfg != nil {
			editorCfg.Apply(&opts, path)
		}

		if unindented {
			opts.IndentWidth = 0
		}

		// Tool config layer: ONE tool per format, no stacking.
		switch formatName {
		case "json", "jsonc":
			// .prettierrc owns JSON/JSONC formatting.
			prettierCfg.Apply(&opts, path)
		case "yaml":
			// .yamlfmt owns YAML if found; otherwise .prettierrc.
			if yamlfmtCfg != nil {
				yamlfmtCfg.Apply(&opts)
			} else {
				prettierCfg.Apply(&opts, path)
			}
		case "toml":
			// taplo.toml owns TOML formatting.
			if taploCfg != nil {
				taploCfg.Apply(&opts)
			}
		default:
			// HCL, XML, INI, Properties, ENV: editorconfig only.
		}

		// CLI flags (highest priority)
		applyCLIFormatFlags(&opts, cfg)
		return opts
	}, prettierCfg
}

// printPrettierWarnings emits any .prettierrc config warnings to stderr.
// Safe to call with nil (no-op when prettier config was not used).
func printPrettierWarnings(pc *formatter.PrettierConfig) {
	if pc == nil {
		return
	}
	for _, w := range pc.Warnings() {
		fmt.Fprintf(os.Stderr, "cfv: warning: %s\n", w)
	}
}

// buildFormatIgnores constructs the format-ignore matcher from external tool configs.
// Returns nil in Tier 1 (.cfv.toml exists) or when --no-config is set.
// In Tier 2, loads .prettierignore, taplo exclude, and yamlfmt exclude/.yamlfmtignore.
func buildFormatIgnores(cfg *cfvConfig, rc *resolvedConfig) *formatter.FormatIgnores {
	// --no-config: no config discovery at all.
	if cfg.noConfig != nil && *cfg.noConfig {
		return nil
	}

	// Tier 1: .cfv.toml exists — external ignores not read.
	if rc.formatCfg != nil {
		return nil
	}

	// Tier 2: load ignores from the same tool configs used for formatting.
	taploCfg := formatter.LoadTaplo(".")
	yamlfmtCfg := formatter.LoadYamlfmt(".")

	return formatter.BuildFormatIgnores(".", taploCfg, yamlfmtCfg)
}

// formatDefaults returns the hardcoded default options for a specific format.
// formatDefaults returns the default formatting options for a specific format.
// Each format defines its canonical defaults in its own package. This function
// delegates to those packages — single source of truth, no drift.
func formatDefaults(formatName string) formatter.Options {
	switch formatName {
	case "json":
		return jsonfmt.DefaultOptions()
	case "jsonc":
		return jsoncfmt.DefaultOptions()
	case "yaml":
		return yamlfmt.DefaultOptions()
	case "toml":
		return tomlfmt.DefaultOptions()
	case "xml":
		return xmlfmt.DefaultOptions()
	case "hcl":
		return hclfmt.DefaultOptions()
	case "ini":
		return inifmt.DefaultOptions()
	case "properties":
		return propfmt.DefaultOptions()
	case "env":
		return envfmt.DefaultOptions()
	default:
		// Unknown format — safe fallback with common defaults.
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			LineEnding:   formatter.LineEndingLF,
		}
	}
}

// applyFormatOptions overlays non-nil config values onto opts.
func applyFormatOptions(opts *formatter.Options, cfg *configfile.FormatOptions) {
	if cfg.Indent != nil {
		opts.IndentWidth = *cfg.Indent
	}
	if cfg.UseTabs != nil && *cfg.UseTabs {
		opts.IndentStyle = formatter.IndentTabs
	}
	if cfg.SortKeys != nil {
		opts.SortKeys = *cfg.SortKeys
	}
	if cfg.TrailingNewline != nil {
		opts.FinalNewline = *cfg.TrailingNewline
	}
	if cfg.LineEnding != nil {
		switch *cfg.LineEnding {
		case "crlf":
			opts.LineEnding = formatter.LineEndingCRLF
		default:
			opts.LineEnding = formatter.LineEndingLF
		}
	}
	if cfg.MaxLineWidth != nil {
		opts.MaxLineWidth = *cfg.MaxLineWidth
	}
	if cfg.QuoteStyle != nil {
		switch *cfg.QuoteStyle {
		case "double":
			opts.QuoteStyle = formatter.QuoteDouble
		case "single":
			opts.QuoteStyle = formatter.QuoteSingle
		default:
			opts.QuoteStyle = formatter.QuotePreserve
		}
	}
	if cfg.TrailingCommas != nil {
		switch *cfg.TrailingCommas {
		case "all":
			opts.TrailingCommas = formatter.TrailingCommasAll
		case "none":
			opts.TrailingCommas = formatter.TrailingCommasNone
		default:
			opts.TrailingCommas = formatter.TrailingCommasPreserve
		}
	}
	if cfg.IndentSequences != nil {
		if *cfg.IndentSequences {
			opts.IndentSequences = formatter.SequenceIndentEnabled
		} else {
			opts.IndentSequences = formatter.SequenceIndentDisabled
		}
	}
}

// applyCLIFormatFlags overlays explicitly-set CLI flags onto opts.
func applyCLIFormatFlags(opts *formatter.Options, cfg *cfvConfig) {
	if isFlagSet(cfg.fs, "indent") && *cfg.fmtIndent > 0 {
		opts.IndentWidth = *cfg.fmtIndent
	}
	if isFlagSet(cfg.fs, "use-tabs") {
		opts.IndentStyle = formatter.IndentTabs
	}
	if isFlagSet(cfg.fs, "sort-keys") {
		opts.SortKeys = true
	}
	if isFlagSet(cfg.fs, "no-sort-keys") {
		opts.SortKeys = false
	}
	if isFlagSet(cfg.fs, "line-ending") {
		switch *cfg.fmtLineEnding {
		case "crlf":
			opts.LineEnding = formatter.LineEndingCRLF
		default:
			opts.LineEnding = formatter.LineEndingLF
		}
	}
	if isFlagSet(cfg.fs, "max-line-width") {
		opts.MaxLineWidth = *cfg.fmtMaxLineWidth
	}
	if isFlagSet(cfg.fs, "quote-style") {
		switch *cfg.fmtQuoteStyle {
		case "double":
			opts.QuoteStyle = formatter.QuoteDouble
		case "single":
			opts.QuoteStyle = formatter.QuoteSingle
		default:
			opts.QuoteStyle = formatter.QuotePreserve
		}
	}
}
