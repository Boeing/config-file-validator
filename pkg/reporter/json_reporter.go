package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JsonReporter struct {
	outputDest string
}

func NewJsonReporter(outputDest string) *JsonReporter {
	return &JsonReporter{
		outputDest: outputDest,
	}
}

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
// if outputDest flag is provided, output results to a file.
func (jr JsonReporter) Print(reports []Report) error {
	var report reportJSON

	for _, r := range reports {
		status := "passed"
		errorStr := ""
		if !r.IsValid {
			status = "failed"
			errorStr = r.ValidationError.Error()
		}

		// Convert Windows-style file paths.
		if strings.Contains(r.FilePath, "\\") {
			r.FilePath = strings.ReplaceAll(r.FilePath, "\\", "/")
		}

		report.Files = append(report.Files, fileStatus{
			Path:   r.FilePath,
			Status: status,
			Error:  errorStr,
		})
	}

	report.Summary.Passed = 0
	report.Summary.Failed = 0
	for _, f := range report.Files {
		if f.Status == "passed" {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	jsonBytes = append(jsonBytes, '\n')
	fmt.Print(string(jsonBytes))

	if jr.outputDest != "" {
		return outputBytesToFile(jr.outputDest, "result", "json", jsonBytes)
	}

	return nil
}
