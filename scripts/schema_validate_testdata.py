#!/usr/bin/env python3
"""Validate specbackfill schemas over synthetic patch and pilot fixtures."""

from __future__ import annotations

import argparse
import csv
import json
import os
import re
import shlex
import shutil
import subprocess
import sys
import tempfile
from pathlib import Path
from typing import Any


class SchemaError(Exception):
    pass


SUPPORTED_SCHEMA_KEYS = {
    "$schema",
    "$id",
    "$defs",
    "$ref",
    "additionalProperties",
    "allOf",
    "const",
    "contains",
    "description",
    "enum",
    "if",
    "items",
    "maxLength",
    "minItems",
    "minLength",
    "oneOf",
    "pattern",
    "properties",
    "required",
    "then",
    "title",
    "type",
}


def run(cmd: list[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(cmd, cwd=cwd, text=True, capture_output=True)


def go_command() -> list[str]:
    configured = os.environ.get("SPECBACKFILL_GO") or os.environ.get("GO")
    if configured:
        return shlex.split(configured)

    override = shutil.which("go")
    if override:
        return [override]

    mise = shutil.which("mise")
    if mise:
        goroot = command_stdout([mise, "exec", "--", "go", "env", "GOROOT"])
        if goroot:
            go_binary = Path(goroot) / "bin" / "go"
            if go_binary.exists():
                return [str(go_binary)]
        go_binary = command_stdout([mise, "exec", "--", "which", "go"])
        if go_binary and Path(go_binary).exists():
            return [go_binary]

    return ["go"]


def command_stdout(cmd: list[str]) -> str:
    result = subprocess.run(cmd, text=True, capture_output=True)
    if result.returncode != 0:
        return ""
    return result.stdout.strip()


def validate(instance: Any, schema: dict[str, Any], root: dict[str, Any], path: str = "$") -> list[str]:
    errors: list[str] = []

    ref = schema.get("$ref")
    if isinstance(ref, str):
        try:
            errors.extend(validate(instance, resolve_ref(ref, root), root, path))
        except SchemaError as error:
            errors.append(f"{path}: {error}")
        sibling_schema = {key: value for key, value in schema.items() if key != "$ref"}
        if not sibling_schema:
            return errors
        schema = sibling_schema

    if "allOf" in schema:
        for index, subschema in enumerate(schema["allOf"]):
            errors.extend(validate(instance, subschema, root, f"{path}.allOf[{index}]"))

    if "oneOf" in schema:
        matches = 0
        for subschema in schema["oneOf"]:
            if not validate(instance, subschema, root, path):
                matches += 1
        if matches != 1:
            errors.append(f"{path}: expected exactly one matching oneOf schema, got {matches}")

    if "if" in schema and not validate(instance, schema["if"], root, path):
        if "then" in schema:
            errors.extend(validate(instance, schema["then"], root, path))

    if "type" in schema and not type_matches(instance, schema["type"]):
        errors.append(f"{path}: expected type {schema['type']}, got {type_name(instance)}")
        return errors

    if "const" in schema and instance != schema["const"]:
        errors.append(f"{path}: expected const {schema['const']!r}, got {instance!r}")

    if "enum" in schema and instance not in schema["enum"]:
        errors.append(f"{path}: expected one of {schema['enum']!r}, got {instance!r}")

    if "pattern" in schema:
        if not isinstance(instance, str) or re.search(schema["pattern"], instance) is None:
            errors.append(f"{path}: value does not match pattern {schema['pattern']!r}")

    if "minLength" in schema and isinstance(instance, str) and len(instance) < int(schema["minLength"]):
        errors.append(f"{path}: string is shorter than minLength {schema['minLength']}")

    if "maxLength" in schema and isinstance(instance, str) and len(instance) > int(schema["maxLength"]):
        errors.append(f"{path}: string is longer than maxLength {schema['maxLength']}")

    if isinstance(instance, dict):
        required = schema.get("required", [])
        for key in required:
            if key not in instance:
                errors.append(f"{path}: missing required property {key!r}")

        properties = schema.get("properties", {})
        if isinstance(properties, dict):
            for key, subschema in properties.items():
                if key in instance:
                    errors.extend(validate(instance[key], subschema, root, f"{path}.{key}"))

            if schema.get("additionalProperties") is False:
                extra = sorted(set(instance) - set(properties), key=str)
                for key in extra:
                    errors.append(f"{path}: additional property {key!r} is not allowed")

    if isinstance(instance, list):
        if "minItems" in schema and len(instance) < int(schema["minItems"]):
            errors.append(f"{path}: array has fewer than minItems {schema['minItems']}")

        if "items" in schema:
            for index, item in enumerate(instance):
                errors.extend(validate(item, schema["items"], root, f"{path}[{index}]"))

        if "contains" in schema:
            if not any(not validate(item, schema["contains"], root, f"{path}[{index}]") for index, item in enumerate(instance)):
                errors.append(f"{path}: array does not contain a matching item")

    return errors


def unsupported_schema_keywords(schema: dict[str, Any], path: str = "$") -> list[str]:
    unsupported = sorted(set(schema) - SUPPORTED_SCHEMA_KEYS)
    errors = [f"{path}: unsupported schema keyword {key!r}" for key in unsupported]

    for container_key in ("properties", "$defs"):
        container = schema.get(container_key, {})
        if isinstance(container, dict):
            for key, subschema in container.items():
                if isinstance(subschema, dict):
                    errors.extend(unsupported_schema_keywords(subschema, f"{path}.{container_key}.{key}"))

    for key in ("items", "contains", "if", "then"):
        subschema = schema.get(key)
        if isinstance(subschema, dict):
            errors.extend(unsupported_schema_keywords(subschema, f"{path}.{key}"))

    for key in ("allOf", "oneOf"):
        subschemas = schema.get(key, [])
        if isinstance(subschemas, list):
            for index, subschema in enumerate(subschemas):
                if isinstance(subschema, dict):
                    errors.extend(unsupported_schema_keywords(subschema, f"{path}.{key}[{index}]"))

    return errors


def resolve_ref(ref: str, root: dict[str, Any]) -> dict[str, Any]:
    if not ref.startswith("#/"):
        raise SchemaError(f"unsupported ref {ref!r}")
    current: Any = root
    for part in ref[2:].split("/"):
        if not isinstance(current, dict) or part not in current:
            raise SchemaError(f"unresolved ref {ref!r}")
        current = current[part]
    if not isinstance(current, dict):
        raise SchemaError(f"ref {ref!r} does not point to a schema object")
    return current


def type_matches(instance: Any, expected: Any) -> bool:
    if isinstance(expected, list):
        return any(type_matches(instance, item) for item in expected)
    if expected == "object":
        return isinstance(instance, dict)
    if expected == "array":
        return isinstance(instance, list)
    if expected == "string":
        return isinstance(instance, str)
    if expected == "integer":
        return isinstance(instance, int) and not isinstance(instance, bool)
    if expected == "boolean":
        return isinstance(instance, bool)
    if expected == "null":
        return instance is None
    return False


def type_name(instance: Any) -> str:
    if instance is None:
        return "null"
    if isinstance(instance, bool):
        return "boolean"
    if isinstance(instance, dict):
        return "object"
    if isinstance(instance, list):
        return "array"
    if isinstance(instance, int):
        return "integer"
    if isinstance(instance, str):
        return "string"
    return type(instance).__name__


def main() -> int:
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--repo-root", default=".")
    args = parser.parse_args()

    root = Path(args.repo_root).resolve()
    patches = sorted((root / "testdata" / "patches").glob("*.diff"))
    if not patches:
        print("error: no testdata/patches/*.diff files found", file=sys.stderr)
        return 2

    obligations_schema = json.loads((root / "schemas" / "obligations.schema.json").read_text(encoding="utf-8"))
    import_schema = json.loads((root / "schemas" / "local_ai_review_import.schema.json").read_text(encoding="utf-8"))
    pilot_schema = json.loads((root / "schemas" / "pilot_scorecard.schema.json").read_text(encoding="utf-8"))
    unsupported = (
        unsupported_schema_keywords(obligations_schema)
        + unsupported_schema_keywords(import_schema)
        + unsupported_schema_keywords(pilot_schema)
    )
    if unsupported:
        for error in unsupported:
            print(f"error: {error}", file=sys.stderr)
        return 2

    failures: list[str] = []
    pilot_scorecard = root / "examples" / "pilot_scorecard.sample.csv"
    with pilot_scorecard.open(newline="", encoding="utf-8") as handle:
        pilot_rows = list(csv.DictReader(handle))
    if not pilot_rows:
        failures.append(f"{pilot_scorecard.name}: no sample rows")
    for index, row in enumerate(pilot_rows, start=2):
        errors = validate(row, pilot_schema, pilot_schema)
        if errors:
            failures.append(f"{pilot_scorecard.name}: row {index} schema failed: {errors[0]}")
            break

    with tempfile.TemporaryDirectory() as tmp:
        binary = Path(tmp) / "specbackfill"
        build = run([*go_command(), "build", "-o", str(binary), "./cmd/specbackfill"], root)
        if build.returncode != 0:
            print(build.stderr, file=sys.stderr)
            return 2

        for patch in patches:
            obligations = run(
                [str(binary), "check", "--diff-file", str(patch), "--emit-obligations", "--fail-on", "off"],
                root,
            )
            if obligations.returncode != 0:
                failures.append(f"{patch.name}: obligations command failed: {obligations.stderr.strip()}")
                continue
            try:
                payload = json.loads(obligations.stdout)
            except json.JSONDecodeError as error:
                failures.append(f"{patch.name}: obligations JSON parse failed: {error}")
                continue
            errors = validate(payload, obligations_schema, obligations_schema)
            if errors:
                failures.append(f"{patch.name}: obligations schema failed: {errors[0]}")
                continue

            imported = run(
                [str(binary), "check", "--diff-file", str(patch), "--emit-local-ai-review-import", "--fail-on", "off"],
                root,
            )
            if imported.returncode != 0:
                failures.append(f"{patch.name}: import command failed: {imported.stderr.strip()}")
                continue
            for index, line in enumerate(imported.stdout.splitlines(), start=1):
                if not line.strip():
                    continue
                try:
                    item = json.loads(line)
                except json.JSONDecodeError as error:
                    failures.append(f"{patch.name}: import line {index} JSON parse failed: {error}")
                    break
                errors = validate(item, import_schema, import_schema)
                if errors:
                    failures.append(f"{patch.name}: import line {index} schema failed: {errors[0]}")
                    break

    print("# schema validation over testdata")
    print(f"patches: {len(patches)}")
    print(f"pilot_scorecard_rows: {len(pilot_rows)}")
    print(f"failures: {len(failures)}")
    for failure in failures:
        print(f"- {failure}")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())
