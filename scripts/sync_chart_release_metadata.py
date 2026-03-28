#!/usr/bin/env python3
"""Keep Helm chart release metadata aligned with the packaged version."""

from __future__ import annotations

import argparse
import re
from pathlib import Path


def replace_one(text: str, pattern: str, replacement: str) -> str:
    updated, count = re.subn(pattern, replacement, text, count=1, flags=re.MULTILINE)
    if count != 1:
        raise ValueError(f"expected exactly one match for pattern: {pattern}")
    return updated


def sync_chart_metadata(text: str, version: str, repo: str) -> str:
    tag = version if version.startswith("v") else f"v{version}"
    icon_url = f"https://raw.githubusercontent.com/{repo}/{tag}/docs/images/pulse-logo.svg"
    docs_url = f"https://github.com/{repo}/blob/{tag}/docs/KUBERNETES.md"

    text = replace_one(text, r"^version: .*$", f"version: {version}")
    text = replace_one(text, r'^appVersion: ".*"$', f'appVersion: "{version}"')
    text = replace_one(text, r"^icon: .*$", f"icon: {icon_url}")
    text = replace_one(
        text,
        r"(^\s+- name: Documentation\n\s+url: ).*$",
        rf"\1{docs_url}",
    )
    return text


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument("--chart", required=True, help="Path to Chart.yaml")
    parser.add_argument("--version", required=True, help="Chart/app version")
    parser.add_argument("--repo", default="rcourtman/Pulse", help="GitHub owner/repo")
    args = parser.parse_args()

    chart_path = Path(args.chart)
    original = chart_path.read_text(encoding="utf-8")
    updated = sync_chart_metadata(original, args.version, args.repo)
    if updated != original:
        chart_path.write_text(updated, encoding="utf-8")
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
