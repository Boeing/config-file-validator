package reporter

import (
	"errors"
	"testing"
)

func Test_stdoutReport(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReport(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithBackslashPath := Report{
		"good.xml",
		"\\fake\\path\\good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithBackslashPath}

	jsonReporter := JsonReporter{}
	err := jsonReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_stdoutReportSingleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := "pass-fail"

	groupReports := map[string][]Report{"pass-fail": reports}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.PrintSingleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_stdoutReportDoubleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := []string{"pass-fail", "filetype"}

	groupReports := map[string]map[string][]Report{"pass-fail": {"pass-fail": reports}, "filetype": {"filetype": reports}}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.PrintDoubleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_stdoutReportTripleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := []string{"pass-fail", "filetype", "directory"}

	groupReports := map[string]map[string]map[string][]Report{
		"pass-fail": {"directory": {"filetype": reports}},
		"filetype":  {"directory": {"pass-fail": reports}},
		"directory": {"filetype": {"pass-fail": reports}}}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.PrintTripleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReportSingleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := "pass-fail"

	groupReports := map[string][]Report{"pass-fail": reports}

	stdoutReporter := JsonReporter{}
	err := stdoutReporter.PrintSingleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReportDoubleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := []string{"pass-fail", "filetype"}

	groupReports := map[string]map[string][]Report{"pass-fail": {"pass-fail": reports}, "filetype": {"filetype": reports}}

	stdoutReporter := JsonReporter{}
	err := stdoutReporter.PrintDoubleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}

func Test_jsonReportTripleGroup(t *testing.T) {
	reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}

	groupOutput := []string{"pass-fail", "filetype", "directory"}

	groupReports := map[string]map[string]map[string][]Report{
		"pass-fail": {"directory": {"filetype": reports}},
		"filetype":  {"directory": {"pass-fail": reports}},
		"directory": {"filetype": {"pass-fail": reports}}}

	stdoutReporter := JsonReporter{}
	err := stdoutReporter.PrintTripleGroup(groupReports, groupOutput)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}
