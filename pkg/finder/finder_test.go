package finder

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/tools"
	"github.com/Boeing/config-file-validator/pkg/validator"
)

func Test_fsFinder(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures"),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Error("Unable to find files")
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_fsFinderExcludeDirs(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures"),
		WithExcludeDirs([]string{"subdir"}),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Error("Unable to find files")
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_fsFinderExcludeFileTypes(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures/exclude-file-types"),
		WithExcludeFileTypes([]string{"json"}),
	)

	files, err := fsFinder.Find()

	if len(files) != 1 {
		fmt.Println(files)
		t.Errorf("Wrong amount of files, expected 1 got %d", len(files))
	}

	if err != nil {
		t.Error("Unable to find files")
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
			expectedFilesCount: 2,
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
			WithPathRoots(tt.inputPathRoot),
			WithDepth(tt.inputDepth),
		)

		files, err := fsFinder.Find()

		if len(files) != tt.expectedFilesCount {
			t.Errorf("Wrong amount of files, expected %d got %d", tt.expectedFilesCount, len(files))
		}

		if err != nil {
			t.Error("Unable to find files")
		}
	}
}

func Test_fsFinderCustomTypes(t *testing.T) {
	jsonFileType := filetype.FileType{
		Name:       "json",
		Extensions: tools.ArrToMap("json"),
		Validator:  validator.JSONValidator{},
	}
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures"),
		WithExcludeDirs([]string{"subdir"}),
		WithFileTypes([]filetype.FileType{jsonFileType}),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Error("Unable to find files")
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_fsFinderPathNoExist(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("/bad/path"),
	)

	_, err := fsFinder.Find()

	if err == nil {
		t.Error("Error not returned")
	}
}

func Test_FileSystemFinderMultipleFinder(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots(
			"../../test/fixtures/subdir/good.json",
			"../../test/fixtures/good.json",
			"./",
		),
	)

	files, err := fsFinder.Find()

	if len(files) != 2 {
		t.Errorf("No. files found don't match got:%v, want:%v", len(files), 2)
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_FileSystemFinderDuplicateFiles(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots(
			"../../test/fixtures/subdir/",
		),
	)

	files, err := fsFinder.Find()

	if len(files) != 4 {
		t.Errorf("No. files found don't match got:%v, want:%v", len(files), 4)
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_FileSystemFinderAbsPath(t *testing.T) {
	path := "../../test/fixtures/subdir/good.json"
	absPath, err := filepath.Abs(path)
	if err != nil {
		t.Fatal("Cannot form absolute path")
	}
	fsFinder := FileSystemFinderInit(
		WithPathRoots(path, absPath),
	)

	files, err := fsFinder.Find()

	if len(files) != 1 {
		t.Errorf("No. files found don't match got:%v, want:%v", len(files), 1)
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_FileSystemFinderUpperCaseExtension(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures/uppercase-extension"),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Error("Unable to find files")
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_FileSystemFinderMixedCaseExtension(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures/mixedcase-extension"),
	)

	files, err := fsFinder.Find()

	if len(files) < 1 {
		t.Error("Unable to find files")
	}

	if err != nil {
		t.Error("Unable to find files")
	}
}

func Test_FileFinderBadPath(t *testing.T) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots(
			"../../test/fixtures/subdir",
			"/bad/path",
		),
	)

	_, err := fsFinder.Find()

	if err == nil {
		t.Error("Error should be thrown for bad path")
	}
}

func Test_FileFinderPathWithWhitespaces(t *testing.T) {
	tests := []struct {
		name      string
		path      string
		expectErr bool
	}{
		{
			name: "no whitespace",
			path: "../../test/fixtures/subdir",
		},
		{
			name: "leading whitespace",
			path: "  ../../test/fixtures/subdir",
		},
		{
			name: "trailing whitespace",
			path: "../../test/fixtures/subdir  ",
		},
		{
			name: "leading and trailing whitespace",
			path: "  ../../test/fixtures/subdir  ",
		},
		{
			name:      "whitespace in middle of path",
			path:      "../../test/  fixtures  /subdir",
			expectErr: true,
		},
		{
			name:      "leading whitespace + whitespace in middle of path",
			path:      "  ../../test/  fixtures  /subdir",
			expectErr: true,
		},
		{
			name:      "trailing whitespace + whitespace in middle of path",
			path:      "../../test/  fixtures  /subdir  ",
			expectErr: true,
		},
		{
			name:      "leading and trailing whitespace + whitespace in middle of path",
			path:      "  ../../test/  fixtures  /subdir  ",
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			fsFinder := FileSystemFinderInit(
				WithPathRoots(tt.path),
			)

			files, err := fsFinder.Find()

			if tt.expectErr {
				if err == nil {
					t.Error("Error should be thrown for bad path")
				}
			} else {
				if len(files) < 1 {
					t.Error("Unable to find file")
				}

				if err != nil {
					t.Error("Unable to find file")
				}
			}
		})
	}
}

func Benchmark_Finder(b *testing.B) {
	fsFinder := FileSystemFinderInit(
		WithPathRoots("../../test/fixtures/"),
	)

	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		_, _ = fsFinder.Find()
	}
}
