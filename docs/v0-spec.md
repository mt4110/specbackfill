# specbackfill v0 Specification

| Field | Value |
| --- | --- |
| Status | v0 normative source of truth |
| Product | `specbackfill` |
| Primary command | `specbackfill check` |
| Product identity | deterministic change-contract compiler |
| Core concept | companion obligation |
| Detection core | rule-based |
| Finding semantics | diff-local omission for unresolved companion obligations |
| Repository note | This repository is phase-limited. This document defines the intended v0 contract; it does not prove implementation completeness. |

This document defines the normative v0 behavior for `specbackfill`.

- The README is the entry point.
- `AGENTS.md` defines change discipline.
- This file is the behavioral source of truth.

## 1. Normative Language

The key words **MUST**, **MUST NOT**, **SHOULD**, **SHOULD NOT**, and **MAY** are normative requirements in this document.

## 2. Product Definition

`specbackfill` is a CLI and deterministic change-contract compiler for git diffs.

It analyzes changed anchors in a diff, infers **companion obligations** that those changes appear to create, and reports unresolved obligations as **diff-local omission** findings.

A companion obligation is a rule-derived expectation that one or more companion artifact categories should move with a changed anchor in the same diff.

A diff-local omission finding is a claim of the following form:

> This diff appears to change something that normally requires companion artifacts, but the relevant companions did not move in the same diff.

The tool MUST NOT claim repository-wide truth when only diff-local evidence is available.

For current v0, `specbackfill check` MUST preserve the existing finding-oriented CLI contract. The obligation-first model defines product semantics and artifact boundaries. The explicit `--emit-obligations` mode emits a separately versioned obligation/status JSON artifact and MUST NOT remove or silently reinterpret the normal findings JSON contract.

### 2.1 Obligation status boundary

An explicit obligation artifact MAY classify companion obligations with these statuses:

| Status | Meaning |
| --- | --- |
| `satisfied` | companion evidence moved in the same diff |
| `missing` | required companion evidence did not move in the same diff |
| `unknown` | the diff created an anchor, but the rule cannot confidently decide companion status from diff evidence alone |
| `suppressed` | the rule matched an anchor, but a documented suppression or negative condition prevents an unresolved finding |

Status terms MUST remain diff-local. `unknown` MUST NOT be reported as `missing`. `suppressed` MUST be explainable when an implementation exposes suppression diagnostics.

Normal user-facing findings are the unresolved `missing` side of the obligation model. They MUST keep the required finding fields defined in this document.

Current v0 reserves `unknown` for a future rule state and the default rule pack does not emit it. When a default v0 rule cannot confidently decide status from diff evidence, it SHOULD either emit no obligation or emit a documented `suppressed` obligation when a negative condition is visible. `unknown` MUST remain non-finding output unless a future version explicitly changes that contract.

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

`specbackfill` owns deterministic companion-obligation checks: stable rule IDs,
diff-local evidence, companion status semantics, required finding fields, text
output, JSON output, and exit behavior. Downstream tools, including
`local-ai-review`, MAY consume `specbackfill` JSON, store it, summarize it, or
compare it with model or human review findings. They SHOULD NOT reimplement the
same deterministic companion rules while `specbackfill` remains the active
rule-engine source of truth.

`review-firewall` MAY consume `specbackfill` output as a review signal source,
but it MUST NOT become the owner of diff-local companion obligation detection.
`local-ai-review` owns probabilistic review, review history, and prompt
calibration. `review-firewall` owns review-comment triage and routing.

## 4. Command-Line Interface

### 4.0 Version surface

v0 MUST support:

```bash
specbackfill --version
```

The version output SHOULD be a single line that includes the CLI name, tool
version, commit, and build timestamp:

```text
specbackfill v0.1.0-alpha commit=<sha> built=<date>
```

Release builds SHOULD inject version metadata at build time. Source builds MAY
show `unknown` for commit or build timestamp when that metadata was not
provided.

