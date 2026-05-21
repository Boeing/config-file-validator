---
sidebar_position: 1
slug: /introduction
---

# Introduction

Config File Validator validates configuration files across 16 formats. A single static binary that replaces per-format tools like yamllint, jsonlint, and xmllint.

Point it at a directory and it finds every config file, detects the format, and checks syntax. Add `--schemastore` and it also validates content against the correct schema, automatically, with no configuration.

## Supported formats

**Syntax + Schema:** `JSON` `JSONC` `YAML` `TOML` `XML` `TOON` `SARIF`

**Syntax:** `HCL` `INI` `HOCON` `ENV` `CSV` `Properties` `EDITORCONFIG` `Justfile` `PList`

## When to use it

- **CI pipelines** — a [GitHub Action](./integrations/github-actions.md) posts validation results as PR comments with inline annotations. For other CI systems, use JSON, JUnit, or SARIF output.
- **Pre-commit hooks** — a ready-made [pre-commit hook](./integrations/pre-commit.md) validates changed config files on every commit. No setup beyond adding the hook.
- **Monorepos** — validates all config formats in a single pass. No per-format tooling to install or maintain.
- **Schema enforcement** — go beyond syntax checking. Require that config files declare and conform to a schema. Catch wrong field names, invalid values, and missing required keys — not just malformed syntax.

## Next steps

- [Installation](./installation.md) — install via Homebrew, binary download, `go install`, or Docker
- [Quick Start](./quick-start.md) — validate your first directory in under a minute
