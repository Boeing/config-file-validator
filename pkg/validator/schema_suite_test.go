package validator

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	jsonschema "github.com/santhosh-tekuri/jsonschema/v6"
)

// TestJSONSchemaTestSuite runs the official JSON Schema Test Suite against
// santhosh-tekuri/jsonschema/v6 to verify keyword-level correctness.

type suiteTestCase struct {
	Description string          `json:"description"`
	Data        json.RawMessage `json:"data"`
	Valid       bool            `json:"valid"`
}

func TestJSONSchemaTestSuite(t *testing.T) {
	suiteDir := "/tmp/json-schema-test-suite/tests"
	if _, err := os.Stat(suiteDir); err != nil {
		t.Skip("JSON Schema Test Suite not found at /tmp/json-schema-test-suite")
	}

	drafts := []struct {
		name string
		dir  string
		d    *jsonschema.Draft
	}{
		{"draft4", filepath.Join(suiteDir, "draft4"), jsonschema.Draft4},
		{"draft7", filepath.Join(suiteDir, "draft7"), jsonschema.Draft7},
	}

	for _, draft := range drafts {
		t.Run(draft.name, func(t *testing.T) {
			files, err := filepath.Glob(filepath.Join(draft.dir, "*.json"))
			if err != nil {
				t.Fatal(err)
			}

			for _, file := range files {
				keyword := filepath.Base(file)
				t.Run(keyword, func(t *testing.T) {
					data, err := os.ReadFile(file)
					if err != nil {
						t.Fatal(err)
					}

					var groups []struct {
						Description string          `json:"description"`
						Schema      json.RawMessage `json:"schema"`
						Tests       []suiteTestCase `json:"tests"`
					}
					if err := json.Unmarshal(data, &groups); err != nil {
						t.Fatal(err)
					}

					for _, group := range groups {
						t.Run(group.Description, func(t *testing.T) {
							// Compile schema
							var schemaDoc any
							if err := json.Unmarshal(group.Schema, &schemaDoc); err != nil {
								t.Fatalf("unmarshal schema: %v", err)
							}

							c := jsonschema.NewCompiler()
							c.DefaultDraft(draft.d)
							c.AssertFormat()
							if err := c.AddResource("test-schema.json", schemaDoc); err != nil {
								t.Fatalf("add resource: %v", err)
							}
							sch, err := c.Compile("test-schema.json")
							if err != nil {
								t.Fatalf("compile: %v", err)
							}

							for _, tc := range group.Tests {
								t.Run(tc.Description, func(t *testing.T) {
									var doc any
									if err := json.Unmarshal(tc.Data, &doc); err != nil {
										t.Fatalf("unmarshal data: %v", err)
									}

									err := sch.Validate(doc)
									got := err == nil

									if got != tc.Valid {
										t.Errorf("expected valid=%v, got valid=%v (err=%v)", tc.Valid, got, err)
									}
								})
							}
						})
					}
				})
			}
		})
	}
}
