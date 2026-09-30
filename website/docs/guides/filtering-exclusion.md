---
---

# Filtering & Exclusion

The validator provides several ways to control which files are validated. If any filter says skip a file, it's skipped.

## Exclude directories

Use `--exclude-dirs` to skip specific subdirectories during traversal:

```shell
cfv check --exclude-dirs=node_modules,vendor,.git .
```

The flag accepts a comma-separated list of directory names. These are matched against directory basenames at any depth — `node_modules` excludes every `node_modules` directory in the tree, not just the top-level one.

In `.cfv.toml`:

```toml
exclude-dirs = ["node_modules", "vendor", ".git", "dist", "build"]
```

## Exclude file types

Use `--exclude-file-types` to skip files of specific types:

```shell
cfv check --exclude-file-types=csv,env .
```

This filters by resolved file type, not by extension. Extensionless known files (like `.gitconfig` or `.babelrc`) are excluded when they resolve to an excluded type.

JSON and JSONC are treated as a family — excluding `json` excludes both. Similarly, `yaml` covers both `.yaml` and `.yml` files.

In `.cfv.toml`:

```toml
exclude-file-types = ["csv", "env"]
```

## Include only specific file types

Use `--file-types` to validate only the listed types:

```shell
cfv check --file-types=json,yaml,toml .
```

`--file-types` and `--exclude-file-types` cannot be used together.

In `.cfv.toml`:

```toml
file-types = ["json", "yaml", "toml"]
```

## Recursion depth

By default, the validator recurses without limit. Use `--depth` to restrict how deep it goes:

Disable recursion (only files in the immediate search path):

```shell
cfv check --depth=0 .
```

Limit to 2 levels deep:

```shell
cfv check --depth=2 .
```

In `.cfv.toml`:

```toml
depth = 3
```

## Gitignore

Use `--gitignore` to skip files and directories matched by `.gitignore` patterns:

```shell
cfv check --gitignore .
```

This respects:
- `.gitignore` files at every level of the directory tree
- `.git/info/exclude`
- The global git ignore file (configured via `core.excludesFile` in `~/.gitconfig`)

The flag is only active inside a Git repository. Outside a repo, it has no effect.

In `.cfv.toml`:

```toml
gitignore = true
```

## Ignore files

Use `--ignore-file` to apply gitignore-style patterns from files such as `.dockerignore`, `.prettierignore`, or `.eslintignore`:

```shell
cfv check --ignore-file=.dockerignore --ignore-file=.prettierignore .
```

Each path is resolved relative to the search path root. Missing ignore files are skipped, and repeated flags are additive.

In `.cfv.toml`:

```toml
ignore-files = [".dockerignore", ".prettierignore"]
```

With an environment variable:

```shell
CFV_IGNORE_FILES=.dockerignore,.prettierignore cfv check .
```

## Evaluation order

A file is validated only if it passes every active filter. During traversal, `cfv check` applies each filter in order:

1. Is the directory excluded by `--exclude-dirs`? → skip the entire directory
2. Is the file or directory matched by `.gitignore` or an explicit ignore file? → skip
3. Is the file deeper than `--depth`? → skip
4. Is the file's type excluded by `--exclude-file-types`? → skip
5. Is `--file-types` set and the file's type not in the list? → skip

If none of these apply, the file is validated.

## Common configurations

### Typical project

```toml
exclude-dirs = ["node_modules", ".git", "vendor", "dist"]
gitignore = true
ignore-files = [".dockerignore"]
```

### CI pipeline (strict)

```toml
exclude-dirs = ["node_modules", "vendor"]
gitignore = true
ignore-files = [".prettierignore"]
file-types = ["json", "yaml", "toml", "xml"]
```

### Monorepo (limit scope)

```toml
exclude-dirs = ["node_modules", ".git"]
depth = 4
gitignore = true
```
