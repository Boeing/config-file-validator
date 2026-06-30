---
---

# Exit Codes

| Code | Meaning                                                                           |
|------|-----------------------------------------------------------------------------------|
| `0`  | All files are valid                                                               |
| `1`  | One or more validation errors (syntax or schema)                                  |
| `2`  | Runtime or configuration error (invalid flags, unreadable files, bad `.cfv.toml`) |

Use exit code `1` as a CI gate. Exit code `2` indicates a problem with the invocation itself — `cfv check` couldn't run as intended.
