<p align="center">
  <img src="./img/logo.png" width="160" height="160" alt="Config File Validator logo"/>
</p>
<h1 align="center">Config File Validator</h1>

<p align="center">
<img id="cov" src="https://img.shields.io/badge/Coverage-92%25-brightgreen" alt="Code Coverage">
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
  <a href="https://pkg.go.dev/github.com/Boeing/config-file-validator/v3">
    <img src="https://pkg.go.dev/badge/github.com/Boeing/config-file-validator/v3.svg" alt="Go Reference">
  </a>
  <a href="https://github.com/boeing/config-file-validator/actions/workflows/go.yml">
    <img src="https://github.com/boeing/config-file-validator/actions/workflows/go.yml/badge.svg" alt="Pipeline Status">
  </a>
</p>

<p align="center">
  Your code has standards. Your config files should too.
</p>

<p align="center">
  cfv is a static binary that validates, formats, and enforces schemas on configuration files.
</p>

<p align="center">

| | |
|---|---|
| **Syntax validation** | JSON, JSONC, YAML, TOML, XML, HCL, INI, Properties, ENV, HOCON, CSV, Justfile, KDL, CUE, PList, EDITORCONFIG, TOON, SARIF |
| **Schema enforcement** | JSON, JSONC, YAML, TOML, XML, TOON, SARIF |
| **Formatting** | JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, ENV |

</p>

<p align="center">
  <a href="#install">Install</a> •
  <a href="#usage">Usage</a> •
  <a href="https://boeing.github.io/config-file-validator">Docs</a> •
  <a href="./CONTRIBUTING.md">Contributing</a>
</p>

---

## Install

```shell
brew install config-file-validator
```

```shell
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

<details>
<summary>More options (Winget, Scoop, MacPorts, binary downloads)</summary>

```shell
winget install Boeing.config-file-validator
```

```shell
scoop install config-file-validator
```

```shell
sudo port install config-file-validator
```

Pre-built binaries for macOS, Linux, and Windows: [GitHub Releases](https://github.com/Boeing/config-file-validator/releases).

</details>

## Usage

Check all config files in the current directory:

```shell
cfv check .
```

Apply safe fixes (formatting, trailing commas, type coercion):

```shell
cfv check --fix .
```

Preview formatting changes as a diff:

```shell
cfv format --diff .
```

`cfv check` validates syntax, enforces schemas, and checks formatting in one pass. Exits with code 1 if any file fails.

### CI

```shell
cfv check --reporter=junit:results.xml --schemastore .
```

Reporters: `standard`, `json`, `junit`, `sarif`, `github`. See [CI/CD setup](https://boeing.github.io/config-file-validator/docs/integrations/ci-cd).

### Existing tool configs

cfv reads `.prettierrc`, `taplo.toml`, `.yamlfmt`, and `.editorconfig` files and applies their settings automatically. Projects that use a [`.cfv.toml`](https://boeing.github.io/config-file-validator/docs/guides/configuration-file) get all formatting settings from that file instead.

## Why cfv

You probably have some combination of prettier, yamlfmt, taplo, terraform fmt, xmllint, jsonlint, and v8r wired into your pipeline.

cfv replaces all of them. It's a single static binary with no runtime dependencies.

- **18 formats validated** (JSON, YAML, TOML, XML, HCL, INI, Properties, ENV, HOCON, CSV, Justfile, KDL, CUE, PList, EDITORCONFIG, JSONC, TOON, SARIF)
- **9 formats formatted** (JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, ENV)
- **Schema validation** via JSON Schema, XSD, and automatic [SchemaStore](https://www.schemastore.org/) lookup
- **Duplicate key detection** for YAML (always), JSON and INI (opt-in via `.cfv.toml`)
- **Gitignore-aware** file discovery
- [**Pre-commit hook**](https://boeing.github.io/config-file-validator/docs/integrations/pre-commit) and [**GitHub Action**](https://github.com/Boeing/validate-configs-action) with PR annotations

## Documentation

Full docs at [boeing.github.io/config-file-validator](https://boeing.github.io/config-file-validator):

- [CLI Reference](https://boeing.github.io/config-file-validator/docs/reference/cli-flags)
- [Configuration File](https://boeing.github.io/config-file-validator/docs/guides/configuration-file)
- [Schema Validation](https://boeing.github.io/config-file-validator/docs/guides/schema-validation)
- [Formatting](https://boeing.github.io/config-file-validator/docs/guides/formatting)
- [Using cfv with Existing Tools](https://boeing.github.io/config-file-validator/docs/guides/existing-tools)
- [Go Library](https://boeing.github.io/config-file-validator/docs/integrations/go-library)

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## License

[Apache 2.0](./LICENSE)
