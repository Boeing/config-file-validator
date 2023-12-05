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

// Prints the report for when one group is passed in the groupby flag
func (jr JsonReporter) PrintSingleGroup(groupReports map[string][]Report) error {
	var jsonReport groupReportJSON
	totalPassed := 0
	totalFailed := 0
	jsonReport.Files = make(map[string][]fileStatus)
	jsonReport.Summary = make(map[string][]summary)

	for group, reports := range groupReports {
		report, err := createSingleGroupJson(reports, group)
		if err != nil {
			return err
		}

		jsonReport.Files[group] = report.Files
		jsonReport.Summary[group] = append(jsonReport.Summary[group], report.Summary)

		totalPassed += report.Summary.Passed
		totalFailed += report.Summary.Failed

	}

	jsonReport.TotalPassed = totalPassed
	jsonReport.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(jsonReport, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

// Prints the report for when two groups are passed in the groupby flag
func (jr JsonReporter) PrintDoubleGroup(groupReports map[string]map[string][]Report) error {
	var jsonReport doubleGroupReportJSON
	totalPassed := 0
	totalFailed := 0
	jsonReport.Files = make(map[string]map[string][]fileStatus)
	jsonReport.Summary = make(map[string]map[string][]summary)

	for group, group2 := range groupReports {
		jsonReport.Files[group] = make(map[string][]fileStatus, 0)
		jsonReport.Summary[group] = make(map[string][]summary, 0)
		for group2, reports := range group2 {
			report, err := createSingleGroupJson(reports, group)
			if err != nil {
				return err
			}

			jsonReport.Files[group][group2] = report.Files
			jsonReport.Summary[group][group2] = append(jsonReport.Summary[group][group2], report.Summary)

			totalPassed += report.Summary.Passed
			totalFailed += report.Summary.Failed

		}
	}

	jsonReport.TotalPassed = totalPassed
	jsonReport.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(jsonReport, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))
	return nil
}

// Prinnts the report for when three groups are passed in the groupby flag
func (jr JsonReporter) PrintTripleGroup(groupReports map[string]map[string]map[string][]Report) error {
	var jsonReport tripleGroupReportJSON
	totalPassed := 0
	totalFailed := 0
	jsonReport.Files = make(map[string]map[string]map[string][]fileStatus)
	jsonReport.Summary = make(map[string]map[string]map[string][]summary)

	for group, group2 := range groupReports {
		jsonReport.Files[group] = make(map[string]map[string][]fileStatus, 0)
		jsonReport.Summary[group] = make(map[string]map[string][]summary, 0)

		for group2, group3 := range group2 {
			jsonReport.Files[group][group2] = make(map[string][]fileStatus, 0)
			jsonReport.Summary[group][group2] = make(map[string][]summary, 0)

			for group3, reports := range group3 {
				report, err := createSingleGroupJson(reports, group)
				if err != nil {
					return err
				}

				jsonReport.Files[group][group2][group3] = report.Files
				jsonReport.Summary[group][group2][group3] = append(jsonReport.Summary[group][group2][group3], report.Summary)

				totalPassed += report.Summary.Passed
				totalFailed += report.Summary.Failed

			}

		}
	}

	jsonReport.TotalPassed = totalPassed
	jsonReport.TotalFailed = totalFailed

	jsonBytes, err := json.MarshalIndent(jsonReport, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func createSingleGroupJson(reports []Report, groupOutput string) (reportJSON, error) {
	var jsonReport reportJSON

	for _, report := range reports {
		status := "passed"
		errorStr := ""
		if !report.IsValid {
			status = "failed"
			errorStr = report.ValidationError.Error()
		}

		// Convert Windows-style file paths.
		if strings.Contains(report.FilePath, "\\") {
			report.FilePath = strings.ReplaceAll(report.FilePath, "\\", "/")
		}

		jsonReport.Files = append(jsonReport.Files, fileStatus{
			Path:   report.FilePath,
			Status: status,
			Error:  errorStr,
		})

		currentPassed := 0
		currentFailed := 0
		for _, f := range jsonReport.Files {
			if f.Status == "passed" {
				currentPassed++
			} else {
				currentFailed++
			}
		}

		jsonReport.Summary.Passed = currentPassed
		jsonReport.Summary.Failed = currentFailed
	}

	return jsonReport, nil

}
