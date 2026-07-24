---
sidebar_position: 7
---

# Formatting

`cfv format` checks whether config files match their canonical format. It reports files that need changes and exits with code 1 if any are found.

```shell
cfv format .
```

To rewrite files in-place:

```shell
cfv format --fix .
```

To see what would change as a unified diff without modifying files:

```shell
cfv format --diff .
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

Running `cfv format --fix` twice always produces the same output. If a file is already formatted, it is left untouched.

## Configuration

Format settings are resolved per file, lowest priority first:

`.editorconfig` → `taplo.toml` → `.cfv.toml [format]` → `.cfv.toml [format.<type>]` → CLI flags

### `.editorconfig`

If your project has an `.editorconfig`, cfv reads it and uses it as the baseline,
so formatted output matches what your editors already produce without any cfv
configuration. The properties cfv understands:

| EditorConfig property  | cfv option       |
|------------------------|------------------|
| `indent_style`         | Spaces or tabs   |
| `indent_size`          | Indent width     |
| `end_of_line`          | Line ending      |
| `insert_final_newline` | Trailing newline |

Resolution follows the EditorConfig spec: glob sections are matched against each
file individually, `.editorconfig` files in parent directories apply to nested
files, and `root = true` stops the upward search. Other properties are ignored,
and an unreadable or malformed `.editorconfig` is skipped rather than failing the
run.

Pass `--no-editorconfig` to ignore `.editorconfig` entirely.

### `taplo.toml`

Rust projects usually configure TOML formatting with [taplo](https://taplo.tamasfe.dev/).
cfv reads the nearest `taplo.toml` (or `.taplo.toml`), searching the current
directory and its parents, and applies it to TOML files only:

| Taplo option                   | cfv option       |
|--------------------------------|------------------|
| `formatting.indent_string`     | Indent width, or tabs |
| `formatting.column_width`      | Max line width   |
| `formatting.trailing_newline`  | Trailing newline |
| `formatting.reorder_keys`      | Sort keys        |
| `formatting.crlf`              | Line ending      |
| `formatting.array_trailing_comma` | Trailing commas |

Everything else is ignored, including `[[rule]]` sections and the options cfv
has no equivalent for: `align_entries`, `align_comments`, `compact_arrays`,
`compact_inline_tables`, `array_auto_expand`, and `array_auto_collapse`. A
project that relies on those may still see cfv diverge from taplo on those
specific behaviors. An unreadable or malformed `taplo.toml` is skipped rather
than failing the run.

Pass `--no-taplo-config` to ignore `taplo.toml` entirely.

### `.cfv.toml`

Format settings live in `.cfv.toml` at the root of your project.

Global defaults apply to all formats. Per-format sections override them:

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

With this config, all formats use 2-space indent with keys unsorted, except TOML which sorts keys alphabetically.

### JSONC trailing commas

By default, JSONC formatting adds a trailing comma to expanded objects and
arrays, matching Prettier's `trailingComma: "all"` behavior. Collapsed
single-line collections do not receive a trailing comma, and strict JSON is
unchanged.

Use `trailing-commas` to override the JSONC behavior:

```toml
[format.jsonc]
trailing-commas = "none" # "all" (default), "none", or "preserve"
```

The `"preserve"` mode applies the trailing-comma style already present in the
file to all expanded collections.

## CLI flags

These flags override `.cfv.toml`, `taplo.toml`, and `.editorconfig` settings for a single invocation:

| Flag | Effect |
|------|--------|
| `--indent <n>` | Set indent width |
| `--sort-keys` | Sort keys alphabetically |
| `--no-final-newline` | Omit trailing newline |
| `--no-editorconfig` | Ignore `.editorconfig` files |
| `--no-taplo-config` | Ignore `taplo.toml` files |

Example: check formatting with 4-space indent regardless of config file:

```shell
cfv format --indent 4 .
```

## CI usage

`cfv format` (without `--fix`) is designed for CI. It exits 0 if all files are formatted correctly, exits 1 if any file needs changes. Combine with `--reporter json` or `--reporter github` for structured output:

```shell
cfv format --reporter github .
```

In a pre-commit hook, use `--fix` so files are corrected before commit:

```shell
cfv format --fix .
```
