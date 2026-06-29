# feat: `validator schema generate` — infer JSON Schema from config files

## Problem

Schema validation is the most powerful feature of this tool, but adoption is low because setup is painful. Teams have to hand-write JSON Schema files, host them, map every path, and maintain them as configs evolve. Most teams skip it entirely.

## Proposal

One command to go from zero enforcement to full schema validation:

```shell
# Generate a schema from a known-good config file
validator schema generate deploy/production.yml > schemas/deploy.json

# Map it in .cfv.toml
# schema-map = ["deploy/**/*.yml:schemas/deploy.json"]

# Done — every matching file is now enforced
validator .
```

The command parses the input file, infers types and structure, and emits a JSON Schema that validates files with the same shape.

## How Inference Works

| Observed value | Inferred schema |
|---|---|
| `"hello"` | `{"type": "string"}` |
| `42` | `{"type": "integer"}` |
| `3.14` | `{"type": "number"}` |
| `true` | `{"type": "boolean"}` |
| `[1, 2, 3]` | `{"type": "array", "items": {"type": "integer"}}` |
| `{"key": "val"}` | `{"type": "object", "properties": {...}, "required": [...]}` |

Default behavior is strict: all keys are `required`, `additionalProperties: false`. Flags to relax:

```
--all-optional          Don't mark fields as required
--additional-properties Allow keys not in the source file
```

## Supported Input Formats

Any format the validator can parse as a key-value structure: JSON, JSONC, YAML, TOML, XML, CUE, HCL, HOCON, Properties, INI, ENV, PList.

Formats without key-value structure (CSV, Justfile, KDL) are out of scope.

## Multi-File Inference (stretch goal)

```shell
validator schema generate deploy/staging.yml deploy/production.yml > schemas/deploy.json
```

Given multiple files, take the union of observed keys. Keys present in all files → `required`. Keys present in some → optional. Types widened to accommodate all observations.

This handles "staging and production differ slightly but should share one schema."

## Example

**Input** (`config.yml`):
```yaml
name: myapp
port: 8080
debug: false
database:
  host: db.internal
  pool_size: 10
```

**Output** (`schemas/config.json`):
```json
{
  "$schema": "https://json-schema.org/draft/2020-12/schema",
  "type": "object",
  "required": ["name", "port", "debug", "database"],
  "additionalProperties": false,
  "properties": {
    "name": {"type": "string"},
    "port": {"type": "integer"},
    "debug": {"type": "boolean"},
    "database": {
      "type": "object",
      "required": ["host", "pool_size"],
      "properties": {
        "host": {"type": "string"},
        "pool_size": {"type": "integer"}
      }
    }
  }
}
```

## Implementation Notes

- Core function: `ValueToSchema(any) map[string]any` — recursively converts parsed config tree to JSON Schema.
- Reuses existing format parsers. No new dependencies.
- New subcommand: `validator schema generate [flags] <file...>`
- New package: `pkg/schema/` (thin — just the inference logic + JSON output).
- Output is always JSON Schema draft 2020-12.

## What This Unlocks

- **30-second onboarding** — point at a file, get enforcement
- **Config drift detection** — generate from production, validate staging against it
- **Progressive strictness** — start generated, tighten over time
- **Zero external tooling** — no schema registry, no web editors, no copy-paste

## Open Questions

1. Enum detection — if a string value is `"production"`, should we suggest an enum constraint?
2. Output format — YAML option for teams that prefer it?
3. Future: `validator schema diff` to compare two schemas?
