# AGENTS.md

Read this file and [`docs/v0-spec.md`](./docs/v0-spec.md) before making changes.

## 1. Mission

`specbackfill` is a phase-limited CLI for inferring **diff-local omissions** from code diffs.

It does **not** review overall code quality.
It does **not** claim repository-wide absence.
It asks a narrower question:

> For this diff, what companion artifacts should have moved with it, and which of them did not move in the same diff?

## 2. Canonical names

Use these names consistently:

- Repository name: `specbackfill`
- Binary / CLI name: `specbackfill`
- Primary command: `specbackfill check`
- Core concept: **companion artifacts**
- Finding semantics: **diff-local omission**
- Detection core: **rule-based**
- Normative spec: `docs/v0-spec.md`

Do not introduce alternate product names in code, docs, examples, or tests unless explicitly requested.

## 3. Product posture

`specbackfill` is not a general AI code reviewer.

It is a small deterministic pre-review gate for structural omissions that should be visible from the diff alone. Its value is not breadth or intelligence. Its value is quiet, needle-threading precision:

- concrete evidence
- reproducible rule output
- stable text and JSON reports
- conservative wording that does not overclaim

Think of it as a small blocker before human or AI review: if a diff changes a schema, API contract, config surface, public error/status contract, or generated spec/client artifact, `specbackfill` checks whether the expected companion artifacts moved in the same diff. It catches the loose threads between implementation changes and their companion artifacts.

This pairs cleanly with `local-ai-review`: `specbackfill` should emit evidence-backed, rule-based diff-local omissions first; `local-ai-review` can then perform broader local LLM review on the PR diff. Do not merge these roles. `specbackfill` should provide deterministic structure before AI interpretation, not become another generic AI reviewer.

`specbackfill` and `local-ai-review` are different products. `specbackfill` owns the deterministic companion-artifact rule engine: stable rule IDs, diff-local evidence, fixture coverage, text output, JSON output, and CLI exit behavior. `local-ai-review` may consume that output as an adapter or downstream review layer, but it must not become the place where `DB001`, `API001`, `CFG001`, or other deterministic companion rules are reimplemented.

If a proposed change makes the tool sound smarter than it is, depends on non-deterministic detection, claims repository-wide absence, or turns the project into a broad code review assistant, stop and keep the scope narrow.

## 4. Non-negotiables

- Detection must remain rule-based.
- Findings must remain diff-local.
- Every finding must include `rule_id`, `severity`, `confidence`, `title`, `why`, `evidence`, and `expected_companions`.
- If evidence cannot be shown, do not emit the finding.
- Rule evaluation must stand on diff input alone.
- Do not make v0 depend on PR metadata, issue trackers, network calls, or external services.

## 5. First PR readiness bar

The first PR that claims working behavior should be a verified v0 MVP, not a complete future product.

It should guarantee only the CLI contract and implemented rules included in that PR:

- `specbackfill check` works for `--diff-file`, working tree diff, and `--base/--head`.
- `--format text|json` output is stable and covered by tests or goldens.
- `--fail-on error|warn|off` exit behavior is covered by tests.
- Implemented rules are reproducible from diff input alone.
- README files distinguish implemented rules from planned rules.
- Local agent files such as `.codex/` and `.codex-*` are ignored and not committed.
- Internal prompt/design scratch files are not mixed into the first PR unless explicitly requested and cleaned for canonical naming.

For the current v0 MVP line, the implemented-rule bar is:

- `DB001`
- `DB002`
- `CFG001`
- `API001`
- `AUTH001`
- `ERR001`
- `OPS001`
- `DOC001`

## 6. Phase discipline

Implement only the requested phase.

Allowed default phases:

- Phase 0: docs scaffold
- Phase 1: `check` command skeleton
- Phase 2: default v0 rules
- Phase 3: fixture hardening / false-positive control
- Phase 4: CI usability
- Phase 5: AI explanation only
- Phase 6: design contract / ownership boundary only

