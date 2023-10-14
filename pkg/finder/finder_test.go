package finder

import (
	"fmt"
	"testing"

	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

func Test_fsFinder(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures"),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}

}

func Test_fsFinderExcludeDirs(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures"),
		WithExcludeDirs([]string{"subdir"}),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_fsFinderExcludeFileTypes(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures/exclude-file-types"),
		WithExcludeFileTypes([]string{"json"}),
	)

	files, err := fsFinder.Find()

	if len(files) != 1 {
		fmt.Println(files)
		t.Errorf("Wrong amount of files, expected 1 got %d", len(files))
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_fsFinderWithDepth(t *testing.T) {

	type test struct {
		name               string
		inputDepth         int
		inputPathRoot      string
		expectedFilesCount int
	}

	tests := []test{
		{
			name:               "recursion disabled",
			inputDepth:         0,
			inputPathRoot:      "../",
			expectedFilesCount: 0,
		},
		{
			name:               "recursion enabled",
			inputDepth:         4,
			inputPathRoot:      "../../test/fixtures/with-depth",
			expectedFilesCount: 1,
		},
		{
			name:               "recursion enabled with lesser depth in the folder structure",
			inputDepth:         9,
			inputPathRoot:      "../../test/fixtures/with-depth",
			expectedFilesCount: 2,
		},
	}

	for _, tt := range tests {
		fsFinder := FileSystemFinderInit(
			WithPathRoot(tt.inputPathRoot),
			WithDepth(tt.inputDepth),
		)

		files, err := fsFinder.Find()

		if len(files) != tt.expectedFilesCount {
			t.Errorf("Wrong amount of files, expected %d got %d", tt.expectedFilesCount, len(files))
		}

		if err != nil {
			t.Errorf("Unable to find files")
		}
	}
}

func Test_fsFinderCustomTypes(t *testing.T) {
	jsonFileType := filetype.FileType{
		Name:       "json",
		Extensions: []string{"json"},
		Validator:  validator.JsonValidator{},
	}
	fsFinder := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures"),
		WithExcludeDirs([]string{"subdir"}),
		WithFileTypes([]filetype.FileType{jsonFileType}),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_fsFinderPathNoExist(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoot("/bad/path"),
	)

	_, err := fsFinder.Find()

	if err == nil {
		t.Errorf("Error not returned")
	}
}
