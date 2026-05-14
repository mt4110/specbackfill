#!/usr/bin/env python3
from __future__ import annotations

import csv
import contextlib
import importlib.util
import io
import os
import tempfile
import unittest
from unittest import mock
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
        self.assertTrue(metrics["sample_ok"])
        self.assertFalse(metrics["fair_sample"])
        self.assertEqual(metrics["useful"], 3)
        self.assertEqual(metrics["false_positive"], 1)
        self.assertEqual(metrics["duplicate_with_review_firewall"], 1)
        self.assertFalse(metrics["continue_checks"]["sample_size"])
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

    def test_unknown_header_columns_are_rejected(self) -> None:
        record = row()
        record["private_review_text"] = "should not be allowed"
        path = temp_csv_path()
        self.addCleanup(path.unlink, missing_ok=True)
        with path.open("w", newline="", encoding="utf-8") as handle:
            writer = csv.DictWriter(handle, fieldnames=evaluate_pilot.REQUIRED_COLUMNS + ["private_review_text"])
            writer.writeheader()
            writer.writerow(record)

        with self.assertRaisesRegex(ValueError, "unexpected columns: private_review_text"):
            evaluate_pilot.read_rows(path)

    def test_missing_trailing_notes_value_is_rejected(self) -> None:
        values = [row()[column] for column in evaluate_pilot.REQUIRED_COLUMNS[:-1]]
        raw = ",".join(evaluate_pilot.REQUIRED_COLUMNS) + "\n" + ",".join(values) + "\n"
        path = write_raw_csv(raw)
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "missing required values: notes"):
            evaluate_pilot.read_rows(path)

    def test_invalid_bool_message_lists_allowed_values(self) -> None:
        with self.assertRaisesRegex(ValueError, "'1', 'true', 'yes', 'y'"):
            evaluate_pilot.parse_bool("maybe", field="evidence_ok", row_number=2)

    def test_small_sample_override_does_not_trigger_archive(self) -> None:
        decision, metrics = evaluate_pilot.evaluate(
            [row(operator_verdict="not_actionable")],
            args(allow_small_sample=True),
        )

        self.assertEqual(decision, "continue_advisory_only")
        self.assertTrue(metrics["sample_ok"])
        self.assertFalse(metrics["fair_sample"])
        self.assertEqual(metrics["archive_reasons"], [])

    def test_small_sample_override_does_not_allow_continue(self) -> None:
        decision, metrics = evaluate_pilot.evaluate(
            [row(operator_verdict="useful_fixed", was_actioned="true")],
            args(allow_small_sample=True, local_ai_review_import="yes"),
        )

        self.assertEqual(decision, "continue_advisory_only")
        self.assertTrue(metrics["sample_ok"])
        self.assertFalse(metrics["fair_sample"])
        self.assertFalse(metrics["continue_checks"]["sample_size"])

    def test_fair_sample_can_continue_when_thresholds_pass(self) -> None:
        rows = [
            row(
                sample_id=f"S{index:03}",
                sample_ref=f"real-diff-{index:03}",
                obligation_id=f"obl-v1-{index:016x}",
            )
            for index in range(30)
        ]
        decision, metrics = evaluate_pilot.evaluate(
            rows,
            args(allow_small_sample=False, local_ai_review_import="yes"),
        )

        self.assertEqual(decision, "continue")
        self.assertTrue(metrics["fair_sample"])
        self.assertTrue(metrics["continue_checks"]["sample_size"])
        self.assertEqual(metrics["sample_ref_count"], 30)

    def test_duplicate_verdict_requires_matching_flag(self) -> None:
        path = write_scorecard([row(operator_verdict="duplicate_with_local_ai_review")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "requires duplicate_with_local_ai_review=true"):
            evaluate_pilot.read_rows(path)

    def test_duplicate_flag_requires_matching_verdict(self) -> None:
        path = write_scorecard([row(duplicate_with_review_firewall="true")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "duplicate flags require the matching duplicate operator_verdict"):
            evaluate_pilot.read_rows(path)

    def test_duplicate_flags_are_mutually_exclusive(self) -> None:
        path = write_scorecard(
            [
                row(
                    operator_verdict="duplicate_with_local_ai_review",
                    duplicate_with_local_ai_review="true",
                    duplicate_with_review_firewall="true",
                )
            ]
        )
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "cannot both be true"):
            evaluate_pilot.read_rows(path)

    def test_public_safe_labels_must_be_opaque(self) -> None:
        path = write_scorecard([row(source_label="org/private-repo")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "opaque public-safe label"):
            evaluate_pilot.read_rows(path)

    def test_sample_id_must_be_opaque(self) -> None:
        path = write_scorecard([row(sample_id="https://example.invalid/pr/1")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "opaque public-safe label"):
            evaluate_pilot.read_rows(path)

    def test_public_safe_notes_reject_email_like_values(self) -> None:
        path = write_scorecard([row(notes="follow up with person@example.com")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "email-like values"):
            evaluate_pilot.read_rows(path)

    def test_public_safe_notes_reject_github_token_like_values(self) -> None:
        path = write_scorecard([row(notes="token ghp_abcdefghijklmnopqrstuvwx")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "token-like values"):
            evaluate_pilot.read_rows(path)

    def test_public_safe_notes_reject_github_pat_like_values(self) -> None:
        path = write_scorecard([row(notes="token github_pat_abcdefghijklmnopqrstuvwx")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "token-like values"):
            evaluate_pilot.read_rows(path)

    def test_public_safe_notes_reject_openai_token_like_values(self) -> None:
        path = write_scorecard([row(notes="token sk-abcdefghijkl")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "token-like values"):
            evaluate_pilot.read_rows(path)

    def test_duplicate_sample_ids_are_rejected(self) -> None:
        path = write_scorecard([row(), row(rule_id="CFG001")])
        self.addCleanup(path.unlink, missing_ok=True)

        with self.assertRaisesRegex(ValueError, "duplicate sample_id"):
            evaluate_pilot.read_rows(path)

    def test_main_returns_tool_error_for_directory_input(self) -> None:
        with tempfile.TemporaryDirectory() as directory:
            stderr = io.StringIO()
            with mock.patch.object(evaluate_pilot.sys, "argv", ["evaluate_pilot.py", directory]):
                with contextlib.redirect_stderr(stderr):
                    code = evaluate_pilot.main()

        self.assertEqual(code, 2)
        self.assertIn("error:", stderr.getvalue())


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
        "obligation_id": "obl-v1-0000000000000001",
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
