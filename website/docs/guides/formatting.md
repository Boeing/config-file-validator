---
sidebar_position: 7
---

# Formatting

`cfv check` includes formatting as part of its validation. Files that are syntactically valid and schema-correct but not formatted get reported with ~ and cause exit code 1.

To fix formatting (and all other safe fixes) in one pass:

```shell
cfv check --fix .
```

The `cfv format` subcommand is also available for format-only workflows:

```shell
cfv format .          # Report files that need formatting (exit 1 if any)
cfv format --fix .    # Rewrite files in-place
cfv format --diff .   # Show what would change as a unified diff
```

## Supported formats

Formatting is available for 9 formats: JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, and ENV.

All other formats supported by cfv (HOCON, CSV, KDL, etc.) are validation-only.

## What gets normalized

- **Indentation** — consistent indent width across the file
- **Spacing around separators** — colons, equals signs, commas
- **Trailing newlines** — ensures files end with a single newline
- **Flow collection spacing** (YAML) — normalizes `{key: value}` and `[a, b]`
- **Key sorting** (opt-in) — alphabetical ordering of keys

## What format does NOT do

- Reflow prose in comments or multi-line strings
- Change quoting style unless explicitly configured
- Reorder sections (e.g., TOML tables stay where you put them)

## Idempotency

Running `cfv format --fix` twice produces the same output. If a file is already formatted, it is left untouched.

## Configuration

cfv resolves format settings differently depending on whether your project has a `.cfv.toml`.

### Projects with `.cfv.toml`

If `.cfv.toml` exists, it is the sole source of formatting configuration. External tool configs (`.prettierrc`, `taplo.toml`, `.yamlfmt`, `.editorconfig`) are not read.

```toml
[format]
indent = 2
sort-keys = false

[format.yaml]
indent = 2

[format.toml]
indent = 2
sort-keys = true
```

Global `[format]` settings apply to all formats. Per-format sections override them. CLI flags override both.

This means: once you adopt `.cfv.toml`, all formatting behavior is defined in one place. No interaction with other config files.

See [Configuration Keys](../reference/configuration-keys.md) for all available format options.

### Projects without `.cfv.toml`

Without a `.cfv.toml`, cfv reads your existing tool configs so that formatting matches what those tools already produce. Each format has one owner:

| Format    | Config read             | Fallback         |
|-----------|-------------------------|------------------|
| JSON      | `.prettierrc`           | `.editorconfig`  |
| JSONC     | `.prettierrc`           | `.editorconfig`  |
| YAML      | `.yamlfmt`, then `.prettierrc` | `.editorconfig` |
| TOML      | `taplo.toml`            | `.editorconfig`  |
| HCL       | —                       | `.editorconfig`  |
| XML       | —                       | `.editorconfig`  |
| INI       | —                       | `.editorconfig`  |
| Properties | —                      | `.editorconfig`  |
| ENV       | —                       | `.editorconfig`  |

`.editorconfig` always applies as a baseline. If a format-specific config exists, it takes precedence over `.editorconfig` for the properties they share.

For YAML: if `.yamlfmt` is found, it owns YAML formatting. If not, cfv falls back to `.prettierrc`. The two are never combined.

See [Using cfv with Existing Tools](./existing-tools.md) for details on what cfv reads from each config format.

### Disabling config discovery

To format using only built-in defaults (ignoring all config files):

```shell
cfv format --no-config .
```

To keep tool configs but ignore `.editorconfig`:

```shell
cfv format --no-editorconfig .
```

### `.editorconfig`

cfv reads `.editorconfig` following the standard spec: glob sections matched per file, parent directories apply to nested files, `root = true` stops the upward search.

| EditorConfig property  | Effect           |
|------------------------|------------------|
| `indent_style`         | Spaces or tabs   |
| `indent_size`          | Indent width     |
| `end_of_line`          | Line ending      |
| `insert_final_newline` | Trailing newline |

Other properties are ignored. A malformed `.editorconfig` is skipped.

### JSONC trailing commas

By default, JSONC formatting adds a trailing comma to expanded objects and arrays, matching Prettier's `trailingComma: "all"` behavior. Collapsed single-line collections never get a trailing comma, and strict JSON is unchanged.

Override this in `.cfv.toml`:

```toml
[format.jsonc]
trailing-commas = "none"
```

Options: `"all"` (default), `"none"`, or `"preserve"` (keep whatever style the file already uses).

## CLI flags

These flags override all config file settings for a single invocation:

| Flag | Effect |
|------|--------|
| `--indent <n>` | Set indent width |
| `--sort-keys` | Sort keys alphabetically |
| `--no-sort-keys` | Disable key sorting (overrides config) |
| `--use-tabs` | Use tab indentation |
| `--line-ending <lf\|crlf>` | Set line ending style |
| `--max-line-width <n>` | Set max line width hint |
| `--quote-style <double\|single\|preserve>` | Set quote style (YAML) |
| `--no-config` | Ignore all config files |
| `--no-editorconfig` | Ignore `.editorconfig` only |

Example: check formatting with 4-space indent regardless of config:

```shell
cfv format --indent 4 .
```

## CI usage

For most CI pipelines, `cfv check` is all you need. It validates syntax, schema, and formatting in one command:

```shell
cfv check --reporter github .
```

If you want a format-only gate (without schema or syntax checking), use `cfv format` directly:

```shell
cfv format --reporter json:results.json .
```

In a pre-commit hook, use `--fix` so files are corrected before commit:

```shell
cfv check --fix .
```
