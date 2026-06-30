package hclfmt_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
)

var f = hclfmt.Formatter{}
var defaultOpts = formatter.Options{}

// TestFixtures runs all .input.hcl -> .expected.hcl fixture pairs.
func TestFixtures(t *testing.T) {
	t.Parallel()
	inputs, err := filepath.Glob("testdata/*.input.hcl")
	require.NoError(t, err)
	require.NotEmpty(t, inputs, "no fixture files found")

	for _, input := range inputs {
		name := strings.TrimSuffix(filepath.Base(input), ".input.hcl")
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			expected := strings.Replace(input, ".input.", ".expected.", 1)

			src, err := os.ReadFile(input)
			require.NoError(t, err)
			want, err := os.ReadFile(expected)
			require.NoError(t, err)

			got, err := f.Format(src, defaultOpts)
			require.NoError(t, err, "Format(%s) should not error", name)
			require.Equal(t, string(want), string(got), "unexpected output for %s", name)
		})
	}
}

// TestIdempotency verifies Format(Format(x)) == Format(x) for all fixtures.
func TestIdempotency(t *testing.T) {
	t.Parallel()
	expected, err := filepath.Glob("testdata/*.expected.hcl")
	require.NoError(t, err)
	require.NotEmpty(t, expected)

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

			require.Equal(t, string(first), string(second),
				"Format is not idempotent for %s", name)
		})
	}
}

// TestInvalidHCLReturnsError verifies that unparseable input returns an error.
func TestInvalidHCLReturnsError(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name string
		src  string
	}{
		{"unclosed block", "resource \"aws_instance\" \"web\" {\n"},
		{"bad token", "@@@ invalid"},
		{"missing equals", "key value\n"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := f.Format([]byte(tc.src), defaultOpts)
			require.Error(t, err, "expected error for invalid HCL: %s", tc.src)
		})
	}
}

// TestOptionsIgnored verifies that HCL formatting is canonical regardless
// of options passed (HCL has one style).
func TestOptionsIgnored(t *testing.T) {
	t.Parallel()
	src := []byte("variable \"x\" {\n  default = 1\n}\n")
	custom := formatter.Options{IndentWidth: 4, SortKeys: true}

	got, err := f.Format(src, custom)
	require.NoError(t, err)
	require.Equal(t, string(src), string(got), "HCL formatter should ignore options")
}

// FuzzHCLFormatter verifies no panics and idempotency on arbitrary inputs.
func FuzzHCLFormatter(f *testing.F) {
	f.Add([]byte("variable \"x\" {\n  default = 1\n}\n"))
	f.Add([]byte("resource \"null\" \"x\" {}\n"))

	fmter := hclfmt.Formatter{}

	f.Fuzz(func(t *testing.T, data []byte) {
		result, err := fmter.Format(data, formatter.Options{})
		if err != nil {
			return
		}
		result2, err := fmter.Format(result, formatter.Options{})
		if err != nil {
			t.Fatalf("second format pass failed: %v", err)
		}
		if string(result) != string(result2) {
			t.Fatalf("not idempotent: %q -> %q -> %q", data, result, result2)
		}
	})
}
