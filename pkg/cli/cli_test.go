package cli

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/Boeing/config-file-validator/v3/internal/testhelper"
	"github.com/Boeing/config-file-validator/v3/pkg/filetype"
	"github.com/Boeing/config-file-validator/v3/pkg/finder"
	"github.com/Boeing/config-file-validator/v3/pkg/formatter"
	"github.com/Boeing/config-file-validator/v3/pkg/reporter"
	"github.com/Boeing/config-file-validator/v3/pkg/schemastore"
	"github.com/Boeing/config-file-validator/v3/pkg/validator"
)

type captureReporter struct {
	reports []reporter.Report
}

func (r *captureReporter) Print(reports []reporter.Report) error {
	r.reports = reports
	return nil
}

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

func Test_CLIWithBrokenSymlink(t *testing.T) {
	dir := testhelper.CreateFixtureDir(t, "json")

	target := filepath.Join(dir, "broken.json")
	err := os.Symlink("/nonexistent_target_xyz", target)
	require.NoError(t, err)

	rep := &captureReporter{}
	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(dir),
	)
	cli := Init(WithFinder(fsFinder), WithReporters(rep))
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)

	var failed int
	var issueType reporter.IssueType
	for _, r := range rep.reports {
		if r.HasErrors() {
			failed++
			if len(r.Issues) > 0 {
				issueType = r.Issues[0].Type
			}
		}
	}
	require.Equal(t, 1, failed)
	require.Equal(t, reporter.IssueTypeSyntax, issueType)
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

func Test_CLIQuadGroupJSON(t *testing.T) {
	file := testhelper.CreateFixtureFile(t, "json")

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(file),
	)
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(reporter.NewJSONReporter("")),
		WithGroupOutput([]string{"filetype", "directory", "pass-fail", "error-type"}),
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

func Test_CLISchemaMapUnsupportedValidatorWarnsAndPasses(t *testing.T) {
	dir := t.TempDir()
	envFile := testhelper.WriteFile(t, dir, ".env", "KEY=VALUE\n")
	schema := testhelper.WriteFile(t, dir, "schema.json", `{"type": "object"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(envFile),
	)
	capturingReporter := &captureReporter{}
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(capturingReporter),
		WithSchemaMap(map[string]string{".env": schema}),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
	require.Len(t, capturingReporter.reports, 1)
	require.Equal(t, reporter.StatusPass, capturingReporter.reports[0].Status)
	require.Len(t, capturingReporter.reports[0].Notes, 1)
	require.Contains(t, capturingReporter.reports[0].Notes[0], "--schema-map matched this file")
	require.Contains(t, capturingReporter.reports[0].Notes[0], "does not support schema validation")
}

func Test_CLISchemaMapUnsupportedValidatorFailsWithRequireSchema(t *testing.T) {
	dir := t.TempDir()
	envFile := testhelper.WriteFile(t, dir, ".env", "KEY=VALUE\n")
	schema := testhelper.WriteFile(t, dir, "schema.json", `{"type": "object"}`)

	fsFinder := finder.FileSystemFinderInit(
		finder.WithPathRoots(envFile),
	)
	capturingReporter := &captureReporter{}
	cli := Init(
		WithFinder(fsFinder),
		WithReporters(capturingReporter),
		WithSchemaMap(map[string]string{".env": schema}),
		WithRequireSchema(true),
	)
	exitStatus, err := cli.Run()
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
	require.Len(t, capturingReporter.reports, 1)
	require.Equal(t, reporter.StatusFail, capturingReporter.reports[0].Status)
	require.Empty(t, capturingReporter.reports[0].Notes)
	require.Equal(t, reporter.IssueTypeSchema, capturingReporter.reports[0].Issues[0].Type)
	require.Contains(t, capturingReporter.reports[0].Issues[0].Message, "--schema-map matched this file")
	require.Contains(t, capturingReporter.reports[0].Issues[0].Message, "does not support schema validation")
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

// =============================================================================
// Format tests
// =============================================================================

func Test_FormatCleanFiles(t *testing.T) {
	dir := t.TempDir()
	// Already-formatted JSON (2-space, sorted keys)
	testhelper.WriteFile(t, dir, "clean.json", "{\n  \"a\": 1,\n  \"b\": 2\n}\n")

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			SortKeys:     true,
		}
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
	require.Len(t, rep.reports, 1)
	require.Equal(t, reporter.StatusPass, rep.reports[0].Status)
}

func Test_FormatUnformattedFile(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "messy.json", `{"b":2,"a":1}`)

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			SortKeys:     true,
		}
	})
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
	require.Len(t, rep.reports, 1)
	require.Equal(t, reporter.StatusUnformatted, rep.reports[0].Status)
}

func Test_FormatWithFix(t *testing.T) {
	dir := t.TempDir()
	path := testhelper.WriteFile(t, dir, "messy.json", `{"b":2,"a":1}`)

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep), WithFix(true))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			SortKeys:     true,
		}
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)

	// Verify file was rewritten
	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Contains(t, string(content), "\"a\": 1")
	require.Contains(t, string(content), "\"b\": 2")
}

func Test_FormatWithDiff(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "messy.json", `{"b":2,"a":1}`)

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep), WithDiff(true))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			SortKeys:     true,
		}
	})
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus)
	// In diff mode, reports are quiet (diff goes to stdout directly)
	for _, r := range rep.reports {
		require.True(t, r.IsQuiet)
	}
}

func Test_FormatSkipsUnparseableFiles(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "bad.json", `{"not valid json`)
	testhelper.WriteFile(t, dir, "good.json", "{\n  \"a\": 1\n}\n")

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{
			IndentStyle:  formatter.IndentSpaces,
			IndentWidth:  2,
			FinalNewline: true,
			SortKeys:     true,
		}
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
	// Only the parseable file should be in reports
	require.Len(t, rep.reports, 1)
	require.Contains(t, rep.reports[0].FilePath, "good.json")
}

func Test_FormatBrokenSymlink(t *testing.T) {
	dir := t.TempDir()
	testhelper.WriteFile(t, dir, "good.json", "{\n  \"a\": 1\n}\n")
	err := os.Symlink("/nonexistent_target_xyz", filepath.Join(dir, "broken.json"))
	require.NoError(t, err)

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
	})
	require.NoError(t, err)
	// Broken symlink should be reported as a failure
	var failCount int
	for _, r := range rep.reports {
		if r.Status == reporter.StatusFail {
			failCount++
		}
	}
	require.Equal(t, 1, failCount)
	require.Equal(t, 1, exitStatus)
}

