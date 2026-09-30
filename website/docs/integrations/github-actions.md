---
---

# GitHub Actions

A first-party GitHub Action runs the validator in your CI pipeline and posts results as PR comments with inline annotations on affected files and lines.

## Basic setup

```yaml
- uses: Boeing/validate-configs-action@v2.0.0
```

This validates all config files in the repository and annotates the PR with any errors.

## Full workflow example

```yaml
name: Validate Config Files
on: [pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - uses: Boeing/validate-configs-action@v2.0.0
```

## Configuration

The action accepts the same flags as the CLI. See the [validate-configs-action](https://github.com/Boeing/validate-configs-action) repository for full usage and configuration options.

## Using the CLI directly

If you need more control than the action provides, install the binary and run it directly:

```yaml
name: Validate Config Files
on: [pull_request]

jobs:
  validate:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
      - name: Install config-file-validator
        run: go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
      - name: Validate
        run: cfv check --schemastore .
```

## SARIF upload

Upload results to GitHub Code Scanning for persistent tracking:

```yaml
- name: Validate
  run: cfv check --reporter=sarif:results.sarif .
  continue-on-error: true

- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
  if: always()
```
