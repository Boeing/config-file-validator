package cli

import (
	"fmt"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// Group Reports by File Type
func GroupByFileType(reports []reporter.Report) map[string][]reporter.Report {
	reportByFile := make(map[string][]reporter.Report)

	for _, report := range reports {
		fileType := strings.Split(report.FileName, ".")[1]
		fileType = strings.ToLower(fileType)
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
		directory := ""
		// Check if the filepath is in Windows format
		if strings.Contains(report.FilePath, "\\") {
			directoryPath := strings.Split(report.FilePath, "\\")
			directory = strings.Join(directoryPath[:len(directoryPath)-1], "\\")
			directory = directory + "\\"
		} else {
			directoryPath := strings.Split(report.FilePath, "/")
			directory = strings.Join(directoryPath[:len(directoryPath)-1], "/")
			directory = directory + "/"
		}

		if reportByDirectory[directory] == nil {
			reportByDirectory[directory] = []reporter.Report{report}
		} else {
			reportByDirectory[directory] = append(reportByDirectory[directory], report)
		}
	}

	return reportByDirectory
}

// Group Reports by single grouping
func GroupBySingle(reports []reporter.Report, groupBy string) (map[string][]reporter.Report, error) {
	var groupReport map[string][]reporter.Report

	// Group by the groupings in reverse order
	// This allows for the first grouping to be the outermost grouping
	for i := len(groupBy) - 1; i >= 0; i-- {
		switch groupBy {
		case "pass-fail":
			groupReport = GroupByPassFail(reports)
		case "filetype":
			groupReport = GroupByFileType(reports)
		case "directory":
			groupReport = GroupByDirectory(reports)
		default:
			return nil, fmt.Errorf("unable to group by %s", groupBy)
		}
	}
	return groupReport, nil
}

// Group Reports for two groupings
func GroupByDouble(reports []reporter.Report, groupBy []string) (map[string]map[string][]reporter.Report, error) {
	groupReport := make(map[string]map[string][]reporter.Report)

	firstGroup, err := GroupBySingle(reports, groupBy[0])
	if err != nil {
		return nil, err
	}
	for key := range firstGroup {
		groupReport[key] = make(map[string][]reporter.Report)
		groupReport[key], err = GroupBySingle(firstGroup[key], groupBy[1])
		if err != nil {
			return nil, err
		}
	}

	return groupReport, nil
}

// Group Reports for three groupings
func GroupByTriple(reports []reporter.Report, groupBy []string) (map[string]map[string]map[string][]reporter.Report, error) {
	groupReport := make(map[string]map[string]map[string][]reporter.Report)

	firstGroup, err := GroupBySingle(reports, groupBy[0])
	if err != nil {
		return nil, err
	}
	for key := range firstGroup {
		groupReport[key] = make(map[string]map[string][]reporter.Report)
		groupReport[key], err = GroupByDouble(firstGroup[key], groupBy[1:])
		if err != nil {
			return nil, err
		}
	}

	return groupReport, nil
}
