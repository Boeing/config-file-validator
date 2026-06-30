package reporter

import (
	"strings"
	"testing"
)

func TestGitHubReporter_ValidFilesEmitNothing(t *testing.T) {
	reports := []Report{
		{FilePath: "a.json", Status: StatusPass},
		{FilePath: "b.yaml", Status: StatusPass},
	}
	got := buildGitHubReport(reports)
	if got != "" {
		t.Fatalf("expected empty output for all-valid reports, got %q", got)
	}
}

func TestGitHubReporter_FormatsErrorWithLineAndCol(t *testing.T) {
	reports := []Report{
		{
			FilePath: "config/bad.json", Status: StatusFail,
			Issues: []Issue{{Type: IssueTypeSyntax, Message: "unexpected token", Line: 3, Column: 12}},
		},
	}
	got := buildGitHubReport(reports)
	want := "::error file=config/bad.json,line=3,col=12::unexpected token\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_OmitsZeroLineAndCol(t *testing.T) {
	reports := []Report{
		{FilePath: "x.toml", Status: StatusFail, Issues: []Issue{{Type: IssueTypeSyntax, Message: "bad"}}},
	}
	got := buildGitHubReport(reports)
	want := "::error file=x.toml::bad\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_MultipleIssuesEmitMultipleAnnotations(t *testing.T) {
	reports := []Report{
		{
			FilePath: "schema/config.json", Status: StatusFail,
			Issues: []Issue{
				{Type: IssueTypeSchema, Message: "field one must be string", Line: 4, Column: 2},
				{Type: IssueTypeSchema, Message: "field two must be integer", Line: 8, Column: 6},
				{Type: IssueTypeSchema, Message: "field three is required", Line: 12, Column: 10},
			},
		},
	}

	got := buildGitHubReport(reports)
	want := strings.Join([]string{
		"::error file=schema/config.json,line=4,col=2::field one must be string",
		"::error file=schema/config.json,line=8,col=6::field two must be integer",
		"::error file=schema/config.json,line=12,col=10::field three is required",
		"",
	}, "\n")
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_UnformattedEmitsWarning(t *testing.T) {
	reports := []Report{
		{
			FilePath: "main.toml", Status: StatusUnformatted,
			Issues: []Issue{{Type: IssueTypeFormat, Message: "inconsistent indentation"}},
		},
	}
	got := buildGitHubReport(reports)
	want := "::warning file=main.toml::inconsistent indentation\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_EscapesMessageSpecialChars(t *testing.T) {
	reports := []Report{
		{
			FilePath: "x.yml", Status: StatusFail,
			Issues: []Issue{{Type: IssueTypeSyntax, Message: "got 100% bad\nthen worse", Line: 1}},
		},
	}
	got := buildGitHubReport(reports)
	if !strings.Contains(got, "100%25 bad") {
		t.Fatalf("expected %% escape, got %q", got)
	}
	if !strings.Contains(got, "%0Athen") {
		t.Fatalf("expected newline escape, got %q", got)
	}
}

func TestGitHubReporter_EscapesPropertySpecialChars(t *testing.T) {
	reports := []Report{
		{
			FilePath: "weird,path:with/special.yaml", Status: StatusFail,
			Issues: []Issue{{Type: IssueTypeSyntax, Message: "bad", Line: 2}},
		},
	}
	got := buildGitHubReport(reports)
	if !strings.Contains(got, "%3A") || !strings.Contains(got, "%2C") {
		t.Fatalf("expected colon/comma escapes in file property, got %q", got)
	}
}

func TestGitHubReporter_MultipleReportsSkipsValid(t *testing.T) {
	reports := []Report{
		{FilePath: "a.json", Status: StatusFail, Issues: []Issue{{Type: IssueTypeSyntax, Message: "e1", Line: 1}}},
		{FilePath: "b.yaml", Status: StatusPass},
		{FilePath: "c.toml", Status: StatusFail, Issues: []Issue{{Type: IssueTypeSyntax, Message: "e2", Line: 5, Column: 3}}},
	}
	got := buildGitHubReport(reports)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected 2 annotation lines (skip valid), got %d: %q", len(lines), got)
	}
	if !strings.Contains(lines[0], "a.json") || !strings.Contains(lines[1], "c.toml") {
		t.Fatalf("unexpected line ordering: %q", got)
	}
}

func TestGitHubReporter_QuietModeSuppressesStdout(t *testing.T) {
	reports := []Report{
		{FilePath: "a.json", Status: StatusFail, Issues: []Issue{{Type: IssueTypeSyntax, Message: "e1"}}, IsQuiet: true},
	}
	gr := NewGitHubReporter("")
	if err := gr.Print(reports); err != nil {
		t.Fatalf("Print returned error in quiet mode: %v", err)
	}
	if buildGitHubReport(reports) == "" {
		t.Fatal("buildGitHubReport should still emit content in quiet mode")
	}
}
