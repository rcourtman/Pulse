#!/usr/bin/env python3
"""Validate and render the model-selected visual release-note plan."""

from __future__ import annotations

import argparse
import json
import re
import sys
from pathlib import Path
from typing import Any


MAX_CAPTURES = 3
MAX_STEPS = 12
ALLOWED_LOCATOR_KINDS = {"role", "text", "label", "testid"}
ALLOWED_ACTIONS = {"click", "wait"}
ALLOWED_ROLES = {
    "button",
    "checkbox",
    "dialog",
    "heading",
    "link",
    "menuitem",
    "option",
    "radio",
    "row",
    "tab",
}
ID_PATTERN = re.compile(r"^[a-z0-9]+(?:-[a-z0-9]+)*$")


class PlanError(ValueError):
    pass


def _text(value: Any, field: str, *, maximum: int, required: bool = True) -> str:
    if not isinstance(value, str):
        raise PlanError(f"{field} must be a string")
    value = value.strip()
    if required and not value:
        raise PlanError(f"{field} must not be empty")
    if len(value) > maximum:
        raise PlanError(f"{field} must be {maximum} characters or fewer")
    if any(ord(character) < 32 for character in value):
        raise PlanError(f"{field} must be one line without control characters")
    if ";" in value or "\u2014" in value:
        raise PlanError(f"{field} must not contain semicolons or em dashes")
    return value


def _public_text(value: Any, field: str, *, maximum: int, required: bool = True) -> str:
    value = _text(value, field, maximum=maximum, required=required)
    if any(character in value for character in ("[", "]", "<", ">", "|")):
        raise PlanError(f"{field} must be plain text without Markdown or HTML delimiters")
    return value


