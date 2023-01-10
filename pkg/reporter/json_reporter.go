package reporter

import (
	"encoding/json"
	"fmt"
)

type JsonReporter struct {
	ValidCount   int      `json:"valid"`
	InvalidCount int      `json:"invalid"`
	ReportOutput []Report `json:"report"`
}

// Print implements the Reporter interface by outputting
// the report content to stdout
func (jr JsonReporter) Print(reports []Report) error {
	for _, report := range reports {
		if !report.IsValid {
			jr.ValidCount = jr.ValidCount + 1
		} else {
			jr.InvalidCount = jr.InvalidCount + 1
		}
	}

	jr.ReportOutput = reports

	output, err := json.Marshal(jr)
	if err != nil {
		return err
	}

	fmt.Println(string(output))

	return nil
}

//Custom JSON marshaller or changing reporter to store the error message itself