The normal findings JSON top-level `version` field remains the v0 output
contract version. It is not the release binary version. The `tool.version`
field in `obligations.v1` artifacts and the `tool_version` field in
`local_ai_review_import.v1` items MUST use the same runtime tool version shown
by `specbackfill --version`.

### 4.1 Primary command

```bash
specbackfill check [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|json|markdown]
                  [--fail-on error|warn|off]
                  [--summary]
                  [--explain]
                  [--emit-obligations]
                  [--emit-local-ai-review-import]
```

### 4.2 Daily action command

v0 MUST support the following action-oriented command:

```bash
specbackfill todo [--base <ref> --head <ref> | --diff-file <file>]
                  [--format text|markdown]
                  [--fail-on error|warn|off]
```

The `todo` command evaluates the same deterministic companion obligations as
`specbackfill check`, but renders only unresolved obligations as a compact
action list. It MUST NOT create findings, change rule evaluation, call network
services, invoke non-deterministic detection, inspect PR metadata, or mutate
repository files.

Each unresolved obligation SHOULD include:

- rule ID and severity
- changed anchor
- missing companion category scoped to this diff
- a smallest next action

Common obligations SHOULD fit in fewer than 10 rendered lines. The command MUST
return exit code `1` using the same `--fail-on` threshold semantics as
`specbackfill check`.

The `todo` command intentionally does not emit JSON in v0. Machine-readable
callers SHOULD use `specbackfill check --format json`, `--emit-obligations`, or
`--emit-local-ai-review-import`.

### 4.3 Rule discovery commands

v0 MUST support the following rule discovery commands:

```bash
specbackfill rules list
specbackfill rules show <RULE_ID>
```

The `rules` command group shows implemented default v0 rule metadata. It MUST
NOT inspect a diff, emit findings, call network services, invoke AI, or change
`specbackfill check` behavior.

`rules list` SHOULD show implemented rule IDs, default severities, and concise
descriptions.

`rules show <RULE_ID>` SHOULD show the rule title, default severity, typical
triggers, expected companions, and common non-reporting cases.

The `rules` command group returns exit code `0` on success and exit code `2`
for invalid arguments, unknown rule IDs, or tool errors. It MUST NOT return
exit code `1` because it does not evaluate findings.

### 4.4 Fixture visibility command

v0 MAY support the following repository-maintainer command:

```bash
specbackfill fixtures report
```

The `fixtures` command group reports synthetic fixture coverage for the
implemented default v0 rules. It is a trust and contributor aid. It MUST NOT
emit findings, inspect PR metadata, call network services, invoke AI, or change
`specbackfill check` behavior.

`fixtures report` SHOULD show positive, companion-present, negative, and edge
fixture counts by rule.

### 4.5 Input sources

| Input source | When used | Requirement |
| --- | --- | --- |
| working tree diff | when neither `--base/--head` nor `--diff-file` is provided | MUST support |
| git range diff | when both `--base` and `--head` are provided | MUST support |
| unified diff file | when `--diff-file <file>` is provided | MUST support |

The implementation MAY reject invalid combinations of input flags as tool errors.

### 4.6 Output formats

v0 MUST support the following formats:

| Format | Role |
| --- | --- |
| `text` | human-oriented |
| `json` | CI and downstream processing |
| `markdown` | human-oriented Markdown output for logs, copied reports, or downstream posting by another tool |

`--explain` MAY add grounded explanations for existing findings. It MUST NOT create findings, change rule evaluation, or make the core command depend on AI availability.

`--summary` MAY render summary-only output for the selected format. Summary mode
MUST NOT change rule evaluation, finding identity, or exit-code threshold
behavior.

`--emit-obligations` MUST render a versioned obligation artifact as JSON. It is
an explicit output mode separate from `--format json`; the normal `json` format
MUST remain finding-oriented and backward compatible. `--emit-obligations` MUST
NOT change rule evaluation or exit-code threshold behavior.
`--emit-obligations` MAY be combined with `--format json` for caller clarity,
but MUST NOT be combined with `--format text`, `--format markdown`, `--summary`,
or `--explain`.
The obligation artifact MAY include satisfied and suppressed obligations so
callers can inspect why anchor or candidate evidence did not become a finding.