def _locator(value: Any, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise PlanError(f"{field} must be an object")
    allowed = {"kind", "value", "role", "name", "exact", "nth"}
    unknown = set(value) - allowed
    if unknown:
        raise PlanError(f"{field} has unsupported fields: {', '.join(sorted(unknown))}")
    kind = value.get("kind")
    if kind not in ALLOWED_LOCATOR_KINDS:
        raise PlanError(f"{field}.kind must be one of {', '.join(sorted(ALLOWED_LOCATOR_KINDS))}")
    normalized: dict[str, Any] = {"kind": kind}
    if kind == "role":
        role = _text(value.get("role"), f"{field}.role", maximum=32)
        if role not in ALLOWED_ROLES:
            raise PlanError(f"{field}.role is not supported")
        normalized["role"] = role
        normalized["name"] = _text(value.get("name"), f"{field}.name", maximum=120)
    else:
        normalized["value"] = _text(value.get("value"), f"{field}.value", maximum=160)
    exact = value.get("exact", True)
    if not isinstance(exact, bool):
        raise PlanError(f"{field}.exact must be a boolean")
    normalized["exact"] = exact
    nth = value.get("nth", 0)
    if not isinstance(nth, int) or isinstance(nth, bool) or not 0 <= nth <= 20:
        raise PlanError(f"{field}.nth must be an integer from 0 to 20")
    normalized["nth"] = nth
    return normalized


def _state(value: Any, field: str) -> dict[str, Any]:
    if not isinstance(value, dict):
        raise PlanError(f"{field} must be an object")
    unknown = set(value) - {"route", "steps", "ready"}
    if unknown:
        raise PlanError(f"{field} has unsupported fields: {', '.join(sorted(unknown))}")
    route = _text(value.get("route"), f"{field}.route", maximum=240)
    if (
        not route.startswith("/")
        or route.startswith("//")
        or "://" in route
        or "\\" in route
    ):
        raise PlanError(f"{field}.route must be a same-origin absolute path")
    steps = value.get("steps", [])
    if not isinstance(steps, list) or len(steps) > MAX_STEPS:
        raise PlanError(f"{field}.steps must be a list with at most {MAX_STEPS} entries")
    normalized_steps = []
    for index, step in enumerate(steps):
        step_field = f"{field}.steps[{index}]"
        if not isinstance(step, dict):
            raise PlanError(f"{step_field} must be an object")
        unknown_step = set(step) - {"action", "locator"}
        if unknown_step:
            raise PlanError(
                f"{step_field} has unsupported fields: {', '.join(sorted(unknown_step))}"
            )
        action = step.get("action")
        if action not in ALLOWED_ACTIONS:
            raise PlanError(f"{step_field}.action must be click or wait")
        normalized_steps.append(
            {"action": action, "locator": _locator(step.get("locator"), f"{step_field}.locator")}
        )
    if value.get("ready") is None:
        raise PlanError(f"{field}.ready must identify visible content in the captured view")
    return {
        "route": route,
        "steps": normalized_steps,
        "ready": _locator(value["ready"], f"{field}.ready"),
    }


def validate_plan(raw: Any) -> dict[str, Any]:
    if not isinstance(raw, dict):
        raise PlanError("visual plan must be a JSON object")
    unknown = set(raw) - {"schema_version", "captures"}
    if unknown:
        raise PlanError(f"visual plan has unsupported fields: {', '.join(sorted(unknown))}")
    if raw.get("schema_version") != 1:
        raise PlanError("visual plan schema_version must be 1")
    captures = raw.get("captures")
    if not isinstance(captures, list) or len(captures) > MAX_CAPTURES:
        raise PlanError(f"visual plan captures must be a list with at most {MAX_CAPTURES} entries")

    normalized_captures = []
    seen_ids: set[str] = set()
    for index, capture in enumerate(captures):
        field = f"captures[{index}]"
        if not isinstance(capture, dict):
            raise PlanError(f"{field} must be an object")
        unknown_capture = set(capture) - {
            "id",
            "title",
            "description",
            "viewport",
            "before",
            "after",
        }
        if unknown_capture:
            raise PlanError(
                f"{field} has unsupported fields: {', '.join(sorted(unknown_capture))}"
            )
        capture_id = _text(capture.get("id"), f"{field}.id", maximum=48)
        if not ID_PATTERN.fullmatch(capture_id):
            raise PlanError(f"{field}.id must be lower-case words separated by hyphens")
        if capture_id in seen_ids:
            raise PlanError(f"duplicate capture id: {capture_id}")
        seen_ids.add(capture_id)

        viewport = capture.get("viewport")
        if not isinstance(viewport, dict) or set(viewport) != {"width", "height"}:
            raise PlanError(f"{field}.viewport must contain only width and height")
        width = viewport.get("width")
        height = viewport.get("height")
        if not isinstance(width, int) or isinstance(width, bool) or not 320 <= width <= 1920:
            raise PlanError(f"{field}.viewport.width must be an integer from 320 to 1920")
        if not isinstance(height, int) or isinstance(height, bool) or not 568 <= height <= 1440:
            raise PlanError(f"{field}.viewport.height must be an integer from 568 to 1440")

        before = capture.get("before")
        normalized_captures.append(
            {
                "id": capture_id,
                "title": _public_text(capture.get("title"), f"{field}.title", maximum=90),
                "description": _public_text(
                    capture.get("description", ""),
                    f"{field}.description",
                    maximum=240,
                    required=False,
                ),
                "viewport": {"width": width, "height": height},
                "before": None if before is None else _state(before, f"{field}.before"),
                "after": _state(capture.get("after"), f"{field}.after"),
            }
        )
    return {"schema_version": 1, "captures": normalized_captures}


def load_plan(path: str) -> dict[str, Any]:
    if path == "-":
        raw_text = sys.stdin.read()
    else:
        raw_text = Path(path).read_text(encoding="utf-8")
    try:
        raw = json.loads(raw_text)
    except json.JSONDecodeError as exc:
        raise PlanError(f"visual plan is not valid JSON: {exc}") from exc
    return validate_plan(raw)


def asset_names(plan: dict[str, Any]) -> list[str]:
    names: list[str] = []
    for capture in plan["captures"]:
        if capture["before"] is not None:
            names.append(f"release-note-{capture['id']}-before.png")
        names.append(f"release-note-{capture['id']}-now.png")
    return names


def json_schema() -> dict[str, Any]:
    locator_position = {
        "exact": {"type": "boolean"},
        "nth": {"type": "integer", "minimum": 0, "maximum": 20},
    }
    locator = {
        "anyOf": [
            {
                "type": "object",
                "additionalProperties": False,
                "required": ["kind", "role", "name", "exact", "nth"],
                "properties": {
                    "kind": {"const": "role", "type": "string"},
                    "role": {"enum": sorted(ALLOWED_ROLES), "type": "string"},
                    "name": {"type": "string", "minLength": 1, "maxLength": 120},
                    **locator_position,
                },
            },
            {
                "type": "object",
                "additionalProperties": False,
                "required": ["kind", "value", "exact", "nth"],
                "properties": {
                    "kind": {
                        "enum": sorted(ALLOWED_LOCATOR_KINDS - {"role"}),
                        "type": "string",
                    },
                    "value": {"type": "string", "minLength": 1, "maxLength": 160},
                    **locator_position,
                },
            },
        ]
    }
    state = {
        "type": "object",
        "additionalProperties": False,
        "required": ["route", "steps", "ready"],
        "properties": {
            "route": {"type": "string", "minLength": 1, "maxLength": 240},
            "steps": {
                "type": "array",
                "maxItems": MAX_STEPS,
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": ["action", "locator"],
                    "properties": {
                        "action": {"enum": sorted(ALLOWED_ACTIONS)},
                        "locator": locator,
                    },
                },
            },
            "ready": locator,
        },
    }
    return {
        "type": "object",
        "additionalProperties": False,
        "required": ["schema_version", "captures"],
        "properties": {
            "schema_version": {"const": 1, "type": "integer"},
            "captures": {
                "type": "array",
                "maxItems": MAX_CAPTURES,
                "items": {
                    "type": "object",
                    "additionalProperties": False,
                    "required": [
                        "id",
                        "title",
                        "description",
                        "viewport",
                        "before",
                        "after",
                    ],
                    "properties": {
                        "id": {
                            "type": "string",
                            "maxLength": 48,
                            "pattern": ID_PATTERN.pattern,
                        },
                        "title": {"type": "string", "minLength": 1, "maxLength": 90},
                        "description": {"type": "string", "maxLength": 240},
                        "viewport": {
                            "type": "object",
                            "additionalProperties": False,
                            "required": ["width", "height"],
                            "properties": {
                                "width": {
                                    "type": "integer",
                                    "minimum": 320,
                                    "maximum": 1920,
                                },
                                "height": {
                                    "type": "integer",
                                    "minimum": 568,
                                    "maximum": 1440,
                                },
                            },
                        },
                        "before": {"anyOf": [state, {"type": "null"}]},
                        "after": state,
                    },
                },
            },
        },
    }


