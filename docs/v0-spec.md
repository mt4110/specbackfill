# specbackfill v0 Specification

| Field | Value |
| --- | --- |
| Status | v0 normative source of truth |
| Product | `specbackfill` |
| Primary command | `specbackfill check` |
| Detection core | rule-based |
| Finding semantics | diff-local omission |
| Repository note | This repository is phase-limited. This document defines the intended v0 contract; it does not prove implementation completeness. |

This document defines the normative v0 behavior for `specbackfill`.

- The README is the entry point.
- `AGENTS.md` defines change discipline.
- This file is the behavioral source of truth.

## 1. Normative Language

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are normative requirements in this document.

## 2. Product Definition

`specbackfill` is a CLI that analyzes code diffs and emits findings about **diff-local omissions**.

A diff-local omission is a claim of the following form:

> This diff appears to change something that normally requires companion artifacts, but the relevant companions did not move in the same diff.

The tool MUST NOT claim repository-wide truth when only diff-local evidence is available.

### Allowed wording

- "Schema changed, but no migration moved with this diff."
- "New config detected, but no docs/default companion moved with this diff."

### Not allowed wording

- "Migration is missing."
- "Tests are absent."
- "Documentation does not exist."
- Any statement that implies global absence from diff-local evidence alone.

## 3. Scope

v0 is intentionally narrow and optimized for backend/service repositories.

### Strongest expected targets

- Go services
- TypeScript / Node backends
- Prisma or SQL migration workflows
- OpenAPI, proto, or GraphQL schemas
- CLI/config/worker/queue behavior

### Out of scope for v0

| Category | Status |
| --- | --- |
| Repository-wide semantic truth | MUST NOT claim |
| AST-first parsing | MUST NOT require |
| Diff-external metadata | MUST NOT require |
| AI-dependent detection | MUST NOT require |
| Auto-fixes | out of scope |
| GitHub App behavior | out of scope |
| Automatic PR comment posting | out of scope |
| Repository-specific config files | out of scope |
| Rule DSLs | out of scope |
| Plugin systems | out of scope |
| Web services or dashboards | out of scope |
| Database connections or runtime inspection | out of scope |
| Background worker hosting or runtime inspection | out of scope |
| SARIF output | out of scope |

v0 MUST remain usable with diff input alone.

### Relationship to downstream review tools

`specbackfill` is not `local-ai-review` and MUST NOT become a broad AI review
system.

`specbackfill` owns deterministic companion-artifact checks: stable rule IDs,
diff-local evidence, required finding fields, text output, JSON output, and
exit behavior. Downstream tools, including `local-ai-review`, MAY consume
`specbackfill` JSON, store it, summarize it, or compare it with model or human
review findings. They SHOULD NOT reimplement the same deterministic companion
rules while `specbackfill` remains the active rule-engine source of truth.

## 4. Command-Line Interface

### 4.1 Primary command

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json]
                  [--fail-on error|warn|off]
                  [--explain]
