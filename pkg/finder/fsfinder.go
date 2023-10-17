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
	PathRoots        []string
	FileTypes        []filetype.FileType
	ExcludeDirs      []string
	ExcludeFileTypes []string
	Depth            *int
}

type FSFinderOptions func(*FileSystemFinder)

// Set the CLI SearchPath
func WithPathRoots(paths ...string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.PathRoots = paths
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
	defaultPathRoots := []string{"."}

	fsfinder := &FileSystemFinder{
		PathRoots:   defaultPathRoots,
		FileTypes:   filetype.FileTypes,
		ExcludeDirs: defaultExludeDirs,
	}

	for _, opt := range opts {
		opt(fsfinder)
	}

	return fsfinder
}

// Find implements the FileFinder interface by calling findOne on
// all the PathRoots and providing the aggregated FileMetadata after
// ignoring all the duplicate files
func (fsf FileSystemFinder) Find() ([]FileMetadata, error) {
	seen := make(map[string]struct{}, 0)
	uniqueMatches := make([]FileMetadata, 0)
	for _, pathRoot := range fsf.PathRoots {
		matches, err := fsf.findOne(pathRoot)
		if err != nil {
			return nil, err
		}
		for _, match := range matches {
			absPath, err := filepath.Abs(match.Path)
			if err != nil {
				return nil, err
			}
			if _, ok := seen[absPath]; ok {
				continue
			}
			uniqueMatches = append(uniqueMatches, match)
			seen[absPath] = struct{}{}
		}
	}
	return uniqueMatches, nil
}

// findOne recursively walks through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array for a
// single path and returns the file metadata.
func (fsf FileSystemFinder) findOne(pathRoot string) ([]FileMetadata, error) {
	var matchingFiles []FileMetadata

	// check that the path exists before walking it or the error returned
	// from filepath.Walk will be very confusing and undescriptive
	if _, err := os.Stat(pathRoot); os.IsNotExist(err) {
		return nil, err
	}

	err := filepath.WalkDir(pathRoot,
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
