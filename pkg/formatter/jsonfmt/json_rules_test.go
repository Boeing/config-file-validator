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

// Test_JSON_NoBracketSpacingOnArrays asserts that arrays never have
// interior bracket spacing: ["a", "b"] not [ "a", "b" ].
func Test_JSON_NoBracketSpacingOnArrays(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":["a","b"]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, `["a", "b"]`, "inline arrays must NOT have bracket spacing")
	require.NotContains(t, out, `[ "a"`, "no space after opening bracket")
	require.NotContains(t, out, `"b" ]`, "no space before closing bracket")
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

// Test_JSON_MultilineObjectInArrayPreventsCollapse asserts that an array
// containing a multiline child object does NOT collapse to a single line.
func Test_JSON_MultilineObjectInArrayPreventsCollapse(t *testing.T) {
	t.Parallel()
	// This array has an object whose formatted form is multiline (description is long).
	src := []byte(`{"items":[{"name":"first","description":"a value long enough to force the object to expand past the eighty column limit"}]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	// The array must expand because its child is multiline.
	require.Contains(t, out, "\n    {\n", "array with multiline child must not collapse")
}

// Test_JSON_KeyPrefixIncludedInWidthCalc asserts that the key + ": " prefix
// is counted toward MaxLineWidth when deciding to collapse/expand an array.
func Test_JSON_KeyPrefixIncludedInWidthCalc(t *testing.T) {
	t.Parallel()
	// Array content alone is ~67 chars. At depth 0 with "key": prefix (7 chars),
	// total = 67 + 2(indent) + 7 = 76 → fits. But if nested deeply...
	src := []byte(`{"config":{"deeply":{"nested":{"key":["short","array","that","fits","because","key","is","long"]}}}}`)
	opts := jsonfmt.DefaultOptions()

	got, err := jsonfmt.Formatter{}.Format(src, opts)
	require.NoError(t, err)
	out := string(got)

	// At depth 4, prefix is 8(indent) + 7("key": ) = 15. Array = 67. Total = 82 > 80.
	// So the array MUST expand despite being short in isolation.
	require.Contains(t, out, "\n          \"short\"", "key prefix must be counted in width calculation")
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

// Test_JSON_ConciseArrayFillsLine asserts that all-numeric arrays use fill
// layout: packing multiple elements per line up to MaxLineWidth.
func Test_JSON_ConciseArrayFillsLine(t *testing.T) {
	t.Parallel()
	src := []byte(`{"data":[1,2,3,4,5,6,7,8,9,10,11,12,13,14,15,16,17,18,19,20]}`)
	got, err := jsonfmt.Formatter{}.Format(src, jsonfmt.DefaultOptions())
	require.NoError(t, err)
	out := string(got)

	// Fill layout packs multiple numbers per line, NOT one-per-line.
	lines := strings.Split(out, "\n")
	var dataLines []string
	for _, l := range lines {
		if strings.Contains(l, ",") && !strings.Contains(l, "\"data\"") {
			dataLines = append(dataLines, l)
		}
	}
	// With 20 small numbers and 80 col width, fill should pack them in far
	// fewer lines than 20 (one-per-line would be 20 lines).
	require.Less(t, len(dataLines), 10, "concise fill should pack multiple numbers per line, got %d data lines", len(dataLines))
}

// Test_JSON_DefaultMaxLineWidthIs80 asserts that the default MaxLineWidth is 80
// by testing a value that's exactly at the boundary.
func Test_JSON_DefaultMaxLineWidthIs80(t *testing.T) {
	t.Parallel()
	opts := jsonfmt.DefaultOptions()

	// This array + key + braces = ~70 chars inline → stays inline.
	shortSrc := []byte(`{"k":["aaaa","bbbb","cccc","dddd","eeee","ffff","gggg"]}`)
	shortGot, err := jsonfmt.Formatter{}.Format(shortSrc, opts)
	require.NoError(t, err)
	shortOut := string(shortGot)
	require.Contains(t, shortOut, `["aaaa"`, "under-80 content should stay inline")

	// This array + key + braces = ~90 chars inline → must expand.
	longSrc := []byte(`{"k":["aaaaaa","bbbbbb","cccccc","dddddd","eeeeee","ffffff","gggggg","hhhhhh"]}`)
	longGot, err := jsonfmt.Formatter{}.Format(longSrc, opts)
	require.NoError(t, err)
	longOut := string(longGot)
	require.Contains(t, longOut, "\n    \"aaaaaa\"", "over-80 content should expand")
}

// Test_JSON_MaxLineWidthRespectsOption asserts that MaxLineWidth option
// changes where collapse/expand decisions happen.
func Test_JSON_MaxLineWidthRespectsOption(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":["aaaa","bbbb","cccc","dddd"]}`)

	// With width 30, this should expand.
	narrow := jsonfmt.DefaultOptions()
	narrow.MaxLineWidth = 30
	got, err := jsonfmt.Formatter{}.Format(src, narrow)
	require.NoError(t, err)
	require.Contains(t, string(got), "\n    \"aaaa\"", "should expand at width 30")

	// With width 200, this should stay inline.
	wide := jsonfmt.DefaultOptions()
	wide.MaxLineWidth = 200
	got2, err := jsonfmt.Formatter{}.Format(src, wide)
	require.NoError(t, err)
	require.Contains(t, string(got2), `["aaaa"`, "should stay inline at width 200")
}
