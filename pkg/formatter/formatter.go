// Package formatter defines the interface for config file formatters.
//
// A Formatter transforms source bytes into a canonically formatted version.
// Each formatter implementation lives in its own sub-package (e.g.,
// formatter/json, formatter/yaml) and is registered on the filetype.FileType
// it handles.
//
// Design constraints:
//   - Stateless: Format may be called concurrently on different files.
//   - Idempotent: Format(Format(x, opts), opts) == Format(x, opts).
//   - Comment-preserving: output must contain every comment from the input.
//   - Consistent output: all formatters report issues using the same message
//     shape so cfv's output looks like one tool, not a patchwork.
package formatter

// Formatter transforms source bytes into canonically formatted output.
//
// Implementations must be stateless and safe for concurrent use.
// Implementations must preserve all comments present in the source.
type Formatter interface {
	// Format returns the canonically formatted version of src.
	// If src is already canonical, Format returns src unchanged (byte-equal).
	// Returns an error if src cannot be parsed (unparseable input is not
	// a formatting issue — it's a syntax error handled by the validator).
	Format(src []byte, opts Options) ([]byte, error)
}

// Options controls formatting behavior. Each formatter uses the fields that
// apply to its format and ignores the rest. Zero values mean "use the
// format-specific default."
//
// Options are resolved by the CLI before being passed to a formatter:
//
//	CLI flags > .cfv.toml [format.<type>] > .cfv.toml [format] > taplo.toml > .editorconfig > hardcoded defaults
type Options struct {
	// IndentStyle selects spaces or tabs. Zero value = format default.
	IndentStyle IndentStyle

	// IndentWidth is spaces per indent level. Ignored when IndentStyle is Tabs.
	// Zero value = format default.
	IndentWidth int

	// FinalNewline ensures the file ends with exactly one newline.
	FinalNewline bool

	// LineEnding selects the line terminator for output.
	LineEnding LineEnding

	// SortKeys sorts object/map keys alphabetically when true.
	SortKeys bool

	// MaxLineWidth is the target maximum line width. 0 = no limit.
	// Formatters use this as a hint, not a hard constraint.
	MaxLineWidth int

	// QuoteStyle controls quoting of string scalars.
	// Only applies to formats with multiple quoting conventions (YAML).
	// JSON always uses double quotes per spec; this field is ignored for JSON.
	QuoteStyle QuoteStyle

	// XMLWhitespaceSensitivity controls XML whitespace handling.
	// Ignore: reformat all indentation (default for config files).
	// Preserve: only modify existing indentation, never insert newlines.
	XMLWhitespaceSensitivity XMLWhitespace

	// XMLSelfClosingSpace adds a space before /> in self-closing tags.
	// true: <br /> ; false: <br/>
	XMLSelfClosingSpace bool

	// TrailingCommas controls trailing commas on multiline collections.
	// Only applies to formats that permit them (JSONC).
	TrailingCommas TrailingCommas
}

// TrailingCommas controls trailing commas on multiline collections.
type TrailingCommas int

const (
	// TrailingCommasPreserve matches the style already used by the file:
	// a file with any trailing comma gets them everywhere, a file with
	// none keeps none.
	TrailingCommasPreserve TrailingCommas = iota
	// TrailingCommasAll always adds trailing commas. It is the JSONC default,
	// matching Prettier's trailingComma: "all" behavior.
	TrailingCommasAll
	// TrailingCommasNone always removes trailing commas.
	TrailingCommasNone
)

// IndentStyle selects between spaces and tabs.
type IndentStyle int

const (
	// IndentDefault means the formatter uses its own convention.
	IndentDefault IndentStyle = iota
	// IndentSpaces uses spaces for indentation.
	IndentSpaces
	// IndentTabs uses tabs for indentation.
	IndentTabs
)

// LineEnding selects the line terminator.
type LineEnding int

const (
	// LineEndingDefault uses the formatter's convention (typically LF).
	LineEndingDefault LineEnding = iota
	// LineEndingLF uses \n.
	LineEndingLF
	// LineEndingCRLF uses \r\n.
	LineEndingCRLF
)

// QuoteStyle controls string scalar quoting.
type QuoteStyle int

const (
	// QuotePreserve keeps the original quoting style from the source.
	QuotePreserve QuoteStyle = iota
	// QuoteDouble forces double-quoted strings.
	QuoteDouble
	// QuoteSingle forces single-quoted strings.
	QuoteSingle
)

// XMLWhitespace controls XML whitespace sensitivity.
type XMLWhitespace int

const (
	// XMLWhitespaceIgnore treats all whitespace as insignificant.
	// The formatter inserts newlines and indentation freely.
	// Default for config files (POM, .csproj, plist).
	XMLWhitespaceIgnore XMLWhitespace = iota
	// XMLWhitespacePreserve only modifies existing indentation.
	// Never inserts or removes newlines. Safe for XHTML/SVG.
	XMLWhitespacePreserve
)

// ErrSkipped indicates the formatter cannot process this file but it is not
// a syntax error. The file should be reported to the user with the reason
// but not counted as a failure or formatted.
type ErrSkipped struct {
	Reason string
}

// Error implements the error interface.
func (e *ErrSkipped) Error() string { return "skipped: " + e.Reason }

// DefaultFormatOptions returns the global default formatting options.
// These are overridden by .cfv.toml [format] settings and per-format
// overrides in .cfv.toml [format.<type>].
//
// Zero values mean "use the format-specific default," so this only sets
// options that have a universal reasonable default.
func DefaultFormatOptions() Options {
	return Options{
		FinalNewline: true,
		LineEnding:   LineEndingLF,
	}
}
