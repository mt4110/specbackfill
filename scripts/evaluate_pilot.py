#!/usr/bin/env python3
"""Evaluate a public-safe specbackfill pilot scorecard CSV.

The script scores human-labeled deterministic companion obligations. It does
not read raw diffs, PR bodies, private review text, or review-history stores.
Rows should use opaque labels and short public-safe notes.
"""

from __future__ import annotations

import argparse
import csv
import re
import sys
from collections import Counter
from pathlib import Path


SCHEMA_VERSION = "pilot_scorecard.v1"
STABLE_OBLIGATION_ID_RE = re.compile(r"^obl-v1-[0-9a-f]{16}$")
OPAQUE_LABEL_RE = re.compile(r"^[A-Za-z0-9][A-Za-z0-9._-]{0,79}$")
PUBLIC_SAFETY_PATTERNS = [
    (
        re.compile(r"https?://", re.IGNORECASE),
        "URLs are not allowed in public-safe scorecard notes",
    ),
    (
        re.compile(r"[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}"),
        "email-like values are not allowed in public-safe scorecard notes",
    ),
    (
        re.compile(
            r"\b(?:gh[pousr]_[A-Za-z0-9_]{20,}|github_pat_[A-Za-z0-9_]{20,}|sk-[A-Za-z0-9_=-]{8,})"
        ),
        "token-like values are not allowed in public-safe scorecard notes",
    ),
    (
        re.compile(r"-----BEGIN [A-Z ]*PRIVATE KEY-----"),
        "private-key material is not allowed in public-safe scorecard notes",
    ),
]

REQUIRED_COLUMNS = [
    "schema_version",
    "sample_id",
    "source_label",
    "sample_ref",
    "rule_id",
    "obligation_id",
    "status",
    "severity",
    "operator_verdict",
    "fp_reason",
    "was_actioned",
    "duplicate_with_local_ai_review",
    "duplicate_with_review_firewall",
    "evidence_ok",
    "status_reason_ok",
    "notes",
]

STATUSES = {"satisfied", "missing", "unknown", "suppressed"}
SEVERITIES = {"info", "warn", "error", "unknown"}
VERDICTS = {
    "useful_fixed",
    "useful_noted",
    "covered_by_existing_safeguard",
    "false_positive",
    "unclear",
    "duplicate_with_local_ai_review",
    "duplicate_with_review_firewall",
    "not_actionable",
}
FP_REASONS = {
    "",
    "ambiguous_anchor",
    "companion_present_not_recognized",
    "docs_only",
    "example_only",
    "generated_only",
    "migration_only",
    "not_diff_local",
    "sample_only",
    "severity_too_high",
    "tests_only",
    "unrelated_companion",
    "other",
}
BOOL_FIELDS = {
    "was_actioned",
    "duplicate_with_local_ai_review",
    "duplicate_with_review_firewall",
    "evidence_ok",
    "status_reason_ok",
}

USEFUL_VERDICTS = {"useful_fixed", "useful_noted"}
FALSE_POSITIVE = "false_positive"
DUPLICATE_LOCAL_AI_REVIEW = "duplicate_with_local_ai_review"
DUPLICATE_REVIEW_FIREWALL = "duplicate_with_review_firewall"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("csv_path", help="pilot_scorecard.v1 CSV path")
    parser.add_argument(
        "--local-ai-review-import",
        choices=["yes", "no"],
        default="no",
        help="whether the pilot exercised the local_ai_review_import.v1 path",
    )
    parser.add_argument(
        "--allow-small-sample",
        action="store_true",
        help="allow samples below --min-sample-size for smoke tests and dry runs; continue/archive decisions still require a fair sample",
    )
    parser.add_argument(
        "--min-sample-size",
        type=int,
        default=30,
        help="minimum row count for a fair pilot sample",
    )
    parser.add_argument(
        "--suppression-iterations",
        type=int,
        default=0,
        help="completed suppression/fixture hardening iterations before archive checks",
    )
    parser.add_argument(
        "--unique-deterministic-artifact-value",
        choices=["yes", "no"],
        default="yes",
        help="whether the pilot found unique value in the deterministic artifact beyond duplicate review signals",
    )
    parser.add_argument(
        "--fail-on-archive",
        action="store_true",
        help="return exit code 1 when the computed decision is archive",
    )
    return parser.parse_args()


