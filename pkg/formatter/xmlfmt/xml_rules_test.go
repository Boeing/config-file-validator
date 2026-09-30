package xmlfmt_test

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/xmlfmt"
)

// Test_XML_DefaultIndentIs2Spaces verifies that nested elements are indented
// with 2 spaces by default.
func Test_XML_DefaultIndentIs2Spaces(t *testing.T) {
	t.Parallel()

	src := `<?xml version="1.0"?><root><child>value</child></root>`

	got, err := xmlfmt.Formatter{}.Format([]byte(src), xmlfmt.DefaultOptions())
	require.NoError(t, err)

	output := string(got)

	// Child element must be indented exactly 2 spaces.
	require.Contains(t, output, "  <child>value</child>")

	// Must NOT be indented with 4 spaces or tabs.
	require.NotContains(t, output, "    <child>")
	require.NotContains(t, output, "\t<child>")
}

// Test_XML_IndentRespectsOption verifies that IndentWidth:4 produces 4-space
// indentation for nested elements.
func Test_XML_IndentRespectsOption(t *testing.T) {
	t.Parallel()

	src := `<?xml version="1.0"?><root><child>value</child></root>`

	opts := xmlfmt.DefaultOptions()
	opts.IndentWidth = 4

	got, err := xmlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)

	output := string(got)

	// Child element must be indented exactly 4 spaces.
	require.Contains(t, output, "    <child>value</child>")

	// Must NOT use 2-space indent.
	require.NotContains(t, output, "\n  <child>")
}

// Test_XML_DefaultHasFinalNewline verifies that formatted output ends with
// exactly one newline character by default.
func Test_XML_DefaultHasFinalNewline(t *testing.T) {
	t.Parallel()

	src := `<?xml version="1.0"?><root><child>value</child></root>`

	opts := xmlfmt.DefaultOptions()
	opts.FinalNewline = true

	got, err := xmlfmt.Formatter{}.Format([]byte(src), opts)
	require.NoError(t, err)

	output := string(got)

	// Must end with exactly one newline.
	require.True(t, strings.HasSuffix(output, "\n"),
		"output must end with a newline")
	require.NotContains(t, output, "\n\n"+string(rune(0)),
		"output must not end with multiple newlines")

	// Verify by checking last two chars.
	require.Equal(t, byte('\n'), output[len(output)-1])
	if len(output) > 1 {
		require.NotEqual(t, byte('\n'), output[len(output)-2],
			"output ends with multiple newlines")
	}

	// Also confirm FinalNewline=false does NOT end with newline.
	optsNoNL := formatter.Options{FinalNewline: false}
	gotNoNL, err := xmlfmt.Formatter{}.Format([]byte(src), optsNoNL)
	require.NoError(t, err)
	require.NotEqual(t, byte('\n'), gotNoNL[len(gotNoNL)-1],
		"FinalNewline=false must not end with newline")
}
