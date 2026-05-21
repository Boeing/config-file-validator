---
---

# CLI Flags

```
validator [OPTIONS] [<search_path>...]
```

If no search path is provided, the validator searches the current directory. Use `-` to read from stdin (requires `--file-types`).

## Flags

| Flag | Type | Default | Description |
|------|------|---------|-------------|
| `-depth` | int | unlimited | Maximum recursion depth. `0` disables recursion. |
| `-exclude-dirs` | string | — | Comma-separated list of directory names to skip. |
| `-exclude-file-types` | string | — | Comma-separated list of file types to ignore. Cannot be used with `-file-types`. |
| `-file-types` | string | all | Comma-separated list of file types to validate. Cannot be used with `-exclude-file-types`. |
| `-gitignore` | bool | `false` | Skip files matched by `.gitignore` patterns. Only active inside a Git repository. |
| `-globbing` | bool | `false` | Treat positional arguments as glob patterns. |
| `-groupby` | string | — | Group output by: `filetype`, `directory`, `pass-fail`, `error-type`. Comma-separated. |
| `-quiet` | bool | `false` | Suppress all stdout output. Errors still print to stderr. |
| `-reporter` | string | `standard` | Output format and optional path. Format: `<type>:<path>`. Types: `standard`, `json`, `junit`, `sarif`. Repeatable. |
| `-require-schema` | bool | `false` | Fail files that support schema validation but don't declare a schema. |
| `-no-schema` | bool | `false` | Disable all schema validation. Cannot be combined with `-require-schema`, `-schema-map`, or `-schemastore`. |
| `-schema-map` | string | — | Map a glob pattern to a schema file. Format: `<pattern>:<schema_path>`. Repeatable. |
| `-schemastore` | bool | `false` | Enable automatic schema lookup by filename using the SchemaStore catalog. |
| `-schemastore-path` | string | — | Path to a local SchemaStore clone. Implies `-schemastore`. |
| `-config` | string | auto | Path to a `.cfv.toml` configuration file. |
| `-no-config` | bool | `false` | Disable automatic `.cfv.toml` discovery. |
| `-type-map` | string | — | Map a glob pattern to a file type. Format: `<pattern>:<type>`. Repeatable. |
| `-version` | bool | — | Print the version and exit. |
