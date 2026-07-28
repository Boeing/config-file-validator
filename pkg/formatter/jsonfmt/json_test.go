package jsonfmt_test

import (
	stdjson "encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsonfmt"
)

var f = jsonfmt.Formatter{}
var defaultOpts = jsonfmt.DefaultOptions()

// TestFixtures runs all .input.json -> .expected.json fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.json")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.json")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expected := strings.Replace(input, ".input.", ".expected.", 1)

			src, err := os.ReadFile(input)
			require.NoError(t, err)
			want, err := os.ReadFile(expected)
			require.NoError(t, err)

			optsFile := "testdata/" + name + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			got, err := f.Format(src, opts)
			require.NoError(t, err, "Format(%s) should not error", name)
			require.Equal(t, string(want), string(got), "unexpected output for %s", name)
		})
	}
}

// TestIdempotency verifies Format(Format(x)) == Format(x) for all fixtures.
func TestIdempotency(t *testing.T) {
	t.Parallel()
	expected, err := filepath.Glob("testdata/*.expected.json")
	require.NoError(t, err)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			first, err := f.Format(src, defaultOpts)
			require.NoError(t, err)
			second, err := f.Format(first, defaultOpts)
			require.NoError(t, err)

			require.Equal(t, first, second,
				"Format is not idempotent for %s", name)
		})
	}
}

// TestInvalidJSONReturnsError verifies that unparseable input returns an error.
func TestInvalidJSONReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"trailing comma", `{"key": "value",}`},
		{"unclosed brace", `{"key": "value"`},
		{"bare string", `hello`},
		{"empty input", ``},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid JSON: %s", tc.src)
		})
	}
}

// TestSortKeysFalse verifies that SortKeys=false preserves key order.
func TestSortKeysFalse(t *testing.T) {
	t.Parallel()
	src := []byte(`{"z":1,"a":2,"m":3}`)
	opts := jsonfmt.DefaultOptions()
	opts.SortKeys = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)

	// Key order should be preserved: z, a, m.
	gotStr := string(got)
	zPos := strings.Index(gotStr, `"z"`)
	aPos := strings.Index(gotStr, `"a"`)
	mPos := strings.Index(gotStr, `"m"`)
	require.Less(t, zPos, aPos, "z should come before a when SortKeys=false")
	require.Less(t, aPos, mPos, "a should come before m when SortKeys=false")
}

// TestShortArrayStaysOnOneLine verifies that arrays fitting within the default
// max line width are kept on a single line rather than expanded.
func TestShortArrayStaysOnOneLine(t *testing.T) {
	t.Parallel()
	src := []byte(`{"scripts":["pnpm install","pnpm build"]}`)
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), `["pnpm install", "pnpm build"]`)
}

// TestLongArrayIsExpanded verifies that an array exceeding the default max line
// width is expanded to multiple lines.
func TestLongArrayIsExpanded(t *testing.T) {
	t.Parallel()
	src := []byte(`{"items":["aaaaaaaaaa","bbbbbbbbbb","cccccccccc","dddddddddd","eeeeeeeeee","ffffffffff"]}`)
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\n    \"aaaaaaaaaa\"")
}

// TestFinalNewlineFalse verifies that FinalNewline=false strips the trailing newline.
func TestFinalNewlineFalse(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value"}`)
	opts := jsonfmt.DefaultOptions()
	opts.FinalNewline = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.False(t, len(got) > 0 && got[len(got)-1] == '\n',
		"expected no trailing newline, got %q", string(got))
}

// TestDefaultHasFinalNewline verifies the default adds a trailing newline.
func TestDefaultHasFinalNewline(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value"}`)
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.True(t, len(got) > 0 && got[len(got)-1] == '\n',
		"expected trailing newline, got %q", string(got))
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value","num":42}`)
	opts := jsonfmt.DefaultOptions()
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

func TestCRLFPreservesBlankLines(t *testing.T) {
	t.Parallel()
	src := []byte("{\r\n  \"a\": 1,\r\n\r\n  \"b\": 2\r\n}\r\n")
	opts := jsonfmt.DefaultOptions()
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Equal(t, "{\r\n  \"a\": 1,\r\n\r\n  \"b\": 2\r\n}\r\n", string(got))
}

// TestIndentWidth4 verifies 4-space indent produces correctly indented output.
func TestIndentWidth4(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value","number":42,"description":"a somewhat longer value that definitely exceeds eighty"}`)
	opts := jsonfmt.DefaultOptions()
	opts.IndentWidth = 4

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "    \"key\"", "expected 4-space indent")
}

// TestIsFormattedHelper verifies the IsFormatted convenience function.
func TestIsFormattedHelper(t *testing.T) {
	t.Parallel()
	alreadyFormatted := []byte("{ \"a\": 1, \"b\": 2 }\n")
	notFormatted := []byte(`{"b":2,"a":1}`)

	ok, err := formatter.IsFormatted(f, alreadyFormatted, defaultOpts)
	require.NoError(t, err)
	require.True(t, ok, "expected already-formatted file to be reported as formatted")

	ok, err = formatter.IsFormatted(f, notFormatted, defaultOpts)
	require.NoError(t, err)
	require.False(t, ok, "expected unformatted file to be reported as not formatted")
}

