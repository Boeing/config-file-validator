<p align="center">
  <img src="./img/logo.png" width="160" height="160" alt="Config File Validator logo"/>
</p>

<h1 align="center">Config File Validator</h1>

<p align="center">
  <img id="cov" src="https://img.shields.io/badge/Coverage-92%25-brightgreen" alt="Code Coverage">
  <a href="https://scorecard.dev/viewer/?uri=github.com/Boeing/config-file-validator"><img src="https://api.scorecard.dev/projects/github.com/Boeing/config-file-validator/badge" alt="OpenSSF Scorecard"></a>
  <a href="https://www.bestpractices.dev/projects/9027"><img src="https://www.bestpractices.dev/projects/9027/badge" alt="OpenSSF Best Practices"></a>
  <a href="https://opensource.org/licenses/Apache-2.0"><img src="https://img.shields.io/badge/License-Apache_2.0-blue.svg" alt="Apache 2 License"></a>
  <a href="https://github.com/avelino/awesome-go"><img src="https://awesome.re/mentioned-badge.svg" alt="Awesome Go"></a>
  <a href="https://pkg.go.dev/github.com/Boeing/config-file-validator/v3"><img src="https://pkg.go.dev/badge/github.com/Boeing/config-file-validator/v3.svg" alt="Go Reference"></a>
  <a href="https://github.com/boeing/config-file-validator/actions/workflows/go.yml"><img src="https://github.com/boeing/config-file-validator/actions/workflows/go.yml/badge.svg" alt="Pipeline Status"></a>
</p>

<p align="center">
  <img src="./img/demo.svg" width="780" alt="cfv validating config files"/>
</p>

## Install

```shell
brew install config-file-validator
```

```shell
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

<details>
<summary>Winget, Scoop, MacPorts, binary downloads</summary>

- Winget: `winget install Boeing.config-file-validator`
- Scoop: `scoop install config-file-validator`
- MacPorts: `sudo port install config-file-validator`
- Binaries: [GitHub Releases](https://github.com/Boeing/config-file-validator/releases)
</details>

## Usage

Validate syntax, enforce schemas, and check formatting:

```shell
cfv check .
```

Fix formatting, trailing commas, and type coercion:

```shell
cfv check --fix .
```

Preview formatting changes as a diff:

```shell
cfv format --diff .
```

Exits with code 1 if any file fails.

## Supported Formats

| Format          | Extensions              | Syntax | Format | Schema |
|-----------------|-------------------------|:------:|:------:|:------:|
| JSON            | `.json`                 |   ✅    |   ✅    |   ✅    |
| JSONC           | `.jsonc`                |   ✅    |   ✅    |   ✅    |
| YAML            | `.yaml`, `.yml`         |   ✅    |   ✅    |   ✅    |
| TOML            | `.toml`                 |   ✅    |   ✅    |   ✅    |
| XML             | `.xml`                  |   ✅    |   ✅    |   ✅    |
| TOON            | `.toon`                 |   ✅    |   —    |   ✅    |
| SARIF           | `.sarif`                |   ✅    |   —    |   ✅    |
| HCL             | `.hcl`, `.tf`, `.tfvars`|   ✅    |   ✅    |   —    |
| INI             | `.ini`                  |   ✅    |   ✅    |   —    |
| Properties      | `.properties`           |   ✅    |   ✅    |   —    |
| ENV             | `.env`                  |   ✅    |   ✅    |   —    |
| HOCON           | `.hocon`                |   ✅    |   —    |   —    |
| CSV             | `.csv`                  |   ✅    |   —    |   —    |
| EDITORCONFIG    | `.editorconfig`         |   ✅    |   —    |   —    |
| Justfile        | `.just`                 |   ✅    |   —    |   —    |
| KDL             | `.kdl`                  |   ✅    |   —    |   —    |
| CUE             | `.cue`                  |   ✅    |   —    |   —    |
| Apple PList XML | `.plist`                |   ✅    |   —    |   —    |

Schema validation uses [JSON Schema](https://json-schema.org/), [XSD](https://www.w3.org/XML/Schema), and automatic [SchemaStore](https://www.schemastore.org/) lookup. Formatting reads your existing `.prettierrc`, `taplo.toml`, `.yamlfmt`, and `.editorconfig` files. See [Formatting](https://boeing.github.io/config-file-validator/docs/guides/formatting) and [Schema Validation](https://boeing.github.io/config-file-validator/docs/guides/schema-validation).

## CI

```shell
cfv check --reporter=junit:results.xml --schemastore .
```

Reporters: `standard`, `json`, `junit`, `sarif`, `github`. The `github` reporter emits workflow commands so errors appear as inline PR annotations.

A [GitHub Action](https://github.com/Boeing/validate-configs-action) and [pre-commit hook](https://boeing.github.io/config-file-validator/docs/integrations/pre-commit) are also available.

## Documentation

[boeing.github.io/config-file-validator](https://boeing.github.io/config-file-validator): [CLI Reference](https://boeing.github.io/config-file-validator/docs/reference/cli-flags) · [Configuration](https://boeing.github.io/config-file-validator/docs/guides/configuration-file) · [Formatting](https://boeing.github.io/config-file-validator/docs/guides/formatting) · [Go Library](https://boeing.github.io/config-file-validator/docs/integrations/go-library)

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## License

[Apache 2.0](./LICENSE)
