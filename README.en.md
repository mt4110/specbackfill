<p align="right"><a href="./README.md">日本語</a></p>

# specbackfill / deterministic change-contract compiler for companion obligations

specbackfill is a rule-based **deterministic change-contract compiler** for git diffs. It extracts the **companion obligations** created by a change. The current v0 `check` command reports obligations that lack companion evidence in the same diff as **diff-local omission** findings.

It does not claim repository-wide absence. It asks which obligations this diff created, and whether the companion artifacts that satisfy them moved in the same diff.

It catches the boring review comments teams keep repeating, using only the diff:

- schema changed, but no migration moved in the same diff
- public API changed, but no contract test moved in the same diff
- env/config was introduced, but no default/docs companion moved in the same diff
- authz logic changed, but no allow/deny test moved in the same diff
- worker/retry behavior changed, but no runbook/observability companion moved in the same diff

The current repository contains the `specbackfill check` v0 MVP, the `specbackfill rules` command for inspecting implemented rules, the `specbackfill fixtures` command for fixture coverage, and verification fixtures.

This README is the English counterpart to the Japanese primary entry point. The behavioral source of truth is [docs/v0-spec.md](./docs/v0-spec.md), and contributor constraints live in [AGENTS.md](./AGENTS.md).

## What Centers This Repository

- [README.md](./README.md): Japanese primary entry point
- [README.en.md](./README.en.md): English counterpart
- [docs/v0-spec.md](./docs/v0-spec.md): v0 source of truth
- [AGENTS.md](./AGENTS.md): constraints for implementation and documentation changes
- [LICENSE](./LICENSE): license

## Product Boundary

- The detection core is **rule-based**. AI may explain findings later, but must not invent them.
- The central concept is a **companion obligation**. A finding is a user-facing unresolved obligation.
- Findings are strictly **diff-local**. The tool must not claim repository-wide absence.
- Every finding requires **evidence**. If the evidence cannot be shown, the finding must not be emitted.
- v0 must stand on diff input alone. It does not depend on PR titles, PR descriptions, or issue context.
- It is advisory-first. Until pilot evidence justifies stricter operation, do not position it primarily as a blocking gate.
- Obligation/status JSON is available explicitly through the `--emit-obligations` versioned artifact. Normal `--format json` remains the findings contract.
- When passing output to `local-ai-review`, `--emit-local-ai-review-import` emits JSONL for the deterministic static layer adapter. It does not create AI findings or PR comments.

When paired with a local LLM review system such as `local-ai-review`, specbackfill should act as the deterministic static layer and emit companion obligation output. The AI layer may explain, organize, or store those findings, but specbackfill itself must not invent AI findings.

specbackfill is not `local-ai-review` or `review-firewall`. specbackfill remains the source of truth for deterministic rule IDs, obligation semantics, evidence, fixtures, and CLI JSON. `local-ai-review` owns probabilistic review, history, and prompt calibration; `review-firewall` owns review-comment triage and routing.

## v0 Contract

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json|markdown]
                  [--fail-on error|warn|off]
                  [--summary]
                  [--explain]
                  [--emit-obligations]
                  [--emit-local-ai-review-import]
