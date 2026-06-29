# Feature Proposal: Schema Generation from Config Files

## Summary

Add a `validator schema generate <file>` command that infers a JSON Schema from an existing config file. This lets teams go from zero enforcement to full schema validation in under a minute — no manual schema authoring required.

## Problem

Schema validation is powerful but adoption is low because the setup cost is high:

1. Someone has to write a JSON Schema by hand (or find one on SchemaStore)
2. The schema has to be hosted or checked into the repo
3. Every file path needs a `--schema-map` entry
4. As config files evolve, schemas fall behind

Most teams skip all of this and rely on "the app will crash if the config is wrong." That's fine until a bad config reaches production.

## Proposed Solution

```shell
# Generate a schema from a known-good config file
validator schema generate deploy/production.yml

# Output: JSON Schema to stdout (redirect to file)
validator schema generate deploy/production.yml > schemas/deploy.json
```

The command parses the input file, infers types and structure, and emits a JSON Schema that would validate files with the same shape.

### Workflow

```shell
# 1. Generate
validator schema generate deploy/production.yml > schemas/deploy.json

# 2. Review and tighten (optional — add required fields, enums, bounds)

# 3. Map in .cfv.toml
#    schema-map = ["deploy/**/*.yml:schemas/deploy.json"]

# 4. Every future run enforces the schema across all matching files
validator .
```

## Inference Rules

Given an input file, the generator produces a schema by observing actual values:

| Observed value | Inferred schema |
|---|---|
| `"hello"` | `{"type": "string"}` |
| `42` | `{"type": "integer"}` |
| `3.14` | `{"type": "number"}` |
| `true` / `false` | `{"type": "boolean"}` |
| `null` | `{"type": "null"}` |
| `[1, 2, 3]` | `{"type": "array", "items": {"type": "integer"}}` |
| `{"key": "val"}` | `{"type": "object", "properties": {...}, "required": [...]}` |
| Mixed-type array `[1, "a"]` | `{"type": "array", "items": {"oneOf": [...]}}` |

### Key behaviors

- All observed object keys are marked `required` by default (strict). Users relax as needed.
- Nested objects recurse — full depth inference.
- `additionalProperties: false` by default (strict). Relaxed with a flag if desired.
- Arrays infer item schema from the union of all elements.
- Empty arrays become `{"type": "array"}` with no items constraint.

## Supported Input Formats

Any format the validator can parse and represent as a key-value tree:

- JSON / JSONC
- YAML
- TOML
- XML (element structure)
- CUE
- HCL (attributes and blocks)
- HOCON
- Properties (flat key=value → flat object schema)
- INI (sections → nested objects)
- ENV (flat key=value)
- PList

Formats that don't have a natural key-value structure (CSV, Justfile, KDL) are out of scope.

## Output

Always JSON Schema (draft 2020-12). The output is a valid `.json` file suitable for use with `--schema-map` or `$schema` annotation.

## CLI Flags

```
validator schema generate [flags] <file>

Flags:
  --all-optional     Don't mark fields as required (lenient mode)
  --additional-properties   Allow keys not present in the source file
  --output, -o       Write schema to file instead of stdout
```

## Multi-File Inference (Future)

```shell
# Generate schema from multiple known-good files (union of all shapes)
validator schema generate deploy/staging.yml deploy/production.yml > schemas/deploy.json
```

When given multiple files, the generator takes the union of all observed keys. Keys present in all files are `required`; keys present in some files are optional. Types are widened to accommodate all observations.

This handles the common case: "staging and production have slightly different fields but should share one schema."

## Examples

### Input (YAML)

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: myapp
  namespace: production
  labels:
    app: myapp
    tier: backend
spec:
  replicas: 3
  template:
    spec:
      containers:
        - name: myapp
          image: myapp:1.2.3
          ports:
            - containerPort: 8080
```

### Generated Schema (trimmed)

```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["apiVersion", "kind", "metadata", "spec"],
  "additionalProperties": false,
  "properties": {
    "apiVersion": {"type": "string"},
    "kind": {"type": "string"},
    "metadata": {
      "type": "object",
      "required": ["name", "namespace", "labels"],
      "properties": {
        "name": {"type": "string"},
        "namespace": {"type": "string"},
        "labels": {
          "type": "object",
          "properties": {
            "app": {"type": "string"},
            "tier": {"type": "string"}
          }
        }
      }
    },
    "spec": {
      "type": "object",
      "required": ["replicas", "template"],
      "properties": {
        "replicas": {"type": "integer"},
        "template": { ... }
      }
    }
  }
}
```

## Implementation Notes

- The core is a `ValueToSchema(any) map[string]any` function that recursively converts a parsed config tree into a JSON Schema structure.
- Parsing already exists for all supported formats — reuse the existing unmarshaling paths.
- No new dependencies required. JSON Schema is just JSON output.
- `cmd/validator/` adds a `schema` subcommand with `generate` action.
- Package: `pkg/schema/generate.go` (new package, thin).

## What This Enables

- **30-second onboarding**: "point at a file, get enforcement"
- **Config drift detection**: generate schema from production, validate staging against it
- **Progressive strictness**: start with generated schema (permissive), tighten over time
- **No external tooling**: no schema registry, no web editor, no copy-paste from SchemaStore
- **Composable with existing features**: output feeds directly into `--schema-map` and `.cfv.toml`

## Non-Goals

- Not a replacement for hand-written schemas with complex validation logic (regex patterns, conditional schemas, `if/then/else`)
- Not a schema evolution/migration tool
- No format conversion (YAML→JSON, etc.)
- No schema hosting or registry features

## Open Questions

1. Should enum detection be attempted? (e.g., if a string is `"production"`, suggest `{"enum": ["staging", "production"]}` as a comment?)
2. Should the command offer `--format yaml` to output the schema as YAML instead of JSON? (Some teams prefer YAML schemas.)
3. Should there be a `validator schema diff` command that compares two schemas? (Future scope.)
