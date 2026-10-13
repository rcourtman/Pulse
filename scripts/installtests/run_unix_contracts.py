#!/usr/bin/env python3
"""Keep native installer checks exhaustive inside bounded package invocations.

Each Go invocation retains the existing ten-minute package deadline; individual
fixture, transport and cleanup deadlines are unchanged. Discover the actual
package inventory so future tests, examples and fuzz seeds remain included.
The general inventory grew to 500 checks and exhausted that package alarm on
macOS Intel. Split it deterministically rather than extending safety deadlines.
Run the existing NAS process/boot-owner control before frontend preparation;
the remaining phase still requires that identity in the discovered inventory
and excludes only the check already executed by the workflow's preflight.
"""

import argparse
from pathlib import Path
import subprocess


ROOT = Path(__file__).resolve().parents[2]
PACKAGE = "./scripts/installtests"
PREFLIGHT = ("TestInstallSHNASBootOwnersQuiesceBeforeRecoveryErasure",)
GROUPS = ("nas-preflight", "general-0", "general-1", "reset")


def partition(output):
    preflight, general, reset = [], [], []
    seen = set()
    for name in output.splitlines():
        if not name.startswith(("Test", "Example", "Fuzz")):
            continue
        if not name.isidentifier() or name in seen:
            raise ValueError("invalid or repeated Go test identity")
        seen.add(name)
        if name in PREFLIGHT:
            preflight.append(name)
        (reset if name.startswith("TestRootInstallReset") else general).append(name)
    if set(preflight) != set(PREFLIGHT) or len(general) - len(preflight) < 2 or not reset:
        raise ValueError("missing preflight, general or reset installer inventory")
    # Interleave sorted identities so adjacent fixture families do not all
    # consume the same package budget. Never omit or retry a discovered check.
    general.sort()
    # Remove the preflight only after partitioning so every other check keeps
    # its existing group and order, rather than reshuffling the package budgets.
    return (sorted(preflight),
            [name for name in general[::2] if name not in PREFLIGHT],
            [name for name in general[1::2] if name not in PREFLIGHT],
            sorted(reset))


def main(phase="all"):
    if phase not in ("all", "preflight", "remaining"):
        raise ValueError("unknown native installer phase")
    inventory = subprocess.run(
        ["go", "test", "-list", ".", PACKAGE], cwd=ROOT,
        check=True, capture_output=True, text=True,
    )
    for group, names in zip(GROUPS, partition(inventory.stdout)):
        if phase == "preflight" and group != "nas-preflight":
            continue
        if phase == "remaining" and group == "nas-preflight":
            continue
        print(f"Native Unix installer {group}: {len(names)} discovered checks", flush=True)
        subprocess.run(
            ["go", "test", "-count=1", "-timeout", "10m", "-run", "^(" + "|".join(names) + ")$", PACKAGE],
            cwd=ROOT, check=True,
        )


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--phase", choices=("all", "preflight", "remaining"), default="all")
    main(parser.parse_args().phase)
