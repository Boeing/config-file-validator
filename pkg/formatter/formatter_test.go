package formatter_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// TestOptionsZeroValueIsDefault verifies that zero-value Options means
// "use format defaults" — not "force everything to zero."
func TestOptionsZeroValueIsDefault(t *testing.T) {
	var opts formatter.Options
	require.Equal(t, formatter.IndentDefault, opts.IndentStyle)
	require.Equal(t, 0, opts.IndentWidth)
	require.Equal(t, formatter.LineEndingDefault, opts.LineEnding)
	require.False(t, opts.FinalNewline)
	require.False(t, opts.SortKeys)
	require.Equal(t, 0, opts.MaxLineWidth)
}

// TestDefaultFormatOptions verifies the shared global defaults.
func TestDefaultFormatOptions(t *testing.T) {
	opts := formatter.DefaultFormatOptions()
	require.True(t, opts.FinalNewline, "default should have trailing newline")
	require.Equal(t, formatter.LineEndingLF, opts.LineEnding)
}

// stubFormatter is a minimal Formatter used to test IsFormatted.
type stubFormatter struct{}

func (stubFormatter) Format(_ []byte, _ formatter.Options) ([]byte, error) {
	// "canonical" form: always "canonical\n"
	return []byte("canonical\n"), nil
}

// TestIsFormattedReturnsTrueWhenAlreadyCanonical verifies the contract.IsFormatted helper.
func TestIsFormattedReturnsTrueWhenAlreadyCanonical(t *testing.T) {
	t.Parallel()
	ok, err := formatter.IsFormatted(stubFormatter{}, []byte("canonical\n"), formatter.Options{})
	require.NoError(t, err)
	require.True(t, ok)
}

// TestIsFormattedReturnsFalseWhenNotCanonical verifies IsFormatted for unformatted input.
func TestIsFormattedReturnsFalseWhenNotCanonical(t *testing.T) {
	t.Parallel()
	ok, err := formatter.IsFormatted(stubFormatter{}, []byte("something else"), formatter.Options{})
	require.NoError(t, err)
	require.False(t, ok)
}

// errFormatter always returns an error.
type errFormatter struct{}

func (errFormatter) Format(_ []byte, _ formatter.Options) ([]byte, error) {
	return nil, errors.New("parse error")
}

// TestIsFormattedPropagatesError verifies that formatter errors are returned.
func TestIsFormattedPropagatesError(t *testing.T) {
	t.Parallel()
	_, err := formatter.IsFormatted(errFormatter{}, []byte("anything"), formatter.Options{})
	require.Error(t, err)
}
func TestLoadFixtureOptionsReturnsBaseWhenNoFile(t *testing.T) {
	t.Parallel()
	base := formatter.Options{IndentWidth: 4, SortKeys: true}
	got := formatter.LoadFixtureOptions("/nonexistent/path.json", base)
	require.Equal(t, base, got)
}

// TestLoadFixtureOptionsParsesFile verifies JSON override parsing.
func TestLoadFixtureOptionsParsesFile(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	optsFile := filepath.Join(dir, "test.opts.json")
	require.NoError(t, os.WriteFile(optsFile, []byte(`{"IndentWidth": 4}`), 0o600))

	base := formatter.DefaultFormatOptions()
	got := formatter.LoadFixtureOptions(optsFile, base)
	require.Equal(t, 4, got.IndentWidth)
	// Other fields from base should be preserved.
	require.True(t, got.FinalNewline)
}
