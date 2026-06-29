package validator

import (
	"errors"

	"github.com/Boeing/config-file-validator/v3/pkg/validator/justfile"
)

type JustfileValidator struct{}

var _ Validator = JustfileValidator{}

func (JustfileValidator) ValidateSyntax(b []byte) (bool, error) {
	jf, err := justfile.Parse(b)
	if err != nil {
		var pe *justfile.ParseError
		if errors.As(err, &pe) {
			return false, &ValidationError{
				Err:    errors.New(pe.Message),
				Line:   pe.Pos.Line,
				Column: pe.Pos.Column,
			}
		}
		return false, err
	}

	diags := jf.Validate()
	for _, d := range diags {
		if d.Severity == justfile.SeverityError {
			return false, &ValidationError{
				Err:    errors.New(d.Message),
				Line:   d.Pos.Line,
				Column: d.Pos.Column,
			}
		}
	}

	return true, nil
}
