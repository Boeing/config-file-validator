package validator

import (
	"fmt"

	"github.com/hashicorp/hcl/v2/hclparse"
)

type IncidentRule struct {
	RuleName string  `json:"rule"`
	Category string  `json:"category"`
	Incident int     `json:"incident"`
	Options  Options `json:"options"`
	Server   string  `json:"server"`
	Message  string  `json:"message"`
}

type Options struct {
	Priority string `json:"priority"`
	Color    string `json:"color"`
}

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
	_, diags := hclparse.NewParser().ParseHCL(b, "")
	if diags != nil {
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
