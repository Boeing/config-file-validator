// Package hclfmt provides a Formatter for HCL (HashiCorp Configuration Language) files.
//
// The formatter delegates to hclwrite.Format which produces the canonical
// style used by terraform fmt. No options are supported — HCL has one
// canonical style.
//
// Comments are preserved by the hclwrite formatter.
package hclfmt

import (
	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsyntax"
	"github.com/hashicorp/hcl/v2/hclwrite"

	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
)

// Formatter formats HCL files to canonical style.
// It is stateless and safe for concurrent use.
type Formatter struct{}

var _ formatter.Formatter = Formatter{}

// Format returns the canonically formatted version of src.
// Returns an error if src is not valid HCL.
// Options are ignored — HCL has one canonical style (2-space indent,
// aligned equals, HashiCorp convention).
func (Formatter) Format(src []byte, opts formatter.Options) ([]byte, error) {
	// Validate syntax first — hclwrite.Format silently returns garbage
	// for invalid input rather than erroring.
	_, diags := hclsyntax.ParseConfig(src, "", hcl.Pos{Line: 1, Column: 1})
	if diags.HasErrors() {
		return nil, diags
	}

	result := hclwrite.Format(src)

	// hclwrite.Format returns nil for empty input. An empty file is valid
	// HCL, so return appropriate output based on FinalNewline.
	if len(result) == 0 {
		if opts.FinalNewline {
			return []byte("\n"), nil
		}
		return []byte{}, nil
	}

	return result, nil
}
