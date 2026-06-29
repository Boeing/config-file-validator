package cli

import (
	"fmt"
	"path/filepath"
	"slices"
	"strings"

	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
)

// GroupNode aliases the reporter grouping tree used by grouped output.
type GroupNode = reporter.GroupNode

// GroupByFileType groups reports by file extension.
func GroupByFileType(reports []reporter.Report) map[string][]reporter.Report {
	reportByFile := make(map[string][]reporter.Report)

	for _, report := range reports {
		parts := strings.Split(report.FileName, ".")
		var fileType string
		if len(parts) > 1 {
			fileType = strings.ToLower(parts[len(parts)-1])
		} else {
			fileType = "unknown"
		}
		if fileType == "yml" {
			fileType = "yaml"
		}
		reportByFile[fileType] = append(reportByFile[fileType], report)
	}

	return reportByFile
}

// GroupByPassFail groups reports by pass or fail status.
func GroupByPassFail(reports []reporter.Report) map[string][]reporter.Report {
	reportByPassOrFail := make(map[string][]reporter.Report)

	for _, report := range reports {
		if report.IsValid {
			reportByPassOrFail["Passed"] = append(reportByPassOrFail["Passed"], report)
		} else {
			reportByPassOrFail["Failed"] = append(reportByPassOrFail["Failed"], report)
		}
	}

	return reportByPassOrFail
}

// GroupByErrorType groups reports by error type.
func GroupByErrorType(reports []reporter.Report) map[string][]reporter.Report {
	reportByErrorType := make(map[string][]reporter.Report)

	for _, report := range reports {
		key := report.ErrorType
		if key == "" {
			key = "Passed"
		}
		reportByErrorType[key] = append(reportByErrorType[key], report)
	}

	return reportByErrorType
}

// GroupByDirectory groups reports by containing directory.
func GroupByDirectory(reports []reporter.Report) map[string][]reporter.Report {
	reportByDirectory := make(map[string][]reporter.Report)
	for _, report := range reports {
		normalizedPath := strings.ReplaceAll(report.FilePath, "\\", string(filepath.Separator))
		directory := filepath.Dir(normalizedPath)
		if directory == "." {
			directory = ""
		}
		directory = filepath.ToSlash(directory)

		reportByDirectory[directory] = append(reportByDirectory[directory], report)
	}

	return reportByDirectory
}

// GroupBy groups reports into a recursive tree for any number of grouping levels.
func GroupBy(reports []reporter.Report, groupBy []string) (*GroupNode, error) {
	return groupByLevel("", reports, groupBy)
}

func groupByLevel(key string, reports []reporter.Report, groupBy []string) (*GroupNode, error) {
	node := &GroupNode{Key: key}
	if len(groupBy) == 0 {
		node.Reports = reports
		return node, nil
	}

	groupedReports, err := groupReports(reports, groupBy[0])
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(groupedReports))
	for key := range groupedReports {
		keys = append(keys, key)
	}
	slices.Sort(keys)

	for _, key := range keys {
		child, err := groupByLevel(key, groupedReports[key], groupBy[1:])
		if err != nil {
			return nil, err
		}
		node.Children = append(node.Children, child)
	}

	return node, nil
}

func groupReports(reports []reporter.Report, groupBy string) (map[string][]reporter.Report, error) {
	switch groupBy {
	case "pass-fail":
		return GroupByPassFail(reports), nil
	case "filetype":
		return GroupByFileType(reports), nil
	case "directory":
		return GroupByDirectory(reports), nil
	case "error-type":
		return GroupByErrorType(reports), nil
	default:
		return nil, fmt.Errorf("unable to group by %s", groupBy)
	}
}
