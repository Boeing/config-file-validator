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
