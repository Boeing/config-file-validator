---
sidebar_position: 1
slug: /introduction
---

# Introduction

cfv validates syntax, enforces schemas, and formats configuration files across 18 formats. One static binary replaces the collection of per-format tools you maintain today.

```shell
cfv check .          # Validate syntax and schema
cfv format .         # Report files that need formatting (exit 1 if any)
cfv format --fix .   # Fix formatting in-place
```

## Supported formats

| Format | Validate | Format | Schema |
|--------|:--------:|:------:|:------:|
| JSON | ✓ | ✓ | ✓ |
| JSONC | ✓ | ✓ | ✓ |
| YAML | ✓ | ✓ | ✓ |
| TOML | ✓ | ✓ | ✓ |
| XML | ✓ | ✓ | ✓ |
| HCL | ✓ | ✓ | |
| INI | ✓ | ✓ | |
| Properties | ✓ | ✓ | |
| ENV | ✓ | ✓ | |
| HOCON | ✓ | | |
| CSV | ✓ | | |
| EDITORCONFIG | ✓ | | |
| Justfile | ✓ | | |
| KDL | ✓ | | |
| CUE | ✓ | | |
| PList | ✓ | | |
| TOON | ✓ | | ✓ |
| SARIF | ✓ | | ✓ |

## What it replaces

cfv covers what previously required separate tools:

- **prettier** / **yamlfmt** — YAML and JSON formatting
- **taplo** — TOML formatting
- **terraform fmt** — HCL formatting
- **xmllint** — XML validation and formatting
- **jsonlint** — JSON validation
- **v8r** — schema validation via SchemaStore

All in one binary, zero runtime dependencies.

## When to use it

- **CI pipelines** — `cfv check` and `cfv format` as gate checks. Use JSON, JUnit, SARIF, or GitHub output for machine-readable results.
- **Pre-commit hooks** — validate and format changed config files on every commit.
- **Monorepos** — one tool handles all config formats in a single pass. No per-format tooling to install.
- **Schema enforcement** — catch wrong field names, invalid values, and missing required keys via JSON Schema, XSD, or automatic SchemaStore lookup.

## Next steps

- [Installation](./installation.md) — Homebrew, Winget, `go install`, or binary download
- [Quick Start](./quick-start.md) — validate your first directory
- [Formatting Guide](./guides/formatting.md) — configure and use `cfv format`
- [Schema Validation](./guides/schema-validation.md) — enforce schemas beyond syntax
- [CLI Reference](./reference/cli-flags.md) — all flags and options
