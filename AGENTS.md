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

## 3. Non-negotiables

- Detection must remain rule-based.
- Findings must remain diff-local.
- Every finding must include `rule_id`, `severity`, `confidence`, `title`, `why`, `evidence`, and `expected_companions`.
- If evidence cannot be shown, do not emit the finding.
- Rule evaluation must stand on diff input alone.
- Do not make v0 depend on PR metadata, issue trackers, network calls, or external services.

## 4. Phase discipline

Implement only the requested phase.

Allowed default phases:

- Phase 0: docs scaffold
- Phase 1: `check` command skeleton
- Phase 2: default v0 rules
- Phase 3: fixture hardening / false-positive control
- Phase 4: CI usability
- Phase 5: AI explanation only

Keep changes small.
Do not widen scope on your own.
Do not add future-facing abstractions unless the current phase requires them.

## 5. CLI contract

Unless a task explicitly changes it, preserve this contract:

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json]
                  [--fail-on error|warn|off]
```

Exit codes:

- `0`: no findings at or above the configured threshold
- `1`: findings exist at or above the configured threshold
- `2`: tool error

Do not silently change flags, defaults, exit codes, or output semantics.

## 6. Documentation policy

- `README.md` must remain the Japanese primary entry point.
- `README.en.md` must remain the English counterpart.
- Both READMEs must stay concise, implementation-oriented, and include top navigation links.
- `docs/v0-spec.md` is the v0 source of truth for behavior.
- `AGENTS.md` is for contributor and agent constraints, not the full behavioral spec.
- Keep the repo centered on `README.md`, `README.en.md`, `AGENTS.md`, `docs/v0-spec.md`, and `LICENSE`.
- Do not add badges.
- Do not create a docs forest unless explicitly requested.
- Do not claim features that are not implemented.

## 7. Rule authoring discipline

When adding or changing a rule:

1. Keep it diff-local.
2. Require evidence.
3. Specify expected companions.
4. Keep severity conservative.
5. Add positive and negative coverage when code exists.
6. Avoid noisy findings for generated-only, docs-only, and migration-only diffs.

Prefer multiple signals such as path match, hunk keyword match, and companion absence within touched files.
Do not emit broad findings from path names alone unless the phase explicitly allows it.

## 8. Change discipline

- Prefer the simplest implementation that satisfies the current task.
- Preserve existing behavior unless the task explicitly changes it.
- For docs-only tasks, do not change runtime behavior or command behavior.
- For behavior changes, keep README as an entry point summary and put normative details in `docs/v0-spec.md`.

The fastest way to damage this product is to make it sound smarter than it is.
Be narrow, explicit, and evidence-first.
