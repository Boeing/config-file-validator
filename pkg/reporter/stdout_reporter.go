package reporter

import (
	"fmt"
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
			fmt.Printf("        error: %v\n", report.ValidationError)
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