```

### 4.2 Input sources

| Input source | When used | Requirement |
| --- | --- | --- |
| working tree diff | when neither `--base/--head` nor `--diff-file` is provided | MUST support |
| git range diff | when both `--base` and `--head` are provided | MUST support |
| unified diff file | when `--diff-file <file>` is provided | MUST support |

The implementation MAY reject invalid combinations of input flags as tool errors.

### 4.3 Output formats

v0 MUST support the following formats:

| Format | Role |
| --- | --- |
| `text` | human-oriented |
| `json` | CI and downstream processing |

`--explain` MAY add grounded explanations for existing findings. It MUST NOT create findings, change rule evaluation, or make the core command depend on AI availability.

### 4.4 Severity threshold

v0 MUST support the following `--fail-on` modes:

| Mode | Exit-code behavior |
| --- | --- |
| `error` | only `error` findings cause exit code `1` |
| `warn` | `error` and `warn` findings cause exit code `1` |
| `off` | findings never cause exit code `1` |

`info` findings MUST NOT cause exit code `1`.

### 4.5 Exit codes

The command MUST return:

| Exit code | Meaning |
| --- | --- |
| `0` | no findings exist at or above the configured threshold |
| `1` | findings exist at or above the configured threshold |
| `2` | tool error |

Tool errors include, but are not limited to:

- invalid flag combinations
- unreadable diff input
- malformed unified diff input
- git invocation failures for local/git-range diff acquisition
- unexpected internal failures

### 4.6 CI usage

CI systems SHOULD invoke the same `specbackfill check` CLI contract used locally.

For pull-request style checks, CI SHOULD either:

- provide both refs to `--base <ref> --head <ref>`, ensuring both commits are available in the local checkout
- provide a unified diff through `--diff-file <file>`

`text` output is appropriate for human-readable CI logs. `json` output is for CI and downstream processing.

`--fail-on` controls only whether findings cause exit code `1`; tool errors still cause exit code `2`.

CI integrations MUST NOT require `specbackfill` to depend on PR metadata, GitHub App behavior, automatic PR comment posting, SARIF output, network calls, or external services.

## 5. Diff Model

The implementation MUST normalize diff input into a structured model.

At minimum, the model MUST retain:

- changed file path
- file status when available
- one or more hunks per file when available
- added lines
- removed lines

The model SHOULD normalize paths to forward-slash style internally.
The model SHOULD behave consistently across macOS, Linux, and Windows.

The implementation MAY retain extra metadata, but the minimum model above is required for rule evaluation and evidence rendering.

## 6. Repo Profile

v0 MAY derive a **repo profile** from repository markers.

A repo profile is a secondary narrowing signal that helps determine which rules are likely relevant.

### Example markers

| Marker | Meaning |
| --- | --- |
| `go.mod` | Go repository |
| `package.json`, `tsconfig.json` | Node/TypeScript repository |
| `prisma/schema.prisma` | Prisma-related rules |
| `openapi*.yaml`, `openapi/**/*.yaml` | API spec rules |
| `proto/**/*.proto` | proto-related rules |
| `migrations/`, `db/migrations/`, `prisma/migrations/` | DB migration-related rules |

Repo profile signals MUST NOT be treated as primary proof.
Primary evidence for a finding MUST still come from the diff.

## 7. Findings

### 7.1 Required fields

Every emitted finding MUST include all of the following fields:

| Field | Requirement |
| --- | --- |
| `rule_id` | required |
| `severity` | required |
| `confidence` | required |
| `title` | required |
| `why` | required |
| `evidence` | required |
| `expected_companions` | required |

If the implementation cannot populate evidence, it MUST NOT emit the finding.

### 7.2 Severity

v0 uses the following severities:

- `error`
- `warn`
- `info`

Severity MUST reflect the likely seriousness of the missing companion relation, not the absolute seriousness of the code change itself.

### 7.3 Confidence

Confidence SHOULD be one of:

- `high`
- `medium`
- `low`

Confidence reflects the strength of the match between rule triggers and the available diff evidence.

A low-confidence finding SHOULD either be downgraded or suppressed if it would likely create noisy output.

### 7.4 Evidence

Evidence MUST be concrete and reviewer-visible.

Each evidence item SHOULD include at least:

- file path
- line kind (`added` or `removed` where applicable)
- excerpt from the hunk

Line numbers MAY be included when available.

### 7.5 Expected companions

`expected_companions` is the list of artifact categories the tool believes should likely have moved with the diff.

These are categories, not strict file paths.

Examples include:

- migration file
- migration test
- rollback/backfill note
- contract test
- API docs
- allow test
- deny test
- runbook update

### 7.6 Finding IDs

Implementations MAY include a `finding_id` for emitted findings.

When present, `finding_id` SHOULD be deterministic for the same finding content
and SHOULD be derived only from deterministic finding data such as `rule_id`,
evidence, and expected companion categories.

`finding_id` MUST NOT depend on:

- AI explanations
- model-authored findings
- downstream verdicts
- PR comments
- PR metadata
- GitHub metadata
- network calls
- wall-clock time
- random values

Downstream tools MAY store or compare `finding_id`, but MUST still preserve the
required finding fields.

## 8. Output Semantics

Human-readable output MUST preserve diff-local wording.

### Good wording

- "Schema changed, but no migration moved with this diff."
- "New config detected, but no docs/default companion moved with this diff."

### Bad wording

- "Migration is missing."
- "Tests are absent."
- "Documentation does not exist."

The output SHOULD make it easy for a reviewer to understand:

- what changed
- why the rule fired
- what companion artifacts are expected next

### 8.1 Text output

`text` output SHOULD be concise and evidence-first.

Recommended shape:

```text
[error] DB001 Schema changed, but no migration moved with this diff
  why:
    - persisted data shape changed in the diff
    - no migration file changed in the same diff
  evidence:
    - prisma/schema.prisma:+ email String @unique
  expected companions:
    - migration file
    - migration test
    - rollback/backfill note
