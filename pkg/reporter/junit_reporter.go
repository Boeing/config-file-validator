package reporter

import (
	"encoding/xml"
	"fmt"
	"strings"
	"time"
)

type JunitReporter struct{}

const (
	Header = `<?xml version="1.0" encoding="UTF-8"?>` + "\n"
)

type Message struct {
	InnerXML string `xml:",innerxml"`
}

// https://github.com/testmoapp/junitxml#basic-junit-xml-structure
type Testsuites struct {
	XMLName    xml.Name    `xml:"testsuites"`
	Name       string      `xml:"name,attr,omitempty"`
	Tests      int         `xml:"tests,attr,omitempty"`
	Failures   int         `xml:"failures,attr,omitempty"`
	Errors     int         `xml:"errors,attr,omitempty"`
	Skipped    int         `xml:"skipped,attr,omitempty"`
	Assertions int         `xml:"assertions,attr,omitempty"`
	Time       float32     `xml:"time,attr,omitempty"`
	Timestamp  *time.Time  `xml:"timestamp,attr,omitempty"`
	Testsuites []Testsuite `xml:"testsuite"`
}

type Testsuite struct {
	XMLName    xml.Name    `xml:"testsuite"`
	Name       string      `xml:"name,attr"`
	Tests      int         `xml:"tests,attr,omitempty"`
	Failures   int         `xml:"failures,attr,omitempty"`
	Errors     int         `xml:"errors,attr,omitempty"`
	Skipped    int         `xml:"skipped,attr,omitempty"`
	Assertions int         `xml:"assertions,attr,omitempty"`
	Time       float32     `xml:"time,attr,omitempty"`
	Timestamp  *time.Time  `xml:"timestamp,attr,omitempty"`
	File       string      `xml:"file,attr,omitempty"`
	Testcases  *[]Testcase `xml:"testcase,omitempty"`
	Properties *[]Property `xml:"properties>property,omitempty"`
	SystemOut  *SystemOut  `xml:"system-out,omitempty"`
	SystemErr  *SystemErr  `xml:"system-err,omitempty"`
}

type Testcase struct {
	XMLName         xml.Name         `xml:"testcase"`
	Name            string           `xml:"name,attr"`
	ClassName       string           `xml:"classname,attr"`
	Assertions      int              `xml:"assertions,attr,omitempty"`
	Time            float32          `xml:"time,attr,omitempty"`
	File            string           `xml:"file,attr,omitempty"`
	Line            int              `xml:"line,attr,omitempty"`
	Skipped         *Skipped         `xml:"skipped,omitempty,omitempty"`
	Properties      *[]Property      `xml:"properties>property,omitempty"`
	TestcaseError   *TestcaseError   `xml:"error,omitempty"`
	TestcaseFailure *TestcaseFailure `xml:"failure,omitempty"`
}

type Skipped struct {
	XMLName xml.Name `xml:"skipped"`
	Message string   `xml:"message,attr"`
}

type TestcaseError struct {
	XMLName   xml.Name `xml:"error"`
	Message   Message
	Type      string `xml:"type,omitempty"`
	TextValue string `xml:",chardata"`
}

type TestcaseFailure struct {
	XMLName xml.Name `xml:"failure"`
	// Message   string   `xml:"message,omitempty"`
	Message   Message
	Type      string `xml:"type,omitempty"`
	TextValue string `xml:",chardata"`
}

type SystemOut struct {
	XMLName   xml.Name `xml:"system-out"`
	TextValue string   `xml:",chardata"`
}

type SystemErr struct {
	XMLName   xml.Name `xml:"system-err"`
	TextValue string   `xml:",chardata"`
}

type Property struct {
	XMLName   xml.Name `xml:"property"`
	TextValue string   `xml:",chardata"`
	Name      string   `xml:"name,attr"`
	Value     string   `xml:"value,attr,omitempty"`
}

func checkProperty(property Property, xmlElementName string, name string) error {
	if property.Value != "" && property.TextValue != "" {
		return fmt.Errorf("property %s in %s %s should contain value or a text value, not both",
			property.Name, xmlElementName, name)
	}
	return nil
}

func checkTestCase(testcase Testcase) (err error) {
	if testcase.Properties != nil {
		for propidx := range *testcase.Properties {
			property := (*testcase.Properties)[propidx]
			if err = checkProperty(property, "testcase", testcase.Name); err != nil {
				return err
			}
		}
	}
	return nil
}

func checkTestSuite(testsuite Testsuite) (err error) {
	if testsuite.Properties != nil {
		for pridx := range *testsuite.Properties {
			property := (*testsuite.Properties)[pridx]
			if err = checkProperty(property, "testsuite", testsuite.Name); err != nil {
				return err
			}
		}
	}

	if testsuite.Testcases != nil {
		for tcidx := range *testsuite.Testcases {
			testcase := (*testsuite.Testcases)[tcidx]
			if err = checkTestCase(testcase); err != nil {
				return err
			}
		}
	}
	return nil
}

func (ts Testsuites) checkPropertyValidity() (err error) {
	for tsidx := range ts.Testsuites {
		testsuite := ts.Testsuites[tsidx]
		if err = checkTestSuite(testsuite); err != nil {
			return err
		}
	}
	return nil
}

func (ts Testsuites) getReport() ([]byte, error) {
	err := ts.checkPropertyValidity()
	if err != nil {
		return []byte{}, err
	}

	data, err := xml.MarshalIndent(ts, " ", "  ")
	if err != nil {
		return []byte{}, err
	}

	return data, nil
}

func (jr JunitReporter) Print(reports []Report) error {
	testcases := []Testcase{}
	testErrors := 0

	for _, r := range reports {
		if strings.Contains(r.FilePath, "\\") {
			r.FilePath = strings.ReplaceAll(r.FilePath, "\\", "/")
		}
		tc := Testcase{Name: fmt.Sprintf("%s validation", r.FilePath), File: r.FilePath, ClassName: "config-file-validator"}
		if !r.IsValid {
			testErrors++
			tc.TestcaseFailure = &TestcaseFailure{Message: Message{InnerXML: r.ValidationError.Error()}}
		}
		testcases = append(testcases, tc)
	}
	testsuite := Testsuite{Name: "config-file-validator", Testcases: &testcases, Errors: testErrors}
	testsuiteBatch := []Testsuite{testsuite}
	ts := Testsuites{Name: "config-file-validator", Tests: len(reports), Testsuites: testsuiteBatch}

	data, err := ts.getReport()
	if err != nil {
		return err
	}
	fmt.Println(Header + string(data))
	return nil
}
