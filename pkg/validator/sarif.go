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

	err = attemptToResolveAndConnect(schemaURL.(string))
	if err != nil {
		return false, fmt.Errorf("error - %s", err)
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
		con, err := net.DialTimeout("tcp", addr, time.Millisecond*15)
		if err == nil {
			defer con.Close()
			return nil
		}
	}
	return fmt.Errorf("couldn't establish tcp connection with the host: %s", u.Host)
}