def render_markdown(plan: dict[str, Any], repository: str, tag: str) -> str:
    if not plan["captures"]:
        return ""
    base = f"https://github.com/{repository}/releases/download/{tag}"
    lines = ["## See the difference", ""]
    for capture in plan["captures"]:
        lines.extend([f"### {capture['title']}", ""])
        if capture["description"]:
            lines.extend([capture["description"], ""])
        now_name = f"release-note-{capture['id']}-now.png"
        if capture["before"] is None:
            lines.extend(
                [f"![{capture['title']}]({base}/{now_name})", ""]
            )
            continue
        before_name = f"release-note-{capture['id']}-before.png"
        lines.extend(
            [
                "| Before | Now |",
                "| --- | --- |",
                (
                    f"| ![{capture['title']} before]({base}/{before_name}) "
                    f"| ![{capture['title']} now]({base}/{now_name}) |"
                ),
                "",
            ]
        )
    return "\n".join(lines).rstrip() + "\n"


def main() -> int:
    parser = argparse.ArgumentParser()
    subparsers = parser.add_subparsers(dest="command", required=True)

    validate_parser = subparsers.add_parser("validate")
    validate_parser.add_argument("--plan", required=True)
    validate_parser.add_argument("--output")

    count_parser = subparsers.add_parser("count")
    count_parser.add_argument("--plan", required=True)

    before_count_parser = subparsers.add_parser("before-count")
    before_count_parser.add_argument("--plan", required=True)

    subparsers.add_parser("schema")

    assets_parser = subparsers.add_parser("assets")
    assets_parser.add_argument("--plan", required=True)

    render_parser = subparsers.add_parser("render")
    render_parser.add_argument("--plan", required=True)
    render_parser.add_argument("--repository", required=True)
    render_parser.add_argument("--tag", required=True)
    render_parser.add_argument("--output")

    args = parser.parse_args()
    try:
        if args.command == "schema":
            sys.stdout.write(json.dumps(json_schema(), separators=(",", ":")) + "\n")
            return 0
        plan = load_plan(args.plan)
        if args.command == "validate":
            output = json.dumps(plan, indent=2) + "\n"
        elif args.command == "count":
            output = f"{len(plan['captures'])}\n"
        elif args.command == "before-count":
            output = f"{sum(capture['before'] is not None for capture in plan['captures'])}\n"
        elif args.command == "assets":
            output = "".join(f"{name}\n" for name in asset_names(plan))
        else:
            if not re.fullmatch(r"[A-Za-z0-9_.-]+/[A-Za-z0-9_.-]+", args.repository):
                raise PlanError("repository must be in owner/name form")
            if not re.fullmatch(r"v[0-9A-Za-z][0-9A-Za-z._-]*", args.tag):
                raise PlanError("tag is not a safe release tag")
            output = render_markdown(plan, args.repository, args.tag)
        if getattr(args, "output", None):
            Path(args.output).write_text(output, encoding="utf-8")
        else:
            sys.stdout.write(output)
        return 0
    except (OSError, PlanError) as exc:
        print(f"release-note visuals: {exc}", file=sys.stderr)
        return 1


if __name__ == "__main__":
    raise SystemExit(main())
