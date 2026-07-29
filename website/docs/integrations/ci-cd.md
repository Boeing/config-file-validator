---
---

# CI/CD Pipelines

`cfv check` validates syntax, schema, and formatting in one pass. It exits 1 if any file has issues, making it a single CI gate for all config file quality.

## GitLab CI

```yaml
validate-config:
  stage: test
  image: golang:1.26
  script:
    - go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
    - cfv check --reporter=junit:results.xml --schemastore .
  artifacts:
    reports:
      junit: results.xml
```

## Jenkins

```groovy
stage('Validate Config') {
    steps {
        sh 'cfv check --reporter=junit:results.xml --schemastore .'
    }
    post {
        always {
            junit 'results.xml'
        }
    }
}
```

## Azure DevOps

```yaml
- script: |
    go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
    cfv check --reporter=junit:results.xml --schemastore .
  displayName: 'Validate config files'

- task: PublishTestResults@2
  inputs:
    testResultsFiles: 'results.xml'
    testResultsFormat: 'JUnit'
  condition: always()
```

## GitHub Actions

See [GitHub Actions](./github-actions.md) for the dedicated action with PR annotations.

For a manual setup:

```yaml
- run: |
    go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
    cfv check --reporter=github --schemastore .
```

The `github` reporter produces `::error` and `::warning` annotations that appear inline on the PR diff.

## Output formats

| Format | Flag                             | Use case                      |
|--------|----------------------------------|-------------------------------|
| JUnit  | `--reporter=junit:results.xml`   | Jenkins, GitLab, Azure DevOps |
| SARIF  | `--reporter=sarif:results.sarif` | GitHub Code Scanning          |
| JSON   | `--reporter=json:results.json`   | Custom tooling, scripts       |
| GitHub | `--reporter=github`              | GitHub Actions annotations    |

Multiple reporters can run in a single invocation:

```shell
cfv check --reporter=junit:results.xml --reporter=sarif:results.sarif --schemastore .
```

## Exit codes

| Code | Meaning                        |
|------|--------------------------------|
| `0`  | All files pass                 |
| `1`  | One or more files failed       |
| `2`  | Runtime or configuration error |

Use exit code `1` as your CI gate. Exit code `2` means cfv itself couldn't run as intended (bad flags, unreadable files).
