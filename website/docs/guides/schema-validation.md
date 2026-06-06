---
---

# Schema Validation

The validator checks syntax first. If the file parses correctly and a schema is available, it then validates the file's content against that schema.

Schemas are resolved in order:

1. **From the file** — a `$schema` property in JSON/TOML, a `yaml-language-server` comment in YAML, or an `xsi:noNamespaceSchemaLocation` attribute in XML.
2. **From `--schema-map`** — a glob pattern mapped to a schema file.
3. **From `--schemastore`** — automatic lookup by filename against the SchemaStore catalog.

The first match wins. If no schema is found, the file passes on syntax alone.

## Declaring a schema in your files

Each format uses a different convention to reference a schema.

### JSON and JSONC

Add a `$schema` property at the top level:

```json
{
  "$schema": "https://json.schemastore.org/package.json",
  "name": "my-package",
  "version": "1.0.0"
}
```

### YAML

Add a `yaml-language-server` modeline comment before any content:

```yaml
# yaml-language-server: $schema=https://json.schemastore.org/github-workflow.json
name: CI
on: push
jobs:
  build:
    runs-on: ubuntu-latest
```

### TOML

Add a `$schema` key at the top level:

```toml
"$schema" = "https://json.schemastore.org/pyproject.json"

[project]
name = "my-project"
version = "1.0.0"
```

### TOON

Add a quoted `"$schema"` key at the top level:

```
"$schema": https://example.com/schema.json
host: localhost
port: 5432
```

### XML

Add an `xsi:noNamespaceSchemaLocation` attribute on the root element:

```xml
<?xml version="1.0"?>
<config xmlns:xsi="http://www.w3.org/2001/XMLSchema-instance"
        xsi:noNamespaceSchemaLocation="config.xsd">
  <host>db.example.com</host>
  <port>5432</port>
</config>
```

XML uses XSD (XML Schema Definition) files rather than JSON Schema.

XML files with inline DTD declarations (`<!DOCTYPE>`) are validated against the DTD during syntax checking — no separate schema declaration needed.

### SARIF

The validator reads the `"version"` field from the file to determine whether it's SARIF 2.1.0 or 2.2, then validates against the corresponding built-in schema. No declaration needed.

## Schema references

Schema references can be:

- **URLs** — `https://json.schemastore.org/package.json`
- **Local file paths** — absolute or relative. Relative paths are resolved from the directory containing the config file being validated.

## SchemaStore integration

[SchemaStore](https://www.schemastore.org/) is a community-maintained catalog of JSON Schemas for hundreds of common config files — `package.json`, `tsconfig.json`, GitHub Actions workflows, `pyproject.toml`, and many more.

With `--schemastore`, the validator matches files by name against the SchemaStore catalog and validates them against the corresponding schema. No `$schema` declaration needed in your files.

```shell
validator --schemastore .
```

Schemas are fetched on first use and cached locally. Repeated runs don't require network access.

:::info
Schemas are cached in `~/.cache/cfv/schemas/` (or `$XDG_CACHE_HOME/cfv/schemas/`) with a 24-hour TTL.
:::

### Offline and restricted environments

If network access is restricted, use a local SchemaStore clone:

```shell
git clone --depth=1 https://github.com/SchemaStore/schemastore.git
```

Then point the validator at the local clone:

```shell
validator --schemastore-path=./schemastore .
```

`--schemastore-path` implies `--schemastore` — you don't need both flags.

## External schema mapping

Use `--schema-map` to apply a schema to files matching a glob pattern. This is useful when files don't declare their own schema or when you want to enforce a specific schema across a set of files.

Apply a JSON Schema to all `package.json` files:

```shell
validator --schema-map="**/package.json:schemas/package.schema.json" .
```

Apply an XSD to XML config files:

```shell
validator --schema-map="**/config.xml:schemas/config.xsd" .
```

Specify multiple mappings in one invocation:

```shell
validator \
  --schema-map="**/package.json:schemas/pkg.json" \
  --schema-map="**/*.xml:schemas/config.xsd" \
  .
```

Schema paths can be URLs, absolute paths, or relative paths. Relative paths are resolved from the current working directory.

```shell
validator --schema-map="package.json:https://json.schemastore.org/package.json" .
validator --schema-map="deploy/*.xml:/opt/schemas/config.xsd" .
validator --schema-map="src/config.yaml:schemas/app.schema.json" .
```

The same mappings can be set in `.cfv.toml`:

```toml
[schema-map]
"**/package.json" = "schemas/package.schema.json"
"**/config.xml" = "schemas/config.xsd"
```

## Priority order

When multiple schema sources are available for a file, the validator uses this precedence (highest first):

1. Schema declared in the document (`$schema`, `yaml-language-server`, `xsi:noNamespaceSchemaLocation`)
2. `--schema-map` patterns
3. `--schemastore` catalog lookup

Document-level declarations always take priority. SchemaStore acts as a fallback for files that don't declare their own schema.

## Requiring schemas

Use `--require-schema` to fail validation on files that support schema validation but don't declare a schema:

```shell
validator --require-schema .
```

This affects JSON, JSONC, YAML, TOML, TOON, and XML files. Other formats (INI, CSV, ENV, HCL, HOCON, Properties, PList, EditorConfig, Justfile) are not affected since they have no schema mechanism.

## Disabling schema validation

Use `--no-schema` to skip all schema validation. Only syntax is checked:

```shell
validator --no-schema .
```

`--no-schema` cannot be combined with `--require-schema`, `--schema-map`, or `--schemastore`.
