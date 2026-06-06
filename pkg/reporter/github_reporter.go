package reporter

import (
	"fmt"
	"strings"
)

type GitHubReporter struct {
	outputDest string
}

type githubAnnotation struct {
	message string
	line    int
	column  int
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
		for _, annotation := range githubAnnotations(r) {
			b.WriteString(formatGitHubAnnotation(r, annotation))
			_ = b.WriteByte('\n')
		}
	}
	return b.String()
}

func githubAnnotations(r Report) []githubAnnotation {
	if len(r.ValidationErrors) > 1 {
		annotations := make([]githubAnnotation, 0, len(r.ValidationErrors))
		for i, errMsg := range r.ValidationErrors {
			line, column := r.StartLine, r.StartColumn
			if i < len(r.ErrorLines) && r.ErrorLines[i] > 0 {
				line = r.ErrorLines[i]
			}
			if i < len(r.ErrorColumns) && r.ErrorColumns[i] > 0 {
				column = r.ErrorColumns[i]
			}
			annotations = append(annotations, githubAnnotation{
				message: errMsg,
				line:    line,
				column:  column,
			})
		}
		return annotations
	}

	return []githubAnnotation{{
		message: errorMessage(r),
		line:    r.StartLine,
		column:  r.StartColumn,
	}}
}

func formatGitHubAnnotation(r Report, annotation githubAnnotation) string {
	props := make([]string, 0, 3)
	if r.FilePath != "" {
		props = append(props, "file="+escapeGitHubProperty(r.FilePath))
	}
	if annotation.line > 0 {
		props = append(props, fmt.Sprintf("line=%d", annotation.line))
	}
	if annotation.column > 0 {
		props = append(props, fmt.Sprintf("col=%d", annotation.column))
	}

	if len(props) == 0 {
		return "::error::" + escapeGitHubMessage(annotation.message)
	}
	return "::error " + strings.Join(props, ",") + "::" + escapeGitHubMessage(annotation.message)
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