```

To inspect the implemented rules:

```bash
specbackfill rules list
specbackfill rules show DB001
```

When developing this repository, inspect fixture coverage for implemented rules from the specbackfill repository root:

```bash
specbackfill fixtures report
```

- Inputs: working tree diff / git range diff / unified diff file
- Outputs: `text`, `json`, or `markdown`
- Exit codes: `0` no findings, `1` findings at threshold, `2` tool error
- `--summary`: shows only severity counts and fired rules. It does not change finding evaluation.
- `--explain`: adds grounded explanations tied to existing findings. It does not add findings.
- `--emit-obligations`: emits an `obligations.v1` JSON artifact with `schema_version`, `tool`, `run`, and `obligations`. The artifact also exposes companion evidence for `satisfied` obligations and reason/evidence for `suppressed` obligations. Omit `--format` or use `--format json`.
- `--emit-local-ai-review-import`: emits `local_ai_review_import.v1` JSONL. Each line has deterministic item ID, run ID, rule ID, status, severity, title, diff-local evidence digest, status reason, raw obligation JSON, and `source/import_kind`. Do not combine it with `--format`.
- JSON findings include deterministic `finding_id` and `omission_signature` fields.
- Normal `--format json` is the findings contract. The obligation/status artifact is a separate contract described by [schemas/obligations.schema.json](./schemas/obligations.schema.json).
- The local-ai-review import JSONL adapter contract is described by [schemas/local_ai_review_import.schema.json](./schemas/local_ai_review_import.schema.json).
- `rules`: shows implemented default v0 rule IDs, severities, intent, and expected companions. It does not evaluate a diff.
- `fixtures`: shows synthetic fixture coverage by rule. It does not evaluate a diff.

See [docs/v0-spec.md](./docs/v0-spec.md) for the full normative contract and terminology.

## Install

As a user, install the CLI with `go install`:

```bash
go install github.com/mt4110/specbackfill/cmd/specbackfill@latest
specbackfill check --diff-file change.diff --format text --fail-on off
```

When trying the local checkout, `make install` installs the command to `~/.local/bin/specbackfill`:

```bash
make install
cd /path/to/another/project
specbackfill check --fail-on off
```

When developing this repository itself, `make trial` and `go run ./cmd/specbackfill` are also useful.

## CI Usage

In GitHub Actions, fetch the PR base/head locally and check them as a range diff. Until pilot thresholds pass, run with `--fail-on off` as advisory output.

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
          go-version-file: go.mod

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
- `--fail-on off`: keeps findings advisory and does not fail CI because of them. `warn` exits `1` for `warn` and `error` findings, and `error` exits `1` only for `error` findings.

## Local Verification

`make test` runs the pure Go/Python verification path. mise remains optional for local toolchain convenience.

```bash
make install
make trial
make test
make check
make pr BASE=main HEAD=HEAD
make patch DIFF=testdata/patches/db001_positive.diff
make json
make md
make summary
make rules
make rule RULE=DB001
make fixtures
make pilot-eval
```

Use `make test-mise` only when you explicitly want to check the mise task definition too.

`make trial` is a self-check for this repository. To check another project, install the command with `make install`, then run `specbackfill check --fail-on off` from that project's root.

The `input:` line shows which diff source was evaluated. `git range diff` does not include uncommitted working tree changes. `working tree diff` does not include untracked files unless they are made visible to git, for example with `git add -N`.

The `changed file summary:` line groups evaluated files into coarse categories such as docs, tests, migrations, API specs, and Go source. Each category shows up to three representative files. It is a reading aid only and does not change findings or exit codes.

## Trial Check Completion

Start with `specbackfill check --fail-on off` or an equivalent advisory check such as `make trial`. A trial check for a diff is done when:

- `make check` or `make pr BASE=main HEAD=HEAD` completes without a tool error
- any emitted finding is understandable from its rule ID, evidence, and expected companions
- wording stays diff-local and does not claim repository-wide absence
- noisy, confusing, or wrong findings are kept as candidates for synthetic fixtures
- `make trial`, or `make test` and `make fixtures`, is checked before moving into quality changes

If the same noisy pattern repeats, harden fixtures and suppressions in a small slice before turning the check into a blocking gate.

## Pilot Scorecard

Before turning this into a blocking check, score deterministic obligation output over real diffs. The public repository keeps only anonymized synthetic samples; do not commit real PR titles, bodies, comments, private review text, personal data, or raw private diffs.

```bash
specbackfill check --diff-file change.diff --emit-obligations --fail-on off > obligations.json
specbackfill check --diff-file change.diff --emit-local-ai-review-import --fail-on off > specbackfill-import.jsonl
python3 scripts/evaluate_pilot.py examples/pilot_scorecard.sample.csv --allow-small-sample --local-ai-review-import yes
```

The scorecard contract is [schemas/pilot_scorecard.schema.json](./schemas/pilot_scorecard.schema.json), and the synthetic sample is [examples/pilot_scorecard.sample.csv](./examples/pilot_scorecard.sample.csv). The evaluator returns one decision: `continue`, `continue_advisory_only`, or `archive`. `--allow-small-sample` is only for sample verification; scorecards below 30 rows cannot return `continue`.

`make pilot-eval` uses defaults for checking the synthetic sample. For a real pilot, pass explicit values such as `PILOT_SCORECARD=...` and `PILOT_EVAL_ARGS='--local-ai-review-import yes'`. Do not use `--allow-small-sample` for a green real-pilot decision; score 30+ real rows instead.

Use `duplicate_with_local_ai_review` only when local-ai-review independently produced the same concern before consuming specbackfill output. Use `duplicate_with_review_firewall` only when a pre-existing review or CI signal already had the same concern and review-firewall triaged it. Downstream consumption of specbackfill output is not a duplicate.

When recording a pilot decision in the public repository, use the v2 fields in [examples/pilot_decision_record.template.md](./examples/pilot_decision_record.template.md) and record only anonymized aggregate values from `scripts/evaluate_pilot.py`. If no real pilot data exists, use `Pilot status: not_run` / `Decision: pending` and do not infer metrics. Do not commit real PR titles, bodies, comments, private review text, raw private diffs, personal data, or proprietary repository names. The synthetic sample is for workflow verification, not as evidence for a pilot decision.

If no public-safe aggregate exists for the real pilot, do not create an evaluated decision record. Keep the project advisory-only and rely on the template and README workflow until real pilot evidence is available.

## Implemented Rules

- `DB001`: Schema changed, no migration moved with the diff
- `DB002`: Destructive storage change, no rollback/backfill note
- `API001`: Public API surface changed, no contract test moved
- `CFG001`: New config/env/flag introduced, no docs/default moved
- `AUTH001`: Authn/Authz branch changed, no allow/deny tests or security-sensitive note moved
- `ERR001`: Public error/status/code contract changed, no assertion test moved
- `OPS001`: Worker/queue/retry behavior changed, no observability/runbook moved
- `DOC001`: Generated spec changed, no hand-written explanation moved

## Fixture Coverage

specbackfill is tested against positive, companion-present, negative, and edge synthetic diff fixtures. When developing this repository, current coverage is visible with:

```bash
specbackfill fixtures report
```

The goal is not to maximize rule count. The goal is to keep findings quiet and evidence-backed.

## What It Does Not Claim

specbackfill does not say something is absent from the repository. It only checks whether the companion artifact moved with this diff.

To avoid noise, v0 is designed to suppress findings for cases such as:

- docs-only diffs
- tests-only diffs
- generated-file-only diffs
- example-only or top-level sample-only diffs where no production contract moved
- metadata-only renames
- companion artifacts that moved with concrete companion evidence

## How It Differs

specbackfill is not a general static analyzer, PR comment bot, or team-policy script.

- Semgrep finds code patterns and security/style issues.
- Danger automates team-specific PR chores.
- reviewdog reports linter findings on diffs.
- specbackfill checks whether implementation changes created companion obligations and whether the expected companion artifacts moved in the same diff.

The core finding is not "this code is wrong". The core finding is "this diff created obligation X, but no companion Y moved with this diff."

## License

[MIT License](./LICENSE)

## Status

- This repository is intentionally phase-limited.
- `docs/v0-spec.md` is the v0 source of truth.
- The README stays as an entry point and must not present unimplemented behavior as already shipped.
