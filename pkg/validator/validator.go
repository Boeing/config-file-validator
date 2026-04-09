package validator

import "errors"

// ErrNoSchema is returned by SchemaValidator.ValidateSchema when the document
// supports schema validation but does not declare a schema.
var ErrNoSchema = errors.New("no schema declared")

// ValidationError wraps a validation error with optional source position.
// Line and Column are 1-based. A zero value means the position is unknown.
type ValidationError struct {
	Err    error
	Line   int
	Column int
}

func (e *ValidationError) Error() string { return e.Err.Error() }
func (e *ValidationError) Unwrap() error { return e.Err }

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

// JSONMarshaler is an optional interface for validators whose content can be
// converted to JSON for schema validation. This is used when an external
// schema (e.g. from SchemaStore or --schema-map) is applied to a file that
// does not declare its own $schema.
type JSONMarshaler interface {
	MarshalToJSON(b []byte) ([]byte, error)
}

// XMLSchemaValidator is a marker interface for validators that use XSD
// schema validation instead of JSON Schema. When an external schema is
// applied via --schema-map or --schemastore, the CLI uses ValidateXSD
// instead of JSONSchemaValidate.
type XMLSchemaValidator interface {
	ValidateXSD(b []byte, schemaPath string) (bool, error)
}
