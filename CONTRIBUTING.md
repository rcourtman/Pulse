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
  of reporting. If a result is already displayed in **Settings → Diagnostics**,
  its download buttons reuse that result without running checks again.
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

```bash
git clone https://github.com/rcourtman/Pulse.git
cd Pulse

# Install Go 1.26 and Node.js 24 with your preferred package manager.

# Install the repository and frontend dependencies exactly from their locks
npm ci
npm --prefix frontend-modern ci
```

### Hot Reload Dev Loop

```bash
npm run dev                 # Frontend shell on :5173, backend on :7655
npm run mock:on             # Optional: enable mock data
```

Use `http://127.0.0.1:5173` in the browser for local frontend development. The
frontend dev shell proxies `/api` and `/ws` to the backend on `:7655`; do not
switch your browser to `:7655` unless you are debugging the backend directly.
The managed dev runtime login defaults to `admin` / `adminadminadmin` unless
you override it with `HOT_DEV_AUTH_USER` and `HOT_DEV_AUTH_PASS`.

Backend-only hot reload (requires `air`):

```bash
air -c .air.toml
```

Set `HOT_DEV_USE_PRO=true` to build the Pro variant when available.

Mock mode is supported for development, but the internal developer notes are not shipped in this repository.

---

## Backend Workflow

- Build: `go build ./cmd/pulse`
- Tests: `go test ./...`
- Lint: `golangci-lint run ./...` (install via `go install` if missing)
- Formatting: `gofmt -w ./cmd ./internal ./pkg`

Key entry points:
- HTTP router lives in `internal/api`.
- Monitoring engines live under `internal/monitor`.
- Configuration parsing resides in `internal/config`.

When adding new API endpoints, document them in `docs/API.md` and provide
examples where possible.

---

## Frontend Workflow

- Managed dev runtime: `npm run dev`
- Runtime status: `npm run dev:status`
- Runtime logs: `npm run dev:logs`
- Managed restart: `npm run dev:restart`
- Managed backend restart: `npm run dev:backend-restart`
- Browser proof pack: `npm run dev:verify`
- Foreground managed launcher: `npm run dev:foreground`
- Frontend-only escape hatch: `cd frontend-modern && npm run dev:frontend-only`
- Tests: `npm --prefix frontend-modern test`
- Type check: `npm --prefix frontend-modern run type-check`
- Lint: `npm --prefix frontend-modern run lint`
- Format check: `npm --prefix frontend-modern run format:check`

The same managed runtime wrappers are available from `frontend-modern/` if you
start there by habit, so `npm run dev`, `npm run dev:status`, and
`npm run dev:verify` behave the same way from either workspace.
- Production build: `npm run build` (syncs the Go embed copy in
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
