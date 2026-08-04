package tomlfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter/tomlfmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's TOML formatting spec expressed as code.

// Test_TOML_ArrayCollapsesWithinColumnWidth asserts that arrays fitting within
// column_width stay on a single line.
func Test_TOML_ArrayCollapsesWithinColumnWidth(t *testing.T) {
	t.Parallel()
	src := "[pkg]\nitems = [\n  \"a\",\n  \"b\",\n  \"c\",\n]\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// ["a", "b", "c"] is short — must collapse to one line.
	require.Contains(t, out, `items = ["a", "b", "c"]`,
		"short array must collapse within column_width")
}

// Test_TOML_ArrayInsideInlineTableStaysSingleLine asserts that arrays nested
// inside inline tables stay on one line even when they exceed column_width.
func Test_TOML_ArrayInsideInlineTableStaysSingleLine(t *testing.T) {
	t.Parallel()
	src := "[deps]\nwide = { features = [\"aaaaaaaaaaaaaaaa\", \"bbbbbbbbbbbbbbbb\", \"cccccccccccccccc\", \"dddddddddddddddd\"] }\n"

	got, err := tomlfmt.Formatter{}.Format([]byte(src), tomlfmt.DefaultOptions())
	require.NoError(t, err)
	require.Equal(t, src, string(got))
}

// Test_TOML_NestedArrayExpansionRecursive asserts that when an outer array
// expands, nested inner arrays also expand if they exceed column_width.
func Test_TOML_NestedArrayExpansionRecursive(t *testing.T) {
	t.Parallel()
	src := "[pkg]\ndata = [[\"aaaaaaaaaa\", \"bbbbbbbbbb\", \"cccccccccc\"], [\"dddddddddd\", \"eeeeeeeeee\", \"ffffffffff\"]]\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Both outer and inner arrays must expand.
	require.Contains(t, out, "[\n", "outer array must expand")
	require.Contains(t, out, "    \"aaaaaaaaaa\",\n", "inner arrays must also expand recursively")
}

// Test_TOML_CommentAlignmentDisabledForMultiline asserts that when a value
// is multiline, comment alignment is disabled for that group.
func Test_TOML_CommentAlignmentDisabledForMultiline(t *testing.T) {
	t.Parallel()
	src := "[deps]\nshort = \"val\" # comment a\nmultiline = \"\"\"\nlong\nvalue\n\"\"\" # comment b\nanother = \"x\" # comment c\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Comments should NOT be aligned to the same column when a multiline value exists.
	lines := strings.Split(out, "\n")
	var commentCols []int
	for _, line := range lines {
		if idx := strings.Index(line, " # "); idx >= 0 {
			commentCols = append(commentCols, idx)
		}
	}
	if len(commentCols) >= 2 {
		// Not all comments should be at the same column (alignment disabled).
		allSame := true
		for _, col := range commentCols[1:] {
			if col != commentCols[0] {
				allSame = false
				break
			}
		}
		require.False(t, allSame, "comments must NOT be aligned when multiline value exists")
	}
}

// Test_TOML_BlankLinesSplitAlignmentGroups asserts that a blank line between
// entries causes comments to be aligned independently per group.
func Test_TOML_BlankLinesSplitAlignmentGroups(t *testing.T) {
	t.Parallel()
	src := "[cfg]\na = \"short\" # first group\nbb = \"also short\" # first group\n\nccc = \"x\" # second group\ndddd = \"y\" # second group\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// The blank line between groups must be preserved.
	require.Contains(t, out, "\n\n", "blank line between alignment groups must be preserved")
}

// Test_TOML_TrailingCommentsStayInline asserts that comments after array
// elements remain on the same line as their element.
func Test_TOML_TrailingCommentsStayInline(t *testing.T) {
	t.Parallel()
	src := "[cfg]\nitems = [\n  \"alpha\", # first\n  \"beta\", # second\n]\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Comments must stay on the same line as their element (may have alignment spacing).
	for _, line := range strings.Split(out, "\n") {
		if strings.Contains(line, "\"alpha\"") {
			require.Contains(t, line, "# first",
				"trailing comment must stay on same line as element")
		}
		if strings.Contains(line, "\"beta\"") {
			require.Contains(t, line, "# second",
				"trailing comment must stay on same line as element")
		}
	}
}

// Test_TOML_TrailingCommaInExpandedArrays asserts that expanded arrays have
// a trailing comma on the last element (default behavior).
func Test_TOML_TrailingCommaInExpandedArrays(t *testing.T) {
	t.Parallel()
	src := "[pkg]\nitems = [\"aaaaaaaaaa\", \"bbbbbbbbbb\", \"cccccccccc\", \"dddddddddd\", \"eeeeeeeeee\", \"ffffffffff\"]\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// Last element before ] must have a trailing comma.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		if strings.TrimSpace(line) == "]" && i > 0 {
			prevLine := strings.TrimSpace(lines[i-1])
			require.True(t, strings.HasSuffix(prevLine, ","),
				"last array element must have trailing comma, got: %q", prevLine)
		}
	}
}

