# Contributing to Pulse

Pulse is a single-maintainer project developed with extensive automation,
including coding agents. See
[Development and Automation Transparency](docs/AI_TRANSPARENCY.md) for the
standing disclosure, authority boundaries, and accountability model.

I am not accepting unsolicited external pull requests for this repository.
If you have found a bug, want to propose a feature, or have a concrete
improvement idea, please open an issue instead.

This document also keeps the local development notes needed to reproduce,
debug, and validate issues across the Go backend, SolidJS/TypeScript frontend,
and installer tooling.

## What To Open

- Bug reports: use the bug report issue form and describe the original sequence,
  the version on the affected running instance (or the version or release asset
  attempted if installation never completed), the installation type, and any
  relevant, safely collected evidence. A second reproduction is not required.
- Feature requests: open an issue describing the problem you want solved, the
  workflow you are trying to improve, and any constraints that matter.
- Questions and support requests: use GitHub Discussions when you need help,
  troubleshooting, or general guidance rather than a tracked defect.
- Security issues: follow [SECURITY.md](SECURITY.md) instead of opening a public
  report for sensitive problems.

## Pull Request Policy

- External pull requests are not part of the normal contribution flow for this
  repository.
- Unsolicited pull requests may be closed without detailed review, even when the
  underlying idea is valid.
- A report, diagnostic question or linked design is not an invitation to open
  a pull request. You do not need to write a patch to report a problem.
