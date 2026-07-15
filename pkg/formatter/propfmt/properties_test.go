package propfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/magiconair/properties"
	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/propfmt"
)

var f = propfmt.Formatter{}
var defaultOpts = propfmt.DefaultOptions()

// TestFixtures runs all .input.properties -> .expected.properties fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.properties")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.properties")
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
	expected, err := filepath.Glob("testdata/*.expected.properties")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.properties")
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

// TestInvalidPropertiesReturnsError verifies parse errors.
func TestInvalidPropertiesReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"invalid escape", `key = \uZZZZ`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid properties: %s", tc.src)
		})
	}
}

// TestCommentPreservation verifies comments survive formatting.
func TestCommentPreservation(t *testing.T) {
	t.Parallel()
	src := []byte("# important comment\nkey=value\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "# important comment",
		"comment was not preserved")
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("key=value\nanother=thing\n")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid properties
	f.Add([]byte("key=value\n"))
	f.Add([]byte("# comment\nk1 = v1\nk2 = v2\n"))
	f.Add([]byte("unicode = \\u0041\\u0042\n"))
	f.Add([]byte("multiline = line1\\\n  line2\n"))
	f.Add([]byte("key : value\n"))
	f.Add([]byte(""))
	f.Add([]byte("key = \\uZZZZ"))
	f.Add([]byte{0x00, 0xFF, 0xFE})
	f.Add([]byte("spaces\\ in\\ key = value\n"))

	fmtr := propfmt.Formatter{}
	opts := propfmt.DefaultOptions()

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

// TestContinuationAtEOF verifies that a trailing backslash at EOF (or before
// bare CR / CRLF) does not cause an infinite loop or panic. The fix is in
// tokenizer.go: break at EOF after endsWithOddBackslashes returns true.
func TestContinuationAtEOF(t *testing.T) {
	t.Parallel()
	fmtr := propfmt.Formatter{}
	opts := propfmt.DefaultOptions()

	cases := []struct {
		name  string
		input string
	}{
		{"odd_backslash_eof_no_newline", "key = value\\"},
		{"odd_backslash_before_bare_CR", "key = value\\\r"},
		{"odd_backslash_before_CRLF", "key = value\\\r\n"},
		{"even_backslashes_no_continuation", "key = value\\\\"},
		{"triple_backslash_eof", "key = value\\\\\\"},
		{"continuation_then_eof_empty", "key = \\\n"},
		{"continuation_before_bare_CR_content", "key = val\\\ranother = x"},
		{"multi_continuation_then_eof", "key = a\\\n  b\\\n  c\\"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result, err := fmtr.Format([]byte(tc.input), opts)
			if err != nil {
				// Some inputs may be invalid properties (e.g., bare CR in key).
				// That's fine — the test verifies no infinite loop/panic.
				return
			}
			// Must be idempotent
			result2, err := fmtr.Format(result, opts)
			require.NoError(t, err)
			require.Equal(t, string(result), string(result2))
		})
	}
}

func FuzzFormatWithOptions(f *testing.F) {
	f.Add([]byte("key = value\n"), byte(0))
	f.Add([]byte("# comment\nk1 = v1\nk2 = v2\n"), byte(1))
	f.Add([]byte("multi = line1\\\n  line2\n"), byte(3))

	fmtr := propfmt.Formatter{}
	f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
		opts := propfmt.DefaultOptions()
		if optByte&0x01 != 0 {
			opts.SortKeys = true
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

		// Semantic equivalence.
		origProps, origErr := properties.Load(data, properties.UTF8)
		fmtProps, fmtErr := properties.Load(result, properties.UTF8)
		if origErr == nil && fmtErr != nil {
			t.Fatalf("formatted output is invalid properties: %v\ninput: %q\noutput: %q", fmtErr, data, result)
		}
		if origErr == nil && fmtErr == nil {
			origMap := origProps.Map()
			fmtMap := fmtProps.Map()
			for k, v := range origMap {
				if fmtMap[k] != v {
					t.Fatalf("semantics changed for key %q: %q -> %q\ninput: %q\noutput: %q", k, v, fmtMap[k], data, result)
				}
			}
		}
	})
}
