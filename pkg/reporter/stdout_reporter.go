package reporter

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type StdoutReporter struct {
}

// Print implements the Reporter interface by outputting
// the report content to stdout
func (sr StdoutReporter) Print(reports []Report) error {
	var results string
	var successCount = 0
	var failureCount = 0
	for _, report := range reports {
		if !report.IsValid {
			color.Set(color.FgRed)
			results += fmt.Sprintln("    × " + report.FilePath)
			paddedString := sr.padErrorString(report.ValidationError.Error())
			results += fmt.Sprintf("        error: %v\n", paddedString)
			color.Unset()
			failureCount = failureCount + 1
		} else {
			tmp := fmt.Sprintln("    ✓ " + report.FilePath)
			color.Green(tmp)
			results += tmp
			successCount = successCount + 1
		}
	}
	results += fmt.Sprintf("Summary: %d succeeded, %d failed\n", successCount, failureCount)
	fmt.Printf(results)

	return nil
}

// padErrorString adds padding to every newline in the error
// string, except the first line and removes any trailing newlines
// or spaces
func (sr StdoutReporter) padErrorString(errS string) string {
	errS = strings.TrimSpace(errS)
	lines := strings.Split(errS, "\n")
	for idx := 1; idx < len(lines); idx++ {
		lines[idx] = "               " + lines[idx]
	}
	paddedErr := strings.Join(lines, "\n")
	return paddedErr
}
