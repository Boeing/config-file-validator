package envfmt_test

import (
	"bytes"
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/hashicorp/go-envparse"
	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/envfmt"
)

var update = flag.Bool("update", false, "update .expected.* golden files")

var f = envfmt.Formatter{}
var defaultOpts = envfmt.DefaultOptions()

// TestFixtures runs all .input.env -> .expected.env fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.env")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.env")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expected := strings.Replace(input, ".input.", ".expected.", 1)

			src, err := os.ReadFile(input)
			require.NoError(t, err)

			optsFile := "testdata/" + name + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			got, err := f.Format(src, opts)
			require.NoError(t, err, "Format(%s) should not error", name)

			_, parseErr := envparse.Parse(bytes.NewReader(got))
			require.NoError(t, parseErr,
				"Format output is not valid env for %s", name)

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
	expected, err := filepath.Glob("testdata/*.expected.env")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

	for _, file := range expected {
		name := filepath.Base(file)
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			src, err := os.ReadFile(file)
			require.NoError(t, err)

			baseName := strings.TrimSuffix(name, ".expected.env")
			optsFile := "testdata/" + baseName + ".opts.json"
			opts := formatter.LoadFixtureOptions(optsFile, defaultOpts)

			first, err := f.Format(src, opts)
			require.NoError(t, err)

			_, parseErr := envparse.Parse(bytes.NewReader(first))
			require.NoError(t, parseErr,
				"Format output is not valid env for %s", name)

			second, err := f.Format(first, opts)
			require.NoError(t, err)

			require.Equal(t, string(first), string(second),
				"Format is not idempotent for %s", name)
		})
	}
}

// TestMalformedLineReturnsError verifies parse errors.
func TestMalformedLineReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"no equals", "KEYONLY"},
		{"no equals with text", "some random text"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for malformed env: %s", tc.src)
		})
	}
}

// TestCommentPreservation verifies comments survive formatting.
func TestCommentPreservation(t *testing.T) {
	t.Parallel()
	src := []byte("# important comment\nKEY=value\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Contains(t, string(got), "# important comment",
		"comment was not preserved")
}

// TestCRLFLineEnding verifies CRLF line endings are applied.
func TestCRLFLineEnding(t *testing.T) {
	t.Parallel()
	src := []byte("KEY=value\nANOTHER=thing\n")
	opts := defaultOpts
	opts.LineEnding = formatter.LineEndingCRLF

	got, err := f.Format(src, opts)
	require.NoError(t, err)
	require.Contains(t, string(got), "\r\n", "expected CRLF line endings")
}

// TestEmptyValueAllowed verifies KEY= is valid.
func TestEmptyValueAllowed(t *testing.T) {
	t.Parallel()
	src := []byte("KEY=\n")
	got, err := f.Format(src, defaultOpts)
	require.NoError(t, err)
	require.Equal(t, "KEY=\n", string(got))
}

// FuzzFormat feeds arbitrary bytes to Format and checks:
// - No panics on any input
// - If Format succeeds, output re-parses without error
// - If Format succeeds, formatting is idempotent
func FuzzFormat(f *testing.F) {
	// Seed corpus with valid and invalid env
	f.Add([]byte("KEY=value\n"))
	f.Add([]byte("# comment\nKEY=value\nANOTHER=thing\n"))
	f.Add([]byte("export SECRET=hidden\n"))
	f.Add([]byte("QUOTED=\"hello world\"\n"))
	f.Add([]byte("SINGLE='raw $VAR'\n"))
	f.Add([]byte("EMPTY=\n"))
	f.Add([]byte(""))
	f.Add([]byte("NOEQUALS"))
	f.Add([]byte{0x00, 0xFF, 0xFE})
	f.Add([]byte("KEY=value\n\n# group\nA=1\nB=2\n"))

	fmtr := envfmt.Formatter{}
	opts := envfmt.DefaultOptions()

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

// FuzzENVFormatter seeds from fixture files and checks idempotency.
func FuzzENVFormatter(f *testing.F) {
	inputs, _ := filepath.Glob("testdata/*.input.env")
	for _, path := range inputs {
		data, _ := os.ReadFile(path)
		f.Add(data)
	}

	fmter := envfmt.Formatter{}
	opts := envfmt.DefaultOptions()

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, opts)
		if err != nil {
			return
		}

		result2, err := fmter.Format(result, opts)
		if err != nil {
			t.Fatalf("second format pass failed: %v\nfirst output: %q", err, result)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent:\ninput:  %q\nfirst:  %q\nsecond: %q", data, result, result2)
		}
	})
}
