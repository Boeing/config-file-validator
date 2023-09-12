package finder

import (
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/validator"
	"testing"
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
