package reporter

import (
	"encoding/json"
	"fmt"
)

type SarifReporter struct {
	outputDest string
}

type driver struct {
	Name           string `json:"name"`
	InformationURI string `json:"informationUri"`
}

type tool struct {
	Driver driver `json:"driver"`
}
type artifactLocation struct {
	URI string `json:"uri"`
}

type physicalLocation struct {
	ArtifactLocation artifactLocation `json:"artifactLocation"`
}

type message struct {
	Text string `json:"text"`
}

type location struct {
	PhysicalLocation physicalLocation `json:"physicalLocation"`
}

type result struct {
	Message   message    `json:"message"`
	Locations []location `json:"locations,omitempty"`
	Level     string     `json:"level"`
}

type run struct {
	Tool    tool     `json:"tool"`
	Results []result `json:"results"`
}

type sarifReport struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []run  `json:"runs"`
}

func NewSarifReporter(outputDest string) *SarifReporter {
	return &SarifReporter{
		outputDest: outputDest,
	}
}

func (sr SarifReporter) Print(reports []Report) error {
	report := createSarifReport(reports)

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	jsonBytes = append(jsonBytes, '\n')
	fmt.Print(string(jsonBytes))

	if sr.outputDest != "" {
		return outputBytesToFile(sr.outputDest, "result", "json", jsonBytes)
	}

	return nil
}

func createSarifReport(reports []Report) *sarifReport {

	results := getResultsFromReports(reports)

	sr := &sarifReport{
		Version: "2.1.0",
		Schema:  "http://json.schemastore.org/sarif-2.1.0-rtm.4",
		Runs: []run{
			{Tool: tool{
				Driver: driver{
					Name:           "config-file-validator",
					InformationURI: "https://github.com/Boeing/config-file-validator",
				},
			},
				Results: results,
			},
		},
	}

	return sr
}

func getResultsFromReports(reports []Report) []result {
	results := []result{}
	failedValidations := 0
	successValidations := 0

	for _, report := range reports {
		if report.IsValid {
			successValidations++
			continue
		}
		res := result{
			Message: message{
				Text: report.ValidationError.Error(),
			},
			Locations: []location{
				{
					PhysicalLocation: physicalLocation{
						ArtifactLocation: artifactLocation{
							URI: report.FilePath,
						},
					},
				},
			},
			Level: "error",
		}
		results = append(results, res)
		failedValidations++
	}

	successSummary := result{
		Message: message{
			Text: fmt.Sprintf("Tests Passed: %d", successValidations),
		},
		Level: "note",
	}

	failedSummary := result{
		Message: message{
			Text: fmt.Sprintf("Tests failed: %d", failedValidations),
		},
		Level: "error",
	}

	results = append(results, successSummary, failedSummary)

	return results
}
