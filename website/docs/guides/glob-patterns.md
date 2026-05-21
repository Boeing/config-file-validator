---
---

# Glob Patterns

By default, positional arguments are treated as directory paths. The `-globbing` flag tells the validator to expand them as glob patterns instead. This is required for `**` recursive matching, which most shells don't handle.

Note that `--schema-map`, `--type-map`, and their `.cfv.toml` equivalents always interpret their keys as globs. The `-globbing` flag only affects positional arguments.

## Basic usage

Validate all JSON files in a directory:

```shell
validator -globbing "configs/*.json"
```

Recursively match all YAML files:

```shell
validator -globbing "**/*.yaml"
```

Multiple patterns:

```shell
validator -globbing "configs/*.json" "deploy/**/*.yml"
```

Patterns must be quoted to prevent shell expansion.

## Mixing patterns and paths

Glob patterns and regular directory paths can be combined in the same invocation:

```shell
validator -globbing "src/**/*.json" ./config
```

The validator expands the glob patterns into file lists and traverses the directory paths normally. Results are merged into a single report.

## Pattern syntax

| Pattern | Matches |
|---------|---------|
| `*` | Any sequence of characters within a single path segment |
| `**` | Any sequence of characters across path segments (recursive) |
| `?` | Any single character |
| `[abc]` | Any character in the set |
| `[a-z]` | Any character in the range |
| `[!abc]` | Any character not in the set |

### Examples

| Pattern | Matches | Does not match |
|---------|---------|----------------|
| `*.json` | `config.json` | `dir/config.json` |
| `**/*.json` | `config.json`, `a/b/c.json` | |
| `config.{json,yaml}` | `config.json`, `config.yaml` | `config.toml` |
| `**/test?.yaml` | `test1.yaml`, `dir/testA.yaml` | `tests.yaml` |

## Configuration

In `.cfv.toml`:

```toml
globbing = true
```

With this set, all positional arguments are treated as glob patterns without needing the `-globbing` flag.

## Error handling

If a glob pattern matches no files, the validator reports an error and exits with code 2. If a pattern is syntactically invalid, the error message identifies the malformed pattern.
