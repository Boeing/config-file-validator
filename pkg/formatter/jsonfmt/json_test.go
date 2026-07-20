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
	src := []byte(`{"items":["aaaaaaaaaa","bbbbbbbbbb","cccccccccc","dddddddddd","eeeeeeeeee"]}`)
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.NotContains(t, string(got), `["aaaaaaaaaa",`)
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

// TestIndentWidth4 verifies 4-space indent produces correctly indented output.
func TestIndentWidth4(t *testing.T) {
	t.Parallel()
	src := []byte(`{"key":"value"}`)
	opts := jsonfmt.DefaultOptions()
	opts.IndentWidth = 4

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "    \"key\"", "expected 4-space indent")
}

// TestIsFormattedHelper verifies the IsFormatted convenience function.
func TestIsFormattedHelper(t *testing.T) {
	t.Parallel()
	alreadyFormatted := []byte("{\n  \"a\": 1,\n  \"b\": 2\n}\n")
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
	src := []byte(`{"a":1}`)
	got, err := f.Format(src, formatter.Options{})
	require.NoError(t, err)
	require.Contains(t, string(got), "  \"a\"") // 2-space default indent
}
