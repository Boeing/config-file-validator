<p align="center">
  <img src="./img/logo.png" width="160" height="160" alt="Config File Validator logo"/>
</p>
<h1 align="center">Config File Validator</h1>

<p align="center">
<img id="cov" src="https://img.shields.io/badge/Coverage-93.4%25-brightgreen" alt="Code Coverage">

  <a href="https://scorecard.dev/viewer/?uri=github.com/Boeing/config-file-validator">
    <img src="https://api.scorecard.dev/projects/github.com/Boeing/config-file-validator/badge" alt="OpenSSF Scorecard">
  </a>

  <a href="https://www.bestpractices.dev/projects/9027">
    <img src="https://www.bestpractices.dev/projects/9027/badge" alt="OpenSSF Best Practices">
  </a>

  <a href="https://opensource.org/licenses/Apache-2.0">
  <img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="Apache 2 License">
  </a>

  <a href="https://github.com/avelino/awesome-go">
  <img src="https://awesome.re/mentioned-badge.svg" alt="Awesome Go">
  </a>  

  <a href="https://pkg.go.dev/github.com/Boeing/config-file-validator/v2">
  <img src="https://pkg.go.dev/badge/github.com/Boeing/config-file-validator/v2.svg" alt="Go Reference">
  </a>

  <a href="https://goreportcard.com/report/github.com/Boeing/config-file-validator">
  <img src="https://goreportcard.com/badge/github.com/Boeing/config-file-validator" alt="Go Report Card">
  </a>

  <a href="https://github.com/boeing/config-file-validator/actions/workflows/go.yml">
  <img src="https://github.com/boeing/config-file-validator/actions/workflows/go.yml/badge.svg" alt="Pipeline Status">
  </a>
</p>

<p align="center">
  <a href="https://boeing.github.io/config-file-validator/">Website</a> |
  <a href="https://boeing.github.io/config-file-validator/docs/quick-start">Quick Start</a> |
  <a href="https://boeing.github.io/config-file-validator/docs/reference/cli-flags">CLI Reference</a> |
  <a href="https://boeing.github.io/config-file-validator/docs/guides/schema-validation">Schema Validation</a> |
  <a href="https://boeing.github.io/config-file-validator/docs/integrations/github-actions">Integrations</a>
</p>

## What is it?

Config File Validator validates configuration files across 16 formats. A single static binary that replaces per-format tools like yamllint, jsonlint, and xmllint.

Point it at a directory and it finds every config file, detects the format, and checks syntax. Add `--schemastore` and it also validates content against the correct schema, automatically, with no configuration.

```shell
validator --schemastore .
```

## Features

- JSON, JSONC, YAML, TOML, XML, HCL, INI, HOCON, ENV, CSV, Properties, EDITORCONFIG, Justfile, PList, SARIF, and TOON
- Schema validation via JSON Schema, XSD, and automatic [SchemaStore](https://www.schemastore.org/) lookup
- Auto-detects file types by extension and [known filename](https://boeing.github.io/config-file-validator/docs/reference/known-files)
- JSON, JUnit, and SARIF output for CI pipelines
- [GitHub Action](https://github.com/Boeing/validate-configs-action) with PR annotations
- [Pre-commit hook](https://boeing.github.io/config-file-validator/docs/integrations/pre-commit)
- Project-level [`.cfv.toml`](https://boeing.github.io/config-file-validator/docs/guides/configuration-file) configuration
- Usable as a [Go library](https://boeing.github.io/config-file-validator/docs/integrations/go-library)

Check out the [quick start](https://boeing.github.io/config-file-validator/docs/quick-start) to try it.

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## Contributing

We welcome contributions! See the [contributing guide](./CONTRIBUTING.md).

## License

[Apache 2.0](./LICENSE)
