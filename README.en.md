<p align="right"><a href="./README.md">日本語</a></p>

# specbackfill / 実装差分の追従漏れを点検する CLI (diff-local omission CLI)

specbackfill is a rule-based CLI that treats missing **companion artifacts** in a code diff as a **diff-local omission**. It does not claim repository-wide absence; it asks whether the things that should have moved with this diff actually moved in the same diff.

It catches the boring review comments teams keep repeating, using only the diff:

- schema changed, but no migration moved in the same diff
- public API changed, but no contract test moved in the same diff
- env/config was introduced, but no default/docs companion moved in the same diff
- authz logic changed, but no allow/deny test moved in the same diff
- worker/retry behavior changed, but no runbook/observability companion moved in the same diff

The current repository contains the `specbackfill check` v0 MVP and verification fixtures for the implemented rules.

This README is the English counterpart to the Japanese primary entry point. The behavioral source of truth is [docs/v0-spec.md](./docs/v0-spec.md), and contributor constraints live in [AGENTS.md](./AGENTS.md).

## What Centers This Repository

- [README.md](./README.md): Japanese primary entry point
- [README.en.md](./README.en.md): English counterpart
- [docs/v0-spec.md](./docs/v0-spec.md): v0 source of truth
- [AGENTS.md](./AGENTS.md): constraints for implementation and documentation changes
- [LICENSE](./LICENSE): license

## Product Boundary

- The detection core is **rule-based**. AI may explain findings later, but must not invent them.
- Findings are strictly **diff-local**. The tool must not claim repository-wide absence.
- Every finding requires **evidence**. If the evidence cannot be shown, the finding must not be emitted.
- v0 must stand on diff input alone. It does not depend on PR titles, PR descriptions, or issue context.
- It is a deterministic pre-review gate for catching small structural loose threads before AI or human review.

When paired with a local LLM review system such as `local-ai-review`, specbackfill should run first and emit rule-based omission findings. The AI layer may explain or organize those findings, but specbackfill itself must not invent AI findings.

specbackfill is not `local-ai-review`. specbackfill remains the source of truth for deterministic rule IDs, evidence, fixtures, and CLI JSON; `local-ai-review` is a downstream consumer when it chooses to ingest that output.

## v0 Contract

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json]
                  [--fail-on error|warn|off]
                  [--explain]
```

- Inputs: working tree diff / git range diff / unified diff file
- Outputs: `text` or `json`
- Exit codes: `0` no findings, `1` findings at threshold, `2` tool error
- `--explain`: adds grounded explanations tied to existing findings. It does not add findings.

See [docs/v0-spec.md](./docs/v0-spec.md) for the full normative contract and terminology.

## Install

As a user, install the CLI with `go install`:

```bash
go install github.com/mt4110/specbackfill/cmd/specbackfill@latest
specbackfill check --diff-file change.diff --format text --fail-on off
```

When developing this repository itself, `go run ./cmd/specbackfill` is also useful.

## CI Usage

In GitHub Actions, fetch the PR base/head locally and check them as a range diff. For a first rollout, start with `--fail-on off` as advisory output, review the noise level, then switch to `--fail-on warn` when the team trusts it.

```yaml
name: specbackfill

on:
  pull_request:

jobs:
  check:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v4
        with:
          fetch-depth: 0

      - uses: actions/setup-go@v5
        with:
          go-version: '1.26.2'

      - name: Install specbackfill
        run: go install github.com/mt4110/specbackfill/cmd/specbackfill@latest

      - name: Run specbackfill
        env:
          BASE_SHA: ${{ github.event.pull_request.base.sha }}
          HEAD_SHA: ${{ github.event.pull_request.head.sha }}
        run: |
          specbackfill check \
            --base "$BASE_SHA" \
            --head "$HEAD_SHA" \
            --format text \
            --fail-on off
```

- `fetch-depth: 0`: makes both `--base/--head` commits available locally.
- `--format text`: best for human-readable CI logs. `--format json` is for CI and downstream processing.
- `--fail-on warn`: exits `1` for `warn` and `error` findings. `error` fails only on `error`, and `off` never fails because of findings.

## Local Verification

With mise, install the repository toolchain before running checks:

```bash
mise install
mise run test
mise exec -- go run ./cmd/specbackfill check --diff-file testdata/patches/db001_positive.diff --format text --fail-on off
mise exec -- go run ./cmd/specbackfill check --diff-file testdata/patches/api001_err001_positive.diff --format json --fail-on off
mise exec -- go run ./cmd/specbackfill check --diff-file testdata/patches/db001_positive.diff --format json --fail-on off --explain
```

## Implemented Rules

- `DB001`: Schema changed, no migration moved with the diff
- `DB002`: Destructive storage change, no rollback/backfill note
- `API001`: Public API surface changed, no contract test moved
- `CFG001`: New config/env/flag introduced, no docs/default moved
- `AUTH001`: Authn/Authz branch changed, no allow/deny tests or security-sensitive note moved
- `ERR001`: Public error/status/code contract changed, no assertion test moved
- `OPS001`: Worker/queue/retry behavior changed, no observability/runbook moved
- `DOC001`: Generated spec changed, no hand-written explanation moved

## What It Does Not Claim

specbackfill does not say something is absent from the repository. It only checks whether the companion artifact moved with this diff.

To avoid noise, v0 is designed to suppress findings for cases such as:

- docs-only diffs
- tests-only diffs
- generated-file-only diffs
- example/sample-only diffs where no production contract moved
- metadata-only renames
- companion artifacts that moved with concrete companion evidence

## How It Differs

specbackfill is not a general static analyzer, PR comment bot, or team-policy script.

- Semgrep finds code patterns and security/style issues.
- Danger automates team-specific PR chores.
- reviewdog reports linter findings on diffs.
- specbackfill checks whether implementation changes moved with expected companion artifacts in the same diff.

The core finding is not "this code is wrong". The core finding is "this diff changed X, but no companion Y moved with this diff."

## License

[MIT License](./LICENSE)

## Status

- This repository is intentionally phase-limited.
- `docs/v0-spec.md` is the v0 source of truth.
- The README stays as an entry point and must not present unimplemented behavior as already shipped.
