package validator

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclparse"
)

// HclValidator is used to validate a byte slice that is intended to represent a
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
func (HclValidator) Validate(b []byte) (bool, error) {
	_, diags := hclparse.NewParser().ParseHCL(b, "")
	if diags == nil {
		return true, nil
	}

	subject := diags[0].Subject

	row := subject.Start.Line
	col := subject.Start.Column

	return false, fmt.Errorf("error at line %v column %v: %w", row, col, diags)
}
