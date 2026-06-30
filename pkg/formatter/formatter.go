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
//	CLI flags > .cfv.toml [format.<type>] > .cfv.toml [format] > hardcoded defaults
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
}

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

// Result holds the outcome of formatting a single file.
// It's used internally to communicate between the formatter pipeline and
// the reporter.
type Result struct {
	// FilePath is the path to the file that was formatted.
	FilePath string

	// Changed is true when the formatted output differs from the source.
	Changed bool

	// Err is non-nil when the file could not be formatted (e.g., parse error).
	// A parse error is not a formatting issue — it means the formatter cannot
	// process this file.
	Err error
}

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
