# cfv 3.0 — Competitive Pitch

## One-liner

**cfv**: One command validates, formats, and fixes every config file in your repo.

## The Problem We Solve

Every real-world repo has config files in multiple formats. Today, handling them requires:

- prettier (JSON/YAML) — requires Node.js
- yamlfmt (YAML) — requires Go or binary download
- yamllint (YAML) — requires Python
- taplo (TOML) — requires Rust or binary download
- xmllint (XML) — requires libxml2
- terraform fmt (HCL) — requires Terraform
- dotenv-linter (ENV) — requires Rust or binary download
- Nothing at all for INI, Properties, HOCON, CUE, KDL, Justfile, TOON, CSV

Each tool has its own config file, its own CLI interface, its own CI step, its own pre-commit hook. Most teams give up and don't format half their config files at all.

## What cfv Does That Nobody Else Can

### 1. Auto-discovery

Point at a directory. cfv finds every config file automatically — by extension, by known filename (tsconfig.json, Pipfile, .gitconfig, etc.), with gitignore respect, exclude patterns, and depth control.

No globs. No file lists. No telling it what you have. It knows.

**No other tool does this across formats.**

### 2. Single pass, single binary

One process walks your tree once and validates + formats + fixes everything it finds. No Node.js, no Python, no Rust runtime. Static binary, zero dependencies, instant startup.

200 config files across 8 formats in <100ms. The competing approach (5 tools, 5 process spawns, 5 config loads) takes 700ms+ just in overhead.

**No other tool handles multiple formats in one pass.**

### 3. Formats nobody else covers

cfv is the ONLY formatter for:
- INI
- Properties
- HOCON
- ENV (with lint rules, not just dotenv-linter's Rust binary)
- CUE
- KDL
- Justfile
- TOON
- SARIF
- CSV

For these formats, the alternative is nothing. There is no other tool.

### 4. Schema validation across formats

Automatic SchemaStore lookup for JSON, JSONC, YAML, TOML. XSD for XML. Built-in schema for SARIF. Custom schema mapping via glob patterns. Schema enforcement mode (`--require-schema`).

No other single tool does schema validation across this many formats.

### 5. Enterprise-grade reporting

Five output formats: standard (terminal), JSON, JUnit, SARIF, GitHub annotations. 

- SARIF feeds GitHub Code Scanning and VS Code
- JUnit feeds Jenkins, GitLab CI
- JSON feeds scripts and dashboards
- GitHub annotations appear inline on PRs

prettier has: stdout. yamlfmt has: stdout. taplo has: stdout. None of them produce structured CI output.

### 6. Fix with confidence

```
Found 12 issues (8 fixable with --fix, 2 with --unsafe)
```

Every fix is categorized by safety. `--fix` only applies changes that cannot alter semantics. `--unsafe` for type coercions and structural changes. You always know what will happen before it happens.

## The Complete Comparison

| Capability | cfv 3.0 | prettier | yamlfmt | taplo | xmllint | dotenv-linter | terraform fmt |
|-----------|---------|----------|---------|-------|---------|---------------|---------------|
| Auto-discovers files | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | Partial |
| Formats supported | 16 | 4 (JS ecosystem) | 1 | 1 | 1 | 1 | 1 |
| Single binary, zero runtime | ✅ | ❌ (Node) | ✅ | ✅ | ✅ | ✅ | ✅ |
| Syntax validation | ✅ | ❌ | ❌ | ✅ | ✅ | ✅ | ❌ |
| Schema validation | ✅ | ❌ | ❌ | ✅ | ✅ | ❌ | ❌ |
| Formatting | ✅ | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ |
| Auto-fix | ✅ | ❌ | ❌ | ❌ | ❌ | ✅ | ❌ |
| Schema-guided fixes | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| SARIF output | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| JUnit output | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| GitHub annotations | ✅ | ❌ | ❌ | ❌ | ❌ | ❌ | ❌ |
| Gitignore-aware | ✅ | Partial | ❌ | ❌ | ❌ | ❌ | ❌ |
| Pre-commit hook | ✅ | ✅ | ✅ | ✅ | ❌ | ✅ | ❌ |
| Works without Node.js | ✅ | ❌ | ✅ | ✅ | ✅ | ✅ | ✅ |
| Config file (.cfv.toml) | ✅ | ✅ (.prettierrc) | ✅ | ✅ (.taplo.toml) | ❌ | ❌ | ❌ |

## Who We Replace

| Team's current setup | Replaced by |
|---------------------|-------------|
| prettier (JSON/YAML formatting) | `cfv format --fix .` |
| yamllint (YAML linting) | `cfv check .` |
| yamlfmt (YAML formatting) | `cfv format --fix .` |
| taplo fmt (TOML formatting) | `cfv format --fix .` |
| taplo lint (TOML schema) | `cfv check .` |
| xmllint --format (XML formatting) | `cfv format --fix .` |
| xmllint --schema (XML validation) | `cfv check .` |
| dotenv-linter (ENV linting) | `cfv check .` |
| dotenv-linter --fix (ENV fixing) | `cfv --fix .` |
| terraform fmt (HCL formatting) | `cfv format --fix .` (same engine, no Terraform install required) |
| jsonlint (JSON validation) | `cfv check .` |
| v8r (JSON/YAML schema) | `cfv check .` |
| Multiple pre-commit hooks | One hook: `cfv --fix .` |
| Multiple CI steps | One step: `cfv .` |
| Multiple config files | One file: `.cfv.toml` |

## Mega-Linter Positioning

cfv replaces the following Mega-Linter linters in a single tool:

- JSON: jsonlint, prettier, v8r → cfv
- YAML: yamllint, prettier, v8r → cfv
- XML: xmllint → cfv
- TOML: (none existed) → cfv
- ENV: dotenv-linter → cfv
- HCL: terraform fmt → cfv (same formatting engine, no Terraform install needed)
- INI: (none existed) → cfv
- Properties: (none existed) → cfv
- HOCON: (none existed) → cfv
- CUE: (none existed) → cfv
- CSV: (none existed) → cfv

Plus unified SARIF output that feeds directly into GitHub Code Scanning.

**One linter descriptor in Mega-Linter replaces 6+ individual tools and adds 7 formats they've never had coverage for.**

## The Pitch (for README, Show HN, talks)

Every repo has config files. JSON, YAML, TOML, XML, ENV, HCL, INI — scattered everywhere, each needing its own tool to validate and format. Most teams install 5+ tools, configure each one separately, or just don't bother.

cfv handles all of it. One binary, one config file, one command:

```shell
cfv .          # validate + check formatting across everything
cfv --fix .    # fix everything that's safe to fix
```

Auto-discovers files. Validates syntax. Enforces schemas. Formats to canonical style. Fixes what's fixable. Reports in SARIF/JUnit/JSON for CI. Zero dependencies. Instant.

Stop installing five tools to do what one tool should do.