def parse_bool(raw: str, *, field: str, row_number: int) -> bool:
    value = str(raw).strip()
    if value in {"1", "true", "yes", "y"}:
        return True
    if value in {"0", "false", "no", "n"}:
        return False
    raise ValueError(
        f"row {row_number}: {field} must be one of "
        "'1', 'true', 'yes', 'y', '0', 'false', 'no', or 'n', "
        f"got {raw!r}"
    )


def rate(numerator: int, denominator: int) -> float:
    if denominator == 0:
        return 0.0
    return numerator / denominator


def read_rows(path: Path) -> list[dict[str, str]]:
    if not path.exists():
        raise ValueError(f"missing CSV: {path}")

    with path.open(newline="", encoding="utf-8") as handle:
        reader = csv.DictReader(handle)
        if reader.fieldnames is None:
            raise ValueError("CSV has no header")

        missing = [column for column in REQUIRED_COLUMNS if column not in reader.fieldnames]
        if missing:
            raise ValueError(f"CSV missing required columns: {', '.join(missing)}")
        unexpected = [column for column in reader.fieldnames if column not in REQUIRED_COLUMNS]
        if unexpected:
            raise ValueError(f"CSV has unexpected columns: {', '.join(unexpected)}")

        rows = list(reader)

    if not rows:
        raise ValueError("CSV has no scorecard rows")

    for index, row in enumerate(rows, start=2):
        validate_row(row, index)

    sample_id_counts = Counter((row.get("sample_id") or "").strip() for row in rows)
    duplicate_sample_ids = sorted(sample_id for sample_id, count in sample_id_counts.items() if count > 1)
    if duplicate_sample_ids:
        raise ValueError(f"CSV has duplicate sample_id values: {', '.join(duplicate_sample_ids)}")
    return rows


def validate_row(row: dict[str, str], row_number: int) -> None:
    if None in row:
        raise ValueError(f"row {row_number}: too many CSV columns; quote notes that contain commas")
    omitted = [column for column in REQUIRED_COLUMNS if row.get(column) is None]
    if omitted:
        raise ValueError(f"row {row_number}: missing required values: {', '.join(omitted)}")
    if (row.get("schema_version") or "").strip() != SCHEMA_VERSION:
        raise ValueError(f"row {row_number}: schema_version must be {SCHEMA_VERSION}")
    for field in ["sample_id", "source_label", "sample_ref", "rule_id", "obligation_id"]:
        if not (row.get(field) or "").strip():
            raise ValueError(f"row {row_number}: {field} is required")
    for field in ["sample_id", "source_label", "sample_ref"]:
        value = (row.get(field) or "").strip()
        if not OPAQUE_LABEL_RE.fullmatch(value):
            raise ValueError(
                f"row {row_number}: {field} must be an opaque public-safe label "
                "using only letters, numbers, '.', '_' or '-'"
            )
    if (row.get("status") or "").strip() not in STATUSES:
        raise ValueError(f"row {row_number}: invalid status {row.get('status')!r}")
    if (row.get("severity") or "").strip() not in SEVERITIES:
        raise ValueError(f"row {row_number}: invalid severity {row.get('severity')!r}")
    verdict = (row.get("operator_verdict") or "").strip()
    if verdict not in VERDICTS:
        raise ValueError(f"row {row_number}: invalid operator_verdict {verdict!r}")
    fp_reason = (row.get("fp_reason") or "").strip()
    if fp_reason not in FP_REASONS:
        raise ValueError(f"row {row_number}: invalid fp_reason {fp_reason!r}")
    if verdict == FALSE_POSITIVE and fp_reason == "":
        raise ValueError(f"row {row_number}: false_positive rows require fp_reason")
    if verdict != FALSE_POSITIVE and fp_reason != "":
        raise ValueError(f"row {row_number}: fp_reason should be blank unless verdict is false_positive")
    bools = {
        field: parse_bool(row.get(field, ""), field=field, row_number=row_number)
        for field in BOOL_FIELDS
    }
    if bools["duplicate_with_local_ai_review"] and bools["duplicate_with_review_firewall"]:
        raise ValueError(
            f"row {row_number}: duplicate_with_local_ai_review and duplicate_with_review_firewall cannot both be true"
        )
    if verdict == DUPLICATE_LOCAL_AI_REVIEW and not bools["duplicate_with_local_ai_review"]:
        raise ValueError(
            f"row {row_number}: duplicate_with_local_ai_review verdict requires duplicate_with_local_ai_review=true"
        )
    if verdict == DUPLICATE_REVIEW_FIREWALL and not bools["duplicate_with_review_firewall"]:
        raise ValueError(
            f"row {row_number}: duplicate_with_review_firewall verdict requires duplicate_with_review_firewall=true"
        )
    if verdict not in {DUPLICATE_LOCAL_AI_REVIEW, DUPLICATE_REVIEW_FIREWALL} and (
        bools["duplicate_with_local_ai_review"] or bools["duplicate_with_review_firewall"]
    ):
        raise ValueError(f"row {row_number}: duplicate flags require the matching duplicate operator_verdict")
    if verdict == "useful_fixed" and not bools["was_actioned"]:
        raise ValueError(f"row {row_number}: useful_fixed rows require was_actioned=true")
    if bools["was_actioned"] and verdict != "useful_fixed":
        raise ValueError(f"row {row_number}: was_actioned=true requires operator_verdict=useful_fixed")
    if len(row.get("notes") or "") > 240:
        raise ValueError(f"row {row_number}: notes must be 240 characters or fewer")
    validate_public_safe_notes(row.get("notes") or "", row_number)


