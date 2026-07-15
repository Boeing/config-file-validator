package yamlfmt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	"gopkg.in/yaml.v3"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

var f = yamlfmt.Formatter{}
var defaultOpts = yamlfmt.DefaultOptions()

// yamlUnmarshal is a test helper that validates output is parseable YAML.
func yamlUnmarshal(data []byte, v any) error {
	return yaml.Unmarshal(data, v)
}

// TestFixtures runs all .input.yaml -> .expected.yaml fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.yaml")
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

// TestIdempotency verifies Format(Format(x, opts), opts) == Format(x, opts)
// using fixture-specific options when available.
func TestIdempotency(t *testing.T) {
	t.Parallel()
	expected, err := filepath.Glob("testdata/*.expected.yaml")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			// Use fixture-specific options when available.
			baseName := strings.TrimSuffix(name, ".expected.yaml")
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

// TestInvalidYAMLReturnsError verifies that unparseable input returns an error.
func TestInvalidYAMLReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"bad indentation", "key:\n  a: 1\n b: 2\n"},
		{"tab in mapping", "a:\n\tb: 1\n"},
		{"unclosed flow", "{a: 1, b: 2"},
		{"undefined anchor", "a: *undefined\n"},
		{"reserved indicator", "@ value\n"},
		{"control character", "key: \x00value\n"},
		{"second doc broken", "---\na: 1\n---\n{broken"},
		{"third doc broken", "---\na: 1\n---\nb: 2\n---\n@ bad\n"},
		{"duplicate key in second doc", "---\na: 1\n---\nb: 1\nb: 2\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid YAML: %s", tc.src)
		})
	}
}

// TestDuplicateKeysRejected verifies that duplicate mapping keys produce an
// error. This is caught by yaml.Unmarshal (the Node decoder silently keeps
// both keys).
func TestDuplicateKeysRejected(t *testing.T) {
	t.Parallel()
	src := []byte("a: 1\na: 2\n")
	_, err := f.Format(src, defaultOpts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "already defined")
}

// TestEmptyInputReturnsErrSkipped verifies that empty or whitespace-only
// input returns ErrSkipped rather than silently passing through.
func TestEmptyInputReturnsErrSkipped(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"empty", ""},
		{"whitespace only", "   \n  \n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err)
			var skipped *formatter.ErrSkipped
			require.ErrorAs(t, err, &skipped)
			require.Equal(t, "empty document", skipped.Reason)
		})
	}
}

// TestDocumentMarkerPreserved verifies --- is preserved when present.
func TestDocumentMarkerPreserved(t *testing.T) {
	t.Parallel()
	src := []byte("---\na: 1\nb: 2\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(string(got), "---\n"),
		"expected output to start with ---, got: %q", string(got))
}

// TestDocumentMarkerAbsent verifies --- is NOT added when absent.
func TestDocumentMarkerAbsent(t *testing.T) {
	t.Parallel()
	src := []byte("a: 1\nb: 2\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.False(t, strings.HasPrefix(string(got), "---\n"),
		"expected output NOT to start with ---, got: %q", string(got))
}

// TestMultiDocAlwaysHasMarkers verifies every document in a multi-doc file
// gets a --- separator.
func TestMultiDocAlwaysHasMarkers(t *testing.T) {
	t.Parallel()
	src := []byte("---\na: 1\n---\nb: 2\n---\nc: 3\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	count := strings.Count(string(got), "---\n")
	require.Equal(t, 3, count, "expected 3 --- markers in multi-doc output, got %d:\n%s", count, got)
}

// TestTabsRejectedAsInvalidYAML verifies that YAML with tab indentation
// inside a mapping is rejected — the YAML spec forbids tabs in indentation.
func TestTabsRejectedAsInvalidYAML(t *testing.T) {
	t.Parallel()
	src := []byte("a:\n\tb: 1\n")
	_, err := f.Format(src, defaultOpts)
	require.Error(t, err, "expected error for YAML with tab indentation in mapping")
}

// TestCRLFLineEnding verifies CRLF line endings are applied to output.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("a: 1\nb: 2\n")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
	require.NotContains(t, string(got), "\r\r\n", "must not double-CRLF")
}

