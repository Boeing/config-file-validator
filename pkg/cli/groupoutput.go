package cli

import (
	"strings"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// Group Files by File Type
func GroupByFile(reports []reporter.Report) []reporter.Report {
	mapFiles := make(map[string][]reporter.Report)
	reportByFile := []reporter.Report{}

	for _, report := range reports {
		fileType := strings.Split(report.FileName, ".")[1]
		if mapFiles[fileType] == nil {
			mapFiles[fileType] = []reporter.Report{report}
		} else {
			mapFiles[fileType] = append(mapFiles[fileType], report)
		}
	}

	for _, reports := range mapFiles {
		reportByFile = append(reportByFile, reports...)
	}

	return reportByFile
}

// Group Files by Pass/Fail
func GroupByPassFail(reports []reporter.Report) []reporter.Report {
	mapFiles := make(map[string][]reporter.Report)
	reportByPassOrFail := []reporter.Report{}

	for _, report := range reports {
		if report.IsValid {
			if mapFiles["pass"] == nil {
				mapFiles["pass"] = []reporter.Report{report}
			} else {
				mapFiles["pass"] = append(mapFiles["pass"], report)
			}
		} else {
			if mapFiles["fail"] == nil {
				mapFiles["fail"] = []reporter.Report{report}
			} else {
				mapFiles["fail"] = append(mapFiles["fail"], report)
			}
		}
	}

	for _, reports := range mapFiles {
		reportByPassOrFail = append(reportByPassOrFail, reports...)
	}

	return reportByPassOrFail
}

// Group Files by Directory
func GroupByDirectory(reports []reporter.Report) []reporter.Report {
	mapFiles := make(map[string][]reporter.Report)
	reportByDirectory := []reporter.Report{}

	for _, report := range reports {
		directory := strings.Split(report.FilePath, "/")[1]
		if mapFiles[directory] == nil {
			mapFiles[directory] = []reporter.Report{report}
		} else {
			mapFiles[directory] = append(mapFiles[directory], report)
		}
	}

	for _, reports := range mapFiles {
		reportByDirectory = append(reportByDirectory, reports...)
	}

	return reportByDirectory
}

func GroupBy(reports []reporter.Report, groupBy []string) []reporter.Report {
    for _, group := range groupBy {
        switch group {
        case "file":
            reports = GroupByFile(reports)
        case "directory":
            reports = GroupByDirectory(reports)
        case "pass/fail":
            reports = GroupByPassFail(reports)
        }
    }
    return reports
}