`--emit-local-ai-review-import` MUST render newline-delimited
`local_ai_review_import.v1` JSON items derived from the same deterministic
obligation artifact. It is an explicit downstream adapter mode, not the normal
findings JSON contract and not a replacement for `obligations.v1`.
It MUST NOT change rule evaluation or exit-code threshold behavior.
It MUST NOT be combined with `--format`, `--summary`, `--explain`, or
`--emit-obligations`.

### 4.7 Severity threshold

v0 MUST support the following `--fail-on` modes:

| Mode | Exit-code behavior |
| --- | --- |
| `error` | only `error` findings cause exit code `1` |
| `warn` | `error` and `warn` findings cause exit code `1` |
| `off` | findings never cause exit code `1` |

`info` findings MUST NOT cause exit code `1`.

### 4.8 Exit codes

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

### 4.9 CI usage

CI systems SHOULD invoke the same `specbackfill check` CLI contract used locally.

For pull-request style checks, CI SHOULD either:

- provide both refs to `--base <ref> --head <ref>`, ensuring both commits are available in the local checkout
- provide a unified diff through `--diff-file <file>`

`text` output is appropriate for human-readable CI logs. `markdown` output is
appropriate when another tool will copy or post the report. `json` output is
for CI and downstream processing.

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

In v0, a finding is the user-facing unresolved form of a companion obligation.
It does not mean the repository globally lacks a file, test, migration, or
document.

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
For obligation artifacts, hard machine-readable companion requirements live in `required_companions`. Finding-level `expected_companions` SHOULD NOT list recommended follow-up categories as if they were required unless the rule actually evaluates those categories as obligations.

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

### 7.7 Omission signatures

v0 findings include an `omission_signature`.

`omission_signature` MUST be a deterministic grouping key for the same class of
companion-artifact omission. It is less specific than `finding_id`: multiple
findings may share the same omission signature when they belong to the same
rule-level omission class.

`omission_signature` MUST NOT replace `rule_id`, `finding_id`, evidence, or
expected companion categories. It MUST NOT depend on AI explanations, downstream
verdicts, PR metadata, network calls, wall-clock time, or random values.

Downstream tools MAY use `omission_signature` for aggregation, fixture gap
analysis, or false-positive clustering. It MUST NOT change runtime detection.

## 8. Output Semantics

Human-readable output MUST preserve diff-local wording.

Human-readable output SHOULD show the input source that was evaluated. This
helps distinguish working tree diffs, `--base/--head` git range diffs, and
`--diff-file` input.

For git range input, output SHOULD make clear that uncommitted working tree
changes are not included. For working tree input, output SHOULD make clear that
untracked files are not included unless they are staged or made visible to git.

Human-readable output MAY include a coarse changed-file summary grouped by path
category, such as docs, tests, config/ci, migrations, API specs, or source
language. This summary MAY include a small bounded sample of file paths per
category plus a remaining count. It is only a navigation aid for the evaluated
diff. It MUST NOT affect rule evaluation, JSON findings, exit behavior, or
imply that the diff is complete.

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

When no findings are emitted, text output SHOULD make clear that no implemented
v0 companion-artifact rule fired for the diff. It MUST NOT imply that the diff
is complete or that repository-wide companions exist.

