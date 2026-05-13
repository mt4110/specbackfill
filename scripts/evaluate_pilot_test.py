#!/usr/bin/env python3
from __future__ import annotations

import csv
import importlib.util
import os
import tempfile
import unittest
from pathlib import Path
from types import SimpleNamespace


ROOT = Path(__file__).resolve().parent.parent
MODULE_PATH = ROOT / "scripts" / "evaluate_pilot.py"

spec = importlib.util.spec_from_file_location("evaluate_pilot", MODULE_PATH)
evaluate_pilot = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(evaluate_pilot)


class EvaluatePilotTest(unittest.TestCase):
    def test_sample_scorecard_stays_advisory_only(self) -> None:
        rows = evaluate_pilot.read_rows(ROOT / "examples" / "pilot_scorecard.sample.csv")
        decision, metrics = evaluate_pilot.evaluate(rows, args(local_ai_review_import="yes"))

        self.assertEqual(decision, "continue_advisory_only")
        self.assertEqual(metrics["useful"], 3)
        self.assertEqual(metrics["false_positive"], 1)
        self.assertEqual(metrics["duplicate_with_review_firewall"], 1)
        self.assertFalse(metrics["continue_checks"]["duplicate_with_review_firewall_rate"])
        self.assertTrue(metrics["continue_checks"]["local_ai_review_import_path"])

    def test_false_positive_requires_reason(self) -> None:
        path = write_scorecard([row(operator_verdict="false_positive")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "false_positive rows require fp_reason"):
            evaluate_pilot.read_rows(path)

    def test_non_false_positive_rejects_reason(self) -> None:
        path = write_scorecard([row(fp_reason="generated_only")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "fp_reason should be blank"):
            evaluate_pilot.read_rows(path)

    def test_extra_csv_columns_are_rejected(self) -> None:
        header = ",".join(evaluate_pilot.REQUIRED_COLUMNS)
        values = [row()[column] for column in evaluate_pilot.REQUIRED_COLUMNS]
        raw = header + "\n" + ",".join(values + ["extra"]) + "\n"
        path = write_raw_csv(raw)
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "too many CSV columns"):
            evaluate_pilot.read_rows(path)


def args(**overrides: object) -> SimpleNamespace:
    values = {
        "min_sample_size": 30,
        "allow_small_sample": True,
        "local_ai_review_import": "no",
        "suppression_iterations": 0,
        "unique_deterministic_artifact_value": "yes",
    }
    values.update(overrides)
    return SimpleNamespace(**values)


def row(**overrides: str) -> dict[str, str]:
    values = {
        "schema_version": "pilot_scorecard.v1",
        "sample_id": "S001",
        "source_label": "sample-service",
        "sample_ref": "synthetic-diff",
        "rule_id": "DB001",
        "obligation_id": "obl-v1-sample",
        "status": "missing",
        "severity": "warn",
        "operator_verdict": "useful_noted",
        "fp_reason": "",
        "was_actioned": "false",
        "duplicate_with_local_ai_review": "false",
        "duplicate_with_review_firewall": "false",
        "evidence_ok": "true",
        "status_reason_ok": "true",
        "notes": "public-safe note",
    }
    values.update(overrides)
    return values


def write_scorecard(rows: list[dict[str, str]]) -> Path:
    path = temp_csv_path()
    with path.open("w", newline="", encoding="utf-8") as handle:
        writer = csv.DictWriter(handle, fieldnames=evaluate_pilot.REQUIRED_COLUMNS)
        writer.writeheader()
        writer.writerows(rows)
    return path


def write_raw_csv(text: str) -> Path:
    path = temp_csv_path()
    path.write_text(text, encoding="utf-8")
    return path


def temp_csv_path() -> Path:
    fd, name = tempfile.mkstemp(suffix=".csv")
    os.close(fd)
    return Path(name)


if __name__ == "__main__":
    unittest.main()
