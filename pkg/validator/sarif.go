package validator

import (
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/xeipuuv/gojsonschema"
)

type SarifValidator struct{}

// Validate implements the Validator interface by attempting to
// unmarshal a byte array of json, performing additional validation checks
// in order to determine whether supplied SARIF report matches the SARIF schema.
func (SarifValidator) Validate(b []byte) (bool, error) {
	var report map[string]any
	err := json.Unmarshal(b, &report)
	if err != nil {
		customErr := getCustomErr(b, err)
		return false, customErr
	}

	schemaURL, ok := report["$schema"]
	if !ok {
		return false, errors.New("no schema found")
	}

	if _, ok := schemaURL.(string); !ok {
		return false, errors.New("schema isn't a string")
	}

	// Check if the program is executed in an air-gapped environment.
	err = attemptToResolveAndConnect(schemaURL.(string))
	if err != nil {
		return false, fmt.Errorf("%w", err)
	}

	loadedSchema := gojsonschema.NewReferenceLoader(schemaURL.(string))
	loadedReport := gojsonschema.NewRawLoader(report)

	schema, err := gojsonschema.NewSchema(loadedSchema)
	if err != nil {
		return false, fmt.Errorf("schema isn't valid: %s", schemaURL)
	}
	result, _ := schema.Validate(loadedReport)
	if !result.Valid() {
		return false, formatError(result.Errors())
	}
	return true, nil
}

// formatError function concatenates errors found during schema validation
// into a single formatted error message.
func formatError(resultErrors []gojsonschema.ResultError) error {
	var errorDescription []string
	for _, err := range resultErrors {
		errorDescription = append(errorDescription, err.Description())
	}
	return fmt.Errorf("%s", strings.Join(errorDescription, ", "))
}

// attemptToResolveAndConnect function is used to validate and attempt
// to establish a TCP connection to a host specified by a schema URL.
func attemptToResolveAndConnect(schema string) error {
	u, err := url.ParseRequestURI(schema)
	if err != nil {
		return fmt.Errorf("schema URL isn't valid: %s", schema)
	}
	ips, err := net.LookupIP(u.Host)
	if err != nil {
		return fmt.Errorf("couldn't resolve host: %s", u.Host)
	}
	for _, ip := range ips {
		addr := net.JoinHostPort(ip.String(), u.Scheme)
		con, err := net.DialTimeout("tcp", addr, time.Second*3)
		if err == nil {
			err = con.Close()
			if err != nil {
				return fmt.Errorf("connection was established, but couldn't be closed")
			}
			return nil
		}
	}
	return fmt.Errorf("couldn't establish tcp connection with the host: %s", u.Host)
}
