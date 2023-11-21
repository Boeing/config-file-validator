package reporter

import (
	"fmt"
	"strings"

	"github.com/fatih/color"
)

type StdoutReporter struct{}

// Print implements the Reporter interface by outputting
// the report content to stdout
func (sr StdoutReporter) Print(reports []Report) error {
	var successCount = 0
	var failureCount = 0
	for _, report := range reports {
		if !report.IsValid {
			color.Set(color.FgRed)
			fmt.Println("    × " + report.FilePath)
			paddedString := sr.padErrorString(report.ValidationError.Error())
			fmt.Printf("        error: %v\n", paddedString)
			color.Unset()
			failureCount = failureCount + 1
		} else {
			color.Green("    ✓ " + report.FilePath)
			successCount = successCount + 1
		}
	}
	fmt.Printf("Summary: %d succeeded, %d failed\n", successCount, failureCount)
	return nil
}

func (sr StdoutReporter) PrintGroup(groupReports map[string][]Report) error {
	for key, reports := range groupReports {
		fmt.Println(key)
		for _, report := range reports {
			if !report.IsValid {
				color.Set(color.FgRed)
				fmt.Println("    × " + report.FilePath)
				paddedString := sr.padErrorString(report.ValidationError.Error())
				fmt.Printf("        error: %v\n", paddedString)
				color.Unset()
			} else {
				color.Green("    ✓ " + report.FilePath)
			}
		}
	}
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