```

### 8.2 JSON output

`json` output MUST be machine-usable and SHOULD have a stable top-level shape.

Recommended structure:

```json
{
  "version": "v0",
  "summary": {
    "error": 1,
    "warn": 2,
    "info": 0
  },
  "repo_profile": {
    "go": true,
    "node": false,
    "prisma": true,
    "openapi": true
  },
  "findings": [
    {
      "finding_id": "v0-74e68b9cb49588c2",
      "rule_id": "DB001",
      "severity": "error",
      "confidence": "high",
      "title": "Schema changed, but no migration moved with this diff",
      "why": "A persisted data shape changed in the diff, but no migration file was touched.",
      "evidence": [
        {
          "file": "prisma/schema.prisma",
          "line": 42,
          "kind": "added",
          "excerpt": "email String @unique"
        }
      ],
      "expected_companions": [
        "migration file",
        "migration test",
        "rollback/backfill note"
      ]
    }
  ]
}
```

Implementations MAY add extra fields, but MUST NOT remove the required semantic content.

When `--explain` is enabled, JSON output MAY include an additive top-level `explanations` field. Each explanation MUST be tied to an existing finding and SHOULD include:

- `finding_index`
- `rule_id`
- `source`
- `summary`
- `evidence`
- `expected_companions`

The `findings` array remains the deterministic machine contract. Explanations MUST preserve rule IDs, evidence references, and expected companion categories.

## 9. Default v0 Rule Pack

The default v0 rule pack includes the following rules.

### 9.1 Rule summary

| Rule | Intent | Expected companions | Severity guidance |
| --- | --- | --- | --- |
| `DB001` | schema changed, but no migration companion moved | migration file, migration test, rollback/backfill note | usually `error` when the migration companion is missing |
| `DB002` | destructive storage change without explicit safety path | rollback note, data backfill note, compatibility test | `warn` to `error` depending on confidence and destructive strength |
| `API001` | public API changed without contract verification | contract test, API docs, compatibility or deprecation note | usually `warn`; MAY be `error` for clearly breaking changes |
| `CFG001` | new config/env/flag introduced without docs/default handling | docs, default value handling, upgrade note | generally `warn` |
| `AUTH001` | authn/authz branch changed without allow/deny verification | allow test, deny test, security-sensitive note | generally `warn` |
| `ERR001` | public error/status/code contract changed without assertions | assertion test, API or client note | generally `warn` |
| `OPS001` | worker/queue/retry behavior changed without operational companions | observability note, runbook update, rollback path | generally `warn` |
| `DOC001` | generated spec changed without human explanation | human docs, upgrade note | `info` to `warn` |

### 9.2 DB001 — Schema changed, no migration moved with the diff

**Intent**
Detect a persisted data shape change whose diff does not include a migration companion.

**Typical triggers**

- file paths such as:
  - `schema.prisma`
  - `db/schema.sql`
  - `ent/schema/**`
  - `sqlc/schema/**`
- hunk lines containing patterns such as:
  - `CREATE TABLE`
  - `ALTER TABLE`
  - `DROP COLUMN`
  - `ADD COLUMN`
  - `CREATE INDEX`

**Expected companions**

- migration file
- migration test
- rollback/backfill note

**Severity guidance**

- If no migration companion is present, severity SHOULD be `error`.
- If a migration exists but migration test or rollback/backfill note is absent, severity SHOULD be `warn`.

### 9.3 DB002 — Destructive storage change, no rollback/backfill note

**Intent**
Detect potentially destructive storage changes that likely require an explicit safety path.

**Typical triggers**

- `DROP COLUMN`
- `DROP TABLE`
- nullable to non-null changes
- type narrowing
- unique constraint additions

**Expected companions**

- rollback note
- data backfill note
- compatibility test

**Severity guidance**

- Severity SHOULD be `warn` or `error` depending on the confidence and destructive strength of the detected change.

### 9.4 API001 — Public API surface changed, no contract test moved

**Intent**
Detect public API changes that likely require contract verification and explanation.

**Typical triggers**

- OpenAPI file changes
- proto file changes
- route definition changes
- public handler signature or response shape changes

**Expected companions**

- contract test
- API docs
- compatibility or deprecation note

**Severity guidance**

- Missing contract-test evidence SHOULD usually be `warn`.
- Clearly breaking API changes without a compatibility/deprecation note MAY be `error`.

### 9.5 CFG001 — New config/env/flag introduced, no docs/default moved

**Intent**
Detect operationally meaningful configuration additions without obvious documentation or default handling companions.

**Typical triggers**

- `os.Getenv(`
- `os.LookupEnv(`
- `viper.Get`
- `process.env.`
- CLI flag definition changes
- config struct/schema changes

**Expected companions**

- docs
- default value handling
- upgrade note

**Severity guidance**

- Severity SHOULD generally be `warn`.

### 9.6 AUTH001 — Authn/Authz branch changed, no allow/deny tests moved

**Intent**
Detect security-sensitive authorization logic changes without explicit positive/negative verification.

**Typical triggers**

- lines containing:
  - `authorize`
  - `permission`
  - `role`
  - `scope`
  - `forbidden`
  - `unauthorized`
- middleware / guard / policy changes

**Expected companions**

- allow test
- deny test
- security-sensitive note

**Severity guidance**

- Severity SHOULD generally be `warn`.

### 9.7 ERR001 — Public error/status/code contract changed, no assertion test moved

**Intent**
Detect contract-visible error changes without corresponding assertions or client notes.

**Typical triggers**

- HTTP status code changes
- gRPC code changes
- exported error code changes
- machine-readable error code changes
- user-facing API error shape changes

**Expected companions**

- assertion test
- API or client note

**Severity guidance**

- Severity SHOULD generally be `warn`.

### 9.8 OPS001 — Worker/queue/retry behavior changed, no observability/runbook moved

**Intent**
Detect operational behavior changes that likely require observability or operational guidance updates.

**Typical triggers**

- queue topic changes
- consumer behavior changes
- retry changes
- timeout changes
- cron changes
- backoff changes

**Expected companions**

- observability note
- runbook update
- rollback path

**Severity guidance**

- Severity SHOULD generally be `warn`.

### 9.9 DOC001 — Generated spec changed, no hand-written explanation moved

**Intent**
Detect generated/spec-driven interface changes without a human explanation artifact.

**Typical triggers**

- generated OpenAPI/proto/schema output changes
- generated client changes that strongly imply interface evolution

**Expected companions**

- human docs
- upgrade note

**Severity guidance**

- Severity SHOULD range from `info` to `warn`.

## 10. Rule Evaluation Guidance

### 10.1 Prefer multiple signals

Where possible, rules SHOULD combine:

- path evidence
- hunk content evidence
- companion absence within touched files

A rule SHOULD NOT rely only on broad path naming when a stronger hunk signal is available.

### 10.2 Conservative suppression

The implementation SHOULD avoid noisy findings for cases such as:

- generated-file-only diffs
- docs-only diffs
- tests-only diffs
- example/sample-only diffs where no production contract moved
- migration-only diffs that already represent the expected companion movement

### 10.3 Repo markers are narrowing hints

Repo profile markers SHOULD narrow applicability, not create findings by themselves.

## 11. Validation Expectations

v0 SHOULD be validated with golden diff fixtures.

### Recommended fixture classes

- schema change without migration
- schema change with migration
- destructive migration without rollback/backfill note
- API change without contract test
- env/config change without docs
- authz change without tests
- worker/retry change without runbook
- generated spec change without human docs

### Important negative coverage

- generated-file-only diffs
- docs-only diffs
- migration-only diffs
- companion artifacts already touched in the same diff

The implementation SHOULD keep text and JSON summaries semantically aligned.
The implementation SHOULD behave consistently across working-tree, git-range, and diff-file inputs.

## 12. AI Behavior

AI is out of scope for the detection core in v0.

The optional explanation layer is not a detection layer. It MUST satisfy all of the following:

- consume existing findings rather than inventing new ones
- preserve rule IDs
- preserve evidence references
- keep the core check command fully usable without AI availability
- avoid external service, PR metadata, GitHub App, PR comment posting, SARIF, repository mutation, and auto-fix requirements

If explanation output is unavailable, disabled, or empty, deterministic text and JSON findings MUST remain available.

## 13. Conformance Summary

A conforming v0 implementation of `specbackfill` MUST:

1. accept diff input from working tree, git range, and unified diff file
2. normalize the diff into a structured model
3. optionally derive repo profile markers
4. evaluate deterministic rules over the diff
5. emit only evidence-backed, diff-local findings
6. support `text` and `json` output
7. honor `--fail-on error|warn|off`
8. preserve exit codes `0`, `1`, and `2` as defined above
9. avoid repository-wide claims not supported by the diff
10. remain phase-limited and conservative in scope

The quality bar for this product is not breadth.
It is **precision, explicitness, and honesty about what the diff can actually support**.
