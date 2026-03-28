<p align="right"><a href="./README.md">日本語</a></p>

# specbackfill / 実装差分の追従漏れを点検する CLI (diff-local omission CLI)

specbackfill is a rule-based CLI that treats missing **companion artifacts** in a code diff as a **diff-local omission**. It does not claim repository-wide absence; it asks whether the things that should have moved with this diff actually moved in the same diff.

In the current repository state, the v0 contract and core documentation are being established first.

This README is the English counterpart to the Japanese primary entry point. The behavioral source of truth is [docs/v0-spec.md](./docs/v0-spec.md), and contributor constraints live in [AGENTS.md](./AGENTS.md).

## What Centers This Repository

- [README.md](./README.md): Japanese primary entry point
- [README.en.md](./README.en.md): English counterpart
- [docs/v0-spec.md](./docs/v0-spec.md): v0 source of truth
- [AGENTS.md](./AGENTS.md): constraints for implementation and documentation changes

## Product Boundary

- The detection core is **rule-based**. AI may explain findings later, but must not invent them.
- Findings are strictly **diff-local**. The tool must not claim repository-wide absence.
- Every finding requires **evidence**. If the evidence cannot be shown, the finding must not be emitted.
- v0 must stand on diff input alone. It does not depend on PR titles, PR descriptions, or issue context.

## v0 Contract

This repository is currently phase-limited and centered on docs/spec work. The command below is the v0 CLI contract summary, not a place to redefine behavior outside the spec.

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json]
                  [--fail-on error|warn|off]
```

- Inputs: working tree diff / git range diff / unified diff file
- Outputs: `text` or `json`
- Exit codes: `0` no findings, `1` findings at threshold, `2` tool error

See [docs/v0-spec.md](./docs/v0-spec.md) for the full normative contract and terminology.

## Default v0 Rules

- `DB001`: Schema changed, no migration moved with the diff
- `DB002`: Destructive storage change, no rollback/backfill note
- `API001`: Public API surface changed, no contract test moved
- `CFG001`: New config/env/flag introduced, no docs/default moved
- `AUTH001`: Authn/Authz branch changed, no allow/deny tests moved
- `ERR001`: Public error/status/code contract changed, no assertion test moved
- `OPS001`: Worker/queue/retry behavior changed, no observability/runbook moved
- `DOC001`: Generated spec changed, no hand-written explanation moved

## Status

- This repository is intentionally phase-limited.
- `docs/v0-spec.md` is the v0 source of truth.
- The README stays as an entry point and must not present unimplemented behavior as already shipped.
