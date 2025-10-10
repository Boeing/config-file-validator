package finder

import (
    "testing"

    "github.com/Boeing/config-file-validator/pkg/filetype"
)

func Test_FileSystemFinderMakeAndJust(t *testing.T) {
    fsFinder := FileSystemFinderInit(
        WithPathRoots("../../test/fixtures"),
    )

    files, err := fsFinder.Find()

    if err != nil {
        t.Fatalf("Finder error: %v", err)
    }

    foundMake := false
    foundJust := false
    for _, f := range files {
        if f.FileType.Name == filetype.MakefileFileType.Name {
            foundMake = true
        }
        if f.FileType.Name == filetype.JustfileFileType.Name {
            foundJust = true
        }
    }

    if !foundMake {
        t.Error("Makefile not found by finder")
    }

    if !foundJust {
        t.Error("Justfile not found by finder")
    }
}
