package validator

import (
	"context"
	"encoding/xml"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/lestrrat-go/helium"
	"github.com/lestrrat-go/helium/xsd"
)

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
		return false, err
	}
	return true, nil
}

func (XMLValidator) ValidateSchema(b []byte, filePath string) (bool, error) {
	schemaLoc := extractXSDLocation(b)
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

func extractXSDLocation(b []byte) string {
	decoder := xml.NewDecoder(strings.NewReader(string(b)))
	for {
		tok, err := decoder.Token()
		if err != nil {
			return ""
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		for _, attr := range start.Attr {
			if attr.Name.Local == "noNamespaceSchemaLocation" &&
				attr.Name.Space == "http://www.w3.org/2001/XMLSchema-instance" { //nolint:revive // XSI namespace is a fixed URI
				return strings.TrimSpace(attr.Value)
			}
		}
		return ""
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
