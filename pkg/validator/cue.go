package validator

import (
	"errors"

	cueerrors "cuelang.org/go/cue/errors"
	"cuelang.org/go/cue/parser"
)

type CueValidator struct{}

var _ Validator = CueValidator{}

func (CueValidator) ValidateSyntax(b []byte) (bool, error) {
	_, err := parser.ParseFile("input.cue", b)
	if err == nil {
		return true, nil
	}

	var cerr cueerrors.Error
	if errors.As(err, &cerr) && cerr != nil {
		pos := cerr.Position()
		if pos.IsValid() {
			return false, &ValidationError{
				Err:    err,
				Line:   pos.Line(),
				Column: pos.Column(),
			}
		}
	}

	return false, err
}
