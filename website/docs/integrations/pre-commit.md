---
---

# Pre-commit Hook

The validator has a ready-made [pre-commit](https://pre-commit.com/) hook that validates config files on every commit.

## Setup

Add to your `.pre-commit-config.yaml`:

```yaml
repos:
  - repo: https://github.com/Boeing/config-file-validator
    rev: v2.2.0
    hooks:
      - id: config-file-validator
```

## Available hooks

Two hooks are provided:

| Hook | Behavior |
|------|----------|
| `config-file-validator` | Validates only changed config files. Fast, intended for local development. |
| `config-file-validator-full` | Validates all config files in the repo. Intended for CI. |

## Passing flags

Add flags via the `args` key:

```yaml
hooks:
  - id: config-file-validator
    args: ['--schemastore']
```

```yaml
hooks:
  - id: config-file-validator
    args: ['--exclude-dirs=node_modules,vendor', '--schemastore']
```

## Pinning a version

The `rev` field should point to a release tag. Update it when you upgrade:

```yaml
repos:
  - repo: https://github.com/Boeing/config-file-validator
    rev: v2.2.0
    hooks:
      - id: config-file-validator
```

Run `pre-commit autoupdate` to bump to the latest release automatically.
