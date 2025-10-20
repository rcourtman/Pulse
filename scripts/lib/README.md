# Pulse Script Library

The `scripts/lib` directory houses shared Bash modules used by Pulse installation and maintenance scripts. The goal is to keep production installers modular and testable while still supporting bundled single-file artifacts for curl-based distribution.

## Modules & Namespaces

- Each module lives in `scripts/lib/<module>.sh`.
- Public functions use the `module::function` namespace (`common::log_info`, `proxmox::get_nodes`, etc.).
- Internal helpers can be marked `local` or use a `module::__helper` prefix.
- Scripts should only rely on documented `module::` APIs.

## Using the Library

### Development Mode

```bash
LIB_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/lib" && pwd)"
if [[ -f "${LIB_DIR}/common.sh" ]]; then
  # shellcheck disable=SC1090
  source "${LIB_DIR}/common.sh"
fi

common::init "$@"
common::log_info "Starting installer..."

# Acquire a temp directory (automatically cleaned on exit)
common::temp_dir TMP_DIR --prefix pulse-install-
common::log_info "Working in ${TMP_DIR}"
```

### Bundled Mode

Bundled scripts have the library concatenated into the file during the release process. The `source` guard remains but is a no-op because all functions are already defined.

## Environment Variables

| Variable             | Description                                                                  |
|----------------------|------------------------------------------------------------------------------|
| `PULSE_DEBUG`        | When set to `1`, forces debug logging and enables additional diagnostics.    |
| `PULSE_LOG_LEVEL`    | Sets log level (`debug`, `info`, `warn`, `error`). Defaults to `info`.       |
| `PULSE_NO_COLOR`     | When `1`, disables ANSI colors (used for non-TTY or forced monochrome).      |
| `PULSE_SUDO_CMD`     | Overrides the sudo command (e.g., `/usr/bin/doas`).                          |
| `PULSE_FORCE_INTERACTIVE` | Forces interactive behavior (treats script as running in a TTY).       |

## `common.sh` API Reference

- `common::init "$@"` — Initializes logging, traps, and script metadata. Call once at script start.
- `common::log_info "msg"` — Logs informational messages to stdout.
- `common::log_warn "msg"` — Logs warnings to stderr.
- `common::log_error "msg"` — Logs errors to stderr.
- `common::log_debug "msg"` — Logs debug output (requires log level `debug`).
- `common::fail "msg" [--code N]` — Logs error and exits with optional status code.
- `common::require_command cmd...` — Verifies required commands exist; exits on missing dependencies.
- `common::is_interactive` — Returns success when stdin/stdout are TTYs or forced interactive.
- `common::ensure_root [--allow-sudo] [--args "${COMMON__ORIGINAL_ARGS[@]}"]` — Ensures root privileges, optionally re-executing via sudo.
- `common::sudo_exec command...` — Executes command with sudo, printing guidance if sudo is unavailable.
- `common::run [--label desc] [--retries N] [--backoff "1 2"] -- cmd...` — Executes a command with optional retries and dry-run support.
- `common::run_capture [--label desc] -- cmd...` — Runs a command and prints captured stdout; honors dry-run.
- `common::temp_dir VAR [--prefix name]` — Creates a temporary directory, assigns it to `VAR`, and registers cleanup (avoid command substitution so handlers persist).
- `common::cleanup_push "description" "command"` — Registers cleanup/rollback handler (LIFO order).
- `common::cleanup_run` — Executes registered cleanup handlers; automatically called on exit.
- `common::set_dry_run true|false` — Enables or disables dry-run mode for command wrappers.
- `common::is_dry_run` — Returns success when dry-run mode is active.

## Bundling Workflow

1. Define modules under `scripts/lib/`.
2. Reference them from scripts using the development source guard.
3. Update `scripts/bundle.manifest` to list output artifacts and module order.
4. Run `scripts/bundle.sh` to generate bundled files under `dist/`.
5. Distribute bundled files (e.g., replace production installer).

Headers inserted during bundling (`# === Begin: ... ===`) mark module boundaries and aid debugging. The generated file includes provenance metadata with timestamp and manifest path.
- `systemd.sh` — safe wrappers around `systemctl`, unit creation helpers, and service lifecycle utilities.
- `http.sh` — download helpers, API call wrappers with retries, and GitHub release discovery.

- `systemd::safe_systemctl args...` — Run `systemctl` with timeout protection (container-friendly).
- `systemd::service_exists name` / `systemd::detect_service_name …` — Inspect available unit files.
- `systemd::create_service path [mode]` — Write a unit file from stdin (respects dry-run).
- `systemd::enable_and_start name` / `systemd::restart name` — Common service workflows.
- `http::download --url URL --output FILE [...]` — Robust curl/wget download helper with retries.
- `http::api_call --url URL [...]` — Token-authenticated API invocation; prints response body.
- `http::get_github_latest_release owner/repo` — Fetch latest GitHub release tag.
- `http::parse_bool value` — Normalize truthy/falsy strings.
