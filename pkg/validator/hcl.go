package validator

import (
	"fmt"

	"github.com/hashicorp/hcl/v2"
	"github.com/hashicorp/hcl/v2/hclsimple"
)

// HclValidator is used to validate a byte slice that is indented to represent a
// HashiCorp Configuration Language (HCL) file.
type HclValidator struct{}

// Validate checks if the provided byte slice represents a valid .hcl file.
//
// The hcl parser uses FIFO to determine which error to display to the user. For
// more information, see the documentation at:
//
// https://pkg.go.dev/github.com/hashicorp/hcl/v2#Diagnostics.Error
//
// If the hcl.Diagnostics slice contains more than one error, the wrapped
// error returned by this function will include them as "and {count} other
// diagnostic(s)" in the error message.
//
// If the parsing error does not produce an hcl.Diagnostics slice, a generic
// error will be returned, wrapping the input file.
func (hclv HclValidator) Validate(b []byte) (bool, error) {
	//_, diags := hclparse.NewParser().ParseHCL(b, "")
	err := hclsimple.Decode(".hcl", b, nil, &map[interface{}]interface{}{})
	if err != nil {
		diags := err.(hcl.Diagnostics)
		if len(diags) == 0 {
			return false, fmt.Errorf("error validating .hcl file: %w", diags)
		}

		subject := diags[0].Subject

		row := subject.Start.Line
		col := subject.Start.Column

		return false, fmt.Errorf("error at line %v column %v: %w", row, col, diags)
	}

	return true, nil
}
