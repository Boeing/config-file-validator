package finder

import (
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"testing"
)

func Test_fsFinder(t *testing.T) {
	fsFinder := FileSystemFinder{
		PathRoot: "../../test/fixtures",
		FileTypes: filetype.FileTypes,
		ExcludeDirs: nil,
	}
	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}

}

func Test_fsFinderExcludeDirs(t *testing.T) {
	fsFinder := FileSystemFinder{
		PathRoot: "../../test/fixtures",
		FileTypes: filetype.FileTypes,
		ExcludeDirs: []string{"subdir"},
	}
	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_fsFinderPathNoExist(t *testing.T) {
	fsFinder := FileSystemFinder{
		"/bad/path",
		filetype.FileTypes,
		nil,
	}
	_, err := fsFinder.Find()

	if err == nil {
		t.Errorf("Error not returned")
	}
}