def validate_public_safe_notes(notes: str, row_number: int) -> None:
    for pattern, message in PUBLIC_SAFETY_PATTERNS:
        if pattern.search(notes):
            raise ValueError(f"row {row_number}: {message}")


def bool_value(row: dict[str, str], field: str) -> bool:
    return str(row.get(field, "")).strip() in {"1", "true", "yes", "y"}


def count_duplicate_rows(rows: list[dict[str, str]], verdict: str, flag: str) -> int:
    return sum(
        1
        for row in rows
        if (row.get("operator_verdict") or "").strip() == verdict or bool_value(row, flag)
    )


def evaluate(rows: list[dict[str, str]], args: argparse.Namespace) -> tuple[str, dict[str, object]]:
    total = len(rows)
    verdicts = Counter((row.get("operator_verdict") or "").strip() for row in rows)
    rules = Counter((row.get("rule_id") or "unknown").strip() for row in rows)
    fp_reasons = Counter(
        (row.get("fp_reason") or "").strip()
        for row in rows
        if (row.get("operator_verdict") or "").strip() == FALSE_POSITIVE
    )

    useful = sum(verdicts[verdict] for verdict in USEFUL_VERDICTS)
    false_positive = verdicts[FALSE_POSITIVE]
    actioned = sum(1 for row in rows if bool_value(row, "was_actioned"))
    # `validate_row()` currently constrains actioned rows to the useful-fixed
    # verdict, so this metric is intentionally kept as an alias of `actioned`
    # for compatibility with downstream reporting.
    useful_actioned = actioned
    duplicate_local_ai_review = count_duplicate_rows(
        rows, DUPLICATE_LOCAL_AI_REVIEW, "duplicate_with_local_ai_review"
    )
    duplicate_review_firewall = count_duplicate_rows(
        rows, DUPLICATE_REVIEW_FIREWALL, "duplicate_with_review_firewall"
    )
    evidence_ok = sum(1 for row in rows if bool_value(row, "evidence_ok"))
    stable_ids = sum(
        1
        for row in rows
        if STABLE_OBLIGATION_ID_RE.fullmatch((row.get("obligation_id") or "").strip())
    )

    status_reason_rows = [
        row for row in rows if (row.get("status") or "").strip() in {"satisfied", "suppressed"}
    ]
    status_reason_ok = sum(1 for row in status_reason_rows if bool_value(row, "status_reason_ok"))

    error_rows = [row for row in rows if (row.get("severity") or "").strip() == "error"]
    hard_blocker_false_positive = sum(
        1
        for row in error_rows
        if (row.get("operator_verdict") or "").strip() == FALSE_POSITIVE
    )

    useful_rate = rate(useful, total)
    false_positive_rate = rate(false_positive, total)
    duplicate_local_ai_review_rate = rate(duplicate_local_ai_review, total)
    duplicate_review_firewall_rate = rate(duplicate_review_firewall, total)
    evidence_rate = rate(evidence_ok, total)
    stable_id_rate = rate(stable_ids, total)
    status_reason_rate = rate(status_reason_ok, len(status_reason_rows)) if status_reason_rows else 1.0
    hard_blocker_false_positive_rate = rate(hard_blocker_false_positive, len(error_rows))

    fair_sample = total >= args.min_sample_size
    sample_ok = fair_sample or args.allow_small_sample
    local_ai_review_import_ok = args.local_ai_review_import == "yes"

    source_labels = {(row.get("source_label") or "").strip() for row in rows}
    sample_refs = {(row.get("sample_ref") or "").strip() for row in rows}

    continue_checks = {
        "sample_size": fair_sample,
        "useful_obligations": useful >= 5 or useful_rate >= 0.20,
        "false_positive_rate": false_positive_rate <= 0.25,
        "hard_blocker_false_positive_rate": hard_blocker_false_positive_rate <= 0.10,
        "duplicate_with_local_ai_review_rate": duplicate_local_ai_review_rate <= 0.25,
        "duplicate_with_review_firewall_rate": duplicate_review_firewall_rate <= 0.10,
        "evidence_coverage": evidence_rate >= 0.95,
        "stable_deterministic_ids": stable_id_rate == 1.0,
        "status_reason_coverage": status_reason_rate >= 0.90,
        "local_ai_review_import_path": local_ai_review_import_ok,
    }

    week7_beta_dogfood_checks = {
        "rows_60": total >= 60,
        "repositories_or_corpora_3": len(source_labels) >= 3,
        "local_ai_review_import_path": local_ai_review_import_ok,
        "useful_human_action": useful_actioned >= 1,
    }
    beta_threshold_checks = {
        **continue_checks,
        **week7_beta_dogfood_checks,
        "useful_rate_25": useful_rate >= 0.25,
        "false_positive_rate_20": false_positive_rate <= 0.20,
        "error_severity_false_positive_rate_5": hard_blocker_false_positive_rate <= 0.05,
    }
    beta_blocking_consideration_checks = {
        **beta_threshold_checks,
        "useful_actioned_rows_10": useful_actioned >= 10,
    }

    archive_reasons = []
    if fair_sample and useful == 0:
        archive_reasons.append("AR-01: zero useful obligations after fair sample")
    if fair_sample and false_positive_rate > 0.50 and args.suppression_iterations >= 2:
        archive_reasons.append("AR-02: false-positive rate > 50% after two suppression iterations")
    if (
        fair_sample
        and duplicate_local_ai_review_rate >= 0.60
        and useful_rate < 0.20
        and args.unique_deterministic_artifact_value == "no"
    ):
        archive_reasons.append(
            "AR-03: mostly duplicate with local-ai-review without unique deterministic artifact value"
        )

    if archive_reasons:
        decision = "archive"
    elif all(continue_checks.values()):
        decision = "continue"
    else:
        decision = "continue_advisory_only"

    metrics: dict[str, object] = {
        "total": total,
        "sample_ok": sample_ok,
        "fair_sample": fair_sample,
        "source_label_count": len(source_labels),
        "sample_ref_count": len(sample_refs),
        "local_ai_review_import": args.local_ai_review_import,
        "useful": useful,
        "useful_rate": useful_rate,
        "false_positive": false_positive,
        "false_positive_rate": false_positive_rate,
        "actioned": actioned,
        "actioned_rate": rate(actioned, total),
        "useful_actioned": useful_actioned,
        "useful_actioned_rate": rate(useful_actioned, total),
        "hard_blocker_false_positive": hard_blocker_false_positive,
        "hard_blocker_false_positive_rate": hard_blocker_false_positive_rate,
        "duplicate_with_local_ai_review": duplicate_local_ai_review,
        "duplicate_with_local_ai_review_rate": duplicate_local_ai_review_rate,
        "duplicate_with_review_firewall": duplicate_review_firewall,
        "duplicate_with_review_firewall_rate": duplicate_review_firewall_rate,
        "evidence_ok": evidence_ok,
        "evidence_rate": evidence_rate,
        "stable_ids": stable_ids,
        "stable_id_rate": stable_id_rate,
        "status_reason_ok": status_reason_ok,
        "status_reason_applicable": len(status_reason_rows),
        "status_reason_rate": status_reason_rate,
        "verdicts": verdicts,
        "rules": rules,
        "fp_reasons": fp_reasons,
        "continue_checks": continue_checks,
        "week7_beta_dogfood_checks": week7_beta_dogfood_checks,
        "week7_beta_dogfood_ready": all(week7_beta_dogfood_checks.values()),
        "beta_threshold_checks": beta_threshold_checks,
        "beta_thresholds_pass": all(beta_threshold_checks.values()),
        "beta_blocking_consideration_checks": beta_blocking_consideration_checks,
        "beta_blocking_consideration_auto_checks_pass": all(
            beta_blocking_consideration_checks.values()
        ),
        "archive_reasons": archive_reasons,
    }
    return decision, metrics


