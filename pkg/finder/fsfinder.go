package finder

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/tools"
)

type FileSystemFinder struct {
	PathRoots        []string
	FileTypes        []filetype.FileType
	ExcludeDirs      map[string]struct{}
	ExcludeFileTypes map[string]struct{}
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
		fsf.ExcludeDirs = tools.ArrToMap(excludeDirs...)
	}
}

// WithExcludeFileTypes adds excluded file types to FSFinder.
func WithExcludeFileTypes(types []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeFileTypes = tools.ArrToMap(types...)
	}
}

// WithDepth adds the depth for search recursion to FSFinder
func WithDepth(depthVal int) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.Depth = &depthVal
	}
}

func FileSystemFinderInit(opts ...FSFinderOptions) *FileSystemFinder {
	defaultExcludeDirs := make(map[string]struct{})
	defaultExcludeFileTypes := make(map[string]struct{})
	defaultPathRoots := []string{"."}

	fsfinder := &FileSystemFinder{
		PathRoots:        defaultPathRoots,
		FileTypes:        filetype.FileTypes,
		ExcludeDirs:      defaultExcludeDirs,
		ExcludeFileTypes: defaultExcludeFileTypes,
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
		// remove all leading and trailing whitespace
		trimmedPathRoot := strings.TrimSpace(pathRoot)
		matches, err := fsf.findOne(trimmedPathRoot, seen)
		if err != nil {
			return nil, err
		}
		uniqueMatches = append(uniqueMatches, matches...)
	}
	return uniqueMatches, nil
}

// findOne recursively walks through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array for a
// single path and returns the file metadata.
func (fsf FileSystemFinder) findOne(pathRoot string, seenMap map[string]struct{}) ([]FileMetadata, error) {
	var matchingFiles []FileMetadata

	// check that the path exists before walking it or the error returned
	// from filepath.Walk will be very confusing and undescriptive
	if _, err := os.Stat(pathRoot); os.IsNotExist(err) {
		return nil, err
	}

	var depth int
	if fsf.Depth != nil {
		depth = *fsf.Depth
	}

	maxDepth := strings.Count(pathRoot, string(os.PathSeparator)) + depth

	err := filepath.WalkDir(pathRoot,
		func(path string, dirEntry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}

			// determine if directory is in the excludeDirs list or if the depth is greater than the maxDepth
			if dirEntry.IsDir() {
				_, isExcluded := fsf.ExcludeDirs[dirEntry.Name()]
				if isExcluded || (fsf.Depth != nil && strings.Count(path, string(os.PathSeparator)) > maxDepth) {
					return filepath.SkipDir
				}
			}

			if !dirEntry.IsDir() {
				// filepath.Ext() returns the extension name with a dot so it
				// needs to be removed.

				walkFileName := filepath.Base(path)
				walkFileExtension := strings.TrimPrefix(filepath.Ext(path), ".")
				extensionLowerCase := strings.ToLower(walkFileExtension)

				if _, isExcluded := fsf.ExcludeFileTypes[extensionLowerCase]; isExcluded {
					return nil
				}

				for _, fileType := range fsf.FileTypes {
					_, isKnownFile := fileType.KnownFiles[walkFileName]
					_, hasExtension := fileType.Extensions[extensionLowerCase]

					if !isKnownFile && !hasExtension {
						continue
					}

					absPath, err := filepath.Abs(path)
					if err != nil {
						return err
					}

					if _, seen := seenMap[absPath]; !seen {
						fileMetadata := FileMetadata{dirEntry.Name(), absPath, fileType}
						matchingFiles = append(matchingFiles, fileMetadata)
						seenMap[absPath] = struct{}{}
					}

					return nil
				}
				fsf.ExcludeFileTypes[extensionLowerCase] = struct{}{}
			}

			return nil
		})
	if err != nil {
		return nil, err
	}

	return matchingFiles, nil
}
