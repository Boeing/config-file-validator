---
---

# Configuration File

The validator supports a `.cfv.toml` configuration file for project-level defaults. Instead of passing flags on every invocation, define your settings once and commit them to the repository.

## Discovery

On startup, the validator looks for `.cfv.toml` in the current directory and walks up parent directories until it finds one or reaches the filesystem root.

To specify a file explicitly:

```shell
validator --config=path/to/.cfv.toml .
```

To disable auto-discovery:

```shell
validator --no-config .
```

## Precedence

Most CLI flags can also be set through [environment variables](../reference/environment-variables.md) prefixed with `CFV_`. When multiple sources set the same option, the validator resolves them in this order (highest priority first):

1. CLI flags
2. `.cfv.toml` configuration file
3. Environment variables (`CFV_*`)
4. Built-in defaults

## Example

```toml
exclude-dirs = ["node_modules", ".git", "vendor", "dist"]
exclude-file-types = ["csv"]
ignore-files = [".dockerignore", ".prettierignore"]
depth = 3
quiet = false
gitignore = true
schemastore = true
require-schema = false
reporter = ["standard"]
groupby = ["filetype", "pass-fail"]

[schema-map]
"**/package.json" = "schemas/package.schema.json"
"**/config.xml" = "schemas/config.xsd"

[type-map]
"**/inventory" = "ini"
"**/*.cfg" = "json"

[validators.csv]
delimiter = ";"
comment = "#"

[validators.json]
forbid-duplicate-keys = true
```

## All configuration keys

| Key                  | Type             | Default        | Description                                                         |
|----------------------|------------------|----------------|---------------------------------------------------------------------|
| `exclude-dirs`       | array of strings | `[]`           | Subdirectories to skip during traversal                             |
| `exclude-file-types` | array of strings | `[]`           | File types to ignore                                                |
| `ignore-files`       | array of strings | `[]`           | Gitignore-style pattern files to apply during file discovery        |
| `file-types`         | array of strings | all            | Only validate these file types                                      |
| `depth`              | integer (≥ 0)    | unlimited      | Maximum recursion depth                                             |
| `reporter`           | array of strings | `["standard"]` | Output formats: `standard`, `json`, `junit`, `sarif`, `github`      |
| `groupby`            | array of strings | `[]`           | Group output by: `filetype`, `directory`, `pass-fail`, `error-type` |
| `quiet`              | boolean          | `false`        | Suppress stdout output                                              |
| `require-schema`     | boolean          | `false`        | Fail files that support schemas but don't declare one               |
| `no-schema`          | boolean          | `false`        | Disable all schema validation                                       |
| `schemastore`        | boolean          | `false`        | Enable SchemaStore catalog lookup                                   |
| `schemastore-path`   | string           | —              | Path to local SchemaStore clone (implies `schemastore`)             |
| `globbing`           | boolean          | `false`        | Treat positional arguments as glob patterns                         |
| `gitignore`          | boolean          | `false`        | Skip files matched by `.gitignore` patterns                         |
| `schema-map`         | table            | —              | Map glob patterns to schema files                                   |
| `type-map`           | table            | —              | Map glob patterns to file types                                     |
| `validators`         | table            | —              | Per-validator options (see below)                                   |

## Schema and type maps

Both `schema-map` and `type-map` are TOML tables where keys are glob patterns and values are paths (for schemas) or type names (for type-map).

```toml
[schema-map]
"**/package.json" = "schemas/package.schema.json"
"**/pom.xml" = "schemas/maven.xsd"

[type-map]
"**/inventory" = "ini"
"**/.env.*" = "env"
```

Glob patterns follow the same syntax as `--schema-map` and `--type-map` CLI flags. See [Glob Patterns](./glob-patterns.md) for pattern syntax details.

## Validator options

Some validators accept format-specific options that control how files are parsed. These are set under `[validators.<type>]` tables.

### CSV

| Key           | Type    | Default | Description                                              |
|---------------|---------|---------|----------------------------------------------------------|
| `delimiter`   | string  | `","`   | Field delimiter character. Use `"\t"` for tab-separated. |
| `comment`     | string  | —       | Lines starting with this character are skipped.          |
| `lazy-quotes` | boolean | `false` | Allow bare quotes in unquoted fields.                    |

```toml
[validators.csv]
delimiter = "\t"
comment = "#"
lazy-quotes = true
```

### JSON

| Key                     | Type    | Default | Description                                 |
|-------------------------|---------|---------|---------------------------------------------|
| `forbid-duplicate-keys` | boolean | `false` | Report duplicate keys in objects as errors. |

```toml
[validators.json]
forbid-duplicate-keys = true
```

### INI

| Key                     | Type    | Default | Description                                              |
|-------------------------|---------|---------|----------------------------------------------------------|
| `forbid-duplicate-keys` | boolean | `false` | Report duplicate keys within the same section as errors. |

```toml
[validators.ini]
forbid-duplicate-keys = true
```

:::note
YAML duplicate keys are always rejected by the YAML parser regardless of configuration.
:::

## Validation of the config file itself

The `.cfv.toml` file is validated against a built-in schema on load. Typos in key names and invalid value types are reported immediately as errors (exit code 2).

```
$ validator .
Error: .cfv.toml: unknown key "exlude-dirs" (did you mean "exclude-dirs"?)
```
