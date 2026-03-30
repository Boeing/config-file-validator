package validator

import "errors"

// ErrNoSchema is returned by SchemaValidator.ValidateSchema when the document
// supports schema validation but does not declare a schema.
var ErrNoSchema = errors.New("no schema declared")

// Validator is the base interface that all validators must implement.
// For optional capabilities, use type assertions against SchemaValidator.
type Validator interface {
	ValidateSyntax(b []byte) (bool, error)
}

// SchemaValidator is an optional interface for validators that support
// schema validation. The filePath parameter is the absolute path to the
// file being validated, used to resolve relative schema references.
// ValidateSchema should return ErrNoSchema when the document does not
// declare a schema (e.g. no $schema property in JSON).
type SchemaValidator interface {
	ValidateSchema(b []byte, filePath string) (bool, error)
}
