package jsonfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's formatting spec expressed as code.

// Test_JSON_BracketSpacingOnInlineObjects asserts that inline objects
// have spaces inside braces: { "k": "v" } not {"k": "v"}.
func Test_JSON_BracketSpacingOnInlineObjects(t *testing.T) {
	t.Parallel()
	src := []byte(`{"a":{"b":1}}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	// The inner object { "b": 1 } fits on one line and must have bracket spacing.
	require.Contains(t, out, `{ "b": 1 }`, "inline objects must have bracket spacing")
}

// Test_JSON_NoBracketPaddingOnArrays asserts that array brackets never have
// interior space padding, including when the array is multiline.
func Test_JSON_NoBracketPaddingOnArrays(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":["a","b"]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "[\n    \"a\",", "non-empty arrays must expand")
	require.NotContains(t, out, `[ `, "no space after opening bracket")
}

// Test_JSON_EmptyObjectIsCompact asserts empty objects format as {} not { }.
func Test_JSON_EmptyObjectIsCompact(t *testing.T) {
	t.Parallel()
	src := []byte(`{"empty":{}}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, `"empty": {}`, "empty objects must be compact")
	require.NotContains(t, out, `{ }`, "empty objects must not have interior space")
}

// Test_JSON_EmptyArrayIsCompact asserts empty arrays format as [] not [ ].
func Test_JSON_EmptyArrayIsCompact(t *testing.T) {
	t.Parallel()
	src := []byte(`{"empty":[]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, `"empty": []`, "empty arrays must be compact")
	require.NotContains(t, out, `[ ]`, "empty arrays must not have interior space")
}

// Test_JSON_NonEmptyArraysExpand verifies root and property arrays use one
// element per line while short arrays used as elements stay compact.
func Test_JSON_NonEmptyArraysExpand(t *testing.T) {
	t.Parallel()
	src := []byte(`{"brackets":[["{","}"],["[","]"],["(",")"]],"other":[1,2,3]}`)
	want := "{\n" +
		"  \"brackets\": [\n" +
		"    [\"{\", \"}\"],\n" +
		"    [\"[\", \"]\"],\n" +
		"    [\"(\", \")\"]\n" +
		"  ],\n" +
		"  \"other\": [\n" +
		"    1,\n" +
		"    2,\n" +
		"    3\n" +
		"  ]\n" +
		"}\n"

	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	require.Equal(t, want, string(got))
}

// Test_JSON_MultilineObjectPreservesMultiline asserts that objects already on
// multiple lines in source stay on multiple lines (are not collapsed).
func Test_JSON_MultilineObjectPreservesMultiline(t *testing.T) {
	t.Parallel()
	src := []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n")
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	lines := strings.Split(strings.TrimSpace(out), "\n")
	require.Greater(t, len(lines), 1, "multiline source object must stay multiline")
	require.Contains(t, out, "\n  \"a\"", "must retain expanded format")
}

// Test_JSON_InlineObjectPreservesInline asserts that objects on a single line
// in source stay inline if they fit within MaxLineWidth.
func Test_JSON_InlineObjectPreservesInline(t *testing.T) {
	t.Parallel()
	src := []byte(`{"outer":{"a":1,"b":2}}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	// The inner object fits on one line → stays inline.
	require.Contains(t, out, `{ "a": 1, "b": 2 }`, "short inline object must stay inline")
}

// Test_JSON_ArrayElementsRetainMultilineObjects asserts multiline child
// objects keep their structure after the containing array expands.
func Test_JSON_ArrayElementsRetainMultilineObjects(t *testing.T) {
	t.Parallel()
	// This array has an object whose formatted form is multiline (description is long).
	src := []byte(`{"items":[{"name":"first","description":"a value long enough to force the object to expand past the eighty column limit"}]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "\n    {\n", "multiline array element must remain expanded")
}

// Test_JSON_NeverAddsTrailingCommas asserts that JSON (not JSONC) never
// produces trailing commas in expanded structures.
func Test_JSON_NeverAddsTrailingCommas(t *testing.T) {
	t.Parallel()
	src := []byte(`{"a":"long value that forces expansion past the eighty column threshold for testing purposes here","b":"second"}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	// Find lines ending with comma before } or ] — that would be a trailing comma.
	lines := strings.Split(out, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		if i+1 < len(lines) {
			nextTrimmed := strings.TrimSpace(lines[i+1])
			if (nextTrimmed == "}" || nextTrimmed == "]") && strings.HasSuffix(trimmed, ",") {
				t.Fatalf("trailing comma found on line %d: %q (JSON must never have trailing commas)", i+1, line)
			}
		}
	}
}

// Test_JSON_NumericArraysUseOneElementPerLine asserts numeric arrays follow
// the same expansion rule as every other non-empty array.
func Test_JSON_NumericArraysUseOneElementPerLine(t *testing.T) {
	t.Parallel()
	src := []byte(`{"data":[1,2,3]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	require.Contains(t, string(got), "    1,\n    2,\n    3\n")
}

// Test_JSON_DefaultMaxLineWidthIs80 asserts that the default MaxLineWidth is 80
// for deciding whether inline objects need expansion.
func Test_JSON_DefaultMaxLineWidthIs80(t *testing.T) {
	t.Parallel()
	opts := jsonfmt.DefaultOptions()

	shortSrc := []byte(`{"k":{"a":"aaaa","b":"bbbb"}}`)
	shortGot, err := jsonfmt.Formatter{}.Format(shortSrc, opts)
	require.NoError(t, err)
	shortOut := string(shortGot)
	require.Contains(t, shortOut, `{ "a": "aaaa", "b": "bbbb" }`, "under-80 object should stay inline")

	longSrc := []byte(`{"k":{"first_setting":"a value long enough to consume most of the line","second_setting":"another value"}}`)
	longGot, err := jsonfmt.Formatter{}.Format(longSrc, opts)
	require.NoError(t, err)
	longOut := string(longGot)
	require.Contains(t, longOut, "\n    \"first_setting\"", "over-80 object should expand")
}

// Test_JSON_MaxLineWidthRespectsOption asserts that MaxLineWidth option
// changes where object collapse/expand decisions happen.
func Test_JSON_MaxLineWidthRespectsOption(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":{"a":"aaaa","b":"bbbb","c":"cccc"}}`)

	// With width 30, this should expand.
	narrow := jsonfmt.DefaultOptions()
	narrow.MaxLineWidth = 30
	got, err := jsonfmt.Formatter{}.Format(src, narrow)
	require.NoError(t, err)
	require.Contains(t, string(got), "\n    \"a\"", "should expand at width 30")

	// With width 200, this should stay inline.
	wide := jsonfmt.DefaultOptions()
	wide.MaxLineWidth = 200
	got2, err := jsonfmt.Formatter{}.Format(src, wide)
	require.NoError(t, err)
	require.Contains(t, string(got2), `{ "a": "aaaa"`, "should stay inline at width 200")
}
