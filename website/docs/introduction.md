---
sidebar_position: 1
slug: /introduction
---

# Introduction

cfv validates syntax, enforces schemas, and checks formatting of configuration files across 18 formats. One static binary replaces the collection of per-format tools you maintain today.

```shell
cfv check .          # Validate syntax + schema + formatting
cfv check --fix .    # Fix everything: trailing commas, type coercion, formatting
cfv format --diff .  # Preview formatting changes as a diff
```

`cfv check` is the single CI gate. If any file has a syntax error, a schema violation, or inconsistent formatting, it exits 1.

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

- **CI pipelines** — `cfv check` as a single gate catches syntax, schema, and formatting issues. Use JSON, JUnit, SARIF, or GitHub output for machine-readable results.
- **Pre-commit hooks** — `cfv check --fix` validates and formats changed config files before every commit.
- **Monorepos** — one tool handles all config formats in a single pass. No per-format tooling to install.
- **Schema enforcement** — catch wrong field names, invalid values, and missing required keys via JSON Schema, XSD, or automatic SchemaStore lookup.

## Next steps

- [Installation](./installation.md) — Homebrew, Winget, `go install`, or binary download
- [Quick Start](./quick-start.md) — validate your first directory
- [Formatting Guide](./guides/formatting.md) — configure and use formatting
- [Schema Validation](./guides/schema-validation.md) — enforce schemas beyond syntax
- [CLI Reference](./reference/cli-flags.md) — all flags and options
