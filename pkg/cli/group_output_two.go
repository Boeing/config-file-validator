package cli

import (
	"errors"

	"github.com/Boeing/config-file-validator/pkg/reporter"
)

func GroupByDouble(reports []reporter.Report, groupBy []string) (map[string]map[string][]reporter.Report, error) {
	groupReport := make(map[string]map[string][]reporter.Report)

	if groupBy[0] == "pass-fail" && groupBy[1] == "filetype" {
		passFail := GroupByPassFail(reports)
		for key := range passFail {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByFile(passFail[key])
		}
	} else if groupBy[0] == "pass-fail" && groupBy[1] == "directory" {
		passFail := GroupByPassFail(reports)
		for key := range passFail {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByDirectory(passFail[key])
		}
	} else if groupBy[0] == "filetype" && groupBy[1] == "pass-fail" {
		fileType := GroupByFile(reports)
		for key := range fileType {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByPassFail(fileType[key])
		}
	} else if groupBy[0] == "filetype" && groupBy[1] == "directory" {
		fileType := GroupByFile(reports)
		for key := range fileType {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByDirectory(fileType[key])
		}
	} else if groupBy[0] == "directory" && groupBy[1] == "pass-fail" {
		directory := GroupByDirectory(reports)
		for key := range directory {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByPassFail(directory[key])
		}
	} else if groupBy[0] == "directory" && groupBy[1] == "filetype" {
		directory := GroupByDirectory(reports)
		for key := range directory {
			groupReport[key] = make(map[string][]reporter.Report)
			groupReport[key] = GroupByFile(directory[key])
		}
	} else {
		return nil, errors.New("Invalid group by option")
	}

	return groupReport, nil
}
