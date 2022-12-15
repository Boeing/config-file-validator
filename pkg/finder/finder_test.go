package finder

import (
	"github.com/Boeing/config-file-validator/pkg/filetype"
	"testing"
)

func Test_fsFinder(t *testing.T) {
	fsFinder := FileSystemFinder{}
	files, err := fsFinder.Find(
		"../../test/fixtures",
		filetype.FileTypes,
		nil,
	)

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}

}

func Test_fsFinderExcludeDirs(t *testing.T) {
	fsFinder := FileSystemFinder{}
	files, err := fsFinder.Find(
		"../../test/fixtures",
		filetype.FileTypes,
		[]string{"subdir"},
	)

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_fsFinderPathNoExist(t *testing.T) {
	fsFinder := FileSystemFinder{}
	_, err := fsFinder.Find(
		"/bad/path",
		filetype.FileTypes,
		nil,
	)

	if err == nil {
		t.Errorf("Error not returned")
	}
}
