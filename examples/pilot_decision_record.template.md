# specbackfill Pilot Decision Record v2

This template is for a public-safe Week 3 real pilot decision after evaluating
`pilot_scorecard.v1` rows. Keep real pilot scorecards, raw private diffs,
private PR titles, descriptions, bodies, comments, private review text,
personal data, secrets, and proprietary repository names out of the public
repository.

If no real pilot data exists, set `Pilot status` to `not_run`, set `Decision`
to `pending`, and leave metrics as `TBD`. Do not infer or invent numbers from
synthetic samples.

## Commit Gate

- [ ] A real pilot was evaluated into a public-safe aggregate, or this record
      stays `Pilot status: not_run` / `Decision: pending`.
- [ ] Evaluated metrics are copied from `scripts/evaluate_pilot.py` output, not
      inferred from intuition or synthetic samples.
- [ ] The committed record contains only aggregate values and anonymous labels.
- [ ] The real pilot scorecard itself is not committed.

## Pilot Status

- [ ] not_run
- [ ] running
- [ ] complete

| Field | Value |
| --- | --- |
| Date | YYYY-MM-DD |
| Pilot window | TBD |
| Data sensitivity | public-safe aggregate only |
| Decision | pending / continue / continue_advisory_only / archive |
| Evaluator command | `python3 scripts/evaluate_pilot.py ...` |

## Sample

| Field | Value |
| --- | ---: |
| scorecard rows | TBD |
| fair sample | yes/no/TBD |
| sample refs | TBD |
| repositories/corpora | TBD |
| local-ai-review import exercised | yes/no/TBD |
| suppression iterations | TBD |

## Metrics

| Metric | Value | Threshold |
| --- | ---: | ---: |
| useful count | TBD | >= 5 |
| useful rate | TBD | >= 20% |
| actioned count | TBD | informational / >= 10 for beta |
| actioned rate | TBD | informational |
| false positive count | TBD | informational |
| false positive rate | TBD | <= 25% |
| hard blocker false positive count | TBD | informational |
| hard blocker false positive rate | TBD | <= 10% |
| duplicate with local-ai-review | TBD | <= 25% unless unique value proven |
| duplicate with review-firewall | TBD | <= 10% |
| evidence coverage | TBD | >= 95% |
| stable deterministic IDs | TBD | 100% |
| status reason coverage | TBD | >= 90% |

## Threshold Checks

- [ ] sample_size
- [ ] useful_obligations
- [ ] false_positive_rate
- [ ] hard_blocker_false_positive_rate
- [ ] duplicate_with_local_ai_review_rate
- [ ] duplicate_with_review_firewall_rate
- [ ] evidence_coverage
- [ ] stable_deterministic_ids
- [ ] status_reason_coverage
- [ ] local_ai_review_import_path

## False-Positive Buckets

| Bucket | Count | Fix planned |
| --- | ---: | --- |
| ambiguous_anchor | TBD | TBD |
| companion_present_not_recognized | TBD | TBD |
| docs_only | TBD | TBD |
| example_only | TBD | TBD |
| generated_only | TBD | TBD |
| migration_only | TBD | TBD |
| not_diff_local | TBD | TBD |
| sample_only | TBD | TBD |
| severity_too_high | TBD | TBD |
| tests_only | TBD | TBD |
| unrelated_companion | TBD | TBD |
| other | TBD | TBD |

## Duplicate Interpretation

Confirm duplicates were independent prior signals, not downstream consumption
of `specbackfill` output.

- [ ] local-ai-review duplicate definition checked
- [ ] review-firewall duplicate definition checked

## Decision

- [ ] pending
- [ ] continue
- [ ] continue_advisory_only
- [ ] archive

## Rationale

```text
TBD
```

## Follow-Up

- [ ] Keep advisory-only and collect a real pilot sample.
- [ ] Continue to the next approved phase.
- [ ] Continue advisory-only with suppression/evidence hardening.
- [ ] Archive or freeze.

## Privacy Check

- [ ] no raw private diffs
- [ ] no PR title/body/comment text
- [ ] no private review text
- [ ] no personal data
- [ ] no secrets
- [ ] no proprietary repo names
- [ ] no real pilot scorecard committed
