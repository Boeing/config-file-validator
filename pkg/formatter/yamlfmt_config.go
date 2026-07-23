package formatter

import (
	"os"
	"path/filepath"
	"strings"

	"gopkg.in/yaml.v3"
)

// yamlfmtFileNames are the yamlfmt configuration file names, in search order.
var yamlfmtFileNames = []string{".yamlfmt", ".yamlfmt.yaml"}

// YamlfmtFormatter mirrors the formatter: table of a .yamlfmt config, limited
// to the options cfv can represent. A nil field means the option is unset.
//
// Options cfv has no equivalent for (include_document_start, retain_line_breaks,
// pad_line_comments, and other yamlfmt-only knobs) are ignored.
type YamlfmtFormatter struct {
	Indent        *int    `yaml:"indent"`
	LineEnding    *string `yaml:"line_ending"`
	MaxLineLength *int    `yaml:"max_line_length"`
}

// Yamlfmt is a parsed .yamlfmt config. It only applies to YAML files.
type Yamlfmt struct {
	Formatter YamlfmtFormatter `yaml:"formatter"`
}

// LoadYamlfmt returns the yamlfmt configuration found by walking up from
// startDir, or nil if there is none. .yamlfmt is preferred over .yamlfmt.yaml.
//
// A malformed or unreadable file yields nil rather than an error: it is not the
// file being formatted, and another tool's config should not fail the run.
// Apply is a no-op on a nil *Yamlfmt, so callers need no nil check.
func LoadYamlfmt(startDir string) *Yamlfmt {
	dir, err := filepath.Abs(startDir)
	if err != nil {
		return nil
	}
	for {
		for _, name := range yamlfmtFileNames {
			data, err := os.ReadFile(filepath.Join(dir, name))
			if err != nil {
				continue
			}
			var y Yamlfmt
			if err := yaml.Unmarshal(data, &y); err != nil {
				return nil
			}
			return &y
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil
		}
		dir = parent
	}
}

// Apply overlays the yamlfmt settings cfv can represent onto opts.
// Unset options are left alone.
func (y *Yamlfmt) Apply(opts *Options) {
	if y == nil {
		return
	}
	f := y.Formatter

	if f.Indent != nil && *f.Indent > 0 {
		opts.IndentStyle = IndentSpaces
		opts.IndentWidth = *f.Indent
	}
	if f.LineEnding != nil {
		switch strings.ToLower(strings.TrimSpace(*f.LineEnding)) {
		case "lf", "\n":
			opts.LineEnding = LineEndingLF
		case "crlf", "\r\n":
			opts.LineEnding = LineEndingCRLF
		default:
			// Unrecognised value: leave the caller's line ending alone.
		}
	}
	if f.MaxLineLength != nil {
		// yamlfmt uses 0 for unlimited, which matches cfv's MaxLineWidth.
		if *f.MaxLineLength >= 0 {
			opts.MaxLineWidth = *f.MaxLineLength
		}
	}
}
