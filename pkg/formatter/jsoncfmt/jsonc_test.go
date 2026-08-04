package jsoncfmt_test

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"github.com/tailscale/hujson"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/jsoncfmt"
)

var update = flag.Bool("update", false, "update .expected.* golden files")

var f = jsoncfmt.Formatter{}
var defaultOpts = jsoncfmt.DefaultOptions()

// TestFixtures runs all .input.jsonc -> .expected.jsonc fixture pairs.
// Pass -update to regenerate golden files from current formatter output.
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

			optsFile := "testdata/" + name + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			got, err := f.Format(src, opts)
			require.NoError(t, err, "Format(%s) should not error", name)

			_, parseErr := hujson.Parse(got)
			require.NoError(t, parseErr,
				"Format output is not valid JSONC for %s", name)

			if *update {
				require.NoError(t, os.WriteFile(expected, got, 0o600), //nolint:gosec // path derived from glob within testdata/
					"failed to update golden file %s", expected)
				return
			}

			want, err := os.ReadFile(expected)
			require.NoError(t, err)
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

			_, parseErr := hujson.Parse(first)
			require.NoError(t, parseErr,
				"Format output is not valid JSONC for %s", name)

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

// TestDefaultTrailingCommas verifies the JSONC default adds trailing commas to
// expanded objects and non-empty arrays while leaving empty collections unchanged.
func TestDefaultTrailingCommas(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"expanded object",
			`{"alpha_setting": "value long enough to prevent this object from collapsing", "beta_setting": 2}`,
			"{\n  \"alpha_setting\": \"value long enough to prevent this object from collapsing\",\n  \"beta_setting\": 2,\n}\n",
		},
		{
			"expanded array",
			`{"list": [{"identifier": "first-item-with-a-sufficiently-long-name-to-exceed-eighty"}, {"identifier": "second-item-with-a-sufficiently-long-name-to-exceed-eighty"}]}`,
			"{\n  \"list\": [\n    {\n      \"identifier\": \"first-item-with-a-sufficiently-long-name-to-exceed-eighty\",\n    },\n    {\n      \"identifier\": \"second-item-with-a-sufficiently-long-name-to-exceed-eighty\",\n    },\n  ],\n}\n",
		},
		{
			"non-empty array",
			`[1, 2, 3]`,
			"[\n  1,\n  2,\n  3,\n]\n",
		},
		{
			"collapsed object",
			`{"a": 1, "b": 2}`,
			"{ \"a\": 1, \"b\": 2 }\n",
		},
		{
			"empty collections",
			`{"object_key": {}, "array_key": [], "description": "long enough to prevent root from collapsing"}`,
			"{\n  \"object_key\": {},\n  \"array_key\": [],\n  \"description\": \"long enough to prevent root from collapsing\",\n}\n",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Format([]byte(tc.src), defaultOpts)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))
		})
	}
}

// TestTrailingCommasNoneWithFinalComment verifies that removing a trailing
// comma does not discard a comment attached to the final value.
func TestTrailingCommasNoneWithFinalComment(t *testing.T) {
	t.Parallel()
	opts := defaultOpts
	opts.TrailingCommas = formatter.TrailingCommasNone

	cases := []struct {
		name string
		src  string
		want string
	}{
		{
			"object",
			`{"alpha_setting_name": "value is long enough to prevent collapsing even without comment" /* final comment */,}`,
			"{\n  \"alpha_setting_name\": \"value is long enough to prevent collapsing even without comment\" /* final comment */\n}\n",
		},
		{
			"array",
			`[{"identifier": "first-item-with-enough-length-to-prevent-collapsing-this-obj"} /* final comment */,]`,
			"[\n  {\n    \"identifier\": \"first-item-with-enough-length-to-prevent-collapsing-this-obj\"\n  } /* final comment */\n]\n",
		},
		{
			"array",
			`[1, 2, 3 /* last */,]`,
			"[\n  1,\n  2,\n  3 /* last */\n]\n",
		},
		{
			"nested object",
			`{"outer_container": {"alpha_setting_name": "value is long enough to prevent collapsing even without the comment" /* final comment */,},}`,
			"{\n  \"outer_container\": {\n    \"alpha_setting_name\": \"value is long enough to prevent collapsing even without the comment\" /* final comment */\n  }\n}\n",
		},
		{
			"line comment",
			`{"alpha_setting_name": "value is long enough to prevent collapsing even without comment" // final comment
,}`,
			"{\n  \"alpha_setting_name\": \"value is long enough to prevent collapsing even without comment\" // final comment\n}\n",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Format([]byte(tc.src), opts)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(got))

			second, err := f.Format(got, opts)
			require.NoError(t, err)
			require.Equal(t, got, second, "formatting must be idempotent")
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

func TestCRLFPreservesBlankLines(t *testing.T) {
	t.Parallel()
	src := []byte("{\r\n  // Group A\r\n  \"a\": 1,\r\n\r\n  \"b\": 2\r\n}\r\n")
	opts := jsoncfmt.DefaultOptions()
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Equal(t, "{\r\n  // Group A\r\n  \"a\": 1,\r\n\r\n  \"b\": 2,\r\n}\r\n", string(got))
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

		// Semantic equivalence via hujson parse.
		origVal, origErr := hujson.Parse(data)
		fmtVal, fmtErr := hujson.Parse(result)
		if origErr == nil && fmtErr != nil {
			t.Fatalf("formatted output is invalid JSONC: %v\ninput: %q\noutput: %q", fmtErr, data, result)
		}
		if origErr == nil && fmtErr == nil {
			origVal.Standardize()
			origVal.Minimize()
			fmtVal.Standardize()
			fmtVal.Minimize()
			if string(origVal.Pack()) != string(fmtVal.Pack()) {
				t.Fatalf("semantics changed:\n  orig: %s\n  fmt:  %s", origVal.Pack(), fmtVal.Pack())
			}
		}
	})
}

