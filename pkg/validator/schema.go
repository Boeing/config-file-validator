package validator

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/Boeing/config-file-validator/v3/pkg/tools"
)

// SourcePosition holds a 1-based line and column in the original source file.
type SourcePosition struct {
	Line   int
	Column int
}

// JSONSchemaValidate validates docJSON against the schema at schemaURL.
func JSONSchemaValidate(schemaURL string, docJSON []byte) (bool, error) {
	return validateJSONSchema(schemaURL, docJSON, nil)
}

// JSONSchemaValidateWithPositions validates docJSON against schemaURL and
// annotates errors with source positions from posMap. The map keys are
// context strings like "(root).name".
func JSONSchemaValidateWithPositions(schemaURL string, docJSON []byte, posMap map[string]SourcePosition) (bool, error) {
	return validateJSONSchema(schemaURL, docJSON, posMap)
}

// schemaHTTPTimeout is the timeout for fetching remote schemas over HTTP.
const schemaHTTPTimeout = 10 * time.Second

func validateJSONSchema(schemaURL string, docJSON []byte, posMap map[string]SourcePosition) (bool, error) {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(docJSON))
	if err != nil {
		return false, fmt.Errorf("schema validation error: %w", err)
	}

	c := jsonschema.NewCompiler()
	c.DefaultDraft(jsonschema.Draft7)
	c.AssertFormat()
	c.UseLoader(jsonschema.SchemeURLLoader{
		"file":  jsonschema.FileLoader{},
		"http":  &schemaHTTPLoader{client: &http.Client{Timeout: schemaHTTPTimeout}},
		"https": &schemaHTTPLoader{client: &http.Client{Timeout: schemaHTTPTimeout}},
	})

	sch, err := c.Compile(schemaURL)
	if err != nil {
		return false, fmt.Errorf("schema validation error: %w", err)
	}

	err = sch.Validate(doc)
	if err == nil {
		return true, nil
	}

	var verr *jsonschema.ValidationError
	if !errors.As(err, &verr) {
		return false, fmt.Errorf("schema validation error: %w", err)
	}

	basic := verr.BasicOutput()
	var errs []string
	var positions []SchemaErrorPosition
	for _, unit := range basic.Errors {
		if unit.Error == nil {
			continue
		}
		errs = append(errs, instanceLocationToContext(unit.InstanceLocation)+": "+unit.Error.String())
		var pos SchemaErrorPosition
		if posMap != nil {
			key := instanceLocationToContext(unit.InstanceLocation)
			if sp, found := posMap[key]; found {
				pos = SchemaErrorPosition(sp)
			}
		}
		positions = append(positions, pos)
	}
	if len(errs) == 0 {
		return false, fmt.Errorf("schema validation error: %w", err)
	}
	return false, &SchemaErrors{Prefix: "schema validation failed: ", Items: errs, Positions: positions}
}

// instanceLocationToContext converts a JSON pointer instance location string
// (e.g. "/servers/0/port") to the position map key format "(root).servers.0.port"
// used by buildYAMLPositionMap in yaml.go.
func instanceLocationToContext(loc string) string {
	if loc == "" {
		return "(root)"
	}
	return "(root)." + strings.ReplaceAll(strings.TrimPrefix(loc, "/"), "/", ".")
}

// schemaHTTPLoader implements jsonschema.URLLoader for HTTP/HTTPS schema URLs.
type schemaHTTPLoader struct {
	client *http.Client
}

func (l *schemaHTTPLoader) Load(schemaURL string) (any, error) {
	resp, err := l.client.Get(schemaURL) //nolint:noctx // jsonschema URLLoader interface does not accept context
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return jsonschema.UnmarshalJSON(bytes.NewReader(body))
}

func resolveSchemaURL(schemaURL, filePath string) string {
	if filepath.IsAbs(schemaURL) {
		return tools.FileURL(schemaURL)
	}

	parsed, err := url.Parse(schemaURL)
	if err == nil && parsed.Scheme != "" {
		return schemaURL
	}

	dir := filepath.Dir(filePath)
	absSchema, err := filepath.Abs(filepath.Join(dir, schemaURL))
	if err != nil {
		return schemaURL
	}
	return tools.FileURL(absSchema)
}
