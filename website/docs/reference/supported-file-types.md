---
---

# Supported File Types

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

## Schema types

- JSON, JSONC, YAML, TOML, and TOON files are validated against [JSON Schema](https://json-schema.org/).
- XML files are validated against [XSD](https://www.w3.org/XML/Schema) (XML Schema Definition).
- SARIF files are validated against a built-in schema matched to the file's version field.

## File type families

- `json` includes both JSON and JSONC for filtering purposes (`--file-types`, `--exclude-file-types`).
- `yaml` includes both `.yaml` and `.yml`.

## Known files

Many files are recognized by filename regardless of extension. See [Known Files](./known-files.md).

Justfile is also recognized by the filenames `justfile`, `Justfile`, and `.justfile` regardless of extension.
