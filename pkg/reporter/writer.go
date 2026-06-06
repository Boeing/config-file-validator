package reporter

import (
	"fmt"
	"os"
)

// outputBytesToFile outputs the named file at the destination specified by outputDest.
// if an existing directory is provided to outputDest param, creates a file named with defaultName given at the directory.
// if outputDest specifies a path to the file, creates the file named with outputDest.
// when empty string is given to outputDest param, it returns error.
func outputBytesToFile(outputDest, defaultName, extension string, bytes []byte) error {
	var fileName string
	info, err := os.Stat(outputDest)
	if outputDest == "" {
		return fmt.Errorf("outputDest is an empty string: %w", err)
	} else if !os.IsNotExist(err) && info.IsDir() {
		if extension != "" {
			extension = "." + extension
		}
		fileName = outputDest + "/" + defaultName + extension
	} else {
		fileName = outputDest
	}

	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create a file: %w", err)
	}
	defer file.Close()
	_, err = file.Write(bytes)
	if err != nil {
		return fmt.Errorf("failed to output bytes to a file: %w", err)
	}
	return nil
}