Human-readable output MAY include an anchor scan summary for no-finding output.
The summary SHOULD distinguish "no implemented v0 anchor evidence matched" from
"anchor evidence matched, but no finding remained after companion or suppression
checks." This summary is explanatory only and MUST NOT change finding
evaluation, JSON findings, or exit behavior.

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
```

### 8.2 JSON output

`json` output MUST be machine-usable and SHOULD have a stable top-level shape.
The normal v0 JSON contract is finding-oriented. The obligation artifact is a
separate explicit output mode and MUST NOT remove or silently reinterpret the
required finding fields defined by this document.

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
      "omission_signature": "db001.schema_changed.migration_companion",
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
        "migration file"
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

### 8.2.1 Obligation artifact output

When `--emit-obligations` is enabled, `specbackfill check` MUST emit a JSON
artifact with this top-level shape:

```json
{
  "schema_version": "obligations.v1",
  "tool": {
    "name": "specbackfill",
    "version": "v0"
  },
  "run": {
    "run_id": "run-...",
    "input_kind": "diff_file",
    "base": null,
    "head": null,
    "diff_fingerprint": "sha256:..."
  },
  "obligations": []
}
```

The artifact schema is `schemas/obligations.schema.json`. Each obligation MUST
include at least:

- `obligation_id`
- `rule_id`
- `status`
- `anchor`
- `required_companions`
- `evidence`
- `diff_local_claim`

Rules MAY emit zero or more obligations. Status is assigned per obligation
anchor group, not per rule globally, so one rule can emit both `satisfied` and
`missing` obligations for different anchors in the same diff.
This also means a rule MAY fan out into multiple findings when a diff creates
multiple independently unresolved anchors. Consumers SHOULD treat
`obligation_id` and anchor evidence as the stable unit for deduplication instead
of assuming one obligation per rule.

When an obligation is `satisfied`, companion-present evidence MUST be visible
from `required_companions[*].satisfier_evidence` and summarized by
`status_reason.reason: "companion_present"`.

When an obligation is `suppressed`, the artifact MUST include a `suppression`
object with a documented reason and concrete diff-local evidence. Current v0
suppression reasons are:

- `docs_only`
- `tests_only`
- `example_only`
- `sample_only`
- `generated_only`
- `migration_only`

Suppressed obligations MUST NOT become findings.

The artifact MUST preserve all of the following boundaries:

- deterministic obligation IDs
- deterministic rule IDs
- diff-local anchor evidence
- companion status values with no repository-wide absence claims
- versioned schema and explicit output mode
- downstream-safe separation from AI findings and review-comment routing

### 8.2.2 local-ai-review import JSONL

When `--emit-local-ai-review-import` is enabled, `specbackfill check` MUST emit
one newline-delimited JSON object per obligation using schema
`local_ai_review_import.v1`. The schema is
`schemas/local_ai_review_import.schema.json`.

This adapter format is intentionally smaller than the obligation artifact, but
each item MUST preserve the base deterministic metadata downstream review
history needs to score it separately from AI-authored findings:

- `schema_version`
- `source`
- `import_kind`
- `source_signal`
- `tool_version`
- `run_id`
- `input_kind`
- `diff_fingerprint`
- `item_id`
- `obligation_id`
- `finding_id`
- `omission_signature`
- `rule_id`
- `rule_version`
- `status`
- `severity`
- `confidence`
- `title`
- `why`
- `diff_local_claim`
- `evidence_digest`
- `anchor`
- `required_companions`
- `evidence`
- `suppression`

`source` and `source_signal` MUST be `specbackfill`. `import_kind` MUST be
`deterministic_static_layer`. `item_id` SHOULD be the deterministic
`obligation_id`. `evidence_digest` MUST be a deterministic SHA-256 digest over
the imported evidence and companion/suppression evidence for that item.

Current `specbackfill` producers MUST also emit:

- `status_reason`
- `raw_json`

`status_reason` MUST be emitted as `null` when no status reason applies.
`raw_json` MUST contain the normalized `obligations.v1` obligation object used
to derive the import item so downstream consumers can store the deterministic
source record separately from AI-authored findings.

Compatibility note: during the v0 productization line,
`local_ai_review_import.v1` is the active pre-release adapter contract used for
the Week 5 consumption proof. The v1 schema keeps additive producer fields such
as `status_reason` and `raw_json` optional so previously emitted v1 JSONL can
still validate under the same schema version. New producer output MUST include
those fields, and downstream consumers MUST preserve them when present. Removing
fields or changing the meaning of an existing field requires a new schema
version.

This format MUST preserve all obligation status values, including `satisfied`
and `suppressed`, so downstream tools can display deterministic obligations as
a separate layer from AI findings. It MUST NOT add review history storage,
prompt calibration, AI-generated findings, or PR comment posting to
`specbackfill`.

### 8.3 Markdown output

`markdown` output SHOULD contain the same finding semantics as `text` output in
a Markdown-friendly shape. It MUST preserve rule IDs, severities, why text,
evidence, expected companion categories, and diff-local wording.

When no findings are emitted, markdown output SHOULD preserve the same
diff-local non-claim as text output.

Recommended shape:

```md
### specbackfill findings