def print_report(decision: str, metrics: dict[str, object]) -> None:
    print("# specbackfill pilot evaluation")
    print()
    print(f"rows: {metrics['total']}")
    print(f"sample_ok: {str(metrics['sample_ok']).lower()}")
    print(f"fair_sample: {str(metrics['fair_sample']).lower()}")
    print(f"source_labels: {metrics['source_label_count']}")
    print(f"sample_refs: {metrics['sample_ref_count']}")
    print(f"local_ai_review_import: {metrics['local_ai_review_import']}")
    print()
    print("## metrics")
    print(f"useful: {metrics['useful']} ({metrics['useful_rate']:.1%})")
    print(f"actioned: {metrics['actioned']} ({metrics['actioned_rate']:.1%})")
    print(f"useful_actioned: {metrics['useful_actioned']} ({metrics['useful_actioned_rate']:.1%})")
    print(f"false_positive: {metrics['false_positive']} ({metrics['false_positive_rate']:.1%})")
    print(
        "hard_blocker_false_positive: "
        f"{metrics['hard_blocker_false_positive']} ({metrics['hard_blocker_false_positive_rate']:.1%})"
    )
    print(
        "duplicate_with_local_ai_review: "
        f"{metrics['duplicate_with_local_ai_review']} ({metrics['duplicate_with_local_ai_review_rate']:.1%})"
    )
    print(
        "duplicate_with_review_firewall: "
        f"{metrics['duplicate_with_review_firewall']} ({metrics['duplicate_with_review_firewall_rate']:.1%})"
    )
    print(f"evidence_ok: {metrics['evidence_ok']} ({metrics['evidence_rate']:.1%})")
    print(f"stable_deterministic_ids: {metrics['stable_ids']} ({metrics['stable_id_rate']:.1%})")
    print(
        "status_reason_ok: "
        f"{metrics['status_reason_ok']}/{metrics['status_reason_applicable']} "
        f"({metrics['status_reason_rate']:.1%})"
    )
    print()
    print("## threshold checks")
    for name, passed in metrics["continue_checks"].items():
        label = "pass" if passed else "fail"
        print(f"- {name}: {label}")
    print()
    print("## week 7 beta dogfood checks")
    for name, passed in metrics["week7_beta_dogfood_checks"].items():
        label = "pass" if passed else "fail"
        print(f"- {name}: {label}")
    print(f"week7_beta_dogfood_ready: {str(metrics['week7_beta_dogfood_ready']).lower()}")
    print()
    print("## beta threshold checks")
    for name, passed in metrics["beta_threshold_checks"].items():
        label = "pass" if passed else "fail"
        print(f"- {name}: {label}")
    print(f"beta_thresholds_pass: {str(metrics['beta_thresholds_pass']).lower()}")
    print()
    print("## beta blocking consideration auto-checks")
    for name, passed in metrics["beta_blocking_consideration_checks"].items():
        label = "pass" if passed else "fail"
        print(f"- {name}: {label}")
    print(
        "beta_blocking_consideration_auto_checks_pass: "
        f"{str(metrics['beta_blocking_consideration_auto_checks_pass']).lower()}"
    )
    print("manual_checks_required: no high-severity privacy/release gaps; release artifact/install smoke")
    print()
    print("## verdicts")
    for key, value in metrics["verdicts"].most_common():
        print(f"- {key}: {value}")
    print()
    print("## rules")
    for key, value in metrics["rules"].most_common():
        print(f"- {key}: {value}")
    if metrics["fp_reasons"]:
        print()
        print("## false-positive reasons")
        for key, value in metrics["fp_reasons"].most_common():
            print(f"- {key or 'unspecified'}: {value}")
    print()
    print(f"decision: {decision}")
    if metrics["archive_reasons"]:
        print("archive_reasons:")
        for reason in metrics["archive_reasons"]:
            print(f"- {reason}")


def main() -> int:
    args = parse_args()
    try:
        if args.min_sample_size < 1:
            raise ValueError("--min-sample-size must be 1 or greater")
        if args.suppression_iterations < 0:
            raise ValueError("--suppression-iterations must be 0 or greater")
        rows = read_rows(Path(args.csv_path))
        decision, metrics = evaluate(rows, args)
    except (OSError, ValueError) as error:
        print(f"error: {error}", file=sys.stderr)
        return 2

    print_report(decision, metrics)
    if decision == "archive" and args.fail_on_archive:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
