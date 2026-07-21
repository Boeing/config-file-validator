package formatter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// writeEditorConfig writes an .editorconfig with the given content into dir.
func writeEditorConfig(t *testing.T, dir, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, ".editorconfig"), []byte(content), 0o600))
}

func TestEditorConfigApply_MapsProperties(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, `root = true

[*]
indent_style = tab
indent_size = 4
end_of_line = crlf
insert_final_newline = false
`)

	opts := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
	formatter.NewEditorConfig().Apply(&opts, filepath.Join(dir, "config.json"))

	require.Equal(t, formatter.IndentTabs, opts.IndentStyle)
	require.Equal(t, 4, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingCRLF, opts.LineEnding)
	require.False(t, opts.FinalNewline)
}

func TestEditorConfigApply_PerFileGlobs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, `root = true

[*]
indent_size = 2

[*.yaml]
indent_size = 8
`)

	ec := formatter.NewEditorConfig()

	yamlOpts := formatter.Options{IndentWidth: 3}
	ec.Apply(&yamlOpts, filepath.Join(dir, "app.yaml"))
	require.Equal(t, 8, yamlOpts.IndentWidth)

	jsonOpts := formatter.Options{IndentWidth: 3}
	ec.Apply(&jsonOpts, filepath.Join(dir, "app.json"))
	require.Equal(t, 2, jsonOpts.IndentWidth)
}

func TestEditorConfigApply_NestedOverridesParent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, "root = true\n\n[*]\nindent_size = 2\nend_of_line = lf\n")
	nested := filepath.Join(dir, "nested")
	writeEditorConfig(t, nested, "[*]\nindent_size = 6\n")

	opts := formatter.Options{}
	formatter.NewEditorConfig().Apply(&opts, filepath.Join(nested, "app.json"))

	// Closest file wins for indent_size, parent still supplies end_of_line.
	require.Equal(t, 6, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingLF, opts.LineEnding)
}

func TestEditorConfigApply_RootStopsUpwardSearch(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, "[*]\nindent_size = 9\n")
	nested := filepath.Join(dir, "nested")
	writeEditorConfig(t, nested, "root = true\n\n[*]\nindent_style = space\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewEditorConfig().Apply(&opts, filepath.Join(nested, "app.json"))

	require.Equal(t, formatter.IndentSpaces, opts.IndentStyle)
	require.Equal(t, 2, opts.IndentWidth, "parent .editorconfig must not be read past root = true")
}

func TestEditorConfigApply_IndentSizeTabUsesTabWidth(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, "root = true\n\n[*]\nindent_style = tab\nindent_size = tab\ntab_width = 3\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.NewEditorConfig().Apply(&opts, filepath.Join(dir, "app.json"))

	require.Equal(t, 3, opts.IndentWidth)
}

func TestEditorConfigApply_LeavesOptionsAloneWhenAbsent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// No .editorconfig anywhere under the temp dir, and root = true blocks any
	// real .editorconfig above it from leaking into the test.
	writeEditorConfig(t, dir, "root = true\n")

	want := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
	got := want
	formatter.NewEditorConfig().Apply(&got, filepath.Join(dir, "app.json"))

	require.Equal(t, want, got)
}

func TestEditorConfigApply_MalformedFileIsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeEditorConfig(t, dir, "root = true\n\n[*\nindent_size = not-a-number\n")

	want := formatter.Options{IndentWidth: 2}
	got := want
	formatter.NewEditorConfig().Apply(&got, filepath.Join(dir, "app.json"))

	require.Equal(t, want, got)
}
