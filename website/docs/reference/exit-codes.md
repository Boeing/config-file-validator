---
---

# Exit Codes

## `cfv check`

| Code | Meaning |
|------|---------|
| `0`  | All files pass: valid syntax, schema correct, properly formatted |
| `1`  | One or more files failed (syntax error, schema violation, or formatting issue) |
| `2`  | Runtime or configuration error (invalid flags, unreadable files, bad `.cfv.toml`) |

## `cfv format`

| Code | Meaning |
|------|---------|
| `0`  | All files are already formatted correctly |
| `1`  | One or more files need formatting changes |
| `2`  | Runtime or configuration error |

## Usage in CI

Use exit code `1` as your CI gate. A non-zero exit means something needs attention.

Exit code `2` indicates a problem with the invocation itself, not with the files being checked.
