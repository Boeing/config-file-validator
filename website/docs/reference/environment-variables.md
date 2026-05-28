---
---

# Environment Variables

Most CLI flags can be set via environment variables prefixed with `CFV_`. CLI flags take precedence over environment variables.

| Variable                 | Equivalent Flag       |
|--------------------------|-----------------------|
| `CFV_DEPTH`              | `-depth`              |
| `CFV_EXCLUDE_DIRS`       | `-exclude-dirs`       |
| `CFV_EXCLUDE_FILE_TYPES` | `-exclude-file-types` |
| `CFV_FILE_TYPES`         | `-file-types`         |
| `CFV_IGNORE_FILES`       | `--ignore-file`       |
| `CFV_REPORTER`           | `-reporter`           |
| `CFV_GROUPBY`            | `-groupby`            |
| `CFV_QUIET`              | `-quiet`              |
| `CFV_REQUIRE_SCHEMA`     | `-require-schema`     |
| `CFV_NO_SCHEMA`          | `-no-schema`          |
| `CFV_SCHEMASTORE`        | `-schemastore`        |
| `CFV_SCHEMASTORE_PATH`   | `-schemastore-path`   |
| `CFV_GLOBBING`           | `-globbing`           |
| `CFV_GITIGNORE`          | `-gitignore`          |

## Precedence

When the same option is set in multiple places:

1. CLI flags (highest)
2. `.cfv.toml` configuration file
3. Environment variables
4. Built-in defaults (lowest)

`CFV_IGNORE_FILES` accepts a comma-separated list, for example `.dockerignore,.prettierignore`.
