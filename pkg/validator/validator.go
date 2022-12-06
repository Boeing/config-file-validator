package validator

// Validator is the interface that wraps the basic Validate method

// Validate accepts a byte array of a file or string to be validated
// and returns true or false if the content of the byte array is
// valid or not. If it is not valid, the error return value
// will be populated.
type Validator interface {
	Validate(b []byte) (bool, error)
}
