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

    def test_pilot_scorecard_schema_rejects_useful_fixed_without_action(self) -> None:
        schema = load_schema()
        row = sample_row(operator_verdict="useful_fixed", was_actioned="false")

        errors = schema_validate_testdata.validate(row, schema, schema)

        self.assertTrue(any("was_actioned" in error and "expected one of" in error for error in errors), errors)

    def test_pilot_scorecard_schema_rejects_action_without_useful_fixed(self) -> None:
        schema = load_schema()
        row = sample_row(operator_verdict="useful_noted", was_actioned="true")

        errors = schema_validate_testdata.validate(row, schema, schema)

        self.assertTrue(any("operator_verdict" in error and "useful_fixed" in error for error in errors), errors)

    def test_additional_properties_with_csv_extra_column_are_reported(self) -> None:
        schema = load_schema()
        row = sample_row()
        row[None] = ["extra"]

        errors = schema_validate_testdata.validate(row, schema, schema)

        self.assertTrue(any("additional property None" in error for error in errors))

    def test_local_ai_review_import_keeps_additive_fields_optional(self) -> None:
        schema = load_import_schema()
        evidence = [{"file": "schema.prisma", "line": 3, "kind": "added", "excerpt": "email String @unique"}]
        items = [
            sample_import_item(),
            sample_import_item(
                finding_id=None,
                omission_signature=None,
                status="satisfied",
                required_companions=[
                    {
                        "kind": "migration_companion",
                        "status": "satisfied",
                        "satisfiers": ["prisma/migrations/20260329010101_add_email/migration.sql"],
                        "satisfier_evidence": evidence,
                        "expected_paths": ["prisma/migrations/**"],
                    }
                ],
            ),
            sample_import_item(
                finding_id=None,
                omission_signature=None,
                status="suppressed",
                required_companions=[
                    {
                        "kind": "migration_companion",
                        "status": "suppressed",
                        "satisfiers": [],
                        "satisfier_evidence": [],
                        "expected_paths": ["prisma/migrations/**"],
                    }
                ],
                suppression={"reason": "migration_only", "evidence": evidence},
            ),
        ]

        for item in items:
            errors = schema_validate_testdata.validate(item, schema, schema)
            self.assertEqual(errors, [], item["status"])

    def test_local_ai_review_import_raw_json_uses_obligation_shape(self) -> None:
        schema = load_import_schema()
        item = sample_import_item(
            status_reason=None,
            raw_json={
                "obligation_id": "obl-v1-0000000000000999",
                "rule_id": "DB001",
                "rule_version": "v0",
                "status": "missing",
                "severity": "error",
                "confidence": "high",
                "title": "Schema changed, but no matching migration companion moved with this diff",
                "why": "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
                "diff_local_claim": True,
                "anchor": {},
                "required_companions": [],
                "evidence": [],
                "suppression": None,
                "downstream": {},
            },
        )

        errors = schema_validate_testdata.validate(item, schema, schema)

        self.assertTrue(any("$.raw_json: missing required property 'finding_id'" in error for error in errors), errors)
        self.assertTrue(any("$.raw_json: missing required property 'omission_signature'" in error for error in errors), errors)


def load_schema() -> dict[str, object]:
    return json.loads((ROOT / "schemas" / "pilot_scorecard.schema.json").read_text(encoding="utf-8"))


def load_import_schema() -> dict[str, object]:
    return json.loads((ROOT / "schemas" / "local_ai_review_import.schema.json").read_text(encoding="utf-8"))


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


def sample_import_item(**overrides: object) -> dict[str, object]:
    evidence = [{"file": "schema.prisma", "line": 3, "kind": "added", "excerpt": "email String @unique"}]
    required_companions = [
        {
            "kind": "migration_companion",
            "status": "missing",
            "satisfiers": [],
            "satisfier_evidence": [],
            "expected_paths": ["prisma/migrations/**"],
        }
    ]
    item: dict[str, object] = {
        "schema_version": "local_ai_review_import.v1",
        "source": "specbackfill",
        "import_kind": "deterministic_static_layer",
        "source_signal": "specbackfill",
        "tool_version": "v0",
        "run_id": "run-0000000000000000",
        "input_kind": "diff_file",
        "diff_fingerprint": "sha256:" + "0" * 64,
        "item_id": "obl-v1-0000000000000999",
        "obligation_id": "obl-v1-0000000000000999",
        "finding_id": "v0-0000000000000999",
        "omission_signature": "db001.schema_changed.migration_companion",
        "rule_id": "DB001",
        "rule_version": "v0",
        "status": "missing",
        "severity": "error",
        "confidence": "high",
        "title": "Schema changed, but no matching migration companion moved with this diff",
        "why": "Schema-affecting lines moved in the diff, but no matching migration companion evidence moved with them.",
        "diff_local_claim": True,
        "evidence_digest": "sha256:" + "1" * 64,
        "anchor": {
            "kind": "schema_change",
            "path": "schema.prisma",
            "line": 3,
            "evidence": evidence,
        },
        "required_companions": required_companions,
        "evidence": evidence,
        "suppression": None,
    }
    item.update(overrides)
    return item


if __name__ == "__main__":
    unittest.main()
