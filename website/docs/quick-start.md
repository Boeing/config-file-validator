---
sidebar_position: 3
---

# Quick Start

This page walks through a first run of cfv. It assumes you have already [installed](./installation.md) the tool.

## Validate the current directory

Run `cfv check` with no arguments to validate all recognized config files in the current directory and its subdirectories:

```shell
cfv check
```

cfv finds files by extension (`.json`, `.yaml`, `.toml`, `.xml`, etc.) and by known filename (`.gitconfig`, `Pipfile`, `tsconfig.json`, etc.). It checks syntax, applies any declared schemas, and verifies formatting. Files that pass all three get ✓. Files with errors get ×. Files that are valid but not formatted get ~.

## Validate a specific path

Pass one or more paths as arguments:

```shell
cfv check ./config ./deploy/manifests
```

## Fix everything in one pass

To fix all safe issues at once (trailing commas, schema type coercion, and formatting):

```shell
cfv check --fix .
```

Files that can't be auto-fixed (missing required schema fields, unparseable syntax) are reported as errors.

## Check the results

A successful run exits with code `0`. If any file fails or needs formatting, the exit code is `1` and the output shows what went wrong.

## Common options

Exclude directories you don't want validated:

```shell
cfv check --exclude-dirs=node_modules,vendor,.git .
```

Limit to specific file types:

```shell
cfv check --file-types=json,yaml .
```

Enable automatic schema validation via [SchemaStore](https://www.schemastore.org/):

```shell
cfv check --schemastore .
```

Suppress all output (useful in scripts where you only check the exit code):

```shell
cfv check --quiet .
```

## Preview formatting changes

To see what cfv would change without modifying files:

```shell
cfv format --diff .
```

This prints a unified diff for each file that needs formatting. Nothing is written.

## Schema validation

Files that declare a `$schema` (JSON, TOML) or `yaml-language-server` comment (YAML) are validated against their schema automatically. No flags required.

To also validate files that *don't* declare a schema, use `--schemastore`. This looks up schemas by filename from the [SchemaStore](https://www.schemastore.org/) catalog, covering `package.json`, GitHub Actions workflows, `tsconfig.json`, and hundreds more:

```shell
cfv check --schemastore .
```

See the [Schema Validation](./guides/schema-validation.md) guide for details.

## Use a project config file

Instead of passing flags every time, create a `.cfv.toml` in your project root:

```toml
exclude-dirs = ["node_modules", ".git", "vendor"]
gitignore = true
schemastore = true
reporter = ["standard"]
```

cfv picks it up automatically. See [Configuration File](./guides/configuration-file.md) for all available options.

Flags can also be set via environment variables. See [Environment Variables](./reference/environment-variables.md) for the full list.

## Next steps

- [Schema Validation](./guides/schema-validation.md) — declare schemas, use SchemaStore, map schemas to files
- [Formatting Guide](./guides/formatting.md) — configure formatting options
- [Using cfv with Existing Tools](./guides/existing-tools.md) — how cfv reads your .prettierrc, taplo.toml, and .editorconfig
- [Reporters](./guides/output-reporters.md) — JSON, JUnit, SARIF output for CI
- [GitHub Actions](./integrations/github-actions.md) — run validation in your pipeline
- [CLI Reference](./reference/cli-flags.md) — complete list of flags
