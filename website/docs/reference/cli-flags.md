---
---

# CLI Flags

```
cfv check [flags] [<search_path>...]
```

Bare `cfv [flags] [<search_path>...]` also works and is equivalent to `cfv check`.

If no search path is provided, `cfv check` searches the current directory. Use `-` to read from stdin (requires `--file-types`).

## Subcommands

| Subcommand    | Description                                      |
|---------------|--------------------------------------------------|
| `check`       | Validate config files. Default when omitted.     |
| `format`      | Check or fix formatting of config files.         |
| `version`     | Print the version and exit.                      |
| `help`        | Show help for a subcommand.                      |

## `check` Flags

All flags below apply to the `check` subcommand.

| Flag                  | Type   | Default    | Description                                                                                                        |
|-----------------------|--------|------------|---------------------------------------------------------------------------------------------------------------------|
| `-depth`              | int    | unlimited  | Maximum recursion depth. `0` disables recursion.                                                                   |
| `-exclude-dirs`       | string | —          | Comma-separated list of directory names to skip.                                                                   |
| `-exclude-file-types` | string | —          | Comma-separated list of file types to ignore. Cannot be used with `-file-types`.                                   |
| `-file-types`         | string | all        | Comma-separated list of file types to validate. Cannot be used with `-exclude-file-types`.                         |
| `-gitignore`          | bool   | `false`    | Skip files matched by `.gitignore` patterns. Only active inside a Git repository.                                  |
| `--ignore-file`       | string | —          | Apply gitignore-style patterns from a file relative to each search path. Repeatable.                               |
| `-globbing`           | bool   | `false`    | Treat positional arguments as glob patterns.                                                                       |
| `-groupby`            | string | —          | Group output by: `filetype`, `directory`, `pass-fail`, `error-type`. Comma-separated.                              |
| `-merge-sarif`        | string | —          | External SARIF file to append to SARIF output. Repeatable. Requires `-reporter=sarif`.                             |
| `-merge-sarif-dir`    | string | —          | Directory tree of `.sarif` or `.sarif.json` files to append to SARIF output. Requires `-reporter=sarif`.           |
| `-quiet`              | bool   | `false`    | Suppress all stdout output. Errors still print to stderr.                                                          |
| `-reporter`           | string | `standard` | Output format and optional path. Format: `<type>:<path>`. Types: `standard`, `json`, `junit`, `sarif`, `github`. Repeatable. |
| `-require-schema`     | bool   | `false`    | Fail files that support schema validation but don't declare a schema.                                              |
| `-no-schema`          | bool   | `false`    | Disable all schema validation. Cannot be combined with `-require-schema`, `-schema-map`, or `-schemastore`.        |
| `-schema-map`         | string | —          | Map a glob pattern to a schema file. Format: `<pattern>:<schema_path>`. Repeatable.                                |
| `-schemastore`        | bool   | `false`    | Enable automatic schema lookup by filename using the SchemaStore catalog.                                          |
| `-schemastore-path`   | string | —          | Path to a local SchemaStore clone. Implies `-schemastore`.                                                         |
| `-config`             | string | auto       | Path to a `.cfv.toml` configuration file.                                                                          |
| `-no-config`          | bool   | `false`    | Disable automatic `.cfv.toml` discovery.                                                                           |
| `-type-map`           | string | —          | Map a glob pattern to a file type. Format: `<pattern>:<type>`. Repeatable.                                         |

## `format` Flags

```
cfv format [flags] [<search_path>...]
```

Checks formatting of config files. With `--fix`, rewrites files in place. With `--diff`, prints a unified diff of what would change.

### Format-specific flags

| Flag          | Type   | Default | Description                                                   |
|---------------|--------|---------|---------------------------------------------------------------|
| `-fix`        | bool   | `false` | Rewrite files in place. Mutually exclusive with `-diff`.      |
| `-diff`       | bool   | `false` | Print unified diff of formatting changes. Mutually exclusive with `-fix`. |
| `-indent`     | int    | `2`     | Override indent width (number of spaces per level).           |
| `-sort-keys`  | bool   | `false` | Sort mapping keys alphabetically.                            |
| `-no-editorconfig` | bool | `false` | Ignore `.editorconfig` files when resolving format options. |
| `-no-prettier-config` | bool | `false` | Ignore `.prettierrc` files when resolving format options. |

### Shared flags

These flags work the same as in `check`.

| Flag                  | Type   | Default    | Description                                                                                                        |
|-----------------------|--------|------------|---------------------------------------------------------------------------------------------------------------------|
| `-depth`              | int    | unlimited  | Maximum recursion depth. `0` disables recursion.                                                                   |
| `-exclude-dirs`       | string | —          | Comma-separated list of directory names to skip.                                                                   |
| `-exclude-file-types` | string | —          | Comma-separated list of file types to ignore. Cannot be used with `-file-types`.                                   |
| `-file-types`         | string | all        | Comma-separated list of file types to format. Cannot be used with `-exclude-file-types`.                           |
| `-gitignore`          | bool   | `false`    | Skip files matched by `.gitignore` patterns.                                                                       |
| `-reporter`           | string | `standard` | Output format and optional path. Format: `<type>:<path>`. Types: `standard`, `json`, `junit`, `sarif`, `github`. Repeatable. |
| `-quiet`              | bool   | `false`    | Suppress stdout output when writing to file.                                                                       |
| `-config`             | string | auto       | Path to a `.cfv.toml` configuration file.                                                                          |
| `-no-config`          | bool   | `false`    | Disable automatic `.cfv.toml` discovery.                                                                           |
