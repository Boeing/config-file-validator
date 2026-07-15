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

These flags override `.cfv.toml` settings for a single invocation:

| Flag | Effect |
|------|--------|
| `--indent <n>` | Set indent width |
| `--sort-keys` | Sort keys alphabetically |
| `--no-final-newline` | Omit trailing newline |

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
