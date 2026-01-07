# 📜 Script Library Guide

This guide explains the shared Bash modules in `scripts/lib/` used for building installer scripts.

## 📂 Structure

| File | Purpose |
| :--- | :--- |
| `common.sh` | Logging, error handling, retry helpers, temp dirs. |
| `http.sh` | Curl/wget wrappers, GitHub release helpers. |
| `systemd.sh` | Systemd unit management helpers. |

**Conventions:**
*   **Namespaces:** Functions are exported as `module::function` (e.g., `common::run`).
*   **Bundling:** `./scripts/bundle.sh` inlines modules for distribution.
*   **Compatibility:** Targets Bash 5 on Debian 11+ and Ubuntu LTS.

## 🦴 Script Skeleton

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
  
  http::download --url "${URL}" --output "${WORKDIR}/pulse.tar.gz"
  
  systemd::create_service /etc/systemd/system/pulse.service <<'UNIT'
[Unit]
Description=Pulse Monitoring
UNIT

  systemd::enable_and_start pulse.service
}

main "$@"
```

## 🛠️ Best Practices

*   **Logging:** Use `common::log_info`, `common::log_warn`, etc. They respect `PULSE_LOG_LEVEL`.
*   **Dry Run:** Wrap mutating commands in `common::run` to support `--dry-run`.
*   **Testing:** Use `scripts/tests/run.sh` for linting and `scripts/tests/integration/` for scenarios.

## 📦 Bundling

1.  Update `scripts/bundle.manifest`.
2.  Run `./scripts/bundle.sh`.
3.  Verify `dist/` artifacts.

**Note:** Never edit bundled artifacts manually. Always rebuild from source.
