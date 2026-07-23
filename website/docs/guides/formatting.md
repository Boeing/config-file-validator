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

`.editorconfig` → `.yamlfmt` → `.cfv.toml [format]` → `.cfv.toml [format.<type>]` → CLI flags

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

### `.yamlfmt`

Projects that already configure Google's [yamlfmt](https://github.com/google/yamlfmt)
via `.yamlfmt` or `.yamlfmt.yaml` get those settings applied to YAML formatting
automatically. The options cfv understands:

| yamlfmt option              | cfv option      |
|-----------------------------|-----------------|
| `formatter.indent`          | Indent width    |
| `formatter.line_ending`     | Line ending     |
| `formatter.max_line_length` | Max line width  |

Other yamlfmt options (`include_document_start`, `retain_line_breaks`,
`pad_line_comments`, …) are ignored. Discovery walks up from the working
directory and prefers `.yamlfmt` over `.yamlfmt.yaml`. A malformed file is
skipped rather than failing the run.

Pass `--no-yamlfmt-config` to ignore yamlfmt config files entirely.

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

## CLI flags

These flags override `.cfv.toml` and `.editorconfig` settings for a single invocation:

| Flag | Effect |
|------|--------|
| `--indent <n>` | Set indent width |
| `--sort-keys` | Sort keys alphabetically |
| `--no-final-newline` | Omit trailing newline |
| `--no-editorconfig` | Ignore `.editorconfig` files |
| `--no-yamlfmt-config` | Ignore `.yamlfmt` / `.yamlfmt.yaml` files |

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
