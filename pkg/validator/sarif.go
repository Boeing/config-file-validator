package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/owenrumney/go-sarif/sarif"
	"github.com/xeipuuv/gojsonschema"
	"io"
	"net/http"
)

type SarifValidator struct{}

func (SarifValidator) Validate(b []byte) (bool, error) {
	report, err := sarif.FromBytes(b)
	if err != nil {
		customErr := getCustomErr(b, err)
		return false, customErr
	}

	// TODO: check if schema URLs are valid here (e.g. it is either v2.0 or v2.1.0)
	if report.Schema == "" {
		return false, errors.New("error: no schema specified")
	}

	res, err := http.Get(report.Schema)
	if err != nil || res.StatusCode != 200 {
		return false, fmt.Errorf("error: invalid schema '%s' specified", report.Schema)
	}
	defer res.Body.Close()
	schemaData, err := io.ReadAll(res.Body)
	if err != nil {
		return false, err
	}
	schemaMap, sarifMap := make(map[string]interface{}), make(map[string]interface{})
	json.Unmarshal(schemaData, &schemaMap)
	json.Unmarshal(b, &sarifMap)

	schemaLoader, sarifLoader := gojsonschema.NewRawLoader(schemaMap), gojsonschema.NewRawLoader(sarifMap)

	result, err := gojsonschema.Validate(sarifLoader, schemaLoader)
	if err != nil {
		return false, err
	}
	if !result.Valid() {
		fmt.Println("not valid")
		for _, desc := range result.Errors() {
			fmt.Println("errors", desc)
		}
	}
	return true, nil
}
