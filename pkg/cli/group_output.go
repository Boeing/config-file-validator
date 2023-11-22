package cli

import (
	"strings"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// Group Reports by File Type
func GroupByFile(reports []reporter.Report) map[string][]reporter.Report {
	reportByFile := make(map[string][]reporter.Report)

	for _, report := range reports {
		fileType := strings.Split(report.FileName, ".")[1]
		if fileType == "yml" {
			fileType = "yaml"
		}
		if reportByFile[fileType] == nil {
			reportByFile[fileType] = []reporter.Report{report}
		} else {
			reportByFile[fileType] = append(reportByFile[fileType], report)
		}
	}

	return reportByFile
}

// Group Reports by Pass-Fail
func GroupByPassFail(reports []reporter.Report) map[string][]reporter.Report {
	reportByPassOrFail := make(map[string][]reporter.Report)

	for _, report := range reports {
		if report.IsValid {
			if reportByPassOrFail["Passed"] == nil {
				reportByPassOrFail["Passed"] = []reporter.Report{report}
			} else {
				reportByPassOrFail["Passed"] = append(reportByPassOrFail["Passed"], report)
			}
		} else if reportByPassOrFail["Failed"] == nil {
			reportByPassOrFail["Failed"] = []reporter.Report{report}
		} else {
			reportByPassOrFail["Failed"] = append(reportByPassOrFail["Failed"], report)
		}

	}

	return reportByPassOrFail
}

// Group Reports by Directory
func GroupByDirectory(reports []reporter.Report) map[string][]reporter.Report {
	reportByDirectory := make(map[string][]reporter.Report)
	for _, report := range reports {
		directoryPath := strings.Split(report.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"

		if reportByDirectory[directory] == nil {
			reportByDirectory[directory] = []reporter.Report{report}
		} else {
			reportByDirectory[directory] = append(reportByDirectory[directory], report)
		}
	}

	return reportByDirectory
}

func GroupBySingle(reports []reporter.Report, groupBy []string) map[string][]reporter.Report {

	var groupReport map[string][]reporter.Report

	for i := len(groupBy) - 1; i >= 0; i-- {
		switch groupBy[i] {
		case "pass-fail":
			groupReport = GroupByPassFail(reports)
		case "filetype":
			groupReport = GroupByFile(reports)
		case "directory":
			groupReport = GroupByDirectory(reports)
		}
	}
	return groupReport
}
