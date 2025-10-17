package reporter

import (
	"errors"
	"testing"
)

// Return a slice of Report types
func GetTestReports() []Report {
		reportNoValidationError := Report{
		"good.xml",
		"/fake/path/good.xml",
		true,
		nil,
		false,
	}

	reportWithValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse bad.xml file"),
		false,
	}

	reportWithMultiLineValidationError := Report{
		"bad.xml",
		"/fake/path/bad.xml",
		false,
		errors.New("Unable to parse keys:\nkey1\nkey2"),
		false,
	}

	reports := []Report{reportNoValidationError, reportWithValidationError, reportWithMultiLineValidationError}
	return reports
}

// Validate each report type with default
func Test_Reporters(t *testing.T) {
    tests := []struct {
        name     string
        reporter Reporter
    }{
        {"stdout", NewStdoutReporter("")},
        {"json", JSONReporter{}},
        {"junit", JunitReporter{}},
        {"sarif", SARIFReporter{}},
    }

    reports := GetTestReports()

    for _, tc := range tests {
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            if err := tc.reporter.Print(reports); err != nil {
                t.Fatalf("reporter %q failed: %v", tc.name, err)
            }
        })
    }
}

//
func Test_ReportOutputFile(t *testing.T) {

}