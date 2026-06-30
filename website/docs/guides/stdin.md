---
---

# Reading from Stdin

`cfv check` can read input from stdin instead of traversing the filesystem. Use `-` as the search path and specify the file type with `--file-types`.

## Basic usage

```shell
echo '{"key": "value"}' | cfv check --file-types=json -
```

```shell
cat config.yaml | cfv check --file-types=yaml -
```

## Requirements

- The search path must be exactly `-`
- `-` must be the only search path — it cannot be combined with other paths
- `--file-types` must specify exactly one file type
- Input is read until EOF

## Use cases

Validate config fetched from a remote source:

```shell
curl -s https://config-service.internal/app.json | cfv check --file-types=json -
```

Validate the output of a template engine or merge tool — the processed result, not the source file:

```shell
helm template my-chart | cfv check --file-types=yaml -
```

```shell
envsubst < config.template.yaml | cfv check --file-types=yaml -
```

## Behavior

When reading from stdin:

- The validator reads all input, parses it as the specified type, and reports pass or fail.
- Schema validation applies if the content declares a schema (e.g., a `$schema` property in JSON).
- `--schema-map` patterns do not apply since there is no filename to match against.
- The reported filename in output is `stdin`.
- Exit codes are the same as filesystem mode: `0` for valid, `1` for invalid.
