package validator

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
)

// CsvValidator is used to validate a byte slice that is intended to represent a
// CSV file.
type CsvValidator struct{}

var _ Validator = CsvValidator{}

// Validate checks if the provided byte slice represents a valid .csv file.
// https://pkg.go.dev/encoding/csv
func (CsvValidator) ValidateSyntax(b []byte) (bool, error) {
	csvReader := csv.NewReader(bytes.NewReader(b))
	csvReader.TrimLeadingSpace = true

	for {
		_, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			return false, err
		}
	}

	return true, nil
}

func (CsvValidator) ValidateFormat(_ []byte, _ any) (bool, error) {
	return false, ErrMethodUnimplemented
}
