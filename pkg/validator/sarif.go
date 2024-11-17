package validator

import (
	"encoding/json"
	"errors"
	"fmt"

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

	schemaUrl, ok := report["$schema"]
	if !ok {
		return false, errors.New("error - no schema specified")
	}

	loadedSchema := gojsonschema.NewReferenceLoader(schemaUrl.(string))
	loadedReport := gojsonschema.NewRawLoader(report)

	schema, err := gojsonschema.NewSchema(loadedSchema)
	if err != nil {
		return false, errors.New("error - schema isn't valid")
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

func formatError(resultEerrors []gojsonschema.ResultError) error {
	var errorDescription string
	for _, err := range resultEerrors {
		errorDescription = err.Description()
	}
	return fmt.Errorf("error - %s", errorDescription)
}
