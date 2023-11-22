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

func Test_junitReport(t *testing.T) {
	prop1 := Property{Name: "property1", Value: "value", TextValue: "text value"}
	properties := []Property{prop1}
	testsuite := Testsuite{Name: "config-file-validator", Errors: 0, Properties: &properties}
	testsuiteBatch := []Testsuite{testsuite}
	ts := Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err := ts.getReport()
	if err == nil {
		t.Errorf("Reporting failed on getReport")
	}

	prop2 := Property{Name: "property2", Value: "value"}
	properties2 := []Property{prop2}
	testsuite = Testsuite{Name: "config-file-validator", Errors: 0, Properties: &properties2}
	testsuiteBatch = []Testsuite{testsuite}
	ts = Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err = ts.getReport()
	if err != nil {
		t.Errorf("Reporting failed on getReport")
	}

	tc1 := Testcase{Name: "testcase2", ClassName: "config-file-validator", Properties: &properties}
	testCasesBatch := []Testcase{tc1}
	testsuite = Testsuite{Name: "config-file-validator", Errors: 0, Testcases: &testCasesBatch}
	testsuiteBatch = []Testsuite{testsuite}
	ts3 := Testsuites{Name: "config-file-validator", Tests: 1, Testsuites: testsuiteBatch}

	_, err = ts3.getReport()
	if err == nil {
		t.Errorf("Reporting failed on getReport")
	}

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

	reports := []Report{reportNoValidationError, reportWithBackslashPath, reportWithValidationError}

	junitReporter := JunitReporter{}
	err = junitReporter.Print(reports)
	if err != nil {
		t.Errorf("Reporting failed")
	}
}
