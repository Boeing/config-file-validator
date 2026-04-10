package validator

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
)

// CsvValidator validates CSV files. Zero-value fields use Go's csv defaults
// (comma delimiter, no comment character, strict quotes).
type CsvValidator struct {
	Delimiter  rune
	Comment    rune
	LazyQuotes bool
}

var _ Validator = CsvValidator{}

// ValidateSyntax checks if the provided byte slice represents a valid .csv file.
// https://pkg.go.dev/encoding/csv
func (v CsvValidator) ValidateSyntax(b []byte) (bool, error) {
	csvReader := csv.NewReader(bytes.NewReader(b))
	csvReader.TrimLeadingSpace = true

	if v.Delimiter != 0 {
		csvReader.Comma = v.Delimiter
	}
	if v.Comment != 0 {
		csvReader.Comment = v.Comment
	}
	csvReader.LazyQuotes = v.LazyQuotes

	for {
		_, err := csvReader.Read()
		if errors.Is(err, io.EOF) {
			break
		}

		if err != nil {
			var pe *csv.ParseError
			if errors.As(err, &pe) {
				return false, &ValidationError{
					Err:    err,
					Line:   pe.Line,
					Column: pe.Column,
				}
			}
			return false, err
		}
	}

	return true, nil
}