// TestFinalNewlineFalse verifies that FinalNewline=false strips the trailing
// newline from YAML output.
func TestFinalNewlineFalse(t *testing.T) {
	t.Parallel()
	src := []byte("a: 1\nb: 2\n")
	opts := defaultOpts
	opts.FinalNewline = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEmpty(t, got)
	require.NotEqual(t, byte('\n'), got[len(got)-1],
		"expected no trailing newline, got: %q", got)
}

// TestIndentWidth4 verifies 4-space indent produces correctly indented output.
func TestIndentWidth4(t *testing.T) {
	t.Parallel()
	src := []byte("a:\n  b: 1\n")
	opts := defaultOpts
	opts.IndentWidth = 4

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "    b: 1", "expected 4-space indent")
}

// TestTabOptionReturnsError verifies that IndentTabs returns an error
// (YAML spec forbids tab indentation).
func TestTabOptionReturnsError(t *testing.T) {
	t.Parallel()
	src := []byte("a:\n  b: 1\n")
	opts := defaultOpts
	opts.IndentStyle = formatter.IndentTabs

	_, err := f.Format(src, opts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "tab indentation is not supported")
}

// TestCommentsPreserved verifies that all comment types survive formatting.
func TestCommentsPreserved(t *testing.T) {
	t.Parallel()
	src := []byte("# header\nkey: value # inline\n# footer\nother: thing\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "# header")
	require.Contains(t, string(got), "# inline")
	require.Contains(t, string(got), "# footer")
}

// TestMultiDocPartialDecodeReturnsError verifies that a broken second document
// surfaces an error instead of silently dropping the broken doc.
func TestMultiDocPartialDecodeReturnsError(t *testing.T) {
	t.Parallel()
	src := []byte("---\na: 1\n---\n{broken")
	_, err := f.Format(src, defaultOpts)
	require.Error(t, err, "expected error for broken second document")
	require.Contains(t, err.Error(), "yaml:")
}

// TestQuoteStyleDoesNotAffectKeys verifies that quote-style changes only
// apply to value scalars, never to mapping keys.
func TestQuoteStyleDoesNotAffectKeys(t *testing.T) {
	t.Parallel()
	src := []byte("\"double-key\": 'value1'\n'single-key': 'value2'\nplain-key: 'value3'\n")
	opts := defaultOpts
	opts.QuoteStyle = formatter.QuoteDouble

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	output := string(got)
	// Keys must be unchanged.
	require.Contains(t, output, "\"double-key\":")
	require.Contains(t, output, "'single-key':")
	require.Contains(t, output, "plain-key:")
	// Values must be converted to double.
	require.Contains(t, output, ": \"value1\"")
	require.Contains(t, output, ": \"value2\"")
	require.Contains(t, output, ": \"value3\"")
}

// TestDocumentEndMarkerPreserved verifies ... is preserved when present.
func TestDocumentEndMarkerPreserved(t *testing.T) {
	t.Parallel()
	src := []byte("---\na: 1\n...\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "...\n",
		"expected output to contain ..., got: %q", string(got))
}

// TestDocumentEndMarkerAbsent verifies ... is NOT added when not in source.
func TestDocumentEndMarkerAbsent(t *testing.T) {
	t.Parallel()
	src := []byte("---\na: 1\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.NotContains(t, string(got), "...",
		"expected output NOT to contain ..., got: %q", string(got))
}

// TestSequenceAtRootIsFormattable verifies that root-level sequences are accepted.
func TestSequenceAtRootIsFormattable(t *testing.T) {
	t.Parallel()
	src := []byte("- one\n- two\n- three\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "- one")
}

// TestBareScalarReturnsError verifies that bare scalars at root are rejected.
func TestBareScalarReturnsError(t *testing.T) {
	t.Parallel()
	src := []byte("just a plain string\n")
	_, err := f.Format(src, defaultOpts)
	require.Error(t, err)
	require.Contains(t, err.Error(), "not a mapping or sequence")
}

// TestZeroOptionsUsesDefaults verifies that all-zero Options produces
// sensible output (2-space indent). FinalNewline is false (zero value) so
// the formatter doesn't force a trailing newline.
func TestZeroOptionsUsesDefaults(t *testing.T) {
	t.Parallel()
	src := []byte("a:\n    b: 1\n")
	got, err := f.Format(src, formatter.Options{}) // all zero
	require.NoError(t, err)
	require.Contains(t, string(got), "  b: 1") // 2-space default
}

// TestSortKeysAnchorSafety proves that SortKeys does not reorder entries when
// doing so would break anchor/alias references (producing invalid YAML).
func TestSortKeysAnchorSafety(t *testing.T) {
	t.Parallel()

	sortOpts := defaultOpts
	sortOpts.SortKeys = true

	tests := []struct {
		name         string
		input        string
		expectSorted bool // true = entries SHOULD be sorted; false = entries MUST stay in original order
	}{
		{
			name: "cross_entry_dependency_blocks_sort",
			input: `z_defaults: &db
  host: localhost
a_service:
  db: *db
`,
			expectSorted: false, // a_service uses *db defined in z_defaults — can't reorder
		},
		{
			name: "self_contained_anchors_allow_sort",
			input: `zebra:
  config: &z_cfg
    port: 9090
  server: *z_cfg
alpha:
  value: simple
`,
			expectSorted: true, // anchor and alias are within the SAME entry — safe to sort
		},
		{
			name: "merge_key_blocks_sort",
			input: `z_base: &base
  timeout: 30
a_extended:
  <<: *base
  retries: 3
`,
			expectSorted: false, // merge key references anchor from different entry
		},
		{
			name: "no_anchors_sorts_normally",
			input: `zebra: 1
alpha: 2
`,
			expectSorted: true,
		},
		{
			name: "nested_mappings_still_sorted_when_top_blocked",
			input: `z_defaults: &db
  zoo: 3
  alpha: 1
a_service:
  db: *db
  zebra: 2
  ant: 1
`,
			expectSorted: false, // top level blocked, but nested should sort
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := f.Format([]byte(tc.input), sortOpts)
			require.NoError(t, err, "Format should not error")

			// CRITICAL: output must be valid YAML that yaml.v3 can parse.
			// This is the core assertion — if sorting broke anchor/alias ordering,
			// yaml.v3 would reject it with "unknown anchor referenced".
			var parsed any
			err = yamlUnmarshal(got, &parsed)
			require.NoError(t, err, "formatted output must be valid YAML:\n%s", got)

			// Verify sort behavior.
			output := string(got)
			if tc.expectSorted {
				// Keys should be alphabetically ordered at the top level.
				aIdx := strings.Index(output, "alpha")
				zIdx := strings.Index(output, "zebra")
				require.Greater(t, zIdx, aIdx,
					"expected alpha before zebra (sorted):\n%s", output)
			} else {
				// Keys must remain in original order (z before a).
				zIdx := strings.Index(output, "z_")
				aIdx := strings.Index(output, "a_")
				require.Greater(t, aIdx, zIdx,
					"expected z_ before a_ (unsorted due to anchor dep):\n%s", output)
			}

			// For the nested test case, verify nested keys ARE sorted even though
			// top level is blocked.
			if tc.name == "nested_mappings_still_sorted_when_top_blocked" {
				// Within z_defaults, alpha should come before zoo.
				lines := strings.Split(output, "\n")
				var alphaLine, zooLine int
				for i, line := range lines {
					if strings.Contains(line, "alpha:") {
						alphaLine = i
					}
					if strings.Contains(line, "zoo:") {
						zooLine = i
					}
				}
				require.Greater(t, zooLine, alphaLine,
					"nested keys should be sorted (alpha before zoo):\n%s", output)

				// Within a_service, ant should come before zebra.
				var antLine, zebraLine int
				for i, line := range lines {
					if strings.Contains(line, "ant:") {
						antLine = i
					}
					if strings.Contains(line, "zebra:") {
						zebraLine = i
					}
				}
				require.Greater(t, zebraLine, antLine,
					"nested keys should be sorted (ant before zebra):\n%s", output)
			}

			// Idempotency: formatting again produces same output.
			got2, err := f.Format(got, sortOpts)
			require.NoError(t, err)
			require.Equal(t, string(got), string(got2),
				"must be idempotent:\nfirst:  %q\nsecond: %q", got, got2)
		})
	}
}

// FuzzYAMLFormatter verifies no panics and idempotency on arbitrary inputs.
func FuzzYAMLFormatter(f *testing.F) {
	f.Add([]byte("a: 1\nb: 2\n"))
	f.Add([]byte("---\nkey: value\n"))
	f.Add([]byte("items:\n  - one\n  - two\n"))
	f.Add([]byte("# comment\nkey: value\n"))
	f.Add([]byte("a: \"quoted\"\nb: 'single'\n"))
	f.Add([]byte("---\na: 1\n---\nb: 2\n"))

	fmter := yamlfmt.Formatter{}
	opts := yamlfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, opts)
		if err != nil {
			return // rejected input — fine, just didn't panic
		}

		// Idempotency: formatting the output again must produce identical output.
		result2, err := fmter.Format(result, opts)
		if err != nil {
			t.Fatalf("second format pass failed: %v\nfirst output: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent:\ninput:  %q\nfirst:  %q\nsecond: %q", data, result, result2)
		}
	})
}

