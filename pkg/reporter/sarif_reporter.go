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
	Tool      tool       `json:"tool"`
	Artifacts []artifact `json:"artifacts"`
	Results   []result   `json:"results"`
}

type tool struct {
	Driver driver `json:"driver"`
}

type driver struct {
	Name    string `json:"name"`
	InfoURI string `json:"informationUri"`
}

type artifact struct {
	Location location `json:"location"`
}

type result struct {
	Kind      string           `json:"kind"`
	Level     string           `json:"level"`
	Message   message          `json:"message"`
	Locations []resultLocation `json:"locations"`
}

type message struct {
	Text string `json:"text"`
}

type resultLocation struct {
	PhysicalLocation physicalLocation `json:"physicalLocation"`
}

type physicalLocation struct {
	Location location `json:"artifactLocation"`
}

type location struct {
	URI   string `json:"uri"`
	Index *int   `json:"index,omitempty"`
}

func NewSARIFReporter(outputDest string) *SARIFReporter {
	return &SARIFReporter{
		outputDest: outputDest,
	}
}

func createSARIFReport(reports []Report) (SARIFLog, error) {
	var log SARIFLog

	n := len(reports)

	log.Version = "2.1.0"
	log.Schema = "https://schemastore.azurewebsites.net/schemas/json/sarif-2.1.0-rtm.4.json"

	log.Runs = make([]runs, 1)
	runs := &log.Runs[0]

	runs.Tool.Driver.Name = "config-file-validator"
	runs.Tool.Driver.InfoURI = "https://github.com/Boeing/config-file-validator"

	runs.Artifacts = make([]artifact, n)
	runs.Results = make([]result, n)

	for i, report := range reports {
		if strings.Contains(report.FilePath, "\\") {
			report.FilePath = strings.ReplaceAll(report.FilePath, "\\", "/")
		}

		artifact := &runs.Artifacts[i]
		artifact.Location.URI = report.FilePath

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

		result.Locations = make([]resultLocation, 1)
		location := &result.Locations[0]
		location.PhysicalLocation.Location.URI = report.FilePath
		location.PhysicalLocation.Location.Index = new(int)
		*location.PhysicalLocation.Location.Index = i
	}

	return log, nil
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