- Changed files: 2
- Findings: error=0 warn=1 info=0

#### [warn] API001 Public API changed, but no matching contract-test/docs companion moved with this diff

An explicit API spec file moved in the diff, but no matching contract-test/docs companion evidence moved with it.

Evidence:
- `openapi.yaml`: `+ /users:`

Expected companions:
- contract test
- API docs
- compatibility or deprecation note
```

### 8.4 Summary output

When `--summary` is enabled, output SHOULD focus on changed file count,
severity counts, and rule counts. It MUST NOT change finding evaluation or exit
behavior.

Recommended text shape:

```text
specbackfill summary
changed files: 2

error: 0
warn:  2
info:  0

Rules fired:
- API001: 1
- ERR001: 1
```

## 9. Default v0 Rule Pack

The default v0 rule pack includes the following rules.

### 9.1 Rule summary

| Rule | Intent | Expected companions | Severity guidance |
| --- | --- | --- | --- |
| `DB001` | schema changed, but no migration companion moved | migration file | usually `error` when the migration companion is missing |
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
  - `prisma/schema.prisma`
  - `prisma/**/*.prisma`
  - `db/schema.sql`
  - `ent/schema/**`
  - `sqlc/schema/**`
- hunk lines containing patterns such as:
  - `CREATE TABLE`
  - `ALTER TABLE`
  - `DROP COLUMN`
  - `ADD COLUMN`
  - `CREATE INDEX`

**Required companion**

- migration file

**Recommended follow-ups**

- migration test
- rollback/backfill note

Current v0 DB001 treats the migration file as the required companion. Migration tests and rollback/backfill notes are recommended follow-ups for higher-risk schema changes, but DB001 does not emit separate missing findings for them unless a future version adds atomic companion statuses.

**Severity guidance**

- If no migration companion is present, severity SHOULD be `error`.

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
- example-only or top-level sample-only diffs where no production contract moved
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
The implementation SHOULD cover obligation artifact JSON with golden fixtures
for positive, companion-present, and nearby negative diffs.
The implementation SHOULD behave consistently across working-tree, git-range, and diff-file inputs.

### 11.1 Pilot scorecard contract

v0 MAY include a public-safe pilot scorecard workflow for evaluating
deterministic obligation output over real diffs before tightening CI behavior or
adding rules for breadth.

The scorecard contract is `pilot_scorecard.v1`, described by
`schemas/pilot_scorecard.schema.json`. A scorecard row represents a human
operator verdict for one deterministic obligation or imported deterministic
item. The public sample CSV is synthetic and lives at
`examples/pilot_scorecard.sample.csv`.

The scorecard SHOULD measure at least:

- useful obligations (`useful_fixed` and `useful_noted`)
- false positives and false-positive reasons
- duplicate coverage with `local-ai-review`
- duplicate coverage with `review-firewall`
- diff-local evidence coverage through `evidence_ok`
- deterministic obligation ID coverage
- satisfied or suppressed status-reason clarity
- whether the `local_ai_review_import.v1` path was exercised

The evaluator script `scripts/evaluate_pilot.py` MAY compute a decision of
`continue`, `continue_advisory_only`, or `archive` from scorecard rows and
operator-provided integration flags. That decision is an evaluation result, not
a change to `specbackfill check` exit semantics.

Small synthetic samples MAY opt into sample-size override for smoke testing,
but `continue` and archive decisions SHOULD require the real-pilot minimum
sample size. A small sample MAY verify parsing, scoring, and report shape, but
MUST NOT be used as green product evidence.

Duplicate scorecard labels MUST mean independent prior signals. A
`duplicate_with_local_ai_review` row means `local-ai-review` independently
produced the same actionable concern before consuming `specbackfill` output. A
`duplicate_with_review_firewall` row means a pre-existing review or CI signal
already contained the same actionable concern and `review-firewall` triaged
that signal. Downstream storage, summary, routing, or display of
`specbackfill` output MUST NOT by itself count as a duplicate. The duplicate
boolean fields SHOULD match the duplicate operator verdict so scorecards cannot
accidentally double-count downstream consumption as independent coverage.

Public-safe scorecard labels SHOULD be opaque labels, not repository names,
URLs, personal identifiers, raw paths, PR titles, or review text. Public-safe
notes SHOULD remain short aggregate-safe notes and SHOULD NOT include URLs,
email-like values, token-like values, secrets, or raw diff excerpts.

Any pilot scorecard committed to the public repository MUST be public-safe. It
MUST NOT include raw private diffs, private PR titles, private PR bodies,
private PR comments, private review text, secrets, personal data, or
proprietary repository names. Committed scorecards SHOULD be synthetic samples
only. Real pilot scorecards SHOULD stay local, and real pilot results SHOULD be
represented publicly only as public-safe aggregate decision records.

The pilot workflow MUST NOT add AI/LLM detection, PR comment posting,
`local-ai-review` review-history storage, `review-firewall` triage/routing, new
companion rules, or stricter default CI failure behavior.

### 11.2 Pilot decision record

v0 MAY include a public-safe pilot decision record template for summarizing the
result of real pilot evaluation. The public template lives at
`examples/pilot_decision_record.template.md`.

A pilot decision record SHOULD be derived from `scripts/evaluate_pilot.py`
output and SHOULD contain only anonymous aggregate metrics, public-safe
rationale, and the final decision state. It MUST NOT include raw private diffs,
private PR titles, private PR bodies, private PR comments, private review
text, secrets, personal data, or proprietary repository names.

If no real pilot data exists, the decision record MUST say the pilot is not run
or pending. It MUST NOT infer, invent, or backfill metric values from intuition
or from the synthetic sample. The synthetic sample MAY verify the workflow, but
MUST NOT be used as the evidence base for a pilot decision.

If real pilot rows have not been evaluated into a public-safe aggregate, an
evaluated pilot decision record SHOULD NOT be committed. The project SHOULD
remain advisory-only with `Decision: pending` until public-safe aggregate
evidence exists.

`continue`, `continue_advisory_only`, and `archive` decisions SHOULD correspond
to the evaluator output and the strategy thresholds. Archive decisions SHOULD
only be treated as terminal after the real-pilot sample threshold is met.

A pilot decision record MUST NOT change `specbackfill check` behavior, default
CI failure behavior, JSON schema semantics, rule severity, rule coverage, or
the ownership boundaries between `specbackfill`, `local-ai-review`, and
`review-firewall`.

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
6. support `text`, `json`, and `markdown` output
7. honor `--fail-on error|warn|off`
8. preserve exit codes `0`, `1`, and `2` as defined above
9. avoid repository-wide claims not supported by the diff
10. support rule discovery commands for the implemented default v0 rules
11. support summary-only output without changing rule evaluation
12. support fixture coverage visibility for repository maintainers
13. include deterministic omission signatures for emitted findings
14. treat findings as unresolved companion obligations
15. support explicit versioned obligation artifact output without changing normal findings JSON
16. support `specbackfill --version` with build-time version injection for release builds
17. keep obligation/import tool metadata aligned with the CLI tool version
18. remain phase-limited and conservative in scope

The quality bar for this product is not breadth.
It is **precision, explicitness, and honesty about what the diff can actually support**.