- If you already have a tested patch or branch, link it in the existing issue
  as evidence, following [Sharing a tested patch](#sharing-a-tested-patch).
  Maintainers handle implementation and source review through the project's
  normal process; a supplied patch does not guarantee acceptance.
- If a pull request already exists, especially one requested in an earlier
  conversation, keep its link in that conversation. Do not recreate it or
  refile evidence. It is owed a reply and a disposition in its own thread:
  reviewed landing, or an equivalent maintainer fix with credit and a commit
  link when the contribution is used.

## How To Make An Issue Useful

- Search existing issues before opening a new one.
- Describe what happened before the failure. Do not repeat an action just to
  produce steps if it could cause data loss, an outage, duplicate changes, or
  excessive notifications; say why you have not repeated it instead.
- State the version on the affected running instance. If Pulse never started,
  give the attempted version or release asset (or say "unknown") and identify
  the installer or helper when known. Include an image tag or digest only for a
  running container, not for a bare-metal or LXC install.
- Where already known, give the affected platform and release (for example,
  TrueNAS SCALE or CORE), separately from the Pulse server OS and version.
  Say whether its readings come through the platform API, a Pulse agent, or
  both. Use existing settings or observations; blank or "unknown" is valid.
  Do not run diagnostics, probe, restart or change a connection to fill this in.
- Include only evidence relevant to the symptom: a screenshot or exact redacted
  error may be enough for a visual problem; existing logs or observations may
  explain a connection or data failure. Diagnostics are optional, not a condition
  of reporting. If a result is already displayed in **Settings → Support →
  Diagnostics & Health**, its download buttons reuse that result without running
  checks again.
- **Run Diagnostics** can make live API and guest-agent requests. Do not run it
  during a backup, freeze/thaw or an unresponsive-host incident just to file a
  report. Keep the original evidence instead; a successful one-off check does
  not prove that normal monitoring has recovered or a guest has thawed. See
  [safe diagnostics collection](docs/TROUBLESHOOTING.md#collect-diagnostics-safely).
- Diagnostics downloads save a local file, not an upload. Choose **GitHub (review
  first)** (called **Export for GitHub (sanitized)** in older versions), and review
  files and screenshots locally before posting: a sanitized export is not a
  guarantee that free-text errors contain no private information. Remove
  credentials, session cookies, secret URLs and private host, network or personal
  details, including those echoed in errors. Keep **Full (private)** exports
  private. Do not attach configuration or `.env` files, private keys, **Copy as
  cURL** commands or full network exports. Never put credentials in a command
  line, URL or thread.
- If private information has already been posted, follow the
  [containment steps](docs/TROUBLESHOOTING.md#if-you-already-posted-private-information).
  Removing a post is not credential revocation or proof that every copy is gone;
  do not repost the original evidence to ask for help.
- For CPU, memory or disk-write reports, use existing readings or safe passive
  observations. Where known, say whether they measure the Pulse process, its
  container or the whole host, with units, measurement window and uptime.
  Unavailable readings are valid evidence. Do not restart, create load or change
  polling or retention just to measure; do not attach raw profiles, heap dumps,
  databases or full process command lines.
- Lead with one primary bug or operator outcome. If the context also exposes
  another actionable topic, put it in the issue form's dedicated field. Triage
  will preserve it with a linked disposition; you do not need to refile text
  you already supplied. See [Issue Triage and Topic Integrity](docs/ISSUE_TRIAGE.md).

---

## Project Overview

- **Backend (`cmd/`, `internal/`, `pkg/`)** – Go 1.26 web server that embeds
  the built frontend and exposes REST + WebSocket APIs.
- **Architecture (`ARCHITECTURE.md`)** – High-level system design diagrams and explanations.
- **Frontend (`frontend-modern/`)** – Vite + SolidJS app built with TypeScript.
- **Agents (`cmd/pulse-*-agent`)** – Go binaries distributed alongside Pulse for
  host and Docker telemetry.
- **Documentation (`docs/`)** – Markdown-based guides published to users and
  referenced from the README.
- **Scripts (`scripts/`)** – Bash installers and helpers bundled for
  curl-based distribution.

---

## Getting Started

Use a disposable development VM or machine with no production configuration,
data, host mounts, connected agents or access to monitored systems. A fresh clone
alone is not isolation. The launcher does not load installed configuration or
copy installed targets by default, but local environment files are still shell
inputs and development enables an authentication bypass. That bypass must never
face an untrusted network.
Do not use the installed Pulse directory or a production host to reproduce a
report. Existing evidence is enough when reproduction would be unsafe.

```bash
git clone https://github.com/rcourtman/Pulse.git
cd Pulse

# Install Go 1.26 and Node.js 24 with your preferred package manager.

# Install the repository and frontend dependencies exactly from their locks
npm ci
npm --prefix frontend-modern ci
```

### Hot Reload Dev Loop

In that disposable environment, use a fresh clone and a new shell without
production environment variables. Do not copy installed `.env` files or reuse
a development data directory containing real connections. The launcher sources
repository-local `.env`, `.env.local` and `.env.dev` files before selecting its
configuration mode; they can execute commands and override shell settings.
Review any such local files before starting, rather than assuming the exports
below make untrusted configuration safe.

From the clone root:

```bash
export HOT_DEV_USE_PRO=false HOT_DEV_USE_PROD_DATA=false HOT_DEV_IMPORT_PROD_CONFIG=false
export PULSE_DATA_DIR="$PWD/tmp/contribution-data"
export PULSE_DEV_LAN=false PULSE_DEV_LAB_AGENTS=false
export FRONTEND_DEV_HOST=127.0.0.1 PULSE_DEV_API_HOST=127.0.0.1 BIND_ADDRESS=127.0.0.1
./scripts/hot-dev-bg.sh start
./scripts/hot-dev-bg.sh status
```

This uses the existing Community launcher **without takeover**. If another
process occupies a development port, it refuses to replace it. Do not follow
the refusal's `npm run dev` suggestion to reclaim the port; choose unused
`FRONTEND_DEV_PORT` and `PULSE_DEV_API_PORT` values in the same shell instead.
Use the browser entrypoint printed by `status` when choosing non-default ports.

Use `http://127.0.0.1:5173` in the browser for local frontend development. The
frontend dev shell proxies `/api` and `/ws` to the backend on `:7655`; do not
switch your browser to `:7655` unless you are debugging the backend directly.
The managed dev runtime login defaults to `admin` / `adminadminadmin` unless
you override it with `HOT_DEV_AUTH_USER` and `HOT_DEV_AUTH_PASS`. These are
development-only credentials, not production defaults.

Keep both installed-data selectors **false** for community reproduction:
`HOT_DEV_USE_PROD_DATA=true` deliberately loads `/etc/pulse/.env` and selects
installed data; `HOT_DEV_IMPORT_PROD_CONFIG=true` deliberately copies installed
targets, credentials and keys into the default development directory. Neither
is a workaround for a startup refusal. LAN or lab-agent mode alone does not
select either import route. If startup refuses installed data, choose a separate
empty development directory instead of opting in to make the error disappear.
These selectors prevent accidental installed-data use; they do not sandbox local
environment files or establish host isolation.

The explicit copy helper requires GNU `realpath` and reads installed files only;
it does not repair an installation or choose an encryption key from backups.
Missing keys with retained encrypted files, or different installed/development
keys, stop the import. Preserve both data sets and identify the matching key
independently; do not delete ciphertext or pick the newest backup to bypass the
refusal. Use a separate empty development copy when the existing copy has a
different key. The helper leaves development sessions, CSRF state, OIDC, local
`.env`, history and alerts alone. It stages reads before per-file replacement
and keeps overwritten files privately in `.config-import.*/previous` within the
development directory. Preserve that snapshot with the matching key; it is not
a full-state backup or a consistent snapshot of a running installation. If
publication fails, keep the snapshot and remaining staged files and inspect the
partial import privately before starting anything. The helper starts no service.

```bash
./scripts/hot-dev-bg.sh logs
./scripts/hot-dev-bg.sh stop
```

Run lifecycle commands from the same clone and shell. The `npm run dev`,
`dev:restart` and `dev:verify` shortcuts include `--takeover` and may stop other
listeners; they are not the safe reproduction entrypoint. Mock-mode switching
can also restart a development runtime. Do not use these commands against a
running installation or connect the disposable instance to real infrastructure.

### Cleaning development build output

Stop the disposable development runtime before cleaning its checkout. `make clean`
removes the server binary and frontend build copies; `make distclean` also removes
known generated build and dependency-cache directories. Reinstall dependencies
with `npm ci` and `npm --prefix frontend-modern ci` when needed.

Neither command resets Pulse: local data, environment files, assistant settings,
logs and recovery files outside the known build outputs are preserved. Tracked
outputs and symbolic links (including linked parent directories) are skipped.
If Git cannot verify the checkout or inspect its tracked files, cleanup stops
before removing anything. Removal failures are reported without using `sudo`.
Cleanup does not stop processes or restore anything an earlier cleanup deleted.

The checked-in `.air.toml` also has an installed `/opt/pulse-enterprise` build
path; it is not a portable backend-only shortcut for an arbitrary clone.
Private Pro source is not required for Community reproduction.

---

## Backend Workflow

- Build: `go build ./cmd/pulse`
- Tests: `go test ./...`
- Lint: `golangci-lint run ./...` (install via `go install` if missing)
- Formatting: `gofmt -w path/to/changed.go` (pass the changed Go files, not directories)

Key entry points:
- HTTP router lives in `internal/api`.
- Monitoring engines live under `internal/monitoring`.
- Configuration parsing resides in `internal/config`.

When adding new API endpoints, document them in `docs/API.md` and provide
examples where possible.

---

## Frontend Workflow

- Local runtime: use the isolated [Hot Reload Dev Loop](#hot-reload-dev-loop) above.
- Runtime status: `./scripts/hot-dev-bg.sh status`
- Runtime logs: `./scripts/hot-dev-bg.sh logs`
- Stop the owned development runtime: `./scripts/hot-dev-bg.sh stop`
- Tests: `npm --prefix frontend-modern test`
- Type check: `npm --prefix frontend-modern run type-check`
- Lint: `npm --prefix frontend-modern run lint`
- Format check: `npm --prefix frontend-modern run format:check`

- Production build, from the repository root: `npm --prefix frontend-modern run build` (syncs the Go embed copy in
  `internal/api/frontend-modern/dist` automatically).

Use SolidJS patterns (signals, memos, createEffect) and the shared design-system
components in `components/shared/`. Add screenshots when introducing new
UI-heavy features.

Design-system lint rules are enforced as CI blockers. Avoid hardcoded structural
light/dark classes and broken utility chains; use semantic tokens from
`frontend-modern/DESIGN_SYSTEM.md`.

---

## Installers & Scripts

- Centralised guidance: `docs/internal/SCRIPT_LIBRARY.md`
- Bundling: `make bundle-scripts`
- Tests: `scripts/tests/run.sh` plus integration suites under
  `scripts/tests/integration/`

Document rollout plans and kill switches in `MIGRATION_SCAFFOLDING.md` so future contributors know how to disable risky changes.

---

## Documentation Standards

- Author or update guides in `docs/` when behaviour changes.
- Organise new topics through `docs/README.md` so they appear in the docs index.
- Avoid marketing copy in technical docs—save that for `README.md` or
  external sites.
- Keep instructions evergreen; put release-specific notes in
  `docs/RELEASE_NOTES.md`.

Run `python3 scripts/check_public_docs.py` before submitting public
documentation updates. It verifies local links and rejects retired navigation
claims on the current documentation surface.

---

## Testing Expectations

- Source changes and supplied patch evidence should note the tests actually run
  (`go test`, frontend tests, or `scripts/tests/run.sh`, as applicable), the
  source version and any untested behaviour. A passing test is not proof that
  a change is available in a published release.
- Add regression coverage when fixing bugs.
- Mention manual verification steps (e.g., “Proxmox LXC installer tested on
  PVE 8.1”) if automated coverage is not feasible.

---

## Coding Guidelines

- Adhere to existing formatting tools (`gofmt`, `prettier`, `eslint`).
- Name Go packages with short, meaningful identifiers (avoid `util`).
- Keep functions focused; prefer small helpers over large monoliths.
- Prefer context-aware logging (`logger.Named("component")`) in new Go code.
- Ensure secrets never reach logs and redact sensitive fields in API responses.

---

## Sharing a tested patch

If you already have a patch or branch, link it from the **existing issue**;
do not open a new pull request to submit it. Include the source version, the
reported behaviour it changes, tests actually run and known limits. Existing
test results are useful; do not repeat an unsafe failure or run against a
production installation just to prepare a patch. A patch is optional evidence,
not a condition of reporting and not a promise that the change will be used.

Review the linked branch, diff, logs and screenshots before sharing. Remove
credentials, private configuration, diagnostics and identifying details; a
public branch also exposes its commit history. Do not publish sensitive
security evidence here; follow [SECURITY.md](SECURITY.md) instead.

When a contribution is used, the fix reply credits its author and links the
reviewed commit. A merged change is not necessarily in a published release;
release availability and reporter confirmation remain separate facts. The
development notes above are for local reproduction and validation, not an
invitation to submit a pull request.
