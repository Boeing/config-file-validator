package validator

import (
	"bytes"
	"io"

	"encoding/csv"
)

// CsvValidator is used to validate a byte slice that is intended to represent a
// CSV file.
type CsvValidator struct{}

// Validate checks if the provided byte slice represents a valid .csv file.
// https://pkg.go.dev/encoding/csv
func (csvv CsvValidator) Validate(b []byte) (bool, error) {
	csvReader := csv.NewReader(bytes.NewReader(b))
	csvReader.TrimLeadingSpace = true

	for {
		_, err := csvReader.Read()
		if err == io.EOF {
			break
		}

		if err != nil {
			return false, err
		}
	}

	return true, nil
}
