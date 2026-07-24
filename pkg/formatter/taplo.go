package formatter

import (
	"os"
	"path/filepath"
	"strings"

	toml "github.com/pelletier/go-toml/v2"
)

// taploFileNames are the taplo configuration file names, in search order.
var taploFileNames = []string{"taplo.toml", ".taplo.toml"}

// TaploFormatting mirrors the [formatting] table of a taplo.toml, limited to
// the options cfv can represent. A nil field means the option is unset.
//
// Options cfv has no equivalent for (align_entries, align_comments,
// compact_arrays, compact_inline_tables, array_auto_expand,
// array_auto_collapse) are ignored, so cfv may still diverge from taplo on
// those specific behaviors.
type TaploFormatting struct {
	IndentString       *string `toml:"indent_string"`
	ColumnWidth        *int    `toml:"column_width"`
	TrailingNewline    *bool   `toml:"trailing_newline"`
	ReorderKeys        *bool   `toml:"reorder_keys"`
	CRLF               *bool   `toml:"crlf"`
	ArrayTrailingComma *bool   `toml:"array_trailing_comma"`
}

// Taplo is a parsed taplo.toml. It only applies to TOML files.
type Taplo struct {
	Formatting TaploFormatting `toml:"formatting"`
}

// LoadTaplo returns the taplo configuration found by walking up from startDir,
// or nil if there is none. taplo.toml is preferred over .taplo.toml.
//
// A malformed or unreadable file yields nil rather than an error: it is not the
// file being formatted, and another tool's config should not fail the run.
// Apply is a no-op on a nil *Taplo, so callers need no nil check.
func LoadTaplo(startDir string) *Taplo {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil
	}
	for {
		for _, name := range taploFileNames {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var t Taplo
			if err := toml.Unmarshal(data, &t); err != nil {
				return nil
			}
			return &t
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// Apply overlays the taplo settings cfv can represent onto opts.
// Unset options are left alone.
func (t *Taplo) Apply(opts *Options) {
	if t == nil {
		return
	}
	f := t.Formatting

	if f.IndentString != nil {
		switch s := *f.IndentString; {
		case s != "" && strings.Trim(s, "\t") == "":
			opts.IndentStyle = IndentTabs
		case s != "" && strings.Trim(s, " ") == "":
			opts.IndentStyle = IndentSpaces
			opts.IndentWidth = len(s)
		default:
			// Empty, or a mix of tabs and spaces: no cfv equivalent.
		}
	}
	if f.ColumnWidth != nil {
		opts.MaxLineWidth = *f.ColumnWidth
	}
	if f.TrailingNewline != nil {
		opts.FinalNewline = *f.TrailingNewline
	}
	if f.ReorderKeys != nil {
		opts.SortKeys = *f.ReorderKeys
	}
	if f.CRLF != nil {
		opts.LineEnding = LineEndingLF
		if *f.CRLF {
			opts.LineEnding = LineEndingCRLF
		}
	}
	if f.ArrayTrailingComma != nil {
		opts.TrailingCommas = TrailingCommasNone
		if *f.ArrayTrailingComma {
			opts.TrailingCommas = TrailingCommasAll
		}
	}
}
