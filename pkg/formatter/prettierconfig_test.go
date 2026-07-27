package formatter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// writePrettierRC writes name (e.g. ".prettierrc.json") with the given
// content into dir.
func writePrettierRC(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestPrettierConfigApply_JSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.json", `{
		"tabWidth": 4,
		"useTabs": false,
		"printWidth": 100,
		"endOfLine": "crlf",
		"trailingComma": "all",
		"singleQuote": true
	}`)

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "config.jsonc"))

	require.Equal(t, 4, opts.IndentWidth)
	require.Equal(t, formatter.IndentSpaces, opts.IndentStyle)
	require.Equal(t, 100, opts.MaxLineWidth)
	require.Equal(t, formatter.LineEndingCRLF, opts.LineEnding)
	require.Equal(t, formatter.TrailingCommasAll, opts.TrailingCommas)
	require.Equal(t, formatter.QuoteSingle, opts.QuoteStyle)
}

func TestPrettierConfigApply_BareFileAutoDetectsJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc", `{"tabWidth": 8}`)

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	require.Equal(t, 8, opts.IndentWidth)
}

func TestPrettierConfigApply_BareFileFallsBackToYAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc", "tabWidth: 6\nuseTabs: true\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	require.Equal(t, 6, opts.IndentWidth)
	require.Equal(t, formatter.IndentTabs, opts.IndentStyle)
}

func TestPrettierConfigApply_YAML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.yaml", "printWidth: 120\nsingleQuote: false\n")

	opts := formatter.Options{QuoteStyle: formatter.QuoteSingle}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.yaml"))

	require.Equal(t, 120, opts.MaxLineWidth)
	require.Equal(t, formatter.QuoteDouble, opts.QuoteStyle)
}

func TestPrettierConfigApply_YML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.yml", "tabWidth: 3\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.yaml"))

	require.Equal(t, 3, opts.IndentWidth)
}

func TestPrettierConfigApply_TOML(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.toml", "tabWidth = 4\nendOfLine = \"lf\"\n")

	opts := formatter.Options{IndentWidth: 2, LineEnding: formatter.LineEndingCRLF}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.toml"))

	require.Equal(t, 4, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingLF, opts.LineEnding)
}

func TestPrettierConfigApply_DiscoveryPriorityWithinDir(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// .prettierrc must win over .prettierrc.json in the same directory.
	writePrettierRC(t, dir, ".prettierrc", `{"tabWidth": 5}`)
	writePrettierRC(t, dir, ".prettierrc.json", `{"tabWidth": 9}`)

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	require.Equal(t, 5, opts.IndentWidth)
}

func TestPrettierConfigApply_ClosestDirectoryWins(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.json", `{"tabWidth": 2}`)
	nested := filepath.Join(dir, "nested")
	writePrettierRC(t, nested, ".prettierrc.json", `{"tabWidth": 7}`)

	opts := formatter.Options{IndentWidth: 3}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(nested, "app.json"))

	require.Equal(t, 7, opts.IndentWidth)
}

func TestPrettierConfigApply_WalksUpWhenNotFoundLocally(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.json", `{"tabWidth": 6}`)
	nested := filepath.Join(dir, "nested")
	require.NoError(t, os.MkdirAll(nested, 0o755))

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(nested, "app.json"))

	require.Equal(t, 6, opts.IndentWidth)
}

func TestPrettierConfigApply_UnsupportedJSConfigIsSkippedNotError(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.js", "module.exports = { tabWidth: 4 };\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	// No supported config found: options are left untouched, and applying
	// must not error or panic.
	require.Equal(t, 2, opts.IndentWidth)
}

func TestPrettierConfigApply_MalformedFileIsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writePrettierRC(t, dir, ".prettierrc.json", `{"tabWidth": `)

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewPrettierConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	require.Equal(t, 2, opts.IndentWidth)
}

func TestPrettierConfigApply_LeavesOptionsAloneWhenAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()

	want := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
	got := want
	formatter.NewPrettierConfig().Apply(&got, filepath.Join(dir, "app.json"))

	require.Equal(t, want, got)
}
