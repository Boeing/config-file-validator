<p align="center">
  <img src="./img/logo.png" width="160" height="160" alt="Config File Validator logo"/>
</p>
<h1 align="center">Config File Validator</h1>

<p align="center">
<img id="cov" src="https://img.shields.io/badge/Coverage-93.3%25-brightgreen" alt="Code Coverage">

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

Config File Validator validates config files across 18 formats.

It recursively searches directories for config files, detects their format by extension or filename, and reports errors.

## Install

**Homebrew**
```shell
brew install config-file-validator
```

**Winget**
```shell
winget install Boeing.config-file-validator
```

**Go Install**
```shell
go install github.com/Boeing/config-file-validator/v2/cmd/validator@latest
```

<details>
<summary>More install options</summary>

**MacPorts**

```shell
sudo port install config-file-validator
```

**Scoop**

```shell
scoop install config-file-validator
```

**Binary releases**

Download pre-built binaries for macOS, Linux, and Windows from [GitHub Releases](https://github.com/Boeing/config-file-validator/releases).

</details>

## Usage

Validate all config files in the current directory:

<img src="./img/demo.svg" alt="Config File Validator output showing pass/fail results" width="800" />

See the [CLI reference](https://boeing.github.io/config-file-validator/docs/reference/cli-flags) for all options.

## Features

- Schema validation via JSON Schema, XSD, and automatic [SchemaStore](https://www.schemastore.org/) lookup
- Auto-detects file types by extension and [known filename](https://boeing.github.io/config-file-validator/docs/reference/known-files)
- JSON, JUnit, and SARIF output for CI pipelines
- Watch mode for continuous local validation while editing config files
- [GitHub Action](https://github.com/Boeing/validate-configs-action) with PR annotations
- [Pre-commit hook](https://boeing.github.io/config-file-validator/docs/integrations/pre-commit)
- Project-level [`.cfv.toml`](https://boeing.github.io/config-file-validator/docs/guides/configuration-file) configuration
- Usable as a [Go library](https://boeing.github.io/config-file-validator/docs/integrations/go-library)

## Documentation

Documentation is hosted at https://boeing.github.io/config-file-validator.

## Contributing

We welcome contributions! See the [contributing guide](./CONTRIBUTING.md).

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## License

[Apache 2.0](./LICENSE)
