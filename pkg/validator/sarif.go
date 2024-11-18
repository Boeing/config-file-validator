package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

type SarifValidator struct{}

func (SarifValidator) Validate(b []byte) (bool, error) {
	var report map[string]interface{}
	err := json.Unmarshal(b, &report)
	if err != nil {
		customErr := getCustomErr(b, err)
		return false, customErr
	}

	schemaURL, ok := report["$schema"]
	if !ok {
		return false, errors.New("error - no schema found")
	}

	if _, ok := schemaURL.(string); !ok {
		return false, errors.New("error - schema isn't a string")
	}

	loadedSchema := gojsonschema.NewReferenceLoader(schemaURL.(string))
	loadedReport := gojsonschema.NewRawLoader(report)

	schema, err := gojsonschema.NewSchema(loadedSchema)
	if err != nil {
		return false, fmt.Errorf("error - schema isn't valid: %s", schemaURL)
	}
	result, err := schema.Validate(loadedReport)
	if err != nil {
		return false, errors.New("error - couldn't validate a report")
	}
	if !result.Valid() {
		return false, formatError(result.Errors())
	}
	return true, nil
}

func formatError(resultErrors []gojsonschema.ResultError) error {
	var errorDescription []string
	for _, err := range resultErrors {
		errorDescription = append(errorDescription, err.Description())
	}
	return fmt.Errorf("error - %s", strings.Join(errorDescription, ", "))
}
