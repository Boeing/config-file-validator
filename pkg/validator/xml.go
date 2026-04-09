package validator

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
)

var xmlLineColRe = regexp.MustCompile(`at line (\d+), column (\d+)`)

type XMLValidator struct{}

var _ Validator = XMLValidator{}

// ValidateXSD satisfies the XMLSchemaValidator marker interface.
func (XMLValidator) ValidateXSD(b []byte, schemaPath string) (bool, error) {
	return ValidateXSD(b, schemaPath)
}

func (XMLValidator) ValidateSyntax(b []byte) (bool, error) {
	ctx := context.Background()
	_, err := helium.NewParser().ValidateDTD(true).Parse(ctx, b)
	if err != nil {
		if m := xmlLineColRe.FindStringSubmatch(err.Error()); m != nil {
			line, _ := strconv.Atoi(m[1])
			col, _ := strconv.Atoi(m[2])
			return false, &ValidationError{Err: err, Line: line, Column: col}
		}
		return false, err
	}
	return true, nil
}

func (XMLValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	schemaLoc, err := extractXSDLocation(b)
	if err != nil {
		return false, err
	}
	if schemaLoc == "" {
		return true, ErrNoSchema
	}

	schemaPath := resolveXSDPath(schemaLoc, filePath)
	return ValidateXSD(b, schemaPath)
}

// ValidateXSD validates XML bytes against an XSD file at the given path.
// Exported for use by the CLI when applying external schemas.
func ValidateXSD(b []byte, schemaPath string) (bool, error) {
	ctx := context.Background()
	parser := helium.NewParser()

	schema, err := xsd.NewCompiler().CompileFile(ctx, schemaPath)
	if err != nil {
		return false, fmt.Errorf("schema compilation error: %w", err)
	}

	doc, err := parser.Parse(ctx, b)
	if err != nil {
		return false, fmt.Errorf("xml parse error: %w", err)
	}

	if err := xsd.NewValidator(schema).Validate(ctx, doc); err != nil {
		return false, fmt.Errorf("schema validation failed: %w", err)
	}

	return true, nil
}

const xsiNamespace = "http://www.w3.org/2001/XMLSchema-instance" //nolint:revive // XSI namespace is a fixed URI; DevSkim: ignore DS137138

func extractXSDLocation(b []byte) (string, error) {
	decoder := xml.NewDecoder(strings.NewReader(string(b)))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return "", nil
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local != "noNamespaceSchemaLocation" {
				continue
			}
			if attr.Name.Space == xsiNamespace {
				return strings.TrimSpace(attr.Value), nil
			}
			return "", fmt.Errorf(
				"noNamespaceSchemaLocation uses incorrect namespace %q, expected %q",
				attr.Name.Space, xsiNamespace,
			)
		}
		return "", nil
	}
}

func resolveXSDPath(schemaLoc, filePath string) string {
	if filepath.IsAbs(schemaLoc) {
		return schemaLoc
	}
	if _, err := os.Stat(schemaLoc); err == nil {
		return schemaLoc
	}
	dir := filepath.Dir(filePath)
	return filepath.Join(dir, schemaLoc)
}
