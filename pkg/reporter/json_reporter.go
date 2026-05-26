package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JSONReporter struct {
	outputDest string
}

func NewJSONReporter(outputDest string) *JSONReporter {
	return &JSONReporter{
		outputDest: outputDest,
	}
}

type fileStatus struct {
	Path   string   `json:"path"`
	Status string   `json:"status"`
	Errors []string `json:"errors,omitempty"`
	Notes  []string `json:"notes,omitempty"`
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
	Files       map[string]any `json:"files"`
	Summary     map[string]any `json:"summary"`
	TotalPassed int            `json:"totalPassed"`
	TotalFailed int            `json:"totalFailed"`
}

// Print implements the Reporter interface by outputting
// the report content to stdout as JSON
// if outputDest flag is provided, output results to a file.
func (jr JSONReporter) Print(reports []Report) error {
	report, err := createJSONReport(reports)
	if err != nil {
		return err
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	jsonBytes = append(jsonBytes, '\n')

	if jr.outputDest != "" {
		return outputBytesToFile(jr.outputDest, "result", "json", jsonBytes)
	}

	if len(reports) > 0 && !reports[0].IsQuiet {
		fmt.Print(string(jsonBytes))
	}

	return nil
}

// PrintGroupJSON prints a recursive grouped report to stdout as JSON.
func PrintGroupJSON(groupReports *GroupNode) error {
	files, summaries, totalSummary, err := createGroupJSON(groupReports)
	if err != nil {
		return err
	}

	jsonReport := groupReportJSON{
		Files:       files,
		Summary:     summaries,
		TotalPassed: totalSummary.Passed,
		TotalFailed: totalSummary.Failed,
	}
	jsonBytes, err := json.MarshalIndent(jsonReport, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func createGroupJSON(node *GroupNode) (files map[string]any, summaries map[string]any, total summary, err error) {
	files = make(map[string]any)
	summaries = make(map[string]any)

	for _, child := range node.Children {
		childFiles, childSummary, reportSummary, err := createGroupJSONNode(child)
		if err != nil {
			return nil, nil, summary{}, err
		}
		files[child.Key] = childFiles
		summaries[child.Key] = childSummary
		total.Passed += reportSummary.Passed
		total.Failed += reportSummary.Failed
	}

	return files, summaries, total, nil
}

func createGroupJSONNode(node *GroupNode) (files any, summaries any, total summary, err error) {
	if len(node.Children) == 0 {
		report, err := createJSONReport(node.Reports)
		if err != nil {
			return nil, nil, summary{}, err
		}
		return report.Files, []summary{report.Summary}, report.Summary, nil
	}

	childFiles := make(map[string]any)
	childSummaries := make(map[string]any)
	for _, child := range node.Children {
		files, summaries, reportSummary, err := createGroupJSONNode(child)
		if err != nil {
			return nil, nil, summary{}, err
		}
		childFiles[child.Key] = files
		childSummaries[child.Key] = summaries
		total.Passed += reportSummary.Passed
		total.Failed += reportSummary.Failed
	}

	return childFiles, childSummaries, total, nil
}

// PrintSingleGroupJSON prints a grouped JSON report with one grouping level.
func PrintSingleGroupJSON(groupReports map[string][]Report) error {
	return PrintGroupJSON(groupNodeFromSingle(groupReports))
}

// PrintDoubleGroupJSON prints a grouped JSON report with two grouping levels.
func PrintDoubleGroupJSON(groupReports map[string]map[string][]Report) error {
	return PrintGroupJSON(groupNodeFromDouble(groupReports))
}

// PrintTripleGroupJSON prints a grouped JSON report with three grouping levels.
func PrintTripleGroupJSON(groupReports map[string]map[string]map[string][]Report) error {
	return PrintGroupJSON(groupNodeFromTriple(groupReports))
}

// Creates the json report
func createJSONReport(reports []Report) (reportJSON, error) {
	var jsonReport reportJSON

	for _, report := range reports {
		status := "passed"
		var errs []string
		if !report.IsValid {
			status = "failed"
			errs = report.ValidationErrors
		}

		// Convert Windows-style file paths.
		if strings.Contains(report.FilePath, "\\") {
			report.FilePath = strings.ReplaceAll(report.FilePath, "\\", "/")
		}

		jsonReport.Files = append(jsonReport.Files, fileStatus{
			Path:   report.FilePath,
			Status: status,
			Errors: errs,
			Notes:  report.Notes,
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
