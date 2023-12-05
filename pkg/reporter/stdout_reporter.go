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

// There is repeated code in the following two functions. Trying to consolidate
// the code into one function is difficult because of the output format
func (sr StdoutReporter) PrintSingleGroup(groupReport map[string][]Report) error {
	var successCount = 0
	var failureCount = 0
	var totalSuccessCount = 0
	var totalFailureCount = 0
	for group, reports := range groupReport {
		fmt.Printf("%s\n", group)
		successCount = 0
		failureCount = 0
		for _, report := range reports {
			if !report.IsValid {
				color.Set(color.FgRed)
				fmt.Println("    × " + report.FilePath)
				paddedString := sr.padErrorString(report.ValidationError.Error())
				fmt.Printf("        error: %v\n", paddedString)
				color.Unset()
				failureCount = failureCount + 1
				totalFailureCount = totalFailureCount + 1
			} else {
				color.Green("    ✓ " + report.FilePath)
				successCount = successCount + 1
				totalSuccessCount = totalSuccessCount + 1
			}
		}
		fmt.Printf("Summary: %d succeeded, %d failed\n\n", successCount, failureCount)
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)
	return nil
}

// Prints the report for when two groups are passed in the groupby flag
func (sr StdoutReporter) PrintDoubleGroup(groupReport map[string]map[string][]Report) error {
	var successCount = 0
	var failureCount = 0
	var totalSuccessCount = 0
	var totalFailureCount = 0

	for group, reports := range groupReport {
		fmt.Printf("%s\n", group)
		for group2, reports2 := range reports {
			fmt.Printf("    %s\n", group2)
			successCount = 0
			failureCount = 0
			for _, report := range reports2 {
				if !report.IsValid {
					color.Set(color.FgRed)
					fmt.Println("        × " + report.FilePath)
					paddedString := sr.padErrorString(report.ValidationError.Error())
					fmt.Printf("            error: %v\n", paddedString)
					color.Unset()
					failureCount = failureCount + 1
					totalFailureCount = totalFailureCount + 1
				} else {
					color.Green("        ✓ " + report.FilePath)
					successCount = successCount + 1
					totalSuccessCount = totalSuccessCount + 1
				}
			}
			fmt.Printf("    Summary: %d succeeded, %d failed\n\n", successCount, failureCount)
		}
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)

	return nil
}

// Prints the report for when three groups are passed in the groupby flag
func (sr StdoutReporter) PrintTripleGroup(groupReport map[string]map[string]map[string][]Report) error {
	var successCount = 0
	var failureCount = 0
	var totalSuccessCount = 0
	var totalFailureCount = 0

	for groupOne, header := range groupReport {
		fmt.Printf("%s\n", groupOne)
		for groupTwo, subheader := range header {
			fmt.Printf("    %s\n", groupTwo)
			for groupThree, reports := range subheader {
				fmt.Printf("        %s\n", groupThree)
				successCount = 0
				failureCount = 0
				for _, report := range reports {
					if !report.IsValid {
						color.Set(color.FgRed)
						fmt.Println("            × " + report.FilePath)
						paddedString := sr.padErrorString(report.ValidationError.Error())
						fmt.Printf("                error: %v\n", paddedString)
						color.Unset()
						failureCount = failureCount + 1
						totalFailureCount = totalFailureCount + 1
					} else {
						color.Green("            ✓ " + report.FilePath)
						successCount = successCount + 1
						totalSuccessCount = totalSuccessCount + 1
					}
				}
				fmt.Printf("        Summary: %d succeeded, %d failed\n\n", successCount, failureCount)
			}
		}
	}

	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSuccessCount, totalFailureCount)
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
