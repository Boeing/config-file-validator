package inifmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter/inifmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's INI formatting spec expressed as code.

// Test_INI_KeysIndentedUnderSection asserts that when IndentWidth is set,
// keys under a section header are indented by that many spaces.
func Test_INI_KeysIndentedUnderSection(t *testing.T) {
	t.Parallel()
	src := "[section]\nkey = value\nanother = thing\n"
	opts := inifmt.DefaultOptions()
	opts.IndentWidth = 2

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "  key = value",
		"keys must be indented by 2 spaces under section header")
	require.Contains(t, out, "  another = thing",
		"all keys must be indented by 2 spaces under section header")
}

// Test_INI_KeysNotIndentedByDefault asserts that with default options,
// keys are at column 0 (no indentation).
func Test_INI_KeysNotIndentedByDefault(t *testing.T) {
	t.Parallel()
	src := "[section]\nkey = value\nanother = thing\n"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, "=") {
			require.False(t, strings.HasPrefix(line, " "),
				"keys must not be indented by default, got: %q", line)
			require.False(t, strings.HasPrefix(line, "\t"),
				"keys must not be tab-indented by default, got: %q", line)
		}
	}
}

// Test_INI_SortKeysAlphabetical asserts that SortKeys=true causes keys
// within a section to be sorted alphabetically.
func Test_INI_SortKeysAlphabetical(t *testing.T) {
	t.Parallel()
	src := "[section]\nzebra = 1\nalpha = 2\nmango = 3\n"
	opts := inifmt.DefaultOptions()
	opts.SortKeys = true

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	alphaIdx := strings.Index(out, "alpha = 2")
	mangoIdx := strings.Index(out, "mango = 3")
	zebraIdx := strings.Index(out, "zebra = 1")

	require.True(t, alphaIdx < mangoIdx && mangoIdx < zebraIdx,
		"keys must be sorted alphabetically within section")
}

// Test_INI_SortKeysOffByDefault asserts that original key order is preserved
// when SortKeys is not enabled.
func Test_INI_SortKeysOffByDefault(t *testing.T) {
	t.Parallel()
	src := "[section]\nzebra = 1\nalpha = 2\nmango = 3\n"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	zebraIdx := strings.Index(out, "zebra = 1")
	alphaIdx := strings.Index(out, "alpha = 2")
	mangoIdx := strings.Index(out, "mango = 3")

	require.True(t, zebraIdx < alphaIdx && alphaIdx < mangoIdx,
		"original key order must be preserved by default")
}

// Test_INI_BlankLineBeforeSections asserts that sections are separated
// by a blank line.
func Test_INI_BlankLineBeforeSections(t *testing.T) {
	t.Parallel()
	src := "[first]\nkey1 = val1\n[second]\nkey2 = val2\n"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "\n\n[second]",
		"sections must be separated by a blank line")
}

// Test_INI_SeparatorPreservedForQuotedValues asserts that when a value starts
// with a quote, the original separator (without space padding) is preserved.
func Test_INI_SeparatorPreservedForQuotedValues(t *testing.T) {
	t.Parallel()
	src := "[section]\nkey=\"quoted value\"\n"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "key=\"quoted value\"",
		"separator must be preserved (no space padding) for quoted values")
	require.NotContains(t, out, "key = \"quoted value\"",
		"must NOT add spaces around = when value is quoted")
}

// Test_INI_CommentsPreserved asserts that both # and ; comments survive
// the formatting cycle.
func Test_INI_CommentsPreserved(t *testing.T) {
	t.Parallel()
	src := "; section comment\n[section]\n# key comment\nkey = value\n"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "; section comment",
		"semicolon comments must be preserved")
	require.Contains(t, out, "# key comment",
		"hash comments must be preserved")
}

// Test_INI_DefaultHasFinalNewline asserts that the default output ends with
// exactly one newline character.
func Test_INI_DefaultHasFinalNewline(t *testing.T) {
	t.Parallel()
	src := "[section]\nkey = value"
	opts := inifmt.DefaultOptions()

	got, err := inifmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.True(t, strings.HasSuffix(out, "\n"),
		"output must end with a newline by default")
	require.False(t, strings.HasSuffix(out, "\n\n"),
		"output must not end with multiple newlines")
}
