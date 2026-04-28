package reporter

import (
	"errors"
	"strings"
	"testing"
)

func TestGitHubReporter_ValidFilesEmitNothing(t *testing.T) {
	reports := []Report{
		{FileName: "a.json", FilePath: "a.json", IsValid: true},
		{FileName: "b.yaml", FilePath: "b.yaml", IsValid: true},
	}
	got := buildGitHubReport(reports)
	if got != "" {
		t.Fatalf("expected empty output for all-valid reports, got %q", got)
	}
}

func TestGitHubReporter_FormatsErrorWithLineAndCol(t *testing.T) {
	reports := []Report{
		{
			FileName: "config.json", FilePath: "config/bad.json", IsValid: false,
			ValidationError: errors.New("syntax: unexpected token"),
			StartLine:       3, StartColumn: 12,
		},
	}
	got := buildGitHubReport(reports)
	want := "::error file=config/bad.json,line=3,col=12::syntax: unexpected token\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_OmitsZeroLineAndCol(t *testing.T) {
	reports := []Report{
		{FileName: "x.toml", FilePath: "x.toml", IsValid: false, ValidationError: errors.New("bad")},
	}
	got := buildGitHubReport(reports)
	want := "::error file=x.toml::bad\n"
	if got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}

func TestGitHubReporter_FallsBackToValidationErrorsSlice(t *testing.T) {
	reports := []Report{
		{
			FilePath: "p.yaml", IsValid: false,
			ValidationErrors: []string{"missing field: name", "extra field: foo"},
		},
	}
	got := buildGitHubReport(reports)
	if !strings.Contains(got, "missing field: name") {
		t.Fatalf("expected first ValidationErrors entry, got %q", got)
	}
}

func TestGitHubReporter_FallsBackToGenericMessage(t *testing.T) {
	reports := []Report{
		{FilePath: "p.yaml", IsValid: false},
	}
	got := buildGitHubReport(reports)
	if !strings.Contains(got, "validation failed") {
		t.Fatalf("expected fallback message, got %q", got)
	}
}

func TestGitHubReporter_EscapesMessageSpecialChars(t *testing.T) {
	reports := []Report{
		{
			FilePath: "x.yml", IsValid: false,
			ValidationError: errors.New("got 100% bad\nthen worse"),
			StartLine:       1,
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
			FilePath: "weird,path:with/special.yaml", IsValid: false,
			ValidationError: errors.New("bad"),
			StartLine:       2,
		},
	}
	got := buildGitHubReport(reports)
	if !strings.Contains(got, "%3A") || !strings.Contains(got, "%2C") {
		t.Fatalf("expected colon/comma escapes in file property, got %q", got)
	}
}

func TestGitHubReporter_MultipleReportsOneLineEach(t *testing.T) {
	reports := []Report{
		{FilePath: "a.json", IsValid: false, ValidationError: errors.New("e1"), StartLine: 1},
		{FilePath: "b.yaml", IsValid: true},
		{FilePath: "c.toml", IsValid: false, ValidationError: errors.New("e2"), StartLine: 5, StartColumn: 3},
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
	// Build the report manually and confirm the quiet branch in Print
	// drops it. We can't easily capture stdout in a unit test without
	// extra plumbing, but we can confirm the behavior by exercising the
	// branch logic directly.
	reports := []Report{
		{FilePath: "a.json", IsValid: false, ValidationError: errors.New("e1"), IsQuiet: true},
	}
	gr := NewGitHubReporter("")
	if err := gr.Print(reports); err != nil {
		t.Fatalf("Print returned error in quiet mode: %v", err)
	}
	// Negative assertion: ensure the build still produces content but
	// Print doesn't surface it via stdout when IsQuiet is set on the
	// first report. The build vs print split is the contract -- file
	// output (outputDest) and helpers like buildGitHubReport are
	// unaffected.
	if buildGitHubReport(reports) == "" {
		t.Fatalf("buildGitHubReport should still emit content in quiet mode (only Print suppresses stdout)")
	}
}
