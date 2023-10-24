package validator

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
)

// CsvValidator is used to validate a byte slice that is intended to represent a CSV file.
type CsvValidator struct {
	ExpectedColumns int // The number of columns expected in each row
	RequiredHeader  []string // Required header values
}

// ValidationError represents a validation error.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Validate checks if the provided byte slice represents a valid .csv file.
func (csvv CsvValidator) Validate(b []byte) error {
	csvReader := csv.NewReader(bytes.NewReader(b))
	csvReader.TrimLeadingSpace = true

	// Read the header row
	header, err := csvReader.Read()
	if err != nil {
		return &ValidationError{Message: "Error reading the header row"}
	}

	// Check the number of columns in the header
	if csvv.ExpectedColumns > 0 && len(header) != csvv.ExpectedColumns {
		return &ValidationError{Message: "Unexpected number of columns in the header"}
	}

	// Check for required header values
	for _, requiredHeader := range csvv.RequiredHeader {
		if !contains(header, requiredHeader) {
			return &ValidationError{Message: "Missing required header value: " + requiredHeader}
		}
	}

	// Additional custom validation logic can be added here

	return nil // CSV is valid
}

// Helper function to check if a string exists in a slice of strings
func contains(slice []string, str string) bool {
	for _, s := range slice {
		if s == str {
			return true
		}
	}
	return false
}
