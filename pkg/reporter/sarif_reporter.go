package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type SARIFReporter struct {
	outputDest string
}

type SARIFLog struct {
	Version string `json:"version"`
	Schema  string `json:"$schema"`
	Runs    []runs `json:"runs"`
}

type runs struct {
	Tool    tool     `json:"tool"`
	Results []result `json:"results"`
}

type tool struct {
	Driver driver `json:"driver"`
}

type driver struct {
	Name    string `json:"name"`
	InfoURI string `json:"informationUri"`
	Version string `json:"version"`
}

type result struct {
	Kind      string     `json:"kind"`
	Level     string     `json:"level"`
	Message   message    `json:"message"`
	Locations []location `json:"locations"`
}

type message struct {
	Text string `json:"text"`
}

type location struct {
	PhysicalLocation physicalLocation `json:"physicalLocation"`
}

type physicalLocation struct {
	ArtifactLocation artifactLocation `json:"artifactLocation"`
}

type artifactLocation struct {
	URI string `json:"uri"`
}

func NewSARIFReporter(outputDest string) *SARIFReporter {
	return &SARIFReporter{
		outputDest: outputDest,
	}
}

func createSARIFReport(reports []Report) (*SARIFLog, error) {
	var log SARIFLog

	n := len(reports)

	log.Version = "2.1.0"
	log.Schema = "https://docs.oasis-open.org/sarif/sarif/v2.1.0/errata01/os/schemas/sarif-schema-2.1.0.json"

	log.Runs = make([]runs, 1)
	runs := &log.Runs[0]

	runs.Tool.Driver.Name = "config-file-validator"
	runs.Tool.Driver.InfoURI = "https://github.com/Boeing/config-file-validator"
	runs.Tool.Driver.Version = "1.7.1"

	runs.Results = make([]result, n)

	for i, report := range reports {
		if strings.Contains(report.FilePath, "\\") {
			report.FilePath = strings.ReplaceAll(report.FilePath, "\\", "/")
		}

		result := &runs.Results[i]
		if !report.IsValid {
			result.Kind = "fail"
			result.Level = "error"
			result.Message.Text = report.ValidationError.Error()
		} else {
			result.Kind = "pass"
			result.Level = "none"
			result.Message.Text = "No errors detected"
		}

		result.Locations = make([]location, 1)
		location := &result.Locations[0]

		location.PhysicalLocation.ArtifactLocation.URI = "file:///" + report.FilePath
	}

	return &log, nil
}

func (sr SARIFReporter) Print(reports []Report) error {
	report, err := createSARIFReport(reports)
	if err != nil {
		return err
	}

	sarifBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	sarifBytes = append(sarifBytes, '\n')

	if len(reports) > 0 && !reports[0].IsQuiet {
		fmt.Print(string(sarifBytes))
	}

	if sr.outputDest != "" {
		return outputBytesToFile(sr.outputDest, "result", "sarif", sarifBytes)
	}

	return nil
}
