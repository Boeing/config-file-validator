package reporter

import (
	"fmt"
	"strings"
)

type GitHubReporter struct {
	outputDest string
}

func NewGitHubReporter(outputDest string) *GitHubReporter {
	return &GitHubReporter{outputDest: outputDest}
}

func (gr GitHubReporter) Print(reports []Report) error {
	out := buildGitHubReport(reports)

	if gr.outputDest != "" {
		return outputBytesToFile(gr.outputDest, "result", "txt", []byte(out))
	}

	// Mirror the stdout reporter's quiet-mode contract: when the first
	// report has IsQuiet set, suppress stdout (file output is unaffected).
	if out != "" && (len(reports) == 0 || !reports[0].IsQuiet) {
		fmt.Print(out)
	}
	return nil
}

func buildGitHubReport(reports []Report) string {
	var b strings.Builder
	for _, r := range reports {
		if r.IsValid {
			continue
		}
		b.WriteString(formatGitHubAnnotation(r))
		b.WriteByte('\n')
	}
	return b.String()
}

func formatGitHubAnnotation(r Report) string {
	props := make([]string, 0, 3)
	if r.FilePath != "" {
		props = append(props, "file="+escapeGitHubProperty(r.FilePath))
	}
	if r.StartLine > 0 {
		props = append(props, fmt.Sprintf("line=%d", r.StartLine))
	}
	if r.StartColumn > 0 {
		props = append(props, fmt.Sprintf("col=%d", r.StartColumn))
	}

	msg := errorMessage(r)
	if len(props) == 0 {
		return "::error::" + escapeGitHubMessage(msg)
	}
	return "::error " + strings.Join(props, ",") + "::" + escapeGitHubMessage(msg)
}

func errorMessage(r Report) string {
	if r.ValidationError != nil {
		return r.ValidationError.Error()
	}
	if len(r.ValidationErrors) > 0 {
		return r.ValidationErrors[0]
	}
	return "validation failed"
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
