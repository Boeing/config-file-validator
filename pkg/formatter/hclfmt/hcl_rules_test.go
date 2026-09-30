package hclfmt_test

import (
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter/hclfmt"
)

// Test_HCL_CanonicalStyleApplied verifies that messy HCL input with bad
// spacing gets canonical formatting (aligned =, 2-space indent).
func Test_HCL_CanonicalStyleApplied(t *testing.T) {
	t.Parallel()

	src := `resource "aws_instance" "web" {
ami           =    "abc-123"
instance_type="t2.micro"
tags={
Name   =   "web"
}
}
`
	got, err := hclfmt.Formatter{}.Format([]byte(src), formatter.Options{})
	require.NoError(t, err)

	output := string(got)

	// Canonical HCL uses 2-space indent for block bodies.
	require.Contains(t, output, "  ami")
	require.Contains(t, output, "  instance_type")

	// Canonical HCL aligns = signs within a block and uses spaces around =.
	require.Contains(t, output, `ami           = "abc-123"`)
	require.Contains(t, output, `instance_type = "t2.micro"`)

	// Nested block indented.
	require.Contains(t, output, "  tags")
}

// Test_HCL_OptionsIgnored verifies that passing IndentWidth:4 has no effect
// because HCL has exactly one canonical style.
func Test_HCL_OptionsIgnored(t *testing.T) {
	t.Parallel()

	src := `variable "name" {
  default = "hello"
}
`
	defaultGot, err := hclfmt.Formatter{}.Format([]byte(src), formatter.Options{})
	require.NoError(t, err)

	customGot, err := hclfmt.Formatter{}.Format([]byte(src), formatter.Options{
		IndentWidth: 4,
		SortKeys:    true,
	})
	require.NoError(t, err)

	// Output must be identical regardless of options.
	require.Equal(t, string(defaultGot), string(customGot),
		"HCL formatter must ignore options — output should be identical")

	// Confirm it uses 2-space indent, not 4.
	require.Contains(t, string(customGot), "  default")
	require.NotContains(t, string(customGot), "    default")
}
