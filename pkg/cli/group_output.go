package cli

import (
	"strings"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// Group Files by File Type
func GroupByFile(reports []reporter.Report) map[string][]reporter.Report {
	reportByFile := make(map[string][]reporter.Report)

	for _, report := range reports {
		fileType := strings.Split(report.FileName, ".")[1]
		if fileType == "yml" && reportByFile["yaml"] == nil {
			reportByFile["yaml"] = []reporter.Report{report}
		} else if fileType == "yml" && reportByFile["yaml"] != nil {
			reportByFile["yaml"] = append(reportByFile["yaml"], report)
		} else if reportByFile[fileType] == nil {
			reportByFile[fileType] = []reporter.Report{report}
		} else {
			reportByFile[fileType] = append(reportByFile[fileType], report)
		}
	}

	return reportByFile
}

// Group Files by Pass-Fail
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

// Group Files by Directory
func GroupByDirectory(reports []reporter.Report) map[string][]reporter.Report {
	reportByDirectory := make(map[string][]reporter.Report)
	for _, report := range reports {
		directoryPaths := strings.Split(report.FilePath, "/")
		directory := directoryPaths[len(directoryPaths)-2]
		if reportByDirectory[directory] == nil {
			reportByDirectory[directory] = []reporter.Report{report}
		} else {
			reportByDirectory[directory] = append(reportByDirectory[directory], report)
		}
	}

	return reportByDirectory
}

func GroupBy(reports []reporter.Report, groupBy []string) map[string][]reporter.Report {
	// Iterate through groupBy in reverse order
	// This will make the first command the primary grouping
	groupReports := make(map[string][]reporter.Report)

	for i := len(groupBy) - 1; i >= 0; i-- {
		switch groupBy[i] {
		case "pass-fail":
			groupReports = GroupByPassFail(reports)
		case "filetype":
			groupReports = GroupByFile(reports)
		case "directory":
			groupReports = GroupByDirectory(reports)
		}
	}
	return groupReports
}
