package cli

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v2/internal/testhelper"
	"github.com/Boeing/config-file-validator/v2/pkg/filetype"
	"github.com/Boeing/config-file-validator/v2/pkg/finder"
	"github.com/Boeing/config-file-validator/v2/pkg/reporter"
	"github.com/Boeing/config-file-validator/v2/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v2/pkg/validator"
)

func Test_CLI(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml", "toml")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewStdoutReporter("")),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithMultipleReporters(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml")
	tmpOut := t.TempDir()

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(
			reporter.NewJSONReporter(tmpOut+"/result.json"),
			reporter.JunitReporter{},
		),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithFailedValidation(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "bad.json", testhelper.InvalidContent["json"])

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIBadPath(t *testing.T) {
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots("/bad/path"),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 2, exitStatus)
}

func Test_CLIWithGroup(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json", "yaml")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewStdoutReporter("")),
		WithGroupOutput([]string{"pass-fail", "directory"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIReportErr(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("./wrong/path")),
		WithGroupOutput([]string{""}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLISchemaAutoValidation(t *testing.T) {
	// SARIF files have built-in schema validation — should auto-validate
	file := testhelper.CreateFixtureFile(t, "sarif")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaAutoSkipsNoSchema(t *testing.T) {
	// JSON without $schema — should pass (syntax only)
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIRequireSchemaFailsNoSchema(t *testing.T) {
	// JSON without $schema + --require-schema should fail
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithRequireSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIRequireSchemaPassesWithSchema(t *testing.T) {
	// SARIF has built-in schema — should pass even with --require-schema
	file := testhelper.CreateFixtureFile(t, "sarif")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithRequireSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIRequireSchemaIgnoresNonSchemaTypes(t *testing.T) {
	// INI doesn't implement SchemaValidator — should pass even with --require-schema
	file := testhelper.CreateFixtureFile(t, "ini")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithRequireSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithQuiet(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithQuiet(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIWithUnreadableFile(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	err := os.Chmod(file, 0000)
	require.NoError(t, err)
	defer func() { _ = os.Chmod(file, 0600) }()

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.Error(t, err)
	require.Equal(t, 2, exitStatus)
}

func Test_CLISingleGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIDoubleGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype", "directory"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLITripleGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype", "directory", "pass-fail"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapValid(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "config.json", `{"host": "db", "port": 5432}`)
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"properties": {
			"host": {"type": "string"},
			"port": {"type": "integer"}
		},
		"additionalProperties": false
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.json": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapInvalid(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "config.json", `{"host": "db", "port": "bad"}`)
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"properties": {
			"host": {"type": "string"},
			"port": {"type": "integer"}
		}
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.json": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLISchemaMapGlob(t *testing.T) {
	dir := t.TempDir()
	sub := testhelper.CreateSubdir(t, dir, "configs")
	testhelper.WriteFile(t, sub, "db.json", `{"host": "db", "port": 5432}`)
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"properties": {
			"host": {"type": "string"},
			"port": {"type": "integer"}
		}
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(sub + "/db.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"**/configs/*.json": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapYAML(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "config.yaml", "host: db\nport: 5432\n")
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"properties": {
			"host": {"type": "string"},
			"port": {"type": "integer"}
		},
		"additionalProperties": false
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.yaml"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.yaml": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapTOML(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "config.toml", "host = \"db\"\nport = 5432\n")
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"properties": {
			"host": {"type": "string"},
			"port": {"type": "integer"}
		},
		"additionalProperties": false
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.toml"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.toml": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapUnmatched(t *testing.T) {
	// File doesn't match schema-map pattern — passes syntax-only
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"other.json": "/nonexistent"}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaStoreValid(t *testing.T) {
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, "package.json", `{"name": "app", "version": "1.0.0"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/package.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaStoreInvalid(t *testing.T) {
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, "package.json", `{"name": "app", "version": 123}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/package.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLISchemaStoreUnmatched(t *testing.T) {
	// File not in catalog — passes syntax-only
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, "random.json", `{"key": "value"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/random.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapPriorityOverStore(t *testing.T) {
	// schema-map should win over schemastore
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, "package.json", `{"name": "app", "version": "1.0.0"}`)
	strict := testhelper.WriteFile(t, dir, "strict.json", `{
		"type": "object",
		"required": ["id"],
		"properties": {"id": {"type": "integer"}},
		"additionalProperties": false
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/package.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"package.json": strict}),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus) // fails against strict schema
}

func Test_CLIDocumentSchemaPriorityOverAll(t *testing.T) {
	// Document $schema should win over schema-map and schemastore
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	ownSchema := testhelper.WriteFile(t, dir, "own.json", `{
		"type": "object",
		"properties": {"title": {"type": "string"}}
	}`)
	testhelper.WriteFile(t, dir, "package.json", `{"$schema": "`+ownSchema+`", "title": "hello"}`)
	strict := testhelper.WriteFile(t, dir, "strict.json", `{
		"type": "object",
		"required": ["id"],
		"additionalProperties": false
	}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/package.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"package.json": strict}),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus) // passes against own schema
}

func Test_CLIRequireSchemaWithSchemaStore(t *testing.T) {
	// Unmatched file + --require-schema + --schemastore should fail
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, "random.json", `{"key": "value"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/random.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
		WithRequireSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLINoSchema(t *testing.T) {
	// File with $schema should pass syntax-only when --no-schema is set
	dir := t.TempDir()
	schema := testhelper.WriteFile(t, dir, "schema.json", `{
		"type": "object",
		"required": ["id"],
		"additionalProperties": false
	}`)
	// This file would FAIL schema validation (missing "id", extra "name")
	testhelper.WriteFile(t, dir, "config.json", `{"$schema": "`+schema+`", "name": "test"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithNoSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus) // passes because schema is skipped
}

func Test_CLINoSchemaWithSchemaStore(t *testing.T) {
	// --no-schema skips schemastore matching too
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	// version:123 would fail against package.json schema
	testhelper.WriteFile(t, dir, "package.json", `{"name": "app", "version": 123}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/package.json"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
		WithNoSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus) // passes because schema is skipped
}

func Test_CLISchemaStoreExtensionlessValid(t *testing.T) {
	// .babelrc is a Linguist known file (jsonc) and has a SchemaStore entry
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, ".babelrc", `{"presets": ["env"]}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaStoreExtensionlessInvalid(t *testing.T) {
	// .babelrc with invalid content should fail schema validation
	dir := t.TempDir()
	bundle := setupMiniSchemaStore(t)
	testhelper.WriteFile(t, dir, ".babelrc", `{"presets": ["env"], "unknown_field": true}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaStore(bundle),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func setupMiniSchemaStore(t *testing.T) *schemastore.Store {
	t.Helper()
	dir := t.TempDir()

	catalogDir := dir + "/src/api/json"
	require.NoError(t, os.MkdirAll(catalogDir, 0755))
	schemaDir := dir + "/src/schemas/json"
	require.NoError(t, os.MkdirAll(schemaDir, 0755))

	catalog := `{"schemas":[
		{"name":"package.json","fileMatch":["package.json"],"url":"https://www.schemastore.org/package.json"},
		{"name":"babelrc","fileMatch":[".babelrc"],"url":"https://www.schemastore.org/babelrc.json"}
	]}`
	require.NoError(t, os.WriteFile(catalogDir+"/catalog.json", []byte(catalog), 0600))

	pkgSchema := `{"type":"object","properties":{"name":{"type":"string"},"version":{"type":"string"}}}`
	require.NoError(t, os.WriteFile(schemaDir+"/package.json", []byte(pkgSchema), 0600))

	babelSchema := `{"type":"object","properties":{"presets":{"type":"array","items":{"type":"string"}}},"additionalProperties":false}`
	require.NoError(t, os.WriteFile(schemaDir+"/babelrc.json", []byte(babelSchema), 0600))

	store, err := schemastore.Open(dir)
	require.NoError(t, err)
	return store
}

func Test_CLISchemaMapXMLValid(t *testing.T) {
	dir := t.TempDir()
	xsdContent := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="config">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="host" type="xs:string"/>
        <xs:element name="port" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	schemaPath := testhelper.WriteFile(t, dir, "schema.xsd", xsdContent)
	testhelper.WriteFile(t, dir, "config.xml", `<?xml version="1.0"?><config><host>db</host><port>5432</port></config>`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.xml"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.xml": schemaPath}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIStdinValid(t *testing.T) {
	cli := Init(
		WithStdinData([]byte(`{"key": "value"}`), filetype.JSONFileType),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLIStdinInvalid(t *testing.T) {
	cli := Init(
		WithStdinData([]byte(`{"key": value}`), filetype.JSONFileType),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIStdinYAML(t *testing.T) {
	cli := Init(
		WithStdinData([]byte("key: value\nlist:\n  - one\n"), filetype.YAMLFileType),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}

func Test_CLISchemaMapXMLInvalid(t *testing.T) {
	dir := t.TempDir()
	xsdContent := `<?xml version="1.0" encoding="UTF-8"?>
<xs:schema xmlns:xs="http://www.w3.org/2001/XMLSchema">
  <xs:element name="config">
    <xs:complexType>
      <xs:sequence>
        <xs:element name="host" type="xs:string"/>
        <xs:element name="port" type="xs:integer"/>
      </xs:sequence>
    </xs:complexType>
  </xs:element>
</xs:schema>`
	schemaPath := testhelper.WriteFile(t, dir, "schema.xsd", xsdContent)
	testhelper.WriteFile(t, dir, "config.xml", `<?xml version="1.0"?><config><host>db</host><port>bad</port></config>`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir + "/config.xml"),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithSchemaMap(map[string]string{"config.xml": schemaPath}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_SchemaErrorsMethod(t *testing.T) {
	t.Parallel()
	se := &validator.SchemaErrors{
		Prefix: "test: ",
		Items:  []string{"error1", "error2"},
	}
	require.Equal(t, []string{"error1", "error2"}, se.Errors())
	require.Equal(t, "test: error1; error2", se.Error())
}

func Test_CLINoJSONCNoteOnYAML(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "bad.yaml", "a: b\nc: d:::::::::::::::\n")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(WithFinder(fsFinder))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
}

func Test_CLIStdinWithQuiet(t *testing.T) {
	cli := Init(
		WithStdinData([]byte(`{"key": "value"}`), filetype.JSONFileType),
		WithQuiet(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
}
