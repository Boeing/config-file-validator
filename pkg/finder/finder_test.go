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

func Test_compositeFileFinderMultipleFinder(t *testing.T) {
	f1 := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures/subdir/good.json"),
	)
	f2 := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures/good.json"),
	)
	compFinder := NewCompositeFileFinder([]FileFinder{f1, f2})

	files, err := compFinder.Find()

	if len(files) != 2 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_compositeFileFinderSingleFinder(t *testing.T) {
	f1 := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures/subdir"),
	)
	compFinder := NewCompositeFileFinder([]FileFinder{f1})

	files, err := compFinder.Find()

	if len(files) < 1 {
		t.Errorf("Unable to find files")
	}

	if err != nil {
		t.Errorf("Unable to find files")
	}
}

func Test_compositeFileFinderBadPath(t *testing.T) {
	f1 := FileSystemFinderInit(
		WithPathRoot("../../test/fixtures/subdir"),
	)
	f2 := FileSystemFinderInit(
		WithPathRoot("/bad/path"),
	)
	compFinder := NewCompositeFileFinder([]FileFinder{f1, f2})

	_, err := compFinder.Find()

	if err == nil {
		t.Errorf("Error should be thrown for bad path")
	}
}