// TestTabsNormalizedToSpacesByDefault locks issue #584 for JSONC: default
// indent style is spaces, so tab-indented input is reformatted with spaces.
func TestTabsNormalizedToSpacesByDefault(t *testing.T) {
	t.Parallel()
	src := []byte("{\n\t\"name\": \"my-application-service\",\n\t\"description\": \"a value long enough to prevent collapsing\"\n}\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.NotContains(t, string(got), "\t", "default format must not preserve tabs")
	require.Contains(t, string(got), "  \"name\"")
}

// TestNonEmptyArraysIgnoreLineWidth verifies array expansion is independent of
// nesting depth and MaxLineWidth.
func TestNonEmptyArraysIgnoreLineWidth(t *testing.T) {
	t.Parallel()
	opts := defaultOpts
	opts.MaxLineWidth = 200
	src := []byte(`{"shallow":["a","b"],"nested":{"items":["x","y"]}}`)

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\"shallow\": [\n    \"a\",\n    \"b\",\n  ]")
	require.Contains(t, string(got), "\"items\": [\n      \"x\",\n      \"y\",\n    ]")

	second, err := f.Format(got, opts)
	require.NoError(t, err)
	require.Equal(t, got, second)
}

// TestNumericArraysExpand verifies numbers follow the same one-element-per-line
// rule as other non-empty arrays.
func TestNumericArraysExpand(t *testing.T) {
	t.Parallel()

	t.Run("nested arrays stay compact as elements", func(t *testing.T) {
		t.Parallel()
		got, err := f.Format([]byte(`[[1, 2], [3, 4]]`), defaultOpts)
		require.NoError(t, err)
		require.Equal(t, "[\n  [1, 2],\n  [3, 4],\n]\n", string(got))
	})

	t.Run("short numeric array", func(t *testing.T) {
		t.Parallel()
		src := []byte(`[1, 2, 3, 4, 5]`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Equal(t, "[\n  1,\n  2,\n  3,\n  4,\n  5,\n]\n", string(got))
	})

	t.Run("long numeric array", func(t *testing.T) {
		t.Parallel()
		src := []byte(`[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Contains(t, string(got), "  1,\n  2,\n  3,\n")
		require.Contains(t, string(got), "  23,\n  24,\n  25,\n")
	})

	t.Run("blank line preservation", func(t *testing.T) {
		t.Parallel()
		src := []byte("[1, 2, 3,\n\n4, 5, 6, 7]")
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		want := "[\n  1,\n  2,\n  3,\n\n  4,\n  5,\n  6,\n  7,\n]\n"
		require.Equal(t, want, string(got))
	})

	t.Run("nested numeric array", func(t *testing.T) {
		t.Parallel()
		src := []byte(`{"data": [1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]}`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Contains(t, string(got), "    1,\n    2,\n    3,\n")
	})

	t.Run("negative numbers", func(t *testing.T) {
		t.Parallel()
		src := []byte(`[-1, -2, -3, -4, -5, -6, -7, -8, -9, -10, -11, -12, -13, -14, -15, -16, -17, -18, -19, -20]`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Contains(t, string(got), "  -1,\n  -2,\n  -3,\n")
	})

	t.Run("mixed types", func(t *testing.T) {
		t.Parallel()
		src := []byte(`[1, "two", 3, "four", 5, "six", 7, "eight", 9, "ten", 11, "twelve", 13, "fourteen"]`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Contains(t, string(got), "  1,\n  \"two\"")
	})

	t.Run("comment preservation", func(t *testing.T) {
		t.Parallel()
		src := []byte("[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20 /* comment */]")
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Contains(t, string(got), "  1,\n  2,\n")
		require.Contains(t, string(got), "/* comment */\n]")
	})

	t.Run("expanded format is idempotent", func(t *testing.T) {
		t.Parallel()
		src := []byte("[1, 2, 3,\n\n4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18, 19, 20, 21, 22, 23, 24, 25]")
		first, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		second, err := f.Format(first, defaultOpts)
		require.NoError(t, err)
		require.Equal(t, string(first), string(second), "expanded format must be idempotent")
	})

	t.Run("single element expands", func(t *testing.T) {
		t.Parallel()
		src := []byte(`[42]`)
		got, err := f.Format(src, defaultOpts)
		require.NoError(t, err)
		require.Equal(t, "[\n  42,\n]\n", string(got))
	})
}

// FuzzJSONCFormatter seeds from fixtures and verifies no panics + idempotency.
func FuzzJSONCFormatter(f *testing.F) {
	inputs, _ := filepath.Glob("testdata/*.input.jsonc")
	for _, path := range inputs {
		data, _ := os.ReadFile(path)
		f.Add(data)
	}

	fmter := jsoncfmt.Formatter{}
	opts := jsoncfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, opts)
		if err != nil {
			return // rejected input
		}

		// Idempotency
		result2, err := fmter.Format(result, opts)
		if err != nil {
			t.Fatalf("second format pass failed: %v\nfirst output: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent:\ninput:  %q\nfirst:  %q\nsecond: %q", data, result, result2)
		}
	})
}
