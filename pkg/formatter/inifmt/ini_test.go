package inifmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/inifmt"
)

var f = inifmt.Formatter{}
var defaultOpts = inifmt.DefaultOptions()

// TestFixtures runs all .input.ini -> .expected.ini fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.ini")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.ini")
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
	expected, err := filepath.Glob("testdata/*.expected.ini")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.ini")
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

// TestCommentPreservation verifies comments survive formatting.
func TestCommentPreservation(t *testing.T) {
	t.Parallel()
	src := []byte("; section comment\n[section]\n# key comment\nkey=value\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "; section comment", "section comment lost")
	require.Contains(t, string(got), "# key comment", "key comment lost")
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\nkey=value\n")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

// TestTabIndent verifies tab indentation works.
func TestTabIndent(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\nkey=value\n")
	opts := defaultOpts
	opts.IndentStyle = formatter.IndentTabs

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\tkey", "expected tab-indented key")
}

// TestSpaceIndent verifies space indentation works.
func TestSpaceIndent(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\nkey=value\n")
	opts := defaultOpts
	opts.IndentWidth = 4

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "    key", "expected 4-space indented key")
}

// TestFinalNewlineFalse verifies no trailing newline.
func TestFinalNewlineFalse(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\nkey=value\n")
	opts := defaultOpts
	opts.FinalNewline = false

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.NotEqual(t, byte('\n'), got[len(got)-1], "expected no trailing newline")
}

// TestCRLineEndings verifies bare \r (old Mac) line endings are handled.
func TestCRLineEndings(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\rkey=value\r")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "key = value", "key-value should be formatted")
}

// TestCRLFInputNormalization verifies CRLF input is parsed correctly.
func TestCRLFInputNormalization(t *testing.T) {
	t.Parallel()
	src := []byte("[section]\r\nkey=value\r\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "key = value", "key-value should be formatted")
}

// TestWhitespaceOnlyInput verifies trailing whitespace without newline.
func TestWhitespaceOnlyInput(t *testing.T) {
	t.Parallel()
	src := []byte("   ")
	_, err := f.Format(src, defaultOpts)
	// whitespace-only is either valid empty or error — just no panic
	_ = err
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid INI
	f.Add([]byte("[section]\nkey=value\n"))
	f.Add([]byte("; comment\n[s1]\na = b\n\n[s2]\nc = d\n"))
	f.Add([]byte("# global\nkey=val\n[sec]\nk2=v2\n"))
	f.Add([]byte("[a]\nk1 = v1\nk2 = v2\n[b]\nk3 = v3\n"))
	f.Add([]byte(""))
	f.Add([]byte("just some random text"))
	f.Add([]byte{0x00, 0xFF, 0xFE})
	f.Add([]byte("[[[nested]]]\nbroken"))

	fmtr := inifmt.Formatter{}
	opts := inifmt.DefaultOptions()

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