// FuzzYAMLFormatterWithOptions fuzzes with various option combinations.
func FuzzYAMLFormatterWithOptions(f *testing.F) {
	f.Add([]byte("z: 'hello'\na: 'world'\n"), byte(0))
	f.Add([]byte("---\nlist:\n  - 'one'\n  - 'two'\nmap:\n  z: 1\n  a: 2\n"), byte(1))
	f.Add([]byte("key: 'value'\nnested:\n  b: 'x'\n  a: 'y'\n"), byte(3))
	f.Add([]byte("data: |\n  line1\n  line2\n"), byte(5))

	fmtr := yamlfmt.Formatter{}
	f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
		opts := yamlfmt.DefaultOptions()
		if optByte&0x01 != 0 {
			opts.SortKeys = true
		}
		if optByte&0x02 != 0 {
			opts.QuoteStyle = formatter.QuoteDouble
		}
		if optByte&0x04 != 0 {
			opts.IndentWidth = 4
		}
		if optByte&0x08 != 0 {
			opts.FinalNewline = false
		}
		if optByte&0x10 != 0 {
			opts.QuoteStyle = formatter.QuoteSingle
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

		// Semantic equivalence.
		var origVal, fmtVal any
		if yaml.Unmarshal(data, &origVal) == nil {
			if err := yaml.Unmarshal(result, &fmtVal); err != nil {
				t.Fatalf("formatted output is invalid YAML: %v\ninput: %q\noutput: %q", err, data, result)
			}
			origJSON, _ := json.Marshal(origVal)
			fmtJSON, _ := json.Marshal(fmtVal)
			if string(origJSON) != string(fmtJSON) {
				t.Fatalf("semantics changed:\n  orig: %s\n  fmt:  %s", origJSON, fmtJSON)
			}
		}
	})
}

