---
---

# Reporters

The validator writes results to stdout by default. Use `--reporter` to control the format, `--groupby` to organize results, and `--quiet` to suppress output entirely.

## Reporter types

| Reporter   | Description                    | Use case                                                |
|------------|--------------------------------|---------------------------------------------------------|
| `standard` | Human-readable table (default) | Terminal use, local development                         |
| `json`     | Structured JSON                | CI pipelines, scripting, programmatic consumption       |
| `junit`    | JUnit XML                      | Jenkins, GitLab CI, any system that reads JUnit reports |
| `sarif`    | SARIF 2.1.0                    | GitHub Code Scanning, VS Code SARIF Viewer              |

## Basic usage

```shell
validator --reporter=json .
```

## Output to a file

Append `:<path>` to the reporter name to write results to a file:

```shell
validator --reporter=json:output.json .
```

Use `:-` to explicitly direct output to stdout (useful when combining reporters):

```shell
validator --reporter=json:- --reporter=junit:results.xml .
```

## Multiple reporters

Chain `--reporter` flags to produce multiple outputs in a single run:

```shell
validator --reporter=standard --reporter=json:output.json .
```

```shell
validator --reporter=junit:results.xml --reporter=sarif:results.sarif .
```

In `.cfv.toml`:

```toml
reporter = ["standard", "json:output.json"]
```

## Grouping output

:::note
Grouping only applies to the `standard` and `json` reporters.
:::

Use `--groupby` to organize the report. Supported groupings:

| Value        | Groups by                            |
|--------------|--------------------------------------|
| `filetype`   | File format (JSON, YAML, TOML, etc.) |
| `directory`  | Parent directory                     |
| `pass-fail`  | Validation result                    |
| `error-type` | Type of error (syntax, schema)       |

Combine multiple groupings:

```shell
validator --groupby=filetype,pass-fail .
```

In `.cfv.toml`:

```toml
groupby = ["filetype", "pass-fail"]
```

## Reporters

### JSON

The JSON reporter produces an array of result objects:

```json
[
  {
    "file": "/path/to/config.yaml",
    "status": "pass",
    "message": ""
  },
  {
    "file": "/path/to/broken.json",
    "status": "fail",
    "message": "unexpected EOF"
  }
]
```

### SARIF

The SARIF reporter produces a [SARIF 2.1.0](https://sarifweb.azurewebsites.net/) log. This integrates with:

- **GitHub Code Scanning** — upload with `github/codeql-action/upload-sarif`
- **VS Code** — view with the SARIF Viewer extension
- **Azure DevOps** — native SARIF support in pipeline results

Example GitHub Actions step:

```yaml
- name: Upload SARIF
  uses: github/codeql-action/upload-sarif@v3
  with:
    sarif_file: results.sarif
  if: always()
```

### JUnit

The JUnit reporter produces XML compatible with CI systems that consume JUnit test reports. Each validated file appears as a test case; failures include the error message.
