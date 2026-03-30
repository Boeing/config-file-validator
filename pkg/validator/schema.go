package validator

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/xeipuuv/gojsonschema"
)

func jsonSchemaValidate(schemaURL string, docJSON []byte) (bool, error) {
	schemaLoader := gojsonschema.NewReferenceLoader(schemaURL)
	documentLoader := gojsonschema.NewBytesLoader(docJSON)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return false, fmt.Errorf("schema validation error: %w", err)
	}

	if !result.Valid() {
		var errs []string
		for _, desc := range result.Errors() {
			errs = append(errs, desc.String())
		}
		return false, fmt.Errorf("schema validation failed: %s", strings.Join(errs, "; "))
	}

	return true, nil
}

func resolveSchemaURL(schemaURL, filePath string) string {
	parsed, err := url.Parse(schemaURL)
	if err == nil && parsed.Scheme != "" {
		return schemaURL
	}

	if filepath.IsAbs(schemaURL) {
		return "file://" + schemaURL
	}

	dir := filepath.Dir(filePath)
	absSchema, err := filepath.Abs(filepath.Join(dir, schemaURL))
	if err != nil {
		return schemaURL
	}
	return "file://" + absSchema
}