Keep changes small.
Do not widen scope on your own.
Do not add future-facing abstractions unless the current phase requires them.

## 7. CLI contract

Unless a task explicitly changes it, preserve this contract:

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json|markdown]
                  [--fail-on error|warn|off]
                  [--summary]
```

Exit codes:

- `0`: no findings at or above the configured threshold
- `1`: findings exist at or above the configured threshold
- `2`: tool error

Do not silently change flags, defaults, exit codes, or output semantics.

## 8. Documentation policy

- `README.md` must remain the Japanese primary entry point.
- `README.en.md` must remain the English counterpart.
- Both READMEs must stay concise, implementation-oriented, and include top navigation links.
- `docs/v0-spec.md` is the v0 source of truth for behavior.
- `AGENTS.md` is for contributor and agent constraints, not the full behavioral spec.
- Keep the repo centered on `README.md`, `README.en.md`, `AGENTS.md`, `docs/v0-spec.md`, and `LICENSE`.
- Do not add badges.
- Do not create a docs forest unless explicitly requested.
- Do not claim features that are not implemented.

## 9. Rule authoring discipline

When adding or changing a rule:

1. Keep it diff-local.
2. Require evidence.
3. Specify expected companions.
4. Keep severity conservative.
5. Add positive and negative coverage when code exists.
6. Avoid noisy findings for generated-only, docs-only, and migration-only diffs.

Prefer multiple signals such as path match, hunk keyword match, and companion absence within touched files.
Do not emit broad findings from path names alone unless the phase explicitly allows it.

## 10. Rule promotion gate

A rule, suppression, companion-recognition change, or default severity change can be promoted only if it satisfies all of the following:

1. It preserves diff-local wording.
2. It has concrete diff evidence for every emitted finding.
3. It does not claim repository-wide absence.
4. It has at least one positive fixture when detection changes.
5. It has at least one nearby negative fixture when noise behavior changes.
6. It has at least one companion-present fixture when companion suppression changes.
7. It does not require AI, PR metadata, network calls, external services, or repository mutation.
8. It does not remove stable JSON fields or required finding fields.
9. It improves or preserves fixture coverage and report stability.

Before merging a rule change, reviewers should ask:

- Does the rule only inspect the current diff?
- Does every finding have concrete evidence?
- Are generated/docs/tests/example/sample-only negatives covered when relevant?
- Is companion-present suppression tested?
- Does JSON output remain stable or change only for an intentional contract update?
- Does text output remain conservative and understandable?

## 11. Change discipline

- Prefer the simplest implementation that satisfies the current task.
- Preserve existing behavior unless the task explicitly changes it.
- For docs-only tasks, do not change runtime behavior or command behavior.
- For behavior changes, keep README as an entry point summary and put normative details in `docs/v0-spec.md`.

The fastest way to damage this product is to make it sound smarter than it is.
Be narrow, explicit, and evidence-first.

## 12. `.private_docs` local design contract

If `.private_docs/` exists, treat it as local design context. It is intentionally ignored and should not be mixed into the public PR surface.

Every implementation, rule, fixture, CLI, output, or documentation change must still check whether the change satisfies the design contracts in `.private_docs/`.

Required behavior:

1. Read the relevant `.private_docs/*.md` files before changing behavior.
2. Preserve their invariants.
3. Do not commit `.private_docs/` unless the user explicitly asks to publish that local material.
4. If a local invariant should become public or project-binding, promote the public-safe wording into `docs/v0-spec.md`, `AGENTS.md`, or the READMEs instead of committing `.private_docs/`.
5. Do not store secrets, private diffs, tokens, personal data, or raw proprietary review data in `.private_docs/`.
6. If downstream data from `local-ai-review` is used for rule calibration, commit only public-safe summaries, fixtures, and design rationale outside `.private_docs/`.

A change that ignores `.private_docs/` is incomplete even if `go test ./...` passes.
