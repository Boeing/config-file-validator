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
	Files       map[string][]fileStatus `json:"files"`
	Summary     map[string][]summary    `json:"summary"`
	TotalPassed int                     `json:"totalPassed"`
	TotalFailed int                     `json:"totalFailed"`
}

type doubleGroupReportJSON struct {
	Files       map[string]map[string][]fileStatus `json:"files"`
	Summary     map[string]map[string][]summary    `json:"summary"`
	TotalPassed int                                `json:"totalPassed"`
	TotalFailed int                                `json:"totalFailed"`
}

type tripleGroupReportJSON struct {
	Files       map[string]map[string]map[string][]fileStatus `json:"files"`
	Summary     map[string]map[string]map[string][]summary    `json:"summary"`
	TotalPassed int                                           `json:"totalPassed"`
	TotalFailed int                                           `json:"totalFailed"`
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

// Tried to pass the Print function to this function but it didn't work
// We lose the group output when we do that
// Possible solution: create functions to handle each grouping
// and pass the Print function to those functions
// Similiar to how the group_output.go file is structured
func (jr JsonReporter) PrintSingleGroup(groupReports map[string][]Report, groupOutput string) error {
	var report groupReportJSON
	currentPassed := 0
	currentFailed := 0
	totalPassed := 0
	totalFailed := 0
	report.Files = make(map[string][]fileStatus)
	report.Summary = make(map[string][]summary)

	for group, reports := range groupReports {
		report.Files[group] = make([]fileStatus, 0)
		report.Summary[group] = make([]summary, 0)
		currentPassed = 0
		currentFailed = 0
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

			report.Files[group] = append(report.Files[group], fileStatus{
				Path:   r.FilePath,
				Status: status,
				Error:  errorStr,
			})
		}

		for _, f := range report.Files[group] {
			if f.Status == "passed" {
				currentPassed++
				totalPassed++
			} else {
				currentFailed++
				totalFailed++
			}
		}
		report.Summary[group] = append(report.Summary[group], summary{
			Passed: currentPassed,
			Failed: currentFailed,
		})
	}

	report.TotalPassed = totalPassed
	report.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

// Prints the report for when two groups are passed in the groupby flag
func (jr JsonReporter) PrintDoubleGroup(groupReports map[string]map[string][]Report, groupOutput []string) error {
	var report doubleGroupReportJSON
	currentPassed := 0
	currentFailed := 0
	totalPassed := 0
	totalFailed := 0
	report.Files = make(map[string]map[string][]fileStatus)
	report.Summary = make(map[string]map[string][]summary)

	for group, group2 := range groupReports {
		report.Files[group] = make(map[string][]fileStatus, 0)
		report.Summary[group] = make(map[string][]summary, 0)
		for group2, reports := range group2 {
			currentPassed = 0
			currentFailed = 0
			report.Files[group][group2] = make([]fileStatus, 0)
			report.Summary[group][group2] = make([]summary, 0)
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

				report.Files[group][group2] = append(report.Files[group][group2], fileStatus{
					Path:   r.FilePath,
					Status: status,
					Error:  errorStr,
				})
			}

			for _, f := range report.Files[group][group2] {
				if f.Status == "passed" {
					currentPassed++
					totalPassed++
				} else {
					currentFailed++
					totalFailed++
				}
			}
			report.Summary[group][group2] = append(report.Summary[group][group2], summary{
				Passed: currentPassed,
				Failed: currentFailed,
			})
		}
	}

	report.TotalPassed = totalPassed
	report.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))
	return nil
}

// Prinnts the report for when three groups are passed in the groupby flag
func (jr JsonReporter) PrintTripleGroup(groupReports map[string]map[string]map[string][]Report, groupOutput []string) error {
	var report tripleGroupReportJSON
	currentPassed := 0
	currentFailed := 0
	totalPassed := 0
	totalFailed := 0
	report.Files = make(map[string]map[string]map[string][]fileStatus)
	report.Summary = make(map[string]map[string]map[string][]summary)

	for group, group2 := range groupReports {
		report.Files[group] = make(map[string]map[string][]fileStatus, 0)
		report.Summary[group] = make(map[string]map[string][]summary, 0)

		for group2, group3 := range group2 {
			report.Files[group][group2] = make(map[string][]fileStatus, 0)
			report.Summary[group][group2] = make(map[string][]summary, 0)

			for group4, reports := range group3 {
				currentPassed = 0
				currentFailed = 0
				report.Files[group][group2][group4] = make([]fileStatus, 0)
				report.Summary[group][group2][group4] = make([]summary, 0)

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

					report.Files[group][group2][group4] = append(report.Files[group][group2][group4], fileStatus{
						Path:   r.FilePath,
						Status: status,
						Error:  errorStr,
					})

				}

				for _, f := range report.Files[group][group2][group4] {
					if f.Status == "passed" {
						currentPassed++
						totalPassed++
					} else {
						currentFailed++
						totalFailed++
					}
				}
				report.Summary[group][group2][group4] = append(report.Summary[group][group2][group4], summary{
					Passed: currentPassed,
					Failed: currentFailed,
				})
			}
		}
	}

	report.TotalPassed = totalPassed
	report.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

