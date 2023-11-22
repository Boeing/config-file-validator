package cli

import (
	"strings"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// Group Reports by Pass-Fail and File Extension
func GroupByPassFailFile(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByPassOrFail := make(map[string]map[string][]reporter.Report)

	for _, report := range reports {
		fileExtension := strings.Split(report.FileName, ".")[1]
		if report.IsValid {
			if reportByPassOrFail["Passed"] == nil {
				reportByPassOrFail["Passed"] = make(map[string][]reporter.Report)
				reportByPassOrFail["Passed"][fileExtension] = []reporter.Report{report}
			} else if reportByPassOrFail["Passed"] != nil && reportByPassOrFail["Passed"][fileExtension] == nil {
				reportByPassOrFail["Passed"][fileExtension] = []reporter.Report{report}
			} else if reportByPassOrFail["Passed"] != nil && reportByPassOrFail["Passed"][fileExtension] != nil {
				reportByPassOrFail["Passed"][fileExtension] = append(reportByPassOrFail["Passed"][fileExtension], report)
			}
		} else {
			if reportByPassOrFail["Failed"] == nil {
				reportByPassOrFail["Failed"] = make(map[string][]reporter.Report)
				reportByPassOrFail["Failed"][fileExtension] = []reporter.Report{report}
			} else if reportByPassOrFail["Failed"] != nil && reportByPassOrFail["Failed"][fileExtension] == nil {
				reportByPassOrFail["Failed"][fileExtension] = []reporter.Report{report}
			} else if reportByPassOrFail["Failed"] != nil && reportByPassOrFail["Failed"][fileExtension] != nil {
				reportByPassOrFail["Failed"][fileExtension] = append(reportByPassOrFail["Failed"][fileExtension], report)
			}
		}
	}
	return reportByPassOrFail
}

// Group Reports by Pass-Fail and Directory
func GroupByPassFailDirectory(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByPassOrFail := make(map[string]map[string][]reporter.Report)

	for _, report := range reports {
		directoryPath := strings.Split(report.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"

		if report.IsValid {
			if reportByPassOrFail["Passed"] == nil {
				reportByPassOrFail["Passed"] = make(map[string][]reporter.Report)
				reportByPassOrFail["Passed"][directory] = []reporter.Report{report}
			} else if reportByPassOrFail["Passed"] != nil && reportByPassOrFail["Passed"][directory] == nil {
				reportByPassOrFail["Passed"][directory] = []reporter.Report{report}
			} else if reportByPassOrFail["Passed"] != nil && reportByPassOrFail["Passed"][directory] != nil {
				reportByPassOrFail["Passed"][directory] = append(reportByPassOrFail["Passed"][directory], report)
			}
		} else if reportByPassOrFail["Failed"] == nil && reportByPassOrFail["Failed"][directory] == nil {
			reportByPassOrFail["Failed"] = make(map[string][]reporter.Report)
			reportByPassOrFail["Failed"][directory] = []reporter.Report{report}
		} else if reportByPassOrFail["Failed"] != nil && reportByPassOrFail["Failed"][directory] == nil {
			reportByPassOrFail["Failed"][directory] = []reporter.Report{report}
		} else if reportByPassOrFail["Failed"] != nil && reportByPassOrFail["Failed"][directory] != nil {
			reportByPassOrFail["Failed"][directory] = append(reportByPassOrFail["Failed"][directory], report)
		}

	}

	return reportByPassOrFail
}

// Group Reports by File Extension and Directory
func GroupByFileDirectory(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByFile := make(map[string]map[string][]reporter.Report)

	for _, report := range reports {
		directoryPath := strings.Split(report.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		fileExtension := strings.Split(report.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		if reportByFile[fileExtension] == nil {
			reportByFile[fileExtension] = make(map[string][]reporter.Report)
			reportByFile[fileExtension][directory] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension][directory] == nil {
			reportByFile[fileExtension][directory] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension][directory] != nil {
			reportByFile[fileExtension][directory] = append(reportByFile[fileExtension][directory], report)
		}
	}
	return reportByFile
}

// Group Reports by File Extension and Pass-Fail
func GroupByFilePassFail(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByFile := make(map[string]map[string][]reporter.Report)

	for _, report := range reports {
		fileExtension := strings.Split(report.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		if reportByFile[fileExtension] == nil && report.IsValid {
			reportByFile[fileExtension] = make(map[string][]reporter.Report)
			reportByFile[fileExtension]["Passed"] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension]["Passed"] == nil && report.IsValid {
			reportByFile[fileExtension]["Passed"] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension]["Passed"] != nil && report.IsValid {
			reportByFile[fileExtension]["Passed"] = append(reportByFile[fileExtension]["Passed"], report)
		} else if reportByFile[fileExtension] == nil && !report.IsValid {
			reportByFile = make(map[string]map[string][]reporter.Report)
			reportByFile[fileExtension]["Failed"] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension]["Failed"] == nil && !report.IsValid {
			reportByFile[fileExtension]["Failed"] = []reporter.Report{report}
		} else if reportByFile[fileExtension] != nil && reportByFile[fileExtension]["Failed"] != nil && !report.IsValid {
			reportByFile[fileExtension]["Failed"] = append(reportByFile[fileExtension]["Failed"], report)
		}

	}

	return reportByFile
}

// Group Reports by Directory and Pass-Fail
func GroupByDirectoryPassFail(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByDirectory := make(map[string]map[string][]reporter.Report)
	for _, report := range reports {
		directoryPath := strings.Split(report.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"

		if report.IsValid {
			if reportByDirectory[directory] == nil {
				reportByDirectory[directory] = make(map[string][]reporter.Report)
				reportByDirectory[directory]["Passed"] = []reporter.Report{report}
			} else if reportByDirectory[directory] != nil && reportByDirectory[directory]["Passed"] == nil {
				reportByDirectory[directory]["Passed"] = []reporter.Report{report}
			} else if reportByDirectory[directory] != nil && reportByDirectory[directory]["Passed"] != nil {
				reportByDirectory[directory]["Passed"] = append(reportByDirectory[directory]["Passed"], report)
			}
		} else if reportByDirectory[directory] == nil {
			reportByDirectory[directory] = make(map[string][]reporter.Report)
			reportByDirectory[directory]["Failed"] = []reporter.Report{report}
		} else if reportByDirectory[directory] != nil && reportByDirectory[directory]["Failed"] == nil {
			reportByDirectory[directory]["Failed"] = []reporter.Report{report}
		} else if reportByDirectory[directory] != nil && reportByDirectory[directory]["Failed"] != nil {
			reportByDirectory[directory]["Failed"] = append(reportByDirectory[directory]["Failed"], report)
		}
	}

	return reportByDirectory
}

// Group Reports by Directory and File Extension
func GroupByDirectoryFile(reports []reporter.Report) map[string]map[string][]reporter.Report {
	reportByDirectory := make(map[string]map[string][]reporter.Report)

	for _, report := range reports {
		directoryPath := strings.Split(report.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		fileExtension := strings.Split(report.FileName, ".")[1]

		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		if reportByDirectory[directory] == nil {
			reportByDirectory[directory] = make(map[string][]reporter.Report)
			reportByDirectory[directory][fileExtension] = []reporter.Report{report}
		} else if reportByDirectory[directory] != nil && reportByDirectory[directory][fileExtension] == nil {
			reportByDirectory[directory][fileExtension] = []reporter.Report{report}
		} else if reportByDirectory[directory][fileExtension] != nil {
			reportByDirectory[directory][fileExtension] = append(reportByDirectory[directory][fileExtension], report)
		}

	}

	return reportByDirectory
}

func GroupByDouble(reports []reporter.Report, groupBy []string) map[string]map[string][]reporter.Report {
	var groupReport map[string]map[string][]reporter.Report

	switch {
	case groupBy[0] == "pass-fail" && groupBy[1] == "filetype":
		groupReport = GroupByPassFailFile(reports)
	case groupBy[0] == "pass-fail" && groupBy[1] == "directory":
		groupReport = GroupByPassFailDirectory(reports)
	case groupBy[0] == "filetype" && groupBy[1] == "directory":
		groupReport = GroupByFileDirectory(reports)
	case groupBy[0] == "filetype" && groupBy[1] == "pass-fail":
		groupReport = GroupByFilePassFail(reports)
	case groupBy[0] == "directory" && groupBy[1] == "pass-fail":
		groupReport = GroupByDirectoryPassFail(reports)
	case groupBy[0] == "directory" && groupBy[1] == "filetype":
		groupReport = GroupByDirectoryFile(reports)
	}

	return groupReport
}
