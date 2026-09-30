package propfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter/propfmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's properties formatting spec expressed as code.

// Test_Properties_SeparatorNormalized asserts that non-standard separators
// (colon or whitespace) are normalized to " = " (spaces around equals).
func Test_Properties_SeparatorNormalized(t *testing.T) {
	t.Parallel()
	src := "key1:value1\nkey2 value2\nkey3=value3\n"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "key1 = value1",
		"colon separator must be normalized to ' = '")
	require.Contains(t, out, "key2 = value2",
		"whitespace separator must be normalized to ' = '")
	require.Contains(t, out, "key3 = value3",
		"equals separator must be normalized to ' = '")
}

// Test_Properties_SortKeysAlphabetical asserts that SortKeys=true causes
// keys to be sorted alphabetically.
func Test_Properties_SortKeysAlphabetical(t *testing.T) {
	t.Parallel()
	src := "zebra = 1\nalpha = 2\nmango = 3\n"
	opts := propfmt.DefaultOptions()
	opts.SortKeys = true

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	alphaIdx := strings.Index(out, "alpha = 2")
	mangoIdx := strings.Index(out, "mango = 3")
	zebraIdx := strings.Index(out, "zebra = 1")

	require.True(t, alphaIdx < mangoIdx && mangoIdx < zebraIdx,
		"keys must be sorted alphabetically")
}

// Test_Properties_SortKeysOffByDefault asserts that original key order is
// preserved when SortKeys is not enabled.
func Test_Properties_SortKeysOffByDefault(t *testing.T) {
	t.Parallel()
	src := "zebra = 1\nalpha = 2\nmango = 3\n"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	zebraIdx := strings.Index(out, "zebra = 1")
	alphaIdx := strings.Index(out, "alpha = 2")
	mangoIdx := strings.Index(out, "mango = 3")

	require.True(t, zebraIdx < alphaIdx && alphaIdx < mangoIdx,
		"original key order must be preserved by default")
}

// Test_Properties_BlankLinesBetweenGroupsPreserved asserts that blank lines
// in the source (used to visually group properties) are preserved.
func Test_Properties_BlankLinesBetweenGroupsPreserved(t *testing.T) {
	t.Parallel()
	src := "# database\ndb.host = localhost\ndb.port = 5432\n\n# cache\ncache.ttl = 300\n"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "db.port = 5432\n\n",
		"blank lines between groups must be preserved")
}

// Test_Properties_ContinuationLinesPreserved asserts that multi-line values
// using trailing backslash continuation are collapsed into a single line
// while preserving the decoded value content.
func Test_Properties_ContinuationLinesPreserved(t *testing.T) {
	t.Parallel()
	src := "long.value = line1\\\n  line2\\\n  line3\n"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// The formatter collapses continuations into a single line.
	require.Contains(t, out, "long.value = line1line2line3",
		"continuation lines must be collapsed into single-line value")
	require.NotContains(t, out, "\\\n",
		"continuation backslashes must not remain after formatting")
}

// Test_Properties_CommentsPreserved asserts that both # and ! comments
// survive the formatting cycle.
func Test_Properties_CommentsPreserved(t *testing.T) {
	t.Parallel()
	src := "# hash comment\n! bang comment\nkey = value\n"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "# hash comment",
		"hash comments must be preserved")
	require.Contains(t, out, "! bang comment",
		"bang comments must be preserved")
}

// Test_Properties_DefaultHasFinalNewline asserts that the default output ends
// with exactly one newline character.
func Test_Properties_DefaultHasFinalNewline(t *testing.T) {
	t.Parallel()
	src := "key = value"
	opts := propfmt.DefaultOptions()

	got, err := propfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.True(t, strings.HasSuffix(out, "\n"),
		"output must end with a newline by default")
	require.False(t, strings.HasSuffix(out, "\n\n"),
		"output must not end with multiple newlines")
}
