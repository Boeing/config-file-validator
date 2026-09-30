package formatter

import (
	"strconv"
	"sync"

	editorconfig "github.com/editorconfig/editorconfig-core-go/v2"
)

// EditorConfig resolves .editorconfig settings for individual files.
//
// EditorConfig resolution is per-file: the properties that apply depend on the
// file's path (glob sections) and on the .editorconfig files found walking up
// from its directory (stopping at root = true). The zero value is not usable;
// use NewEditorConfig.
//
// Safe for concurrent use.
type EditorConfig struct {
	mu     sync.Mutex
	config editorconfig.Config
}

// NewEditorConfig returns an EditorConfig backed by a cached parser, so
// .editorconfig files shared by many source files are only parsed once.
func NewEditorConfig() *EditorConfig {
	return &EditorConfig{
		config: editorconfig.Config{Parser: editorconfig.NewCachedParser()},
	}
}

// Apply overlays the .editorconfig properties that apply to path onto opts.
// Properties that are absent, unset, or not representable as an Option are
// left alone.
//
// A malformed or unreadable .editorconfig is ignored rather than reported — it
// is not the file being formatted, and failing the run over someone else's
// editor settings is worse than using the defaults.
func (e *EditorConfig) Apply(opts *Options, path string) {
	// editorconfig.Config is not safe for concurrent use (the cached parser
	// writes to shared maps), and cfv formats files in parallel.
	// A warning means one section was unusable, not that the whole lookup
	// failed, so LoadGraceful lets us keep the properties that did parse.
	e.mu.Lock()
	def, _, err := e.config.LoadGraceful(path)
	e.mu.Unlock()
	if err != nil {
		return
	}

	switch def.IndentStyle {
	case editorconfig.IndentStyleSpaces:
		opts.IndentStyle = IndentSpaces
	case editorconfig.IndentStyleTab:
		opts.IndentStyle = IndentTabs
	default:
		// Unset or unrecognised, so keep the caller's style.
	}

	// indent_size = tab means "use tab_width", which the library has already
	// resolved into TabWidth for us.
	if def.IndentSize == "tab" {
		if def.TabWidth > 0 {
			opts.IndentWidth = def.TabWidth
		}
	} else if n, err := strconv.Atoi(def.IndentSize); err == nil && n > 0 {
		opts.IndentWidth = n
	}

	switch def.EndOfLine {
	case editorconfig.EndOfLineLf:
		opts.LineEnding = LineEndingLF
	case editorconfig.EndOfLineCrLf:
		opts.LineEnding = LineEndingCRLF
	default:
		// Unset, or "cr", which cfv does not emit.
	}

	if def.InsertFinalNewline != nil {
		opts.FinalNewline = *def.InsertFinalNewline
	}
}
