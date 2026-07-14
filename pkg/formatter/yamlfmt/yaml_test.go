package yamlfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/yamlfmt"
)

var f = yamlfmt.Formatter{}
var defaultOpts = yamlfmt.DefaultOptions()

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

// FuzzYAMLFormatterWithOptions fuzzes with sort-keys + quote-style combined.
func FuzzYAMLFormatterWithOptions(f *testing.F) {
	f.Add([]byte("z: 'hello'\na: 'world'\n"))
	f.Add([]byte("---\nlist:\n  - 'one'\n  - 'two'\nmap:\n  z: 1\n  a: 2\n"))
	f.Add([]byte("key: 'value'\nnested:\n  b: 'x'\n  a: 'y'\n"))

	fmter := yamlfmt.Formatter{}
	opts := yamlfmt.DefaultOptions()
	opts.SortKeys = true
	opts.QuoteStyle = formatter.QuoteDouble

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
