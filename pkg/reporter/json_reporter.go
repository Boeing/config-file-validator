package reporter

import (
	"encoding/json"
	"fmt"
	"strings"
)

type JsonReporter struct{}

type fileStatus struct {
	Path   string `json:"path"`
	Status string `json:"status"`
	Error  string `json:"error,omitempty"`
}

type summary struct {
	Passed int `json:"passed"`
	Failed int `json:"failed"`
}

type reportJSON struct {
	Files   []fileStatus `json:"files"`
	Summary summary      `json:"summary"`
}

type singleGroupReportJSON struct {
	Files   map[string][]fileStatus `json:"files"`
	Summary summary                 `json:"summary"`
}

type doubleGroupReportJSON struct {
	Files   map[string]map[string][]fileStatus `json:"files"`
	Summary summary                            `json:"summary"`
}

// Print implements the Reporter interface by outputting
// the report content to stdout as JSON
func (jr JsonReporter) Print(reports []Report) error {
	var report reportJSON

	for _, r := range reports {
		status := "passed"
		errorStr := ""
		if !r.IsValid {
			status = "failed"
			errorStr = r.ValidationError.Error()
		}

		// Convert Windows-style file paths.
		if strings.Contains(r.FilePath, "\\") {
			r.FilePath = strings.ReplaceAll(r.FilePath, "\\", "/")
		}

		report.Files = append(report.Files, fileStatus{
			Path:   r.FilePath,
			Status: status,
			Error:  errorStr,
		})
	}

	report.Summary.Passed = 0
	report.Summary.Failed = 0
	for _, f := range report.Files {
		if f.Status == "passed" {
			report.Summary.Passed++
		} else {
			report.Summary.Failed++
		}
	}

	jsonBytes, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func (jr JsonReporter) PrintSingleGroup(groupReports map[string][]Report, groupOutput string) error {
	var groupReport singleGroupReportJSON

	for _, reports := range groupReports {
		for _, r := range reports {
			status := "passed"
			errorStr := ""
			if !r.IsValid {
				status = "failed"
				errorStr = r.ValidationError.Error()
			}

			// Convert Windows-style file paths.
			if strings.Contains(r.FilePath, "\\") {
				r.FilePath = strings.ReplaceAll(r.FilePath, "\\", "/")
			}

			file := fileStatus{
				Path:   r.FilePath,
				Status: status,
				Error:  errorStr,
			}

			// May be worthwhile to abstract this out into a function.
			switch groupOutput {
			case "filetype":
				fileExtension := strings.Split(r.FileName, ".")[1]
				if fileExtension == "yml" {
					fileExtension = "yaml"
				}
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[fileExtension] = []fileStatus{file}
				} else {
					groupReport.Files[fileExtension] = append(groupReport.Files[fileExtension], file)
				}
			case "pass-fail":
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[status] = []fileStatus{file}
				} else {
					groupReport.Files[status] = append(groupReport.Files[status], file)
				}
			case "directory":
				directoryPath := strings.Split(r.FilePath, "/")
				directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
				directory = directory + "/"
				if groupReport.Files == nil {
					groupReport.Files = make(map[string][]fileStatus)
					groupReport.Files[directory] = []fileStatus{file}
				} else {
					groupReport.Files[directory] = append(groupReport.Files[directory], file)
				}
			default:
				return fmt.Errorf("Invalid group output: %s", groupOutput)
			}
		}
	}
	groupReport.Summary.Passed = 0
	groupReport.Summary.Failed = 0
	for _, files := range groupReport.Files {
		for _, f := range files {
			if f.Status == "passed" {
				groupReport.Summary.Passed++
			} else {
				groupReport.Summary.Failed++
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(groupReport, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(jsonBytes))

	return nil
}

func (jr JsonReporter) PrintDoubleGroup(reports map[string]map[string][]Report, groupOutput []string) error {
	var groupReport doubleGroupReportJSON
	for _, subGroup := range reports {
		for _, reports := range subGroup {
			for _, r := range reports {
				status := "passed"
				errorStr := ""
				if !r.IsValid {
					status = "failed"
					errorStr = r.ValidationError.Error()
				}

				// Convert Windows-style file paths.
				if strings.Contains(r.FilePath, "\\") {
					r.FilePath = strings.ReplaceAll(r.FilePath, "\\", "/")
				}

				file := fileStatus{
					Path:   r.FilePath,
					Status: status,
					Error:  errorStr,
				}
                // TODO: Lifetime issue. Need to fix.
                err := error(nil)
                groupReport, err = doubleGroupBySwitch(file, groupOutput, &groupReport, r, status)
				if err != nil {
					return err
				}
			}
		}
	}

	groupReport.Summary.Passed = 0
	groupReport.Summary.Failed = 0
	for _, subGroup := range groupReport.Files {
		for _, files := range subGroup {
			for _, f := range files {
				if f.Status == "passed" {
					groupReport.Summary.Passed++
				} else {
					groupReport.Summary.Failed++
				}
			}
		}
	}

	jsonBytes, err := json.MarshalIndent(groupReport, "", "  ")
	if err != nil {
		return err
	}

	fmt.Println(string(jsonBytes))
	return nil
}

func doubleGroupBySwitch(file fileStatus, groupOutput []string, groupReport *doubleGroupReportJSON, r Report, status string) (doubleGroupReportJSON, error) {
	switch {
	case groupOutput[0] == "filetype" && groupOutput[1] == "pass-fail":
		fileExtension := strings.Split(r.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[fileExtension] = make(map[string][]fileStatus)
			groupReport.Files[fileExtension][status] = []fileStatus{file}
		} else {
			if groupReport.Files[fileExtension] == nil {
				groupReport.Files[fileExtension] = make(map[string][]fileStatus)
				groupReport.Files[fileExtension][status] = []fileStatus{file}
			} else {
				groupReport.Files[fileExtension][status] = append(groupReport.Files[fileExtension][status], file)
			}
		}
	case groupOutput[0] == "filetype" && groupOutput[1] == "directory":
		fileExtension := strings.Split(r.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		directoryPath := strings.Split(r.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[fileExtension] = make(map[string][]fileStatus)
			groupReport.Files[fileExtension][directory] = []fileStatus{file}
		} else {
			if groupReport.Files[fileExtension] == nil {
				groupReport.Files[fileExtension] = make(map[string][]fileStatus)
				groupReport.Files[fileExtension][directory] = []fileStatus{file}
			} else {
				groupReport.Files[fileExtension][directory] = append(groupReport.Files[fileExtension][directory], file)
			}
		}
	case groupOutput[0] == "pass-fail" && groupOutput[1] == "directory":
		directoryPath := strings.Split(r.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[status] = make(map[string][]fileStatus)
			groupReport.Files[status][directory] = []fileStatus{file}
		} else {
			if groupReport.Files[status] == nil {
				groupReport.Files[status] = make(map[string][]fileStatus)
				groupReport.Files[status][directory] = []fileStatus{file}
			} else {
				groupReport.Files[status][directory] = append(groupReport.Files[status][directory], file)
			}
		}
	case groupOutput[0] == "pass-fail" && groupOutput[1] == "filetype":
		fileExtension := strings.Split(r.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[status] = make(map[string][]fileStatus)
			groupReport.Files[status][fileExtension] = []fileStatus{file}
		} else {
			if groupReport.Files[status] == nil {
				groupReport.Files[status] = make(map[string][]fileStatus)
				groupReport.Files[status][fileExtension] = []fileStatus{file}
			} else {
				groupReport.Files[status][fileExtension] = append(groupReport.Files[status][fileExtension], file)
			}
		}
	case groupOutput[0] == "directory" && groupOutput[1] == "filetype":
		fileExtension := strings.Split(r.FileName, ".")[1]
		if fileExtension == "yml" {
			fileExtension = "yaml"
		}
		directoryPath := strings.Split(r.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[directory] = make(map[string][]fileStatus)
			groupReport.Files[directory][fileExtension] = []fileStatus{file}
		} else {
			if groupReport.Files[directory] == nil {
				groupReport.Files[directory] = make(map[string][]fileStatus)
				groupReport.Files[directory][fileExtension] = []fileStatus{file}
			} else {
				groupReport.Files[directory][fileExtension] = append(groupReport.Files[directory][fileExtension], file)
			}
		}
	case groupOutput[0] == "directory" && groupOutput[1] == "pass-fail":
		directoryPath := strings.Split(r.FilePath, "/")
		directory := strings.Join(directoryPath[:len(directoryPath)-1], "/")
		directory = directory + "/"
		if groupReport.Files == nil {
			groupReport.Files = make(map[string]map[string][]fileStatus)
			groupReport.Files[directory] = make(map[string][]fileStatus)
			groupReport.Files[directory][status] = []fileStatus{file}
		} else {
			if groupReport.Files[directory] == nil {
				groupReport.Files[directory] = make(map[string][]fileStatus)
				groupReport.Files[directory][status] = []fileStatus{file}
			} else {
				groupReport.Files[directory][status] = append(groupReport.Files[directory][status], file)
			}
		}
	default:
		return doubleGroupReportJSON{}, fmt.Errorf("Invalid group output: %s", groupOutput)
	}

	return *groupReport, nil

}
