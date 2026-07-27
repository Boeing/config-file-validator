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

`cfv` is a command-line tool that validates and formats configuration files.

```shell
cfv check .
```

<img src="./img/demo.svg" alt="Config File Validator output showing pass/fail results" width="800" />

## What it does

**Syntax validation** — Detects malformed files: missing braces, duplicate keys, invalid escape sequences.

**Schema validation** — Validates against JSON Schema and XSD. Matches files to [SchemaStore](https://www.schemastore.org/) schemas by filename.

**Formatting** — Checks indentation, spacing, and key ordering. Reads `.editorconfig`, `.prettierrc`, and `taplo.toml`. Output is compatible with prettier and taplo.

**Reporting** — Stdout, JSON, JUnit, or SARIF. Same format across all 18 file types.

## Usage

Validate syntax, schema, and formatting in one pass:

```shell
cfv .
```

Validate syntax and schema:

```shell
cfv check .
```

Validate and auto-fix what can be fixed:

```shell
cfv check --fix .
```

Check formatting:

```shell
cfv format .
```

Format in place:

```shell
cfv format --fix .
```

Preview formatting changes:

```shell
cfv format --diff .
```

## Supported formats

| Format | Syntax | Format | Schema |
|--------|:------:|:------:|:------:|
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

## Install

```shell
brew install config-file-validator
```

```shell
go install github.com/Boeing/config-file-validator/v3/cmd/cfv@latest
```

Single static binary. No runtime dependencies.

<details>
<summary>Winget, MacPorts, Scoop, binary releases</summary>

```shell
winget install Boeing.config-file-validator
```

```shell
sudo port install config-file-validator
```

```shell
scoop install config-file-validator
```

Download pre-built binaries for macOS, Linux, and Windows from [GitHub Releases](https://github.com/Boeing/config-file-validator/releases).

</details>

## Documentation

Full docs at [boeing.github.io/config-file-validator](https://boeing.github.io/config-file-validator).

## Contributing

Contributions welcome. See the [contributing guide](./CONTRIBUTING.md).

## Contributors

<a href="https://github.com/Boeing/config-file-validator/graphs/contributors">
  <img src="https://contrib.rocks/image?repo=Boeing/config-file-validator" alt="Config File Validator contributors" />
</a>

## License

[Apache 2.0](./LICENSE)
