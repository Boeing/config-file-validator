package formatter_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// writeYamlfmt writes a yamlfmt config file with the given name and content into dir.
func writeYamlfmt(t *testing.T, dir, name, content string) {
	t.Helper()
	require.NoError(t, os.MkdirAll(dir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, name), []byte(content), 0o600))
}

func TestYamlfmtApply_MapsOptions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", `formatter:
  type: basic
  indent: 4
  line_ending: crlf
  max_line_length: 120
`)

	opts := formatter.Options{IndentWidth: 2, FinalNewline: true}
	formatter.LoadYamlfmt(dir).Apply(&opts)

	require.Equal(t, formatter.IndentSpaces, opts.IndentStyle)
	require.Equal(t, 4, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingCRLF, opts.LineEnding)
	require.Equal(t, 120, opts.MaxLineWidth)
}

func TestYamlfmtApply_ZeroMaxLineLengthMeansUnlimited(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", "formatter:\n  max_line_length: 0\n")

	opts := formatter.Options{MaxLineWidth: 80}
	formatter.LoadYamlfmt(dir).Apply(&opts)

	require.Equal(t, 0, opts.MaxLineWidth)
}

func TestYamlfmtApply_UnsupportedOptionsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	// include_document_start / retain_line_breaks / pad_line_comments have no
	// cfv equivalent and must not disturb the options that are set.
	writeYamlfmt(t, dir, ".yamlfmt", `formatter:
  type: basic
  include_document_start: true
  retain_line_breaks: true
  pad_line_comments: 2
`)

	want := formatter.Options{IndentWidth: 2, FinalNewline: true}
	got := want
	formatter.LoadYamlfmt(dir).Apply(&got)

	require.Equal(t, want, got)
}

func TestLoadYamlfmt_PrefersDotYamlfmtOverYamlSuffix(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", "formatter:\n  indent: 6\n")
	writeYamlfmt(t, dir, ".yamlfmt.yaml", "formatter:\n  indent: 3\n")

	opts := formatter.Options{}
	formatter.LoadYamlfmt(dir).Apply(&opts)

	require.Equal(t, 6, opts.IndentWidth)
}

func TestLoadYamlfmt_FindsYamlSuffixName(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt.yaml", "formatter:\n  indent: 3\n")

	opts := formatter.Options{}
	formatter.LoadYamlfmt(dir).Apply(&opts)

	require.Equal(t, 3, opts.IndentWidth)
}

func TestLoadYamlfmt_WalksUpToProjectRoot(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", "formatter:\n  indent: 5\n")

	opts := formatter.Options{}
	formatter.LoadYamlfmt(filepath.Join(dir, "a", "b")).Apply(&opts)

	require.Equal(t, 5, opts.IndentWidth)
}

func TestLoadYamlfmt_MalformedFileIsIgnored(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", "formatter:\n  indent: [\n")

	require.Nil(t, formatter.LoadYamlfmt(dir))
}

func TestYamlfmtApply_NilIsNoOp(t *testing.T) {
	t.Parallel()
	want := formatter.Options{IndentWidth: 2, FinalNewline: true}
	got := want
	var y *formatter.Yamlfmt
	y.Apply(&got)
	require.Equal(t, want, got)
}

func TestYamlfmtApply_IgnoresNonPositiveIndent(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	writeYamlfmt(t, dir, ".yamlfmt", "formatter:\n  indent: 0\n")

	opts := formatter.Options{IndentWidth: 2}
	formatter.LoadYamlfmt(dir).Apply(&opts)

	require.Equal(t, 2, opts.IndentWidth)
}
