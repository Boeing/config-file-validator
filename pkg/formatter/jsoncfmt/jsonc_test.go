package jsoncfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
)

var f = jsoncfmt.Formatter{}
var defaultOpts = jsoncfmt.DefaultOptions()

// TestFixtures runs all .input.jsonc -> .expected.jsonc fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.jsonc")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.jsonc")
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
	expected, err := filepath.Glob("testdata/*.expected.jsonc")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.jsonc")
			optsFile := "testdata/" + baseName + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			first, err := f.Format(src, opts)
			require.NoError(t, err)
			second, err := f.Format(first, opts)
			require.NoError(t, err)

			require.Equal(t, string(first), string(second),
				"Format is not idempotent for %s", name)
		})
	}
}

// TestInvalidJSONC verifies parse errors on malformed input.
func TestInvalidJSONC(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"unclosed object", `{"key": "value"`},
		{"unclosed array", `[1, 2, 3`},
		{"trailing garbage", `{"a": 1} garbage`},
		{"unclosed string", `{"key": "unterminated`},
		{"invalid literal", `{"key": undefined}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid JSONC: %s", tc.src)
		})
	}
}

// TestStandardJSONPassthrough verifies standard JSON input produces standard
// JSON output (no trailing commas added).
func TestStandardJSONPassthrough(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"simple object", `{"a": 1, "b": 2}`},
		{"nested", `{"outer": {"inner": true}}`},
		{"array", `{"list": [1, 2, 3]}`},
		{"empty object", `{}`},
		{"empty array", `{"a": []}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Format([]byte(tc.src), defaultOpts)
			require.NoError(t, err)
			// Standard JSON output must not contain trailing commas.
			// A trailing comma is ",\n" followed by a closing bracket.
			output := string(got)
			require.NotContains(t, output, ",\n}", "trailing comma before }")
			require.NotContains(t, output, ",\n]", "trailing comma before ]")
		})
	}
}

// TestSortKeys verifies sorting works correctly with comments attached.
func TestSortKeys(t *testing.T) {
	t.Parallel()
	src := []byte(`{
  // Z comment
  "z_key": 1,
  // A comment
  "a_key": 2,
  "m_key": {
    "z_inner": true,
    "a_inner": false,
  },
}`)
	opts := defaultOpts
	opts.SortKeys = true

	got, err := f.Format(src, opts)
	require.NoError(t, err)

	output := string(got)
	// "a_key" should appear before "m_key" and "z_key"
	aIdx := strings.Index(output, `"a_key"`)
	mIdx := strings.Index(output, `"m_key"`)
	zIdx := strings.Index(output, `"z_key"`)
	require.Less(t, aIdx, mIdx, "a_key should appear before m_key")
	require.Less(t, mIdx, zIdx, "m_key should appear before z_key")

	// Inner keys should also be sorted
	aInnerIdx := strings.Index(output, `"a_inner"`)
	zInnerIdx := strings.Index(output, `"z_inner"`)
	require.Less(t, aInnerIdx, zInnerIdx, "a_inner should appear before z_inner")

	// Comment should travel with its key
	aCommentIdx := strings.Index(output, "// A comment")
	require.Less(t, aCommentIdx, aIdx, "A comment should appear before a_key")
}

// TestCRLF verifies CRLF line ending normalization.
func TestCRLF(t *testing.T) {
	t.Parallel()
	src := []byte("{\"a\": 1, \"b\": 2}")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
	// Should not have bare LF (all \n should be preceded by \r).
	for i, b := range got {
		if b == '\n' && (i == 0 || got[i-1] != '\r') {
			t.Fatalf("found bare LF at position %d", i)
		}
	}
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid JSONC
	f.Add([]byte(`{"key": "value"}`))
	f.Add([]byte(`{"a": 1, "b": [1, 2, 3]}`))
	f.Add([]byte("{\n  // comment\n  \"key\": true,\n}\n"))
	f.Add([]byte(`{"nested": {"inner": {"deep": 42}}}`))
	f.Add([]byte(`[1, 2, 3]`))
	f.Add([]byte(`/* block */ {"a": 1}`))
	f.Add([]byte(`{}`))
	f.Add([]byte(`[]`))
	f.Add([]byte(""))
	f.Add([]byte("{invalid"))
	f.Add([]byte{0x00, 0xFF, 0xFE})
	f.Add([]byte(`{"trailing": true,}`))

	fmtr := jsoncfmt.Formatter{}
	opts := jsoncfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmtr.Format(data, opts)
		if err != nil {
			return
		}

		// If Format succeeded, the output must also parse successfully
		result2, err2 := fmtr.Format(result, opts)
		if err2 != nil {
			t.Fatalf("Format succeeded on input but failed on its own output.\nInput: %q\nOutput: %q\nError: %v",
				data, result, err2)
		}

		// Idempotency: Format(Format(x)) == Format(x)
		if string(result) != string(result2) {
			t.Fatalf("Format is not idempotent.\nFirst:  %q\nSecond: %q", result, result2)
		}
	})
}

func FuzzFormatWithOptions(f *testing.F) {
	f.Add([]byte("{\"a\": 1}\n"), byte(0))
	f.Add([]byte("{\n  // comment\n  \"b\": 2,\n}\n"), byte(1))
	f.Add([]byte("{\"arr\": [1, 2, 3]}\n"), byte(2))

	fmtr := jsoncfmt.Formatter{}
	f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
		opts := jsoncfmt.DefaultOptions()
		if optByte&0x01 != 0 {
			opts.IndentWidth = 4
		}
		if optByte&0x02 != 0 {
			opts.FinalNewline = false
		}

		result, err := fmtr.Format(data, opts)
		if err != nil {
			return
		}

		result2, err := fmtr.Format(result, opts)
		if err != nil {
			t.Fatalf("second format failed: %v\nfirst: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent with opts=%08b:\ninput:  %q\nfirst:  %q\nsecond: %q", optByte, data, result, result2)
		}
	})
}
