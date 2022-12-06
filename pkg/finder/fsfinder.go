package finder

import (
	"git.web.boeing.com/vle-oss/config-file-validator/pkg/filetype"
	"io/fs"
	"os"
	"path/filepath"
)

type FileSystemFinder struct{}

// Find implements the FileFinder interface by recursively
// walking through all subdirectories (excluding the excluded subdirectories)
// and identifying if the file matches a type defined in the fileTypes array.
func (fsf FileSystemFinder) Find(pathRoot string, fileTypes []filetype.FileType, excludeDirs []string) ([]FileMetadata, error) {
	var matchingFiles []FileMetadata

	// check that the path exists before walking it or the error returned
	// from filepath.Walk will be very confusing and undescriptive
	if _, err := os.Stat(pathRoot); os.IsNotExist(err) {
		return nil, err
	}

	err := filepath.Walk(pathRoot, func(path string, fileInfo fs.FileInfo, err error) error {
		// determine if directory is in the excludeDirs list
		for _, dir := range excludeDirs {
			if fileInfo.IsDir() && fileInfo.Name() == dir {
				//log.Info("Skipping directory")
				err := filepath.SkipDir
				if err != nil {
					return err
				}
			}
		}

		if !fileInfo.IsDir() {
			walkFileExtension := filepath.Ext(path)

			for _, fileType := range fileTypes {
				for _, extension := range fileType.Extensions {
					// filepath.Ext() returns the extension name with a dot
					// so it needs to be prepended to the FileType extension
					// in order to match
					if ("." + extension) == walkFileExtension {
						fileMetadata := FileMetadata{fileInfo.Name(), path, fileType}
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
