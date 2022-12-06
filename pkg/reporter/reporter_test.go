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

	reports := []Report{reportNoValidationError, reportWithValidationError}

	stdoutReporter := StdoutReporter{}
	err := stdoutReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}
