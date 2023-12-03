package reporter

import (
	"fmt"
	"os"
)

func outputResultsToFile(outputDest, extension string, results []byte) error {
	var fileName string
	info, err := os.Stat(outputDest)
	if !os.IsNotExist(err) && info.IsDir() {
		fileName = outputDest + "/result." + extension
	} else {
		fileName = outputDest
	}
	file, err := os.Create(fileName)
	if err != nil {
		return fmt.Errorf("failed to create a file: %w", err)
	}
	defer file.Close()
	_, err = file.Write(results)
	if err != nil {
		return fmt.Errorf("failed to output results to a file: %w", err)
	}
	return nil
}
