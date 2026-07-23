---
sidebar_position: 1
slug: /introduction
---

import Head from '@docusaurus/Head';
import { SUPPORTED_FORMATS, SCHEMA_FORMATS, SYNTAX_FORMATS } from '@site/src/data/supportedFormats';

<Head>
  <meta name="description" content={`Validates config files across ${SUPPORTED_FORMATS.length} formats`} />
</Head>

# Introduction

Config File Validator validates config files across {SUPPORTED_FORMATS.length} formats.

It recursively searches directories for config files, detects their format by extension or filename, and reports errors.

## Supported formats

**Syntax + Schema:** {SCHEMA_FORMATS.map(f => `\`${f}\``).join(' ')}

**Syntax:** {SYNTAX_FORMATS.map(f => `\`${f}\``).join(' ')}

## When to use it

- **CI pipelines** — a [GitHub Action](./integrations/github-actions.md) posts validation results as PR comments with inline annotations. For other CI systems, use JSON, JUnit, or SARIF output.
- **Pre-commit hooks** — a ready-made [pre-commit hook](./integrations/pre-commit.md) validates changed config files on every commit. No setup beyond adding the hook.
- **Monorepos** — validates all config formats in a single pass. No per-format tooling to install or maintain.
- **Schema enforcement** — go beyond syntax checking. Require that config files declare and conform to a schema. Catch wrong field names, invalid values, and missing required keys — not just malformed syntax.

## Next steps

- [Installation](./installation.md) — install via Homebrew, Winget, `go install`, or binary download
- [Quick Start](./quick-start.md) — validate your first directory in under a minute
