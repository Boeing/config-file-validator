# Roadmap

## v3.0 — Formatting (RC)

Syntax + schema + formatting in one tool.

- `cfv format .` checks formatting across JSON, JSONC, YAML, TOML, XML, HCL, INI, Properties, ENV
- `cfv format --fix .` rewrites in-place
- `cfv format --diff .` previews changes
- Per-format config in `.cfv.toml`
- Binary renamed to `cfv`, subcommand-based CLI
- `cfv check --fix` auto-fixes JSON trailing commas and schema type coercion

## v3.1 — Stable

Bug fixes and feedback from the RC. No new features.

## v3.2 — Security Validation

Evaluate config files against community-maintained security rules using embedded OPA.

**User experience:**

```shell
cfv check --security .
cfv check --security --fix .
```

**How it works:**

- OPA embedded as a Go library (no external binary)
- Rules sourced from trivy-checks (500+ existing rules for K8s, Docker Compose, GitHub Actions, Terraform)
- File type identification via content inspection (apiVersion+kind → K8s, services: → docker-compose, path-based → GitHub Actions)
- Same pattern as `--schemastore` — cfv is the engine, the community owns the rules

**Severity gating:**

Only HIGH/CRITICAL block CI by default. Configurable via `.cfv.toml`:

```toml
[security]
fail-severity = "high"
```

**Baseline mode:**

Acknowledge pre-existing issues so day-one adoption isn't overwhelming:

```shell
cfv check --security --init .    # baseline existing state
cfv check --security .           # only new violations reported
```

**Auto-fix:**

Where a safe default exists (remove privileged: true, scope permissions, add resource limits), `--fix` applies it.

**Rule sources:**

| Source | Description |
|--------|-------------|
| Bundled | Curated subset shipped with the binary. Offline-capable. |
| Remote | Fetch latest from rule repo, cache locally. Optional. |
| Local | `--security-rules=path/` for custom/corporate policies. |

**Open questions:**

- Binary size impact (~15-20MB for OPA) — acceptable?
- Should `--security` default to on in a future major version?
- Rule customization (disable specific rules without a full baseline)
- Cross-file rules (e.g., "no NetworkPolicy for this namespace") — in scope or out?
