package finder

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"gopkg.in/yaml.v3"
	"fmt"

	"github.com/Boeing/config-file-validator/pkg/filetype"
	"github.com/Boeing/config-file-validator/pkg/misc"
)

type FileSystemFinder struct {
	PathRoots        []string
	FileTypes        []filetype.FileType
	ExcludeDirs      map[string]struct{}
	ExcludeFileTypes map[string]struct{}
	Depth            *int
	UncheckedFiles   map[string]string
	ConfigFile       string
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
		fsf.ExcludeDirs = misc.ArrToMap(excludeDirs...)
	}
}

// WithExcludeFileTypes adds excluded file types to FSFinder.
func WithExcludeFileTypes(types []string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ExcludeFileTypes = misc.ArrToMap(types...)
	}
}

// WithDepth adds the depth for search recursion to FSFinder
func WithDepth(depthVal int) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.Depth = &depthVal
	}
}

// AddFiles adds unchecked files with specified formats to FSFinder (adds support for extensionless files)
func AddFiles(files []string, filesPresent bool) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		if !filesPresent {
			return
		}
		fsf.UncheckedFiles= make(map[string]string)
		for i := range files {
			kv := strings.Split(files[i], ":")
			// Creating <file name/dir>:<file type> mapping to link extensionless files with their actual format
     		path, format := kv[0], kv[1]
			fsf.UncheckedFiles[filepath.Clean(path)] = format
		}
	}
}

// StoreConfig stores config file path to read in data later
func StoreConfig(file string) FSFinderOptions {
	return func(fsf *FileSystemFinder) {
		fsf.ConfigFile = file
	}
}

// ReadConfig reads in a config file in yaml format.
// It converts the stored files and typings to mapped KVpairs,
// similar to AddFiles in function and purpose.
func ReadConfig(file string, fsf FileSystemFinder) (map[string]string, error) {
	if fsf.UncheckedFiles == nil {
		fsf.UncheckedFiles= make(map[string]string)
	}
	configFile, err := os.ReadFile(file)
	if err != nil {
		return nil, fmt.Errorf("Failed to read in config file using path %s", file)
	}

	// Create a map to store the unmarshaled data
	data := make(map[string][]interface{})

	// Unmarshal the YAML data into the map
	err = yaml.Unmarshal(configFile, &data)
	if err != nil {
		return nil, fmt.Errorf("Failed to read in config file data from %s", file)
	}

	// Storing the imported data as 'additional files' for findOne to access
	for key, value := range data {
		for _, val := range value {
			strVal, ok := val.(string)
			if !ok {
				continue
			}
			fsf.UncheckedFiles[filepath.Clean(strVal)] = key
		}
	}
	return fsf.UncheckedFiles, nil
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
		matches, err := fsf.findOne(pathRoot, seen)
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

	// Error checking of config file option
	if fsf.ConfigFile != "" {
		if _, err := os.Stat(fsf.ConfigFile); err != nil {
			return nil, err
		}
		var err error 
		if fsf.UncheckedFiles, err = ReadConfig(fsf.ConfigFile, fsf); err != nil {
			return nil, err
		}
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
				walkFileExtension := strings.TrimPrefix(filepath.Ext(path), ".")
				extensionLowerCase := strings.ToLower(walkFileExtension)

				// If a file is extensionless, check if its stored as an additional file and update the extension. Otherwise ignore
				if len(walkFileExtension) == 0 {
					// Check for relative file path match
					if ret, ok := fsf.UncheckedFiles[path]; ok { 
						walkFileExtension = ret
					} else if ret, ok := fsf.UncheckedFiles[dirEntry.Name()]; ok { // Checking for file name match
						walkFileExtension = ret
					}
				}

				// Checking for case sensitive exclusion
				if _, isExcluded := fsf.ExcludeFileTypes[extensionLowerCase]; isExcluded {
					return nil
				}
				extensionLowerCase := strings.ToLower(walkFileExtension)
				for _, fileType := range fsf.FileTypes {
					if _, isMatched := fileType.Extensions[extensionLowerCase]; isMatched {
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
