package formatter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// writeTaplo writes a taplo config file with the given name and content into dir.
func writeTaplo(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestTaploApply_MapsOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, "taplo.toml", `[formatting]
indent_string = "    "
column_width = 120
trailing_newline = false
reorder_keys = true
crlf = true
array_trailing_comma = false
`)

	opts := formatter.Options{IndentWidth: 0, FinalNewline: true}
	formatter.LoadTaplo(dir).Apply(&opts)

	require.Equal(t, formatter.IndentSpaces, opts.IndentStyle)
	require.Equal(t, 4, opts.IndentWidth)
	require.Equal(t, 120, opts.MaxLineWidth)
	require.False(t, opts.FinalNewline)
	require.True(t, opts.SortKeys)
	require.Equal(t, formatter.LineEndingCRLF, opts.LineEnding)
	require.Equal(t, formatter.TrailingCommasNone, opts.TrailingCommas)
}

func TestTaploApply_TabIndent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, "taplo.toml", "[formatting]\nindent_string = \"\\t\"\n")

	opts := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2}
	formatter.LoadTaplo(dir).Apply(&opts)

	require.Equal(t, formatter.IndentTabs, opts.IndentStyle)
	require.Equal(t, 2, opts.IndentWidth, "indent width is unused for tabs and must not be overwritten")
}

func TestTaploApply_UnsupportedOptionsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// align_entries and friends have no cfv equivalent, and [[rule]] sections
	// are taplo's per-glob overrides — none of it must disturb the options.
	writeTaplo(t, dir, "taplo.toml", `include = ["**/*.toml"]

[formatting]
align_entries = true
align_comments = false
compact_arrays = true
array_auto_expand = false

[[rule]]
include = ["Cargo.toml"]
`)

	want := formatter.Options{IndentWidth: 2, FinalNewline: true}
	got := want
	formatter.LoadTaplo(dir).Apply(&got)

	require.Equal(t, want, got)
}

func TestLoadTaplo_PrefersTaploTomlOverDotted(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, "taplo.toml", "[formatting]\ncolumn_width = 100\n")
	writeTaplo(t, dir, ".taplo.toml", "[formatting]\ncolumn_width = 50\n")

	opts := formatter.Options{}
	formatter.LoadTaplo(dir).Apply(&opts)

	require.Equal(t, 100, opts.MaxLineWidth)
}

func TestLoadTaplo_FindsDottedName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, ".taplo.toml", "[formatting]\ncolumn_width = 50\n")

	opts := formatter.Options{}
	formatter.LoadTaplo(dir).Apply(&opts)

	require.Equal(t, 50, opts.MaxLineWidth)
}

func TestLoadTaplo_WalksUpToProjectRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, "taplo.toml", "[formatting]\nreorder_keys = true\n")

	opts := formatter.Options{}
	formatter.LoadTaplo(filepath.Join(dir, "a", "b")).Apply(&opts)

	require.True(t, opts.SortKeys)
}

func TestLoadTaplo_MalformedFileIsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeTaplo(t, dir, "taplo.toml", "[formatting\nindent_string = \n")

	require.Nil(t, formatter.LoadTaplo(dir))
}

func TestTaploApply_NilIsNoOp(t *testing.T) {
	t.Parallel()

	want := formatter.Options{IndentWidth: 2, FinalNewline: true}
	got := want
	var taplo *formatter.Taplo
	taplo.Apply(&got)

	require.Equal(t, want, got)
}
