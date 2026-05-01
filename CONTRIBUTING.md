# Contributing to Pulse

Pulse is maintained as a single-maintainer project.

I am not accepting unsolicited external pull requests for this repository.
If you have found a bug, want to propose a feature, or have a concrete
improvement idea, please open an issue instead.

This document also keeps the local development notes needed to reproduce,
debug, and validate issues across the Go backend, SolidJS/TypeScript frontend,
and installer tooling.

## What To Open

- Bug reports: use the bug report issue form and include exact reproduction
  steps, Pulse version, installation type, and any relevant logs or diagnostics.
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
- If I want code help on a specific issue, I will explicitly ask for it there.
- Opening an issue first is the right path; it lets me confirm whether the
  change fits the product direction before anyone spends time building a patch.

## How To Make An Issue Useful

- Search existing issues before opening a new one.
- Keep reproduction steps minimal and exact.
- State the Pulse version and image or package you are actually running.
- Include screenshots, logs, API output, or diagnostics when they clarify the
  problem.
- Separate bug reports from feature requests; avoid mixing both into one issue.

---

## Project Overview

- **Backend (`cmd/`, `internal/`, `pkg/`)** – Go 1.25+ web server that embeds
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

# Install dependencies
brew install go node npm # or use your distro equivalents

# Install JS deps
cd frontend-modern
npm install
cd ..
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
- Tests: `npm run test`
- Lint: `npm run lint`
- Format: `npm run format`

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

Run a quick link check (`npm run lint-docs` if available, or `markdownlint`)
before submitting large doc updates.

---

## Testing Expectations

- Every PR should note the tests run (`go test`, `npm test`, `scripts/tests/run.sh`).
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

## Submitting Requested Changes

For maintainer-requested code help on a tracked issue:

1. Link the issue where the maintainer requested the patch.
2. Fork + branch (`git checkout -b feature/my-change`).
2. Make your edits and run relevant tests.
3. Update docs and changelog entries as needed.
4. Open a PR describing:
   - What changed
   - Why it changed
   - Testing performed
   - Rollout / migration concerns

Reviewers will focus on correctness, security, and upgrade paths, so call out
anything unusual up front.
