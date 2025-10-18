package formatter

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
)

type JSONFormatter struct{}

// Format implements the formatter interface by attempting to
// indent the JSON content of the file located at filePath
func (JSONFormatter) Format(filePath string) error {
	fileContent, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("unable to read file: %w", err)
	}
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("unable to get file info: %w", err)
	}
	// Extract the permission bits
	permissions := fileInfo.Mode().Perm()

	dst := bytes.NewBuffer(nil)
	if err := json.Indent(dst, fileContent, "", "  "); err != nil {
		return fmt.Errorf("unable to format file: %w", err)
	}
	err = os.WriteFile(filePath, dst.Bytes(), permissions)
	if err != nil {
		return fmt.Errorf("unable to write file: %w", err)
	}
	return nil
}