// FuzzJSONFormatter verifies no panics and idempotency on arbitrary inputs.
func FuzzJSONFormatter(f *testing.F) {
	f.Add([]byte(`{"key":"value"}`))
	f.Add([]byte(`{"a":1,"b":2,"c":3}`))
	f.Add([]byte(`[1,2,3]`))
	f.Add([]byte(`"hello"`))
	f.Add([]byte(`42`))
	f.Add([]byte(`true`))
	f.Add([]byte(`null`))

	fmter := jsonfmt.Formatter{}
	opts := jsonfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, opts)
		if err != nil {
			return // invalid JSON — expected
		}

		// Output must still be valid JSON.
		if !isValidJSON(result) {
			t.Fatalf("formatter produced invalid JSON from input: %q", data)
		}

		// Idempotency.
		result2, err := fmter.Format(result, opts)
		if err != nil {
			t.Fatalf("second format pass failed: %v", err)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent: input=%q first=%q second=%q", data, result, result2)
		}
	})
}

func isValidJSON(data []byte) bool {
	return stdjson.Valid(data)
}

// TestZeroOptionsUsesJSONDefaults verifies that all-zero Options uses 2-space indent.
func TestZeroOptionsUsesJSONDefaults(t *testing.T) {
	t.Parallel()
	src := []byte(`{"name":"my-application","version":"1.0.0","description":"a long enough value to force expansion past eighty columns total"}`)
	got, err := f.Format(src, formatter.Options{})
	require.NoError(t, err)
	require.Contains(t, string(got), "  \"name\"") // 2-space default indent
}

// TestTabsNormalizedToSpacesByDefault locks issue #584: without editorconfig,
// tab-indented JSON is reformatted with spaces (prettier-compatible default).
func TestTabsNormalizedToSpacesByDefault(t *testing.T) {
	t.Parallel()
	src := []byte("{\n\t\"name\": \"my-app\",\n\t\"version\": \"1.0.0\",\n\t\"description\": \"a longer value to prevent collapsing\"\n}\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.NotContains(t, string(got), "\t", "default format must not preserve tabs")
	require.Contains(t, string(got), "  \"name\"")
	// Explicit IndentTabs still uses tabs when requested.
	tabOpts := defaultOpts
	tabOpts.IndentStyle = formatter.IndentTabs
	gotTabs, err := f.Format(src, tabOpts)
	require.NoError(t, err)
	require.Contains(t, string(gotTabs), "\t")
}

// TestTabIndentCollapsesShortArrays verifies that IndentStyle=IndentTabs uses
// tab characters for indentation when content is expanded.
func TestTabIndentCollapsesShortArrays(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value","num":42,"extra":"field","more":"data","another":"entry here"}`)
	opts := jsonfmt.DefaultOptions()
	opts.IndentStyle = formatter.IndentTabs

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\t\"key\"")
	require.NotContains(t, string(got), "  \"key\"")
}

// TestPreserveBlankLinePrefixNoBlankLine verifies that a source with no blank
// lines between members does not inject blank lines into the output.
func TestPreserveBlankLinePrefixNoBlankLine(t *testing.T) {
	t.Parallel()
	// Single newline between members — no blank line — must not produce blank line in output.
	src := []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.NotContains(t, string(got), "\n\n", "no blank lines expected when source has none")
}

// TestNumberNormalization verifies prettier-compatible number normalization.
// Rules from prettier src/utilities/print-number.js:
// 1. Lowercase E → e
// 2a. Remove unnecessary + and leading zeros in exponent (e+034 → e34)
// 2b. Remove unnecessary scientific notation when exponent is 0 (1e0 → 1)
// 3. Remove extraneous trailing decimal zeros (1.10 → 1.1, but 1.0 stays)
func TestNumberNormalization(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name  string
		input string
		want  string
	}{
		// Rule 1: lowercase
		{"uppercase E", `1.5E10`, `1.5e10`},
		{"uppercase E neg", `1.5E-10`, `1.5e-10`},

		// Rule 2a: strip + and leading zeros from exponent
		{"e+34", `1e+34`, `1e34`},
		{"e034", `1e034`, `1e34`},
		{"e+034", `1e+034`, `1e34`},
		{"E+034", `1E+034`, `1e34`},

		// Rule 2b: remove unnecessary scientific notation (exponent is 0)
		{"1e0", `1e0`, `1`},
		{"1e00", `1e00`, `1`},
		{"2e+00", `2e+00`, `2`},
		{"2e-00", `2e-00`, `2`},
		{"1e-0", `1e-0`, `1`},
		{"1e+0", `1e+0`, `1`},
		{"1.5e0", `1.5e0`, `1.5`},
		{"0.5e0", `0.5e0`, `0.5`},
		{"1E0", `1E0`, `1`},
		{"1E+00", `1E+00`, `1`},

		// Rule 2b: negative number
		{"-1e0", `-1e0`, `-1`},
		{"-2e+00", `-2e+00`, `-2`},

		// Rule 3: trim trailing decimal zeros
		{"1.10", `1.10`, `1.1`},
		{"1.100", `1.100`, `1.1`},
		{"1.0 stays", `1.0`, `1.0`},
		{"-9876.543210", `-9876.543210`, `-9876.54321`},

		// Rules interact: lowercase + strip exponent + trim zeros
		{"1.230E+00", `1.230E+00`, `1.23`},

		// Non-zero exponent preserved
		{"1e1 stays", `1e1`, `1e1`},
		{"1e-1 stays", `1e-1`, `1e-1`},
		{"0.1e1 stays", `0.1e1`, `0.1e1`},

		// Single digit: unchanged
		{"0 stays", `0`, `0`},
		{"5 stays", `5`, `5`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			src := []byte(tc.input)
			got, err := f.Format(src, defaultOpts)
			require.NoError(t, err)
			// Format wraps in trailing newline.
			require.Equal(t, tc.want+"\n", string(got), "input: %s", tc.input)
		})
	}
}
