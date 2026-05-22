---
---

# CI/CD Pipelines

The validator exits with code `1` when any file fails validation, making it usable in any CI system that checks exit codes. Use `--reporter` to produce machine-readable output.

## GitLab CI

```yaml
validate-config:
  stage: test
  image: golang:1.26
  script:
    - go install github.com/Boeing/config-file-validator/v2/cmd/validator@latest
    - validator --reporter=junit:results.xml --schemastore .
  artifacts:
    reports:
      junit: results.xml
```

## Jenkins

```groovy
stage('Validate Config') {
    steps {
        sh 'validator --reporter=junit:results.xml --schemastore .'
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
    go install github.com/Boeing/config-file-validator/v2/cmd/validator@latest
    validator --reporter=junit:results.xml --schemastore .
  displayName: 'Validate config files'

- task: PublishTestResults@2
  inputs:
    testResultsFiles: 'results.xml'
    testResultsFormat: 'JUnit'
  condition: always()
```

## Output formats for CI

| Format | Flag | Use case |
|--------|------|----------|
| JUnit | `--reporter=junit:results.xml` | Jenkins, GitLab, Azure DevOps |
| SARIF | `--reporter=sarif:results.sarif` | GitHub Code Scanning |
| JSON | `--reporter=json:results.json` | Custom tooling, scripts |

Multiple reporters can run in a single invocation:

```shell
validator --reporter=junit:results.xml --reporter=sarif:results.sarif --schemastore .
```

## Exit codes

| Code | Meaning |
|------|---------|
| `0` | All files valid |
| `1` | One or more validation errors |
| `2` | Runtime or configuration error |

Use exit code `1` as your CI gate. Exit code `2` indicates a problem with the validator invocation itself (bad flags, unreadable files).
