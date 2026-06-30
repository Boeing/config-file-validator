package reporter

import (
	"fmt"
	"strings"
)

// GitHubReporter outputs results as GitHub Actions workflow commands.
// Errors emit ::error, format warnings emit ::warning.
type GitHubReporter struct {
	outputDest string
}

// NewGitHubReporter creates a GitHubReporter.
func NewGitHubReporter(outputDest string) *GitHubReporter {
	return &GitHubReporter{outputDest: outputDest}
}

// Print implements the Reporter interface.
func (gr GitHubReporter) Print(reports []Report) error {
	out := buildGitHubReport(reports)

	if gr.outputDest != "" {
		return outputBytesToFile(gr.outputDest, "result", "txt", []byte(out))
	}

	if out != "" && (len(reports) == 0 || !reports[0].IsQuiet) {
		fmt.Print(out)
	}
	return nil
}

func buildGitHubReport(reports []Report) string {
	var b strings.Builder
	for _, r := range reports {
		if r.Status == StatusPass {
			continue
		}
		level := "error"
		if r.Status == StatusUnformatted {
			level = "warning"
		}
		for _, issue := range r.Issues {
			b.WriteString(formatGitHubCommand(level, r.FilePath, issue))
			_ = b.WriteByte('\n')
		}
	}
	return b.String()
}

func formatGitHubCommand(level, filePath string, issue Issue) string {
	props := make([]string, 0, 3)
	if filePath != "" {
		props = append(props, "file="+escapeGitHubProperty(filePath))
	}
	if issue.Line > 0 {
		props = append(props, fmt.Sprintf("line=%d", issue.Line))
	}
	if issue.Column > 0 {
		props = append(props, fmt.Sprintf("col=%d", issue.Column))
	}

	msg := escapeGitHubMessage(issue.Message)
	if len(props) == 0 {
		return "::" + level + "::" + msg
	}
	return "::" + level + " " + strings.Join(props, ",") + "::" + msg
}

func escapeGitHubMessage(s string) string {
	s = strings.ReplaceAll(s, "%", "%25")
	s = strings.ReplaceAll(s, "\r", "%0D")
	s = strings.ReplaceAll(s, "\n", "%0A")
	return s
}

func escapeGitHubProperty(s string) string {
	s = escapeGitHubMessage(s)
	s = strings.ReplaceAll(s, ":", "%3A")
	s = strings.ReplaceAll(s, ",", "%2C")
	return s
}
