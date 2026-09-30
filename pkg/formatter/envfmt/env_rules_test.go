package envfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter/envfmt"
)

// Rule tests: each test names and asserts ONE formatting rule.
// These are cfv's .env formatting spec expressed as code.

// Test_ENV_WhitespaceNormalizedAroundEquals asserts that whitespace before
// the = delimiter is trimmed from the key, and the value after = is preserved
// as-is (env values are opaque strings).
func Test_ENV_WhitespaceNormalizedAroundEquals(t *testing.T) {
	t.Parallel()
	src := "KEY  =val\n ANOTHER =thing\n"
	opts := envfmt.DefaultOptions()

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "KEY=val",
		"whitespace before = must be trimmed from key")
	require.Contains(t, out, "ANOTHER=thing",
		"leading whitespace in key must be trimmed")
	require.NotContains(t, out, "KEY  =",
		"must not preserve extra spaces before =")
}

// Test_ENV_SortKeysAlphabetical asserts that SortKeys=true causes keys
// to be sorted alphabetically.
func Test_ENV_SortKeysAlphabetical(t *testing.T) {
	t.Parallel()
	src := "ZEBRA=1\nALPHA=2\nMANGO=3\n"
	opts := envfmt.DefaultOptions()
	opts.SortKeys = true

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	alphaIdx := strings.Index(out, "ALPHA=2")
	mangoIdx := strings.Index(out, "MANGO=3")
	zebraIdx := strings.Index(out, "ZEBRA=1")

	require.True(t, alphaIdx < mangoIdx && mangoIdx < zebraIdx,
		"keys must be sorted alphabetically")
}

// Test_ENV_SortKeysOffByDefault asserts that original key order is preserved
// when SortKeys is not enabled.
func Test_ENV_SortKeysOffByDefault(t *testing.T) {
	t.Parallel()
	src := "ZEBRA=1\nALPHA=2\nMANGO=3\n"
	opts := envfmt.DefaultOptions()

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	zebraIdx := strings.Index(out, "ZEBRA=1")
	alphaIdx := strings.Index(out, "ALPHA=2")
	mangoIdx := strings.Index(out, "MANGO=3")

	require.True(t, zebraIdx < alphaIdx && alphaIdx < mangoIdx,
		"original key order must be preserved by default")
}

// Test_ENV_QuotedValuesPreserved asserts that values wrapped in quotes
// retain their quotes through formatting.
func Test_ENV_QuotedValuesPreserved(t *testing.T) {
	t.Parallel()
	src := "KEY=\"val with spaces\"\nSINGLE='raw $VAR'\n"
	opts := envfmt.DefaultOptions()

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "KEY=\"val with spaces\"",
		"double-quoted values must be preserved")
	require.Contains(t, out, "SINGLE='raw $VAR'",
		"single-quoted values must be preserved")
}

// Test_ENV_CommentsPreserved asserts that # comments survive formatting.
func Test_ENV_CommentsPreserved(t *testing.T) {
	t.Parallel()
	src := "# database config\nDB_HOST=localhost\n# port\nDB_PORT=5432\n"
	opts := envfmt.DefaultOptions()

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "# database config",
		"header comments must be preserved")
	require.Contains(t, out, "# port",
		"inline group comments must be preserved")
}

// Test_ENV_ExportPrefixPreserved asserts that the "export " prefix on
// key-value lines is preserved through formatting.
func Test_ENV_ExportPrefixPreserved(t *testing.T) {
	t.Parallel()
	src := "export SECRET=hidden\nexport PATH=/usr/bin\n"
	opts := envfmt.DefaultOptions()

	got, err := envfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)
	out := string(got)

	require.Contains(t, out, "export SECRET=hidden",
		"export prefix must be preserved")
	require.Contains(t, out, "export PATH=/usr/bin",
		"export prefix must be preserved on all export lines")
}
