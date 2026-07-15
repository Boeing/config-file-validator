<p align="center">
  <img src="./img/logo.png" width="160" height="160" alt="Config File Validator logo"/>
</p>
<h1 align="center">Config File Validator</h1>

<p align="center">
<img id="cov" src="https://img.shields.io/badge/Coverage-91%25-brightgreen" alt="Code Coverage">

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

  <a href="https://goreportcard.com/report/github.com/Boeing/config-file-validator">
  <img src="https://goreportcard.com/badge/github.com/Boeing/config-file-validator" alt="Go Report Card">
  </a>

  <a href="https://github.com/boeing/config-file-validator/actions/workflows/go.yml">
  <img src="https://github.com/boeing/config-file-validator/actions/workflows/go.yml/badge.svg" alt="Pipeline Status">
  </a>
</p>

Validate and format every config file in your repository. One tool, one command, 18 formats.

## What it does

```shell
cfv check .       # Validate syntax + schema across all config files
cfv check --fix . # Fix what it can: trailing commas, type coercion
cfv format .      # Show formatting issues
cfv format --fix  # Fix formatting in-place
```

cfv replaces the per-format tools you're wiring together today — prettier, yamlfmt, taplo, terraform fmt, xmllint, jsonlint, v8r — with a single static binary.

## Install

**Homebrew**
```shell
brew install config-file-validator
```

**Go Install**
```shell
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

<details>
<summary>More install options</summary>

**Winget**
```shell
winget install Boeing.config-file-validator
```

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

<img src="./img/demo.svg" alt="Config File Validator output showing pass/fail results" width="800" />

Preview what formatting would change:

```shell
cfv format --diff .
```

See the [CLI reference](https://boeing.github.io/config-file-validator/docs/reference/cli-flags) for all options.

## Features

**Validation**
- Syntax checking across 18 file formats
- Schema validation via JSON Schema, XSD, and automatic [SchemaStore](https://www.schemastore.org/) lookup
- Duplicate key detection

**Formatting**
- Normalizes indentation, spacing, and trailing newlines across 9 formats (JSON, JSONC, YAML, TOML, HCL, XML, INI, Properties, ENV)
- Sort keys alphabetically (YAML, JSON, JSONC, TOML, Properties, INI)
- `--diff` preview without modifying files
- AST-driven YAML formatting — matches prettier and yamlfmt output
- Per-format config via [`.cfv.toml`](https://boeing.github.io/config-file-validator/docs/guides/configuration-file)

**Integrations**
- Auto-detects file types by extension and [known filename](https://boeing.github.io/config-file-validator/docs/reference/known-files)
- JSON, JUnit, SARIF, and GitHub output for CI pipelines
- [GitHub Action](https://github.com/Boeing/validate-configs-action) with PR annotations
- [Pre-commit hook](https://boeing.github.io/config-file-validator/docs/integrations/pre-commit)
- Usable as a [Go library](https://boeing.github.io/config-file-validator/docs/integrations/go-library)
- Gitignore-aware file discovery

## Supported Formats

| Format | Validate | Format | Schema |
|--------|:--------:|:------:|:------:|
| JSON | ✓ | ✓ | ✓ |
| JSONC | ✓ | ✓ | ✓ |
| YAML | ✓ | ✓ | ✓ |
| TOML | ✓ | ✓ | ✓ |
| XML | ✓ | ✓ | ✓ |
| HCL | ✓ | ✓ | |
| INI | ✓ | ✓ | |
| Properties | ✓ | ✓ | |
| ENV | ✓ | ✓ | |
| HOCON | ✓ | | |
| CSV | ✓ | | |
| EDITORCONFIG | ✓ | | |
| Justfile | ✓ | | |
| KDL | ✓ | | |
| CUE | ✓ | | |
| PList | ✓ | | |
| TOON | ✓ | | ✓ |
| SARIF | ✓ | | ✓ |

## Documentation

Full documentation at https://boeing.github.io/config-file-validator.

## Contributing

We welcome contributions! See the [contributing guide](./CONTRIBUTING.md).

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## License

[Apache 2.0](./LICENSE)
