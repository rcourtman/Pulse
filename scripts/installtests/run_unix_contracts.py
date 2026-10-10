#!/usr/bin/env python3
"""Keep native installer checks exhaustive inside bounded package invocations.

Each Go invocation retains the existing ten-minute package deadline; individual
fixture, transport and cleanup deadlines are unchanged. Discover the actual
package inventory so future tests, examples and fuzz seeds remain included.
The general inventory grew to 500 checks and exhausted that package alarm on
macOS Intel. Split it deterministically rather than extending safety deadlines.
"""

from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
PACKAGE = "./scripts/installtests"
GROUPS = ("general-0", "general-1", "reset")


def partition(output):
    general, reset = [], []
    seen = set()
    for name in output.splitlines():
        if not name.startswith(("Test", "Example", "Fuzz")):
            continue
        if not name.isidentifier() or name in seen:
            raise ValueError("invalid or repeated Go test identity")
        seen.add(name)
        (reset if name.startswith("TestRootInstallReset") else general).append(name)
    if len(general) < 2 or not reset:
        raise ValueError("missing general or reset installer inventory")
    # Interleave sorted identities so adjacent fixture families do not all
    # consume the same package budget. Never omit or retry a discovered check.
    general.sort()
    return general[::2], general[1::2], sorted(reset)


def main():
    inventory = subprocess.run(
        ["go", "test", "-list", ".", PACKAGE], cwd=ROOT,
        check=True, capture_output=True, text=True,
    )
    for group, names in zip(GROUPS, partition(inventory.stdout)):
        print(f"Native Unix installer {group}: {len(names)} discovered checks", flush=True)
        subprocess.run(
            ["go", "test", "-count=1", "-timeout", "10m", "-run", "^(" + "|".join(names) + ")$", PACKAGE],
            cwd=ROOT, check=True,
        )


if __name__ == "__main__":
    main()