// TestBlockScalarChompingPreservation verifies that formatting preserves
// block scalar chomping semantics (|+, |-, |, >+, >-).
func TestBlockScalarChompingPreservation(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string // expected value after yaml.Unmarshal of formatted output
	}{
		{
			name:     "literal keep (|+)",
			input:    "k: |+\n  text\n\n\n",
			expected: "text\n\n\n",
		},
		{
			name:     "literal strip (|-)",
			input:    "k: |-\n  text\n",
			expected: "text",
		},
		{
			name:     "literal clip (|)",
			input:    "k: |\n  text\n",
			expected: "text\n",
		},
		{
			name:     "folded keep (>+)",
			input:    "k: >+\n  text\n\n\n",
			expected: "text\n\n\n",
		},
		{
			name:     "folded strip (>-)",
			input:    "k: >-\n  text\n",
			expected: "text",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			formatted, err := f.Format([]byte(tc.input), defaultOpts)
			require.NoError(t, err, "Format should not error")

			var result map[string]string
			err = yaml.Unmarshal(formatted, &result)
			require.NoError(t, err, "formatted output must be valid YAML: %q", formatted)

			require.Equal(t, tc.expected, result["k"],
				"chomping semantics corrupted.\nInput:     %q\nFormatted: %q\nGot value: %q",
				tc.input, formatted, result["k"])
		})
	}
}

