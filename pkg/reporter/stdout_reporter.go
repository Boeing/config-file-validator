package reporter

import (
	"fmt"
	"slices"
	"strings"

	"github.com/fatih/color"
)

// StdoutReporter outputs results to stdout (or a file) in human-readable format.
type StdoutReporter struct {
	outputDest string
}

type reportStdout struct {
	Text    string
	Summary summary
}

// NewStdoutReporter creates a StdoutReporter. If outputDest is non-empty,
// output is written to that file instead of stdout.
func NewStdoutReporter(outputDest string) *StdoutReporter {
	return &StdoutReporter{
		outputDest: outputDest,
	}
}

// Print implements the Reporter interface.
func (sr StdoutReporter) Print(reports []Report) error {
	stdoutReport := createStdoutReport(reports, 1)

	if sr.outputDest != "" {
		return outputBytesToFile(sr.outputDest, "result", "txt", []byte(stdoutReport.Text))
	}

	if len(reports) > 0 && !reports[0].IsQuiet {
		fmt.Print(stdoutReport.Text)
	}

	return nil
}

// PrintGroupStdout prints a recursive grouped report to stdout.
func PrintGroupStdout(groupReport *GroupNode) error {
	totalSummary := printGroupNodeStdout(groupReport, nil, 0)
	fmt.Printf("Total Summary: %d succeeded, %d failed\n", totalSummary.Passed, totalSummary.Failed)
	return nil
}

func printGroupNodeStdout(node *GroupNode, groupPath []string, depth int) summary {
	totalSummary := summary{}

	for _, child := range node.Children {
		fmt.Printf("%s%s\n", strings.Repeat("    ", depth), child.Key)
		childSummary := printGroupNodeStdout(child, append(slices.Clone(groupPath), child.Key), depth+1)
		totalSummary.Passed += childSummary.Passed
		totalSummary.Failed += childSummary.Failed
	}

	if len(node.Children) > 0 {
		return totalSummary
	}

	stdoutReport := createStdoutReport(node.Reports, depth)
	totalSummary.Passed += stdoutReport.Summary.Passed
	totalSummary.Failed += stdoutReport.Summary.Failed
	fmt.Println(stdoutReport.Text)
	if len(groupPath) > 0 && checkGroupsForPassFail(groupPath...) {
		summaryDepth := depth - 1
		if summaryDepth < 0 {
			summaryDepth = 0
		}
		fmt.Printf(
			"%sSummary: %d succeeded, %d failed\n\n",
			strings.Repeat("    ", summaryDepth),
			stdoutReport.Summary.Passed,
			stdoutReport.Summary.Failed,
		)
	}

	return totalSummary
}

// PrintSingleGroupStdout prints a grouped report with one grouping level.
func PrintSingleGroupStdout(groupReport map[string][]Report) error {
	return PrintGroupStdout(groupNodeFromSingle(groupReport))
}

// PrintDoubleGroupStdout prints a grouped report with two grouping levels.
func PrintDoubleGroupStdout(groupReport map[string]map[string][]Report) error {
	return PrintGroupStdout(groupNodeFromDouble(groupReport))
}

// PrintTripleGroupStdout prints a grouped report with three grouping levels.
func PrintTripleGroupStdout(groupReport map[string]map[string]map[string][]Report) error {
	return PrintGroupStdout(groupNodeFromTriple(groupReport))
}

// checkGroupsForPassFail returns true if none of the provided groups are "Passed" or "Failed".
func checkGroupsForPassFail(groups ...string) bool {
	for _, group := range groups {
		if group == "Passed" || group == "Failed" {
			return false
		}
	}
	return true
}

// createStdoutReport renders reports to text with consistent symbols:
//
//	✓ path         — pass (green)
//	× path         — fail: syntax or schema error (red)
//	~ path         — unformatted (yellow)
func createStdoutReport(reports []Report, indentSize int) reportStdout {
	result := reportStdout{}
	baseIndent := "    "
	indent := strings.Repeat(baseIndent, indentSize)
	errIndent := strings.Repeat(baseIndent, indentSize+1)

	for _, report := range reports {
		switch report.Status {
		case StatusFail:
			fmtRed := color.New(color.FgRed)
			result.Text += fmtRed.Sprintf("%s× %s\n", indent, report.FilePath)
			for _, issue := range report.Issues {
				msg := formatIssueMessage(issue)
				paddedString := padErrorString(msg)
				result.Text += fmtRed.Sprintf("%serror: %v\n", errIndent, paddedString)
			}
			for _, n := range report.Notes {
				paddedString := padErrorString(n)
				result.Text += color.New(color.FgYellow).Sprintf("%snote: %v\n", errIndent, paddedString)
			}
			result.Summary.Failed++

		case StatusUnformatted:
			fmtYellow := color.New(color.FgYellow)
			result.Text += fmtYellow.Sprintf("%s~ %s\n", indent, report.FilePath)
			for _, issue := range report.Issues {
				paddedString := padErrorString(issue.Message)
				result.Text += fmtYellow.Sprintf("%snot formatted: %v\n", errIndent, paddedString)
			}
			result.Summary.Failed++

		default: // StatusPass
			result.Text += color.New(color.FgGreen).Sprintf("%s✓ %s\n", indent, report.FilePath)
			// Notes on passing files (e.g., schema warnings).
			for _, n := range report.Notes {
				paddedString := padErrorString(n)
				result.Text += color.New(color.FgYellow).Sprintf("%snote: %v\n", errIndent, paddedString)
			}
			result.Summary.Passed++
		}
	}

	return result
}

// formatIssueMessage formats an issue with optional line/column prefix.
func formatIssueMessage(issue Issue) string {
	switch {
	case issue.Line > 0 && issue.Column > 0:
		return fmt.Sprintf("%s: line %d, column %d: %s", issueTypeLabel(issue.Type), issue.Line, issue.Column, issue.Message)
	case issue.Line > 0:
		return fmt.Sprintf("%s: line %d: %s", issueTypeLabel(issue.Type), issue.Line, issue.Message)
	default:
		return fmt.Sprintf("%s: %s", issueTypeLabel(issue.Type), issue.Message)
	}
}

func issueTypeLabel(t IssueType) string {
	switch t {
	case IssueTypeSyntax:
		return "syntax"
	case IssueTypeSchema:
		return "schema"
	case IssueTypeFormat:
		return "format"
	default:
		return "error"
	}
}

// padErrorString adds padding to every newline in the error
// string, except the first line and removes any trailing newlines or spaces.
func padErrorString(errS string) string {
	errS = strings.TrimSpace(errS)
	lines := strings.Split(errS, "\n")
	for idx := 1; idx < len(lines); idx++ {
		lines[idx] = "               " + lines[idx]
	}
	return strings.Join(lines, "\n")
}
