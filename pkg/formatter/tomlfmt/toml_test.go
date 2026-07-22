package tomlfmt_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	toml "github.com/pelletier/go-toml/v2"
	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/tomlfmt"
)

var f = tomlfmt.Formatter{}
var defaultOpts = tomlfmt.DefaultOptions()

// TestFixtures runs all .input.toml -> .expected.toml fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.toml")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.toml")
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
	expected, err := filepath.Glob("testdata/*.expected.toml")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.toml")
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

// TestInvalidTOMLReturnsError verifies that unparseable input returns an error.
func TestInvalidTOMLReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"unclosed string", `key = "unterminated`},
		{"duplicate key", "[section]\nkey = 1\nkey = 2"},
		{"bare invalid", "= value"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid TOML: %s", tc.src)
		})
	}
}

// TestCommentPreservation verifies all comments survive formatting.
func TestCommentPreservation(t *testing.T) {
	t.Parallel()
	src := []byte("# top comment\ntitle=\"value\" # inline\n\n[section]\n# section comment\nkey=\"val\"\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "# top comment", "top comment lost")
	require.Contains(t, string(got), "# inline", "inline comment lost")
	require.Contains(t, string(got), "# section comment", "section comment lost")
}

// TestInlineCommentPreserved verifies inline comments after values are kept.
func TestInlineCommentPreserved(t *testing.T) {
	t.Parallel()
	src := []byte("port=8080 # HTTP port\nhost=\"localhost\" # primary\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "# HTTP port", "inline comment lost")
	require.Contains(t, string(got), "# primary", "inline comment lost")
	// Also verify spacing was normalized.
	require.Contains(t, string(got), "port = 8080", "spacing not normalized")
}

func TestBlankLinesBeforeTables(t *testing.T) {
	t.Parallel()
	src := []byte("[package]\nname = \"app\"\n[dependencies]\nserde = \"1\"\n# binaries\n[[bin]]\nname = \"app\"\n")
	want := "[package]\nname = \"app\"\n\n[dependencies]\nserde = \"1\"\n\n# binaries\n[[bin]]\nname = \"app\"\n"

	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Equal(t, want, string(got))

	gotAgain, err := f.Format(got, defaultOpts)
	require.NoError(t, err)
	require.Equal(t, string(got), string(gotAgain), "existing section breaks must not be doubled")
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("key = \"value\"\nanother = \"thing\"\n")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

// TestIndentedTableContents verifies indent option works.
func TestIndentedTableContents(t *testing.T) {
	t.Parallel()
	src := []byte("[database]\nhost=\"localhost\"\nport=5432\n")
	opts := defaultOpts
	opts.IndentWidth = 2

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "  host = ", "expected 2-space indent")
	require.Contains(t, string(got), "  port = ", "expected 2-space indent")
}

// TestMaxLineWidthBreaksArrays verifies MaxLineWidth decides when an array is
// split across lines. Without it the width is taplo's default of 80.
func TestMaxLineWidthBreaksArrays(t *testing.T) {
	t.Parallel()
	src := []byte("arr = [\"aaaa\", \"bbbb\", \"cccc\"]\n")

	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Equal(t, string(src), string(got), "array fits in 80 columns")

	opts := defaultOpts
	opts.MaxLineWidth = 10
	got, err = f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\n  \"aaaa\",", "expected one element per line")
}

// TestTrailingCommasNone drops the comma after the last array element.
func TestTrailingCommasNone(t *testing.T) {
	t.Parallel()
	src := []byte("arr = [\n  \"aaaa\",\n  \"bbbb\",\n]\n")
	opts := defaultOpts
	opts.MaxLineWidth = 10

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\"bbbb\",\n]", "preserve keeps the trailing comma")

	opts.TrailingCommas = formatter.TrailingCommasNone
	got, err = f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\"aaaa\",\n", "commas between elements are kept")
	require.Contains(t, string(got), "\"bbbb\"\n]", "expected no trailing comma")
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid TOML
	f.Add([]byte("key = \"value\"\n"))
	f.Add([]byte("[section]\nkey = 123\nanother = true\n"))
	f.Add([]byte("# comment\n[table]\narr = [1, 2, 3]\n"))
	f.Add([]byte("[a]\nb = \"c\"\n[a.d]\ne = 2024-01-01\n"))
	f.Add([]byte(""))
	f.Add([]byte("not toml {{{}}}"))
	f.Add([]byte("[broken\nkey"))
	f.Add([]byte{0x00, 0xFF, 0xFE})

	fmtr := tomlfmt.Formatter{}
	opts := tomlfmt.DefaultOptions()

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
	f.Add([]byte("[package]\nname = \"x\"\nversion = \"1.0\"\n"), byte(0))
	f.Add([]byte("[deps]\nserde = {version=\"1.0\", features=[\"derive\"]}\n"), byte(1))
	f.Add([]byte("[[bin]]\nname=\"a\"\npath=\"b\"\n"), byte(3))

	fmtr := tomlfmt.Formatter{}
	f.Fuzz(func(t *testing.T, data []byte, optByte byte) {
		opts := tomlfmt.DefaultOptions()
		if optByte&0x01 != 0 {
			opts.SortKeys = true
		}
		if optByte&0x02 != 0 {
			opts.IndentWidth = 4
		}
		if optByte&0x04 != 0 {
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

		// Semantic equivalence: parsed values must match.
		var origMap, fmtMap map[string]any
		if err := toml.Unmarshal(data, &origMap); err == nil {
			if err := toml.Unmarshal(result, &fmtMap); err != nil {
				t.Fatalf("formatted output is invalid TOML: %v\ninput: %q\noutput: %q", err, data, result)
			}
			// JSON round-trip handles NaN/Inf comparison.
			origJSON, _ := json.Marshal(origMap)
			fmtJSON, _ := json.Marshal(fmtMap)
			if string(origJSON) != string(fmtJSON) {
				t.Fatalf("semantics changed:\n  input:  %s\n  output: %s", origJSON, fmtJSON)
			}
		}
	})
}
