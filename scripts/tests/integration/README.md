# Integration Tests

The scripts in this directory exercise the Pulse installer scripts inside isolated
environments (typically Linux containers). They are intended to catch regressions
that unit-style smoke tests cannot detect (e.g., filesystem layout, systemd unit
generation, binary placement).

## Prerequisites

- Docker or another container runtime supported by the test script. When Docker
  is unavailable the test will skip gracefully.
- Internet access is **not** required; HTTP interactions are stubbed.

## Adding New Integration Tests

1. Place new test scripts in this directory. They should follow the pattern of
   detecting required tooling, skipping when prerequisites are missing, and
   producing clear PASS/FAIL output.
2. Prefer running inside an ephemeral container to avoid modifying the host
   system.
3. Use repository-relative paths (`/workspace` inside the container) and avoid
   relying on network resources.
4. Clean up all temporary files even when the test fails (use traps).

## Reporting

Each integration script is self-contained and prints a concise summary at the
end. CI jobs or developers can invoke them individually without modifying
the top-level smoke test harness.
