package finder

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"slices"

	"github.com/Boeing/config-file-validator/pkg/filetype"
)

type FileSystemFinder struct {
	PathRoot         string
	FileTypes        []filetype.FileType
	ExcludeDirs      []string
	ExcludeFileTypes []string
	Depth            *int
}

type FSFinderOptions func(*FileSystemFinder)

// Set the CLI SearchPath
func WithPathRoot(path string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.PathRoot = path
	}
}

// Add a custom list of file types to the FSFinder
func WithFileTypes(fileTypes []filetype.FileType) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.FileTypes = fileTypes
	}
}

// Add a custom list of file types to the FSFinder
func WithExcludeDirs(excludeDirs []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeDirs = excludeDirs
	}
}

// WithExcludeFileTypes adds excluded file types to FSFinder.
func WithExcludeFileTypes(types []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeFileTypes = types
	}
}

// WithDepth adds the depth for search recursion to FSFinder
func WithDepth(depthVal int) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.Depth = &depthVal
	}
}

func FileSystemFinderInit(opts ...FSFinderOptions) *FileSystemFinder {
	var defaultExludeDirs []string
	defaultPathRoot := "."

	fsfinder := &FileSystemFinder{
		PathRoot:    defaultPathRoot,
		FileTypes:   filetype.FileTypes,
		ExcludeDirs: defaultExludeDirs,
	}

	for _, opt := range opts {
		opt(fsfinder)
	}

	return fsfinder
}

// Find implements the FileFinder interface by recursively
// walking through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array.
func (fsf FileSystemFinder) Find() ([]FileMetadata, error) {
	var matchingFiles []FileMetadata

	// check that the path exists before walking it or the error returned
	// from filepath.Walk will be very confusing and undescriptive
	if _, err := os.Stat(fsf.PathRoot); os.IsNotExist(err) {
		return nil, err
	}

	err := filepath.WalkDir(fsf.PathRoot,
		func(path string, dirEntry fs.DirEntry, err error) error {
			// determine if directory is in the excludeDirs list
			if dirEntry.IsDir() && fsf.Depth != nil && strings.Count(path, string(os.PathSeparator)) > *fsf.Depth {
				// Skip processing the directory
				return fs.SkipDir // This is not reported as an error by filepath.WalkDir
			}

			for _, dir := range fsf.ExcludeDirs {
				if dirEntry.IsDir() && dirEntry.Name() == dir {
					err := filepath.SkipDir
					if err != nil {
						return err
					}
				}
			}

			if !dirEntry.IsDir() {
				// filepath.Ext() returns the extension name with a dot so it
				// needs to be removed.
				walkFileExtension := strings.TrimPrefix(filepath.Ext(path), ".")
				if slices.Contains[[]string](fsf.ExcludeFileTypes, walkFileExtension) {
					return nil
				}

				for _, fileType := range fsf.FileTypes {
					for _, extension := range fileType.Extensions {
						if extension == walkFileExtension {
							fileMetadata := FileMetadata{dirEntry.Name(), path, fileType}
							matchingFiles = append(matchingFiles, fileMetadata)
						}
					}
				}
			}

			return nil
		})

	if err != nil {
		return nil, err
	}

	return matchingFiles, nil
}
