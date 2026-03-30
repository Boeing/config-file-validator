package validator

// Validator is the base interface that all validators must implement.
// For optional capabilities, use type assertions against SchemaValidator.
type Validator interface {
	ValidateSyntax(b []byte) (bool, error)
}

// SchemaValidator is an optional interface for validators that support schema validation.
type SchemaValidator interface {
	ValidateSchema(b []byte) (bool, error)
}