func Test_FormatNoFormatterRegistered(t *testing.T) {
	dir := t.TempDir()
	// .toml has no formatter registered yet
	testhelper.WriteFile(t, dir, "config.toml", "key = \"value\"\n")

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
	})
	require.NoError(t, err)
	require.Equal(t, 0, exitStatus)
	// No reports — file was skipped (no formatter)
	require.Empty(t, rep.reports)
}

func Test_FormatYAMLDefaults(t *testing.T) {
	dir := t.TempDir()
	// YAML with 4-space indent — should get normalized to 2 (YAML default)
	testhelper.WriteFile(t, dir, "config.yaml", "name: app\nserver:\n    host: localhost\n")

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(dir))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep))

	exitStatus, err := cli.Format(func(formatName string) formatter.Options {
		opts := formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true}
		if formatName == "json" {
			opts.SortKeys = true
		}
		return opts
	})
	require.NoError(t, err)
	require.Equal(t, 1, exitStatus) // unformatted (4sp → 2sp)

	// Verify the report mentions the YAML file
	require.Len(t, rep.reports, 1)
	require.Contains(t, rep.reports[0].FilePath, "config.yaml")
	require.Equal(t, reporter.StatusUnformatted, rep.reports[0].Status)
}

func Test_FormatFixUnwritableDirectory(t *testing.T) {
	dir := t.TempDir()
	path := testhelper.WriteFile(t, dir, "messy.json", `{"b":2,"a":1}`)

	// Make directory unwritable — can't create temp file
	require.NoError(t, os.Chmod(dir, 0555))
	defer func() { _ = os.Chmod(dir, 0755) }()

	fsFinder := finder.FileSystemFinderInit(finder.WithPathRoots(path))
	rep := &captureReporter{}
	cli := Init(WithFinder(fsFinder), WithReporters(rep), WithFix(true))

	exitStatus, err := cli.Format(func(_ string) formatter.Options {
		return formatter.Options{IndentStyle: formatter.IndentSpaces, IndentWidth: 2, FinalNewline: true, SortKeys: true}
	})
	require.NoError(t, err)
	// Should report as failed (can't write), not crash
	require.Equal(t, 1, exitStatus)
	require.Len(t, rep.reports, 1)
	require.Equal(t, reporter.StatusFail, rep.reports[0].Status)
	require.NotEmpty(t, rep.reports[0].Issues)
	require.Contains(t, rep.reports[0].Issues[0].Message, "failed to write")
}