// TestNormalizeValueSpacing verifies that extra whitespace between colon and
// value is normalized to a single space.
func TestNormalizeValueSpacing(t *testing.T) {
	t.Parallel()
	fmtr := yamlfmt.Formatter{}
	opts := yamlfmt.DefaultOptions()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"extra_spaces", "key:    value\n", "key: value\n"},
		{"already_correct", "key: value\n", "key: value\n"},
		{"tab_after_colon", "key:\tvalue\n", "key: value\n"},
		{"quoted_with_space", "key:  \"quoted\"\n", "key: \"quoted\"\n"},
		{"anchor_with_space", "key:   &name val\n", "key: &name val\n"},
		{"tag_with_space", "key:   !!str 42\n", "key: !!str 42\n"},
		{"nested", "parent:\n  child:    deep\n", "parent:\n  child: deep\n"},
		{"preserves_internal", "key: value with   spaces\n", "key: value with   spaces\n"},
		{"empty_value", "key:\n", "key:\n"},
		{"multiple_keys", "a:   1\nb:  2\nc: 3\n", "a: 1\nb: 2\nc: 3\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := fmtr.Format([]byte(tc.input), opts)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(result))
		})
	}
}

// TestNormalizeFlowCollections verifies that flow collections get normalized
// spacing via AST-driven re-serialization.
func TestNormalizeFlowCollections(t *testing.T) {
	t.Parallel()
	fmtr := yamlfmt.Formatter{}
	opts := yamlfmt.DefaultOptions()

	cases := []struct {
		name  string
		input string
		want  string
	}{
		{"normalize_spaces", "x: {a:  1, b:   2}\n", "x: {a: 1, b: 2}\n"},
		{"nested_flow", "x: {a: {b: 1}, c: [1, 2]}\n", "x: {a: {b: 1}, c: [1, 2]}\n"},
		{"array_spaces", "x: [1,  2,   3]\n", "x: [1, 2, 3]\n"},
		{"empty_map", "x: {}\n", "x: {}\n"},
		{"empty_array", "x: []\n", "x: []\n"},
		{"quoted_values", "x: {a: \"hello\", b: 'world'}\n", "x: {a: \"hello\", b: 'world'}\n"},
		{"already_normalized", "x: {a: 1, b: 2}\n", "x: {a: 1, b: 2}\n"},
		{"mixed_types", "x: {s: hello, n: 42, b: true, null: null}\n", "x: {s: hello, n: 42, b: true, null: null}\n"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := fmtr.Format([]byte(tc.input), opts)
			require.NoError(t, err)
			require.Equal(t, tc.want, string(result))
		})
	}
}

func TestBlockScalarFinalNewlineFalse(t *testing.T) {
	t.Parallel()
	fmtr := yamlfmt.Formatter{}
	opts := yamlfmt.DefaultOptions()
	opts.FinalNewline = false

	cases := []struct {
		name    string
		input   string
		wantVal string // expected value after yaml.Unmarshal of formatted output
	}{
		{"clip_preserves_newline", "A: |\n  0\n", "0\n"},
		{"keep_preserves_all", "A: |+\n  0\n\n\n", "0\n\n\n"},
		{"strip_removes_newline", "A: |-\n  0\n", "0"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			result, err := fmtr.Format([]byte(tc.input), opts)
			require.NoError(t, err)

			var parsed map[string]string
			err = yaml.Unmarshal(result, &parsed)
			require.NoError(t, err, "formatted output: %q", result)
			require.Equal(t, tc.wantVal, parsed["A"],
				"input=%q output=%q", tc.input, result)
		})
	}
}
