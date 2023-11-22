package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
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

type groupReportJSON struct {
	Files   map[string][]fileStatus `json:"files"`
	Summary summary                 `json:"summary"`
}

// Print implements the Reporter interface by outputting
// the report content to stdout as JSON
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

	fmt.Println(string(jsonBytes))
	return nil
}

func (jr JsonReporter) PrintSingleGroup(groupReports map[string][]Report, groupOutput string) error {
	var groupReport groupReportJSON

	for _, reports := range groupReports {
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

			file := fileStatus{
				Path:   r.FilePath,
				Status: status,
				Error:  errorStr,
			}

			switch groupOutput {
			case "filetype":
				fileExtension := strings.Split(r.FileName, ".")[1]
				if fileExtension == "yml" {
					fileExtension = "yaml"
				}
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[fileExtension] = []fileStatus{file}
				} else {
					groupReport.Files[fileExtension] = append(groupReport.Files[fileExtension], file)
				}
			case "pass-fail":
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[status] = []fileStatus{file}
				} else {
					groupReport.Files[status] = append(groupReport.Files[status], file)
				}
			case "directory":
				directoryPath := strings.Split(r.FilePath, "/")
				directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
				directory = directory + "/"
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[directory] = []fileStatus{file}
				} else {
					groupReport.Files[directory] = append(groupReport.Files[directory], file)
				}
			}
		}
	}
	groupReport.Summary.Passed = 0
	groupReport.Summary.Failed = 0
	for _, files := range groupReport.Files {
		for _, f := range files {
			if f.Status == "passed" {
				groupReport.Summary.Passed++
			} else {
				groupReport.Summary.Failed++
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(groupReport, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))

	return nil
}

func (jr JsonReporter) PrintDoubleGroup(reports map[string]map[string][]Report) error {
	//TODO
	return nil
}
