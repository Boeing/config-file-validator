package reporter

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type StdoutReporter struct {
	outputDest string
}

type reportStdout struct {
	Text    string
	Summary summary
}

func NewStdoutReporter(outputDest string) *StdoutReporter {
	return &StdoutReporter{
		outputDest: outputDest,
	}
}

// Print implements the Reporter interface by outputting
// the report content to stdout
func (sr StdoutReporter) Print(reports []Report) error {
	if len(reports) > 0 && reports[0].IsQuiet {
		return nil
	}

	stdoutReport := createStdoutReport(reports, 1)
	fmt.Println(stdoutReport.Text)

	if sr.outputDest != "" {
		return outputBytesToFile(sr.outputDest, "result", "txt", []byte(stdoutReport.Text))
	}

	return nil
}

// There is repeated code in the following two functions. Trying to consolidate
// the code into one function is difficult because of the output format
func PrintSingleGroupStdout(groupReport map[string][]Report) error {
	totalSuccessCount := 0
	totalFailureCount := 0

	for group, reports := range groupReport {
		fmt.Printf("%s\n", group)
		stdoutReport := createStdoutReport(reports, 1)
		totalSuccessCount += stdoutReport.Summary.Passed
		totalFailureCount += stdoutReport.Summary.Failed
		fmt.Println(stdoutReport.Text)
		fmt.Printf("Summary: %d succeeded, %d failed\n\n", stdoutReport.Summary.Passed, stdoutReport.Summary.Failed)
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)
	return nil
}

// Prints the report for when two groups are passed in the groupby flag
func PrintDoubleGroupStdout(groupReport map[string]map[string][]Report) error {
	totalSuccessCount := 0
	totalFailureCount := 0

	for group, reports := range groupReport {
		fmt.Printf("%s\n", group)
		for group2, reports2 := range reports {
			fmt.Printf("    %s\n", group2)
			stdoutReport := createStdoutReport(reports2, 2)
			totalSuccessCount += stdoutReport.Summary.Passed
			totalFailureCount += stdoutReport.Summary.Failed
			fmt.Println(stdoutReport.Text)
			fmt.Printf("    Summary: %d succeeded, %d failed\n\n", stdoutReport.Summary.Passed, stdoutReport.Summary.Failed)
		}
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)

	return nil
}

// Prints the report for when three groups are passed in the groupby flag
func PrintTripleGroupStdout(groupReport map[string]map[string]map[string][]Report) error {
	totalSuccessCount := 0
	totalFailureCount := 0

	for groupOne, header := range groupReport {
		fmt.Printf("%s\n", groupOne)
		for groupTwo, subheader := range header {
			fmt.Printf("    %s\n", groupTwo)
			for groupThree, reports := range subheader {
				fmt.Printf("        %s\n", groupThree)
				stdoutReport := createStdoutReport(reports, 3)
				totalSuccessCount += stdoutReport.Summary.Passed
				totalFailureCount += stdoutReport.Summary.Failed
				fmt.Println(stdoutReport.Text)
				fmt.Printf("        Summary: %d succeeded, %d failed\n\n", stdoutReport.Summary.Passed, stdoutReport.Summary.Failed)
			}
		}
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)
	return nil
}

// Creates the standard text report
func createStdoutReport(reports []Report, indentSize int) reportStdout {
	result := reportStdout{}
	baseIndent := "    "
	indent, errIndent := strings.Repeat(baseIndent, indentSize), strings.Repeat(baseIndent, indentSize+1)

	for _, report := range reports {
		if !report.IsValid {
			fmtRed := color.New(color.FgRed)
			paddedString := padErrorString(report.ValidationError.Error())
			result.Text += fmtRed.Sprintln(indent + "× " + report.FilePath)
			result.Text += fmtRed.Sprintf(errIndent+"error: %v\n", paddedString)
			result.Summary.Failed++
		} else {
			result.Text += color.New(color.FgGreen).Sprintf(indent + "✓ " + report.FilePath)
			result.Summary.Passed++
		}
	}

	return result
}

// padErrorString adds padding to every newline in the error
// string, except the first line and removes any trailing newlines
// or spaces
func padErrorString(errS string) string {
	errS = strings.TrimSpace(errS)
	lines := strings.Split(errS, "\n")
	for idx := 1; idx < len(lines); idx++ {
		lines[idx] = "               " + lines[idx]
	}
	paddedErr := strings.Join(lines, "\n")
	return paddedErr
}