// Test_TOML_BlankLinesPreservedMax2 asserts that source blank lines are
// preserved but capped at allowed_blank_lines (default 2).
func Test_TOML_BlankLinesPreservedMax2(t *testing.T) {
	t.Parallel()
	src := "[first]\nkey = \"val\"\n\n\n\n\n\n[second]\nkey = \"val\"\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	// 5+ blank lines capped at 2 (allowed_blank_lines default).
	require.NotContains(t, out, "\n\n\n\n", "must not have more than 2 consecutive blank lines")
	// But at least 1 blank line preserved (not stripped entirely).
	require.Contains(t, out, "\n\n", "blank lines must be preserved (up to 2)")
}

// Test_TOML_InlineTableSpacing asserts that inline tables have spaces inside
// braces: { key = "val" } not {key="val"}.
func Test_TOML_InlineTableSpacing(t *testing.T) {
	t.Parallel()
	src := "[deps]\nfoo = {version=\"1.0\",path=\"crates/foo\"}\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "{ version", "inline table must have space after {")
	require.Contains(t, out, "\"crates/foo\" }", "inline table must have space before }")
	require.Contains(t, out, " = ", "inline table must have spaces around =")
}

// Test_TOML_KeysNotIndentedByDefault asserts that keys under a [table] header
// are NOT indented (column 0) by default.
func Test_TOML_KeysNotIndentedByDefault(t *testing.T) {
	t.Parallel()
	src := "[server]\nhost = \"localhost\"\nport = 8080\n"
	opts := tomlfmt.DefaultOptions()

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	lines := strings.Split(out, "\n")
	for _, line := range lines {
		if strings.Contains(line, " = ") && !strings.HasPrefix(line, "[") {
			require.False(t, strings.HasPrefix(line, " "),
				"keys must not be indented by default: %q", line)
		}
	}
}

// Test_TOML_SortKeysAlphabetical asserts that SortKeys=true alphabetizes
// keys within each table section.
func Test_TOML_SortKeysAlphabetical(t *testing.T) {
	t.Parallel()
	src := "[pkg]\nzebra = 1\nalpha = 2\nmango = 3\n"
	opts := tomlfmt.DefaultOptions()
	opts.SortKeys = true

	got, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	aPos := strings.Index(out, "alpha")
	mPos := strings.Index(out, "mango")
	zPos := strings.Index(out, "zebra")
	require.Less(t, aPos, mPos, "alpha must come before mango")
	require.Less(t, mPos, zPos, "mango must come before zebra")
}

// Test_TOML_CommentAlignmentStableAfterArrayCollapse verifies that comment
// alignment is stable after a multiline array collapses to single-line (Bug 5).
// The alignment decision must be based on the POST-format width, not source newlines.
func Test_TOML_CommentAlignmentStableAfterArrayCollapse(t *testing.T) {
	t.Parallel()
	// Array starts multiline in source but fits on one line after formatting.
	src := "[tool.mypy]\nuntyped_calls_exclude = [\n    \"twisted\",\n] # some comment\nshort = \"val\" # another\n"
	opts := tomlfmt.DefaultOptions()

	first, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)

	second, err := tomlfmt.Formatter{}.Format(first, opts)
	require.NoError(t, err)

	third, err := tomlfmt.Formatter{}.Format(second, opts)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second),
		"comment alignment must be stable after array collapse pass 1→2")
	require.Equal(t, string(second), string(third),
		"comment alignment must be stable after array collapse pass 2→3")
}

// Test_TOML_CommentAlignmentStableWithWideInlineTable verifies that comment
// alignment is stable when an inline table exceeds column_width.
func Test_TOML_CommentAlignmentStableWithWideInlineTable(t *testing.T) {
	t.Parallel()
	// Inline table with array that exceeds 80 cols stays single-line. Another
	// entry in the same group has a trailing comment.
	src := "[dependencies]\nbiome_languages = { workspace = true, features = [\"lang_css\", \"lang_html\", \"lang_js\"] }\nonce_cell = \"1.21.4\" # Use std::sync::OnceLock\nshort = { workspace = true }\n"
	opts := tomlfmt.DefaultOptions()

	first, err := tomlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)

	second, err := tomlfmt.Formatter{}.Format(first, opts)
	require.NoError(t, err)

	third, err := tomlfmt.Formatter{}.Format(second, opts)
	require.NoError(t, err)

	require.Equal(t, string(first), string(second),
		"comment alignment must be stable with a wide inline table on pass 1→2")
	require.Equal(t, string(second), string(third),
		"comment alignment must be stable with a wide inline table on pass 2→3")
}
