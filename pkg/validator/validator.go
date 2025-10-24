package validator

import "errors"

var ErrMethodUnimplemented = errors.New("method unimplemented")

// SyntaxValidator is the interface that wraps the ValidateSyntax method
// It accepts a byte array of a file or string to be validated for syntax
// and returns true or false if the content of the byte array is
// syntactically valid or not. If it is not valid, the error return value
// will be populated.
type SyntaxValidator interface {
	ValidateSyntax(b []byte) (bool, error)
}

// FormatValidator is the interface that wraps the ValidateFormat method
// It accepts a byte array of a file or string to be validated for format
// and returns true or false if the content of the byte array is
// valid in the specified format or not. If it is not valid, the error return value
// will be populated.
type FormatValidator interface {
	ValidateFormat(b []byte, options any) (bool, error)
}

type Validator interface {
	SyntaxValidator
	FormatValidator
}
