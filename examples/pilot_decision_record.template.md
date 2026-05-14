# Pilot Decision Record

This template is for a public-safe Phase 4 pilot decision after evaluating
`pilot_scorecard.v1` rows. Keep real pilot scorecards, raw private diffs,
private PR titles, descriptions, bodies, comments, private review text,
personal data, secrets, and proprietary repository names out of the public
repository.

## Summary

| Field | Value |
|---|---|
| Date | YYYY-MM-DD |
| Pilot status | not_run \| evaluated |
| Decision | pending \| continue \| continue_advisory_only \| archive |
| Scorecard source | none \| local/private aggregate \| public-safe aggregate |
| Evaluator command | not_run \| `python3 scripts/evaluate_pilot.py ...` |

If no real pilot data exists, set `Pilot status` to `not_run`, set `Decision`
to `pending`, and leave the metrics below as `TBD`. Do not infer or invent
numbers from synthetic samples. The synthetic sample is only for workflow
verification, not for a product decision.

If this record will be committed, use `public-safe aggregate` or `none` for
`Scorecard source`. Keep `local/private aggregate` only in uncommitted local
records.

## Public Safety

- [ ] Contains only anonymous labels and aggregate metrics.
- [ ] Contains no raw private diffs.
- [ ] Contains no private PR titles, descriptions, bodies, or comments.
- [ ] Contains no private review text.
- [ ] Contains no personal data, secrets, or proprietary repository names.
- [ ] Does not commit the real pilot scorecard.
- [ ] Commits only this public-safe aggregate decision record, if a record is committed.

## Sample

| Metric | Value |
|---|---:|
| Scorecard rows | TBD |
| Fair sample | yes/no/TBD |
| Number of diffs | TBD |
| Number of PRs | TBD |
| Changed-file groups | TBD |
| Contract-change classes covered | TBD |

## Evaluation Metrics

| Metric | Value |
|---|---:|
| Useful obligations | TBD |
| Useful rate | TBD |
| Actioned obligations | TBD |
| Actioned rate | TBD |
| False positives | TBD |
| False-positive rate | TBD |
| Hard-blocker false positives | TBD |
| Hard-blocker false-positive rate | TBD |
| Duplicate with local-ai-review rate | TBD |
| Duplicate with review-firewall rate | TBD |
| Evidence coverage | TBD |
| Stable deterministic ID coverage | TBD |
| Status-reason coverage | TBD |
| local-ai-review import path exercised | yes/no/TBD |

## Rule And Reason Summary

Top useful rules:

1. TBD
2. TBD
3. TBD

Top false-positive reasons:

1. TBD
2. TBD
3. TBD

## Boundary Review

- [ ] `specbackfill` remained a deterministic change-contract compiler for
      diff-local companion obligations.
- [ ] The pilot did not add AI/LLM detection to `specbackfill`.
- [ ] The pilot did not add PR comment posting to `specbackfill`.
- [ ] `local-ai-review` remained the probabilistic review and history owner.
- [ ] `review-firewall` remained the review-comment triage/routing owner.
- [ ] No Phase 5 rule graduation or new rule work is included in this record.

## Decision Rationale

```text
TBD
```

## Next Action

- [ ] Keep advisory-only and collect a real pilot sample.
- [ ] Continue to the next approved phase.
- [ ] Continue advisory-only with suppression/evidence hardening.
- [ ] Archive.
