package yamlfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's YAML formatting spec expressed as code.

// Test_YAML_SequenceIndentedUnderMapping asserts that sequence items under a
// mapping key are indented by tabWidth (default 2) from the key.
func Test_YAML_SequenceIndentedUnderMapping(t *testing.T) {
	t.Parallel()
	src := "items:\n- one\n- two\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// "- one" should be indented 2 spaces under "items:"
	require.Contains(t, out, "items:\n  - one\n  - two\n",
		"sequence items must be indented by tabWidth under mapping key")
}

// Test_YAML_BlankLineMaxOneBetweenElements asserts that consecutive blank lines
// between mapping entries or sequence items are collapsed to at most 1.
func Test_YAML_BlankLineMaxOneBetweenElements(t *testing.T) {
	t.Parallel()
	src := "a: 1\n\n\n\nb: 2\n\n\nc: 3\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// No more than 1 blank line between any elements.
	require.NotContains(t, out, "\n\n\n", "max 1 blank line between elements")
	// But single blank lines ARE preserved.
	require.Contains(t, out, "a: 1\n\nb: 2", "single blank line preserved")
}

// Test_YAML_BlockScalarIndentNormalized asserts that block scalar content is
// indented to parentIndent + tabWidth regardless of source indentation.
func Test_YAML_BlockScalarIndentNormalized(t *testing.T) {
	t.Parallel()
	// Source has 4-space indent but parentIndent=0 + tabWidth=2 → should be 2.
	src := "text: |\n    line one\n    line two\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Content lines should be at 2-space indent (0 + 2).
	require.Contains(t, out, "text: |\n  line one\n  line two\n",
		"block scalar content must be normalized to parentIndent + tabWidth")
}

// Test_YAML_BlockScalarClipRemovesTrailingBlanks asserts that clip-chomped block
// scalars have trailing blank lines removed from their content.
func Test_YAML_BlockScalarClipRemovesTrailingBlanks(t *testing.T) {
	t.Parallel()
	src := "key: |\n  content\n\n\nnext: value\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// The blank lines between the scalar content and "next:" should be removed.
	// Clip chomping means one trailing newline after content, then next key.
	require.NotContains(t, out, "content\n\n\n",
		"clip chomping must remove trailing blank lines in block scalar")
}

// Test_YAML_CommentHasSingleSpaceBefore asserts that inline comments are
// normalized to have exactly one space before the #.
func Test_YAML_CommentHasSingleSpaceBefore(t *testing.T) {
	t.Parallel()
	src := "key: value    # too many spaces\nother: val  # also too many\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Each comment should have exactly " # " (one space before #).
	for _, line := range strings.Split(out, "\n") {
		if idx := strings.Index(line, "#"); idx > 0 {
			// Character before # should be exactly one space.
			before := line[:idx]
			require.True(t, strings.HasSuffix(before, " "),
				"comment must be preceded by a space: %q", line)
			require.False(t, strings.HasSuffix(before, "  "),
				"comment must have exactly ONE space before #: %q", line)
		}
	}
}

// Test_YAML_NoBlankLineBetweenKeyAndValue asserts that blank lines between a
// mapping key's colon and its first child value are stripped.
func Test_YAML_NoBlankLineBetweenKeyAndValue(t *testing.T) {
	t.Parallel()
	src := "automation:\n\n  - alias: Turn off\n  - alias: Turn on\n"
	opts := yamlfmt.DefaultOptions()

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// No blank line between "automation:" and its first child.
	require.Contains(t, out, "automation:\n  - alias",
		"blank line between key colon and first child must be stripped")
	// But blank lines between siblings are still allowed.
	require.NotContains(t, out, "automation:\n\n",
		"no blank line allowed after colon before value")
}

// Test_YAML_SortKeysOffByDefault asserts that without SortKeys option, key
// order is preserved from source.
func Test_YAML_SortKeysOffByDefault(t *testing.T) {
	t.Parallel()
	src := "zebra: 1\nalpha: 2\nmango: 3\n"
	opts := yamlfmt.DefaultOptions()
	// Explicitly confirm SortKeys is false by default.
	require.False(t, opts.SortKeys, "SortKeys must be false by default")

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	zPos := strings.Index(out, "zebra")
	aPos := strings.Index(out, "alpha")
	mPos := strings.Index(out, "mango")
	require.Less(t, zPos, aPos, "original order: zebra before alpha")
	require.Less(t, aPos, mPos, "original order: alpha before mango")
}

// Test_YAML_QuoteSingleWhenConfigured asserts that QuoteSingle option causes
// double-quoted strings to be converted to single quotes.
func Test_YAML_QuoteSingleWhenConfigured(t *testing.T) {
	t.Parallel()
	src := "key: \"value\"\n"
	opts := yamlfmt.DefaultOptions()
	opts.QuoteStyle = formatter.QuoteSingle

	got, err := yamlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "'value'", "QuoteSingle must convert double to single quotes")
	require.NotContains(t, out, "\"value\"", "QuoteSingle must not retain double quotes")
}
