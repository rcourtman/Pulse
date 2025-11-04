# Pulse Script Library Guide

This guide expands on `scripts/lib/README.md` and explains how the shared Bash
modules fit together when you are building or refactoring installer scripts.

---

## Library Structure

```
scripts/
  lib/
    common.sh     # Logging, error handling, retry helpers, temp dirs
    http.sh       # Curl/wget wrappers, GitHub release helpers
    systemd.sh    # Systemd unit management helpers
    README.md     # API-level reference
```

Key conventions:

- **Namespaces:** Exported functions are declared as `module::function` (for
  example `common::run`, `systemd::create_service`). Avoid referencing private
  helpers (`module::__helper`) from other modules.
- **Development vs Bundled mode:** During local development scripts source
  modules from `scripts/lib`. Bundled artifacts produced by
  `make bundle-scripts` contain the modules inline, so the source guards remain
  but resolve to no-ops.
- **Compatibility:** The library targets Bash 5 but must run on Debian 11+
  (Pulse LXC), Ubuntu LTS, and minimal container images. Stick to POSIX shell
  built-ins or guarded GNU extensions.

---

## Recommended Script Skeleton

```bash
#!/usr/bin/env bash
set -euo pipefail

LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/lib" && pwd)"
# shellcheck source=../../scripts/lib/common.sh
source "${LIB_DIR}/common.sh"
# shellcheck source=../../scripts/lib/systemd.sh
source "${LIB_DIR}/systemd.sh"

common::init "$@"
common::require_command curl tar

main() {
  common::log_info "Starting installer..."
  common::temp_dir WORKDIR --prefix pulse-
  download_payload
  install_service
}

download_payload() {
  http::download --url "${PULSE_DOWNLOAD_URL}" --output "${WORKDIR}/pulse.tar.gz"
}

install_service() {
  systemd::create_service /etc/systemd/system/pulse.service <<'UNIT'
[Unit]
Description=Pulse Monitoring
After=network-online.target
UNIT

  systemd::enable_and_start pulse.service
}

main "$@"
```

**Why this layout works**

- `common::init` centralises logging/traps and stores the original CLI args so
  you can re-exec under sudo if required (`common::ensure_root`).
- `common::temp_dir` registers a cleanup handler automatically to keep `/tmp`
  tidy.
- `systemd::create_service` respects `--dry-run` flags and prevents partial
  writes on failure.

---

## Logging and Dry-Run Practices

- Respect `PULSE_LOG_LEVEL` and `PULSE_DEBUG`—they are already wired into
  `common::log_*`.
- Wrap mutating commands in `common::run` or `common::run_capture` to inherit
  retry/backoff logic and `--dry-run` behaviour.
- Provide meaningful `--label` values on long-running steps to improve CI log
  readability.

Example:

```bash
common::run --label "Extract Pulse binary" \
  -- tar -xzf "${ARCHIVE}" -C "${TARGET_DIR}" --strip-components=1
```

When invoked with `--dry-run`, the command prints the operation instead of
executing and exits successfully—keep this in mind when writing tests.

---

## Testing Strategy

- **Smoke tests:** `scripts/tests/run.sh` lints scripts, validates manifests, and
  exercises bundle generation.
- **Integration tests:** Place scenario-specific scripts under
  `scripts/tests/integration/`. They should run quickly (<30s) and clean up
  after themselves.
- **Manual verification:** For destructive operations (e.g., provisioning an LXC
  container), run the script with `--dry-run` to confirm the steps before
  executing against real infrastructure.

When adding new library helpers, accompany them with unit coverage using
`bats` (found under `testing-tools/bats`) or an integration script that covers
the happy path and a failure case.

---

## Bundling Checklist

1. Update `scripts/bundle.manifest` with any newly created scripts.
2. Run `make bundle-scripts` (or `./scripts/bundle.sh`) to regenerate `dist/*`.
3. Inspect the diff to ensure only intentional changes appear.
4. Re-run `scripts/tests/run.sh` to catch lint and shellcheck regressions.

Bundled files embed provenance metadata (timestamp + manifest path). Do not edit
bundled artifacts by hand—always rebuild from sources.

---

## When to Extend the Library

- You need to reuse logic across two or more scripts.
- A helper hides platform-specific differences (e.g., `systemctl` vs `service`
  on legacy systems).
- The code is complex enough that centralised unit tests provide value.

Document new functions in `scripts/lib/README.md` and update this guide if usage
patterns change. Keeping these references in sync helps future contributors
avoid copy/paste or undocumented conventions.