// =============================================================================
// writeFileAtomic unit tests with mock filesystem
// =============================================================================

type mockFile struct {
	name     string
	writeErr error
	closeErr error
	written  []byte
}

func (f *mockFile) Name() string { return f.name }
func (f *mockFile) Write(b []byte) (int, error) {
	if f.writeErr != nil {
		return 0, f.writeErr
	}
	f.written = append(f.written, b...)
	return len(b), nil
}
func (f *mockFile) Close() error { return f.closeErr }

type mockFS struct {
	statInfo  fs.FileInfo
	statErr   error
	file      *mockFile
	createErr error
	chmodErr  error
	renameErr error
	removed   []string
}

func (m *mockFS) Stat(_ string) (fs.FileInfo, error) {
	return m.statInfo, m.statErr
}
func (m *mockFS) CreateTemp(_, _ string) (File, error) {
	if m.createErr != nil {
		return nil, m.createErr
	}
	return m.file, nil
}
func (m *mockFS) Chmod(_ string, _ fs.FileMode) error { return m.chmodErr }
func (m *mockFS) Rename(_, _ string) error            { return m.renameErr }
func (m *mockFS) Remove(path string) error {
	m.removed = append(m.removed, path)
	return nil
}

func Test_writeFileAtomicSuccess(t *testing.T) {
	mf := &mockFile{name: "/tmp/test/.cfv-fmt-123"}
	mfs := &mockFS{file: mf, statErr: os.ErrNotExist}

	err := writeFileAtomicWith(mfs, "/tmp/test/config.json", []byte("data"))
	require.NoError(t, err)
	require.Equal(t, []byte("data"), mf.written)
	require.Empty(t, mfs.removed) // no cleanup needed on success
}

func Test_writeFileAtomicCreateTempFails(t *testing.T) {
	mfs := &mockFS{createErr: errors.New("permission denied"), statErr: os.ErrNotExist}

	err := writeFileAtomicWith(mfs, "/tmp/test/config.json", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "creating temp file")
}

func Test_writeFileAtomicWriteFails(t *testing.T) {
	mf := &mockFile{name: "/tmp/.cfv-fmt-456", writeErr: errors.New("disk full")}
	mfs := &mockFS{file: mf, statErr: os.ErrNotExist}

	err := writeFileAtomicWith(mfs, "/tmp/config.json", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "writing temp file")
	// Temp file should be cleaned up
	require.Contains(t, mfs.removed, "/tmp/.cfv-fmt-456")
}

func Test_writeFileAtomicCloseFails(t *testing.T) {
	mf := &mockFile{name: "/tmp/.cfv-fmt-789", closeErr: errors.New("io error")}
	mfs := &mockFS{file: mf, statErr: os.ErrNotExist}

	err := writeFileAtomicWith(mfs, "/tmp/config.json", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "closing temp file")
	require.Contains(t, mfs.removed, "/tmp/.cfv-fmt-789")
}

func Test_writeFileAtomicChmodFails(t *testing.T) {
	mf := &mockFile{name: "/tmp/.cfv-fmt-abc"}
	mfs := &mockFS{file: mf, statErr: os.ErrNotExist, chmodErr: errors.New("not supported")}

	err := writeFileAtomicWith(mfs, "/tmp/config.json", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "setting permissions")
	require.Contains(t, mfs.removed, "/tmp/.cfv-fmt-abc")
}

func Test_writeFileAtomicRenameFails(t *testing.T) {
	mf := &mockFile{name: "/tmp/.cfv-fmt-def"}
	mfs := &mockFS{file: mf, statErr: os.ErrNotExist, renameErr: errors.New("cross-device link")}

	err := writeFileAtomicWith(mfs, "/tmp/config.json", []byte("data"))
	require.Error(t, err)
	require.Contains(t, err.Error(), "renaming temp file")
	require.Contains(t, mfs.removed, "/tmp/.cfv-fmt-def")
}

func Test_toSchemaURL(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name, input, wantPrefix string
	}{
		{"https URL passthrough", "https://example.com/schema.json", "https://"},
		{"http URL passthrough", "http://example.com/schema.json", "http://"},
		{"relative path becomes file URL", "schemas/my.json", "file://"},
		{"absolute path becomes file URL", "/tmp/schema.json", "file:///tmp/schema.json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got, err := toSchemaURL(tc.input)
			require.NoError(t, err)
			require.True(t, strings.HasPrefix(got, tc.wantPrefix),
				"expected %q to start with %q", got, tc.wantPrefix)
		})
	}
}
