---
---

# File Type Detection

Most files are detected by extension. But the validator also recognizes common config files by name, so `tsconfig.json` is validated as JSONC, `Pipfile` as TOML, and `.gitconfig` as INI. You can also override detection explicitly with `--type-map`.

When multiple mechanisms could match, the validator checks them in this order:

1. **`--type-map` overrides** — explicit glob-to-type mappings take highest priority
2. **Known filenames** — files recognized by name regardless of extension
3. **File extension** — the standard fallback

## Known files

The validator recognizes common configuration files by filename, even without a standard extension. These mappings are sourced from [GitHub Linguist](https://github.com/github-linguist/linguist) and updated with each release.

Examples:

| Type  | Known filenames                                                                         |
|-------|-----------------------------------------------------------------------------------------|
| JSON  | `.arcconfig`, `.watchmanconfig`, `composer.lock`, `bun.lock`, `deno.lock`, `flake.lock` |
| JSONC | `.babelrc`, `.swcrc`, `.jshintrc`, `tsconfig.json`, `jsconfig.json`, `.eslintrc.json`   |
| YAML  | `.clang-format`, `.clang-tidy`, `.clangd`, `.gemrc`                                     |
| TOML  | `Pipfile`, `Cargo.lock`, `poetry.lock`, `uv.lock`, `pdm.lock`                           |
| XML   | `pom.xml`, `build.xml`, `ant.xml`, `.classpath`, `.project`                             |
| INI   | `.gitconfig`, `.gitmodules`, `.npmrc`, `.pylintrc`, `.flake8`, `.curlrc`, `.nanorc`     |

Known filenames take priority over extension matching. For example, `tsconfig.json` is validated as JSONC (not strict JSON) because it's a known JSONC file.

For the complete list, see [`known_files_gen.go`](https://github.com/Boeing/config-file-validator/blob/main/pkg/filetype/known_files_gen.go).

## JSON vs JSONC

Files with the `.json` extension are validated as **strict JSON** — no comments, no trailing commas.

Files with the `.jsonc` extension are validated as **JSONC**, which allows:
- `//` line comments
- `/* */` block comments
- Trailing commas

Many common `.json` files actually use JSONC syntax. The validator detects these by filename and validates them as JSONC automatically — `tsconfig.json`, `jsconfig.json`, and `.eslintrc.json` are a few examples. The full list is part of the [known files](https://github.com/Boeing/config-file-validator/blob/main/pkg/filetype/known_files_gen.go).

For other `.json` files that use JSONC syntax (e.g., VS Code settings), use `--type-map`:

```shell
validator --type-map="**/.vscode/*.json:jsonc" .
```

:::note
For filtering purposes, JSON and JSONC are treated as a family — `--file-types=json` includes both. Similarly, `yaml` covers both `.yaml` and `.yml`.
:::

## Overriding detection with --type-map

Use `--type-map` to force files matching a glob pattern to be validated as a specific type. This is useful for files without extensions or with non-standard extensions.

Treat files named "inventory" as INI:

```shell
validator --type-map="**/inventory:ini" .
```

Map all `.cfg` files to JSON:

```shell
validator --type-map="**/*.cfg:json" .
```

Specify multiple mappings:

```shell
validator --type-map="**/inventory:ini" --type-map="**/*.cfg:json" .
```

In `.cfv.toml`:

```toml
[type-map]
"**/inventory" = "ini"
"**/*.cfg" = "json"
"**/.vscode/*.json" = "jsonc"
```

For the list of valid type names, see [Supported Formats](../introduction.md#supported-formats).

## Files without extensions

If a file has no extension, the validator has no way to determine its type unless the filename is in the known files list (like `Pipfile` or `.gitconfig`) or you've explicitly mapped it with `--type-map`. Everything else is skipped.
