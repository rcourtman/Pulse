#!/usr/bin/env python3
"""Print `go test -json` like plain `go test` and record per-test seconds.

Usage: go test -json ... | record-internal-api-test-seconds.py <seconds-file>

`go test -json` reports every test the way `-v` does, which for internal/api
means thousands of lines per shard. This filter keeps the log as readable as
plain `go test`: package lines (build errors, `ok`, `FAIL`, panics outside a
test) print straight away, and a test's own output is held until the test
ends, printed if it failed and dropped if it passed or was skipped. Output of
a test that never finished (a panic or timeout killed the binary) is printed
at the end, so a crash is never hidden.

Every finished top-level test is written to <seconds-file> as
`<TestName> <elapsed seconds>`, after a `# package-seconds <elapsed>` comment
holding the whole test binary's run time, the same shape as
.github/scripts/internal-api-test-seconds.txt, so
refresh-internal-api-test-seconds.py can rebuild the shard weights from CI.

A bounded <seconds-file>.failures.json sidecar names failed and unfinished
source-level tests without test output or parameterised subtest labels. The
existing timing-artifact upload makes it readable as soon as the shard ends,
even while another workflow job is still running. It is an observation index,
not a passing verdict: absent package completion and omitted names are explicit.

The exit status is non-zero when any test or package failed, but the caller
must still run with `set -o pipefail` so a `go test` failure that produced no
JSON (for example a vet or build error) fails the step.
"""

from __future__ import annotations

import json
import re
import sys
from collections import defaultdict


MAX_FAILURE_IDENTIFIERS = 256
MAX_IDENTIFIER_LENGTH = 256
SOURCE_TEST_NAME = re.compile(r"(?:Test|Fuzz|Example)[A-Za-z0-9_]*")


def bounded_identifiers(names: set[str]) -> dict[str, object]:
    """Never export output, subtest parameters or unsafe source names."""
    safe = sorted(name for name in names
                  if len(name) <= MAX_IDENTIFIER_LENGTH
                  and SOURCE_TEST_NAME.fullmatch(name))
    retained = safe[:MAX_FAILURE_IDENTIFIERS]
    return {"names": retained, "observed_count": len(names),
            "omitted_count": len(names) - len(retained)}


def main() -> int:
    if len(sys.argv) != 2:
        print(f"usage: {sys.argv[0]} <seconds-file>", file=sys.stderr)
        return 2

    held: dict[str, list[str]] = defaultdict(list)
    running: set[str] = set()
    seconds: dict[str, float] = {}
    package_seconds = None
    failed = False
    failed_tests: set[str] = set()
    package_terminal_action = None
    package_terminal_count = 0
    non_json_lines = 0
    out = sys.stdout

    for raw in sys.stdin:
        try:
            event = json.loads(raw)
        except ValueError:
            event = None
        if not isinstance(event, dict):
            non_json_lines += 1
            out.write(raw)
            out.flush()
            continue

        action = event.get("Action")
        test = event.get("Test") or ""
        top = test.split("/", 1)[0]
        text = event.get("Output") or ""

        if not top:
            # Package level: build output, `ok`/`FAIL` summary, panics and
            # timeouts reported outside a test.
            if text:
                out.write(text)
                out.flush()
            if action in ("pass", "fail", "skip"):
                package_terminal_action = action
                package_terminal_count += 1
            if action == "fail":
                failed = True
            if action in ("pass", "fail") and isinstance(event.get("Elapsed"), (int, float)):
                package_seconds = float(event["Elapsed"])
            continue

        if action == "run" and test == top:
            running.add(top)
        if text:
            held[top].append(text)
        if test != top or action not in ("pass", "fail", "skip"):
            continue

        running.discard(top)
        lines = held.pop(top, [])
        if action == "fail":
            failed = True
            failed_tests.add(top)
            out.write("".join(lines))
            out.flush()
        elapsed = event.get("Elapsed")
        if isinstance(elapsed, (int, float)):
            seconds[top] = float(elapsed)

    for top in sorted(running):
        failed = True
        out.write(f"--- {top} did not finish; its output follows\n")
        out.write("".join(held.get(top, [])))
    out.flush()

    with open(sys.argv[1], "w", encoding="utf-8") as handle:
        if package_seconds is not None:
            handle.write(f"# package-seconds {package_seconds:.2f}\n")
        for name, value in seconds.items():
            handle.write(f"{name} {value:.2f}\n")

    # Upload alongside the unchanged timing text; never include raw output or
    # parameterised subtest names. The CI artifact supplies run/source binding.
    index = {
        "schema_version": 1,
        "recorder_exit_code": 1 if failed else 0,
        "failed_top_level_tests": bounded_identifiers(failed_tests),
        "unfinished_top_level_tests": bounded_identifiers(running),
        # Only the final package action matters; retain contrary earlier
        # actions as a count instead of exporting an unbounded event stream.
        "package_terminal_action": package_terminal_action,
        "package_terminal_count": package_terminal_count,
        "non_json_line_count": non_json_lines,
    }
    with open(sys.argv[1] + ".failures.json", "w", encoding="utf-8") as handle:
        json.dump(index, handle, indent=2, sort_keys=True)
        handle.write("\n")

    return 1 if failed else 0


if __name__ == "__main__":
    raise SystemExit(main())
