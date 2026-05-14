#!/usr/bin/env python3
from __future__ import annotations

import csv
import importlib.util
import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parent.parent
MODULE_PATH = ROOT / "scripts" / "schema_validate_testdata.py"

spec = importlib.util.spec_from_file_location("schema_validate_testdata", MODULE_PATH)
schema_validate_testdata = importlib.util.module_from_spec(spec)
assert spec.loader is not None
spec.loader.exec_module(schema_validate_testdata)


class SchemaValidateTestdataTest(unittest.TestCase):
    def test_sample_pilot_scorecard_matches_schema(self) -> None:
        schema = load_schema()
        with (ROOT / "examples" / "pilot_scorecard.sample.csv").open(newline="", encoding="utf-8") as handle:
            rows = list(csv.DictReader(handle))

        self.assertGreater(len(rows), 0)
        for index, row in enumerate(rows, start=2):
            errors = schema_validate_testdata.validate(row, schema, schema)
            self.assertEqual(errors, [], f"row {index}: {errors}")

    def test_pilot_scorecard_schema_rejects_private_like_labels(self) -> None:
        schema = load_schema()
        row = sample_row(source_label="org/private-repo")

        errors = schema_validate_testdata.validate(row, schema, schema)

        self.assertTrue(any("$.source_label" in error and "pattern" in error for error in errors))

    def test_additional_properties_with_csv_extra_column_are_reported(self) -> None:
        schema = load_schema()
        row = sample_row()
        row[None] = ["extra"]

        errors = schema_validate_testdata.validate(row, schema, schema)

        self.assertTrue(any("additional property None" in error for error in errors))


def load_schema() -> dict[str, object]:
    return json.loads((ROOT / "schemas" / "pilot_scorecard.schema.json").read_text(encoding="utf-8"))


def sample_row(**overrides: str) -> dict[object, object]:
    row: dict[object, object] = {
        "schema_version": "pilot_scorecard.v1",
        "sample_id": "S999",
        "source_label": "sample-service",
        "sample_ref": "real-diff-999",
        "rule_id": "DB001",
        "obligation_id": "obl-v1-0000000000000999",
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
    row.update(overrides)
    return row


if __name__ == "__main__":
    unittest.main()
