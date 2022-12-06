package reporter

// The Report object stores information about the report
// and the results of the validation
type Report struct {
	FileName        string
	FilePath        string
	IsValid         bool
	ValidationError error
}

// Reporter is the interface that wraps the Print method

// Print accepts an array of Report objects and determines
// how to output the contents. Output could be stdout,
// files, etc
type Reporter interface {
	Print(reports []Report) error
}
