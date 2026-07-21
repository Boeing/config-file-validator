---
---

# Configuration Keys

All keys supported in `.cfv.toml`. See the [Configuration File](../guides/configuration-file.md) guide for usage details.

## Top-level keys

| Key                  | Type             | Default        | Equivalent Flag        |
|----------------------|------------------|----------------|------------------------|
| `exclude-dirs`       | array of strings | `[]`           | `--exclude-dirs`       |
| `exclude-file-types` | array of strings | `[]`           | `--exclude-file-types` |
| `ignore-files`       | array of strings | `[]`           | `--ignore-file`        |
| `file-types`         | array of strings | all            | `--file-types`         |
| `depth`              | integer (≥ 0)    | unlimited      | `--depth`              |
| `reporter`           | array of strings | `["standard"]` | `--reporter`           |
| `groupby`            | array of strings | `[]`           | `--groupby`            |
| `quiet`              | boolean          | `false`        | `--quiet`              |
| `require-schema`     | boolean          | `false`        | `--require-schema`     |
| `no-schema`          | boolean          | `false`        | `--no-schema`          |
| `schemastore`        | boolean          | `false`        | `--schemastore`        |
| `schemastore-path`   | string           | —              | `--schemastore-path`   |
| `globbing`           | boolean          | `false`        | `--globbing`           |
| `gitignore`          | boolean          | `false`        | `--gitignore`          |

## Table keys

| Key          | Type                   | Equivalent Flag |
|--------------|------------------------|-----------------|
| `schema-map` | table (pattern = path) | `--schema-map`  |
| `type-map`   | table (pattern = type) | `--type-map`    |
| `validators` | table                  | —               |

## Validator options

| Key                                     | Type    | Default | Description                                              |
|-----------------------------------------|---------|---------|----------------------------------------------------------|
| `validators.csv.delimiter`              | string  | `","`   | Field delimiter. Use `"\t"` for tab.                     |
| `validators.csv.comment`                | string  | —       | Lines starting with this character are skipped.          |
| `validators.csv.lazy-quotes`            | boolean | `false` | Allow bare quotes in unquoted fields.                    |
| `validators.json.forbid-duplicate-keys` | boolean | `false` | Report duplicate keys in objects as errors.              |
| `validators.ini.forbid-duplicate-keys`  | boolean | `false` | Report duplicate keys within the same section as errors. |

YAML duplicate keys are always rejected by the parser regardless of configuration.

## Format keys

The `[format]` table controls formatting behavior for `cfv format`.

| Key                | Type    | Default      | Description                                             |
|--------------------|---------|--------------|---------------------------------------------------------|
| `indent`           | integer | `2`          | Spaces per indent level.                                |
| `use-tabs`         | boolean | `false`      | Use tabs instead of spaces for indentation.             |
| `sort-keys`        | boolean | `false`      | Sort mapping keys alphabetically.                       |
| `trailing-newline` | boolean | `true`       | Ensure file ends with a newline.                        |
| `line-ending`      | string  | `"lf"`       | Line ending style: `"lf"` or `"crlf"`.                 |
| `quote-style`      | string  | `"preserve"` | Quote style: `"preserve"`, `"double"`, or `"single"` (YAML only). |
| `trailing-commas`  | string  | `"preserve"` | Trailing commas on multiline collections: `"preserve"` (match the file), `"all"`, or `"none"` (JSONC only). |

### Per-format overrides

Use `[format.<type>]` tables to override settings for a specific format. The same keys as `[format]` are supported.

```toml
[format]
indent = 2
sort-keys = false

[format.yaml]
indent = 2
quote-style = "double"

[format.json]
indent = 4

[format.toml]
sort-keys = true
```

Supported format types: `yaml`, `json`, `jsonc`, `toml`, `hcl`, `xml`, `ini`, `properties`, `env`.

### Resolution order

Settings resolve with this priority (highest wins):

1. CLI flags (`--indent`, `--sort-keys`)
2. Per-format config (`[format.yaml]`)
3. Global format config (`[format]`)
4. Built-in defaults
