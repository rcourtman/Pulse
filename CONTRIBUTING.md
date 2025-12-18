# Contributing to Pulse

Thanks for investing time in Pulse! This document collects the essentials you
need to be productive across the Go backend, React/TypeScript frontend, and the
installer tooling.

---

## Project Overview

- **Backend (`cmd/`, `internal/`, `pkg/`)** – Go 1.23+ web server that embeds
  the built frontend and exposes REST + WebSocket APIs.
- **Architecture (`ARCHITECTURE.md`)** – High-level system design diagrams and explanations.
- **Frontend (`frontend-modern/`)** – Vite + React app built with TypeScript.
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
./scripts/hot-dev.sh        # Backend on :7656, frontend on :7655
npm run mock:on             # Optional: enable mock data
```

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

- Dev server: `cd frontend-modern && npm run dev`
- Tests: `npm run test`
- Lint: `npm run lint`
- Format: `npm run format`
- Production build: `npm run build` (copied into `internal/api/frontend-modern`
  via the Makefile).

Use modern React patterns (hooks, function components) and prefer TanStack Query
for data fetching. Add Storybook stories or screenshots when introducing new
UI-heavy features.

---

## Installers & Scripts

- Centralised guidance: `docs/script-library-guide.md`
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

## Submitting Changes

1. Fork + branch (`git checkout -b feature/my-change`).
2. Make your edits and run relevant tests.
3. Update docs and changelog entries as needed.
4. Open a PR describing:
   - What changed
   - Why it changed
   - Testing performed
   - Rollout / migration concerns

Reviewers will focus on correctness, security, and upgrade paths, so call out
anything unusual up front. Thanks again for contributing!
