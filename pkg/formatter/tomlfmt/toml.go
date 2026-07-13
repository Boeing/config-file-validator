// Package tomlfmt provides a Formatter for TOML files.
//
// The formatter uses a CST-based pipeline:
//   - pelletier/go-toml/v2 validates syntax (Unmarshal into any)
//   - Custom tokenizer produces a lossless token stream
//   - Grouper classifies tokens into logical groups (entries, tables, comments)
//   - Printer emits formatted output with normalized spacing
//
// This preserves all comments and document structure while normalizing
// whitespace around separators and optionally sorting keys.
package tomlfmt

import (
	"strings"

	toml "github.com/pelletier/go-toml/v2"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats TOML files with comment preservation.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// DefaultOptions returns the default formatting options for TOML.
func DefaultOptions() formatter.Options {
	return formatter.Options{
		IndentStyle:  formatter.IndentSpaces,
		IndentWidth:  0, // TOML convention: no indentation
		FinalNewline: true,
		LineEnding:   formatter.LineEndingLF,
		SortKeys:     false,
	}
}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid TOML.
//
// Comments (both standalone and inline) are preserved. Formatting
// normalizes spacing around "=" on key-value pairs.
//
// SortKeys sorts entries alphabetically within each table scope.
// Entries separated by blank lines are sorted independently.
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate syntax first.
	var discard any
	if err := toml.Unmarshal(src, &discard); err != nil {
		return nil, err
	}

	// Tokenize → Group → Print (CST-based pipeline).
	tokens := NewLexer(src).Tokenize()
	groups := NewGrouper(tokens).Group()

	printOpts := PrintOptions{
		Indent:        buildIndent(opts),
		ColumnWidth:   80,
		TrailingComma: true,
		AllowedBlanks: 2,
		SortKeys:      opts.SortKeys,
		FinalNewline:  opts.FinalNewline,
		LineEnding:    opts.LineEnding,
	}

	out := NewPrinter(printOpts).Print(groups)
	return out, nil
}

// buildIndent constructs the indent string from options.
func buildIndent(opts formatter.Options) string {
	if opts.IndentStyle == formatter.IndentTabs {
		return "\t"
	}
	width := opts.IndentWidth
	if width <= 0 {
		return "" // TOML convention: no indentation
	}
	return strings.Repeat(" ", width)
}
