package cli

import (
	"github.com/Boeing/config-file-validator/pkg/reporter"
)

// TODO: Refactor this to be more generic

func GroupByDouble(reports []reporter.Report, groupBy []string) (map[string]map[string][]reporter.Report, error) {
	groupReport := make(map[string]map[string][]reporter.Report)

	groupTopGroup, err := GroupBySingle(reports, groupBy[0])
	if err != nil {
		return nil, err
	}
	for key := range groupTopGroup {
		groupReport[key] = make(map[string][]reporter.Report)
		groupReport[key], err = GroupBySingle(groupTopGroup[key], groupBy[1])
		if err != nil {
			return nil, err
		}
	}

	return groupReport, nil
}
