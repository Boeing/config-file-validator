package reporter

import (
	"encoding/json"
	"fmt"
)

type JsonReporter struct{}

type fileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type summary struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type reportJSON struct {
	Files   []fileStatus `json:"files"`
	Summary summary      `json:"summary"`
}

// Print implements the Reporter interface by outputting
// the report content to stdout as JSON
func (jr JsonReporter) Print(reports []Report) error {
	var reportJSON reportJSON

	for _, r := range reports {
		status := "passed"
		errorStr := ""
		if !r.IsValid {
			status = "failed"
			errorStr = r.ValidationError.Error()
		}

		reportJSON.Files = append(reportJSON.Files, fileStatus{
			Path:   r.FilePath,
			Status: status,
			Error:  errorStr,
		})
	}

	reportJSON.Summary.Passed = 0
	reportJSON.Summary.Failed = 0
	for _, f := range reportJSON.Files {
		if f.Status == "passed" {
			reportJSON.Summary.Passed++
		} else {
			reportJSON.Summary.Failed++
		}
	}

	jsonBytes, err := json.MarshalIndent(reportJSON, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}
