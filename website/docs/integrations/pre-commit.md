---
---

# Pre-commit Hook

cfv provides a [pre-commit](https://pre-commit.com/) hook that validates and formats config files on every commit.

## Setup

Add to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/Boeing/config-file-validator
    rev: v3.0.0
    hooks:
      - id: config-file-validator
        args: ['--fix']
```

With `--fix`, cfv corrects formatting and applies safe fixes (trailing commas, schema type coercion) before the commit completes. Files that can't be auto-fixed cause the hook to fail.

## Available hooks

| Hook                         | Behavior                                                                   |
|------------------------------|----------------------------------------------------------------------------|
| `config-file-validator`      | Validates only changed config files. Fast, intended for local development. |
| `config-file-validator-full` | Validates all config files in the repo. Intended for CI.                   |

## Passing flags

Add flags via the `args` key:

```yaml
hooks:
  - id: config-file-validator
    args: ['--fix', '--schemastore']
```

Without `--fix`, the hook reports issues but does not modify files:

```yaml
hooks:
  - id: config-file-validator
    args: ['--schemastore']
```

## Pinning a version

The `rev` field should point to a release tag. Run `pre-commit autoupdate` to bump to the latest release.
