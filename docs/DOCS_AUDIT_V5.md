# Pulse v5 Documentation Audit (pre-stable)

This is a working audit of Pulse documentation as of `VERSION=5.0.0-rc.4`, focused on release readiness for a v5 stable cut.

## Status (updated 2025-12-18)

Most of the issues identified in this audit have been addressed in-repo:

- Updated install recommendation and bootstrap-token guidance across entrypoints (`README.md`, `docs/INSTALL.md`, `docs/FAQ.md`, `docs/TROUBLESHOOTING.md`, `docs/DOCKER.md`)
- Rewritten AI and API docs to match the current v5 implementation (`docs/AI.md`, `docs/API.md`)
- Rewritten metrics history docs to match SQLite store + tiered retention (`docs/METRICS_HISTORY.md`)
- Fixed adaptive polling defaults and rollout paths (`docs/monitoring/ADAPTIVE_POLLING.md`, `docs/operations/ADAPTIVE_POLLING_ROLLOUT.md`)
- Reduced temperature monitoring contradictions by making the agent the recommended path and scoping sensor-proxy as a legacy/alternative (`docs/TEMPERATURE_MONITORING.md`, `docs/security/TEMPERATURE_MONITORING.md`, `SECURITY.md`, sensor-proxy docs)
- Updated Helm/Kubernetes docs to prefer OCI distribution and flag the legacy agent block (`docs/KUBERNETES.md`, `deploy/helm/pulse/README.md`, `deploy/helm/pulse/values.yaml`)
- Added missing “operator clarity” docs (`docs/DEPLOYMENT_MODELS.md`, `docs/UPGRADE_v5.md`)
- Link validation run: no broken relative `.md` links found at time of update

## Goals

- Identify docs that are **stale**, **contradictory**, or **redundant**
- Identify **missing docs** needed for a v5 stable release
- Produce an actionable “what to change, where” checklist

## Highest-Priority Fixes (release-blockers)

### 1) Temperature monitoring guidance is contradictory

There are multiple competing “truths” about how temperature monitoring works in v5:

- `SECURITY.md` describes container deployments as requiring `pulse-sensor-proxy` and explicitly blocks SSH-based temps in containers.
- Multiple docs under `docs/security/` and `cmd/pulse-sensor-proxy/README.md` claim `pulse-sensor-proxy` is deprecated in favor of the unified agent.
- `docs/TEMPERATURE_MONITORING.md` is an extensive sensor-proxy-first guide and reads as “current”, but conflicts with the “deprecated” banner elsewhere.
- The backend still has extensive support and UX flows for sensor proxy install/register (`/api/install/install-sensor-proxy.sh`, temperature proxy diagnostics, container SSH blocking guidance).

Action:
- Decide the **canonical** v5 story:
  - **Option A (agent-first)**: “Install `pulse-agent --enable-proxmox` on each Proxmox host for temperatures and management. `pulse-sensor-proxy` is legacy or edge-case only.”
  - **Option B (proxy-first for containers)**: “If Pulse runs in Docker/LXC, temperatures require `pulse-sensor-proxy` (socket/HTTPS). The agent is optional for other features.”
- Update all docs to align with the chosen story, and ensure `SECURITY.md` reflects it unambiguously.

Status:
- Docs updated to be agent-first, with `pulse-sensor-proxy` treated as a legacy/alternative option.
- Remaining work is primarily product positioning and long-term deprecation decisions, not broken documentation.

Files involved:
- `SECURITY.md`
- `docs/TEMPERATURE_MONITORING.md`
- `docs/security/TEMPERATURE_MONITORING.md`
- `docs/security/SENSOR_PROXY_HARDENING.md`
- `docs/security/SENSOR_PROXY_NETWORK.md`
- `docs/security/SENSOR_PROXY_APPARMOR.md`
- `docs/operations/SENSOR_PROXY_CONFIG.md`
- `docs/operations/SENSOR_PROXY_LOGS.md`
- `cmd/pulse-sensor-proxy/README.md`

### 2) AI docs do not match the actual v5 API and configuration model

`docs/AI.md` and the AI section in `docs/API.md` appear written for an older/alternate API surface:

- `docs/AI.md` documents `PULSE_AI_PROVIDER` and `PULSE_AI_API_KEY` env vars, but the current implementation persists encrypted AI config in `ai.enc` and supports multi-provider credentials (Anthropic/OpenAI/DeepSeek/Gemini/Ollama) plus Anthropic OAuth.
- `docs/API.md` references endpoints like `POST /api/ai/chat` and `PUT /api/settings/ai` that do not match the router (current endpoints include `/api/ai/execute`, `/api/ai/models`, `/api/settings/ai/update`, OAuth endpoints, patrol stream, cost summary).

Action:
- Rewrite AI docs to match current behavior:
  - Providers actually supported
  - How keys/tokens are stored (encrypted) and what the UI exposes
  - Anthropic OAuth flow and security implications
  - Patrol and command execution (“autonomous mode”) safety controls
  - Correct API endpoints and auth requirements

Files involved:
- `docs/AI.md`
- `docs/API.md`
- `internal/config/ai.go` (source of truth for config fields)
- `internal/api/router.go` (source of truth for endpoints)

Status:
- `docs/AI.md` rewritten to match multi-provider + encrypted config.
- `docs/API.md` AI endpoints updated to match router.

### 3) Installation “recommended path” is inconsistent across docs

- `README.md` recommends “Proxmox LXC (Recommended)” via GitHub `install.sh`.
- `docs/INSTALL.md` and `docs/FAQ.md` currently present Docker as the easiest/recommended path.

Action:
- Pick one recommendation hierarchy and make it consistent:
  - If Proxmox LXC is the primary path, it should be the top section in `docs/INSTALL.md` and the FAQ answer should reflect it.

Files involved:
- `README.md`
- `docs/INSTALL.md`
- `docs/FAQ.md`

Status:
- Install docs now consistently present Proxmox VE LXC installer as the recommended path and include bootstrap-token retrieval.

### 4) Kubernetes/Helm docs and chart docs are out of date for v5

- `docs/KUBERNETES.md` references a chart repo URL and “Docker Agent sidecar”.
- `deploy/helm/pulse/README.md` describes “optional Docker monitoring agent” and defaults to `ghcr.io/rcourtman/pulse-docker-agent`.

Action:
- Update Helm docs to match the v5 agent direction:
  - If `pulse-docker-agent` is deprecated, the chart should not reference it as primary.
  - Align chart distribution instructions (Helm repo vs OCI).

Files involved:
- `docs/KUBERNETES.md`
- `deploy/helm/pulse/README.md`
- `deploy/helm/pulse/values.yaml`
- `deploy/helm/pulse/templates/*`

Status:
- `docs/KUBERNETES.md` updated to prefer OCI chart installs and flag the legacy agent block.
- `deploy/helm/pulse/README.md` and `deploy/helm/pulse/values.yaml` now label the agent workload as legacy.

## Redundant / Duplicated Docs (needs consolidation)

### Auto-update docs: two competing sources

- `docs/AUTO_UPDATE.md` describes “Settings → System Updates” and includes docker image instructions that differ from other docs.
- `docs/operations/AUTO_UPDATE.md` documents systemd timers and edits `/var/lib/pulse/system.json` which appears stale for current config defaults (`/etc/pulse/system.json`).

Action:
- Choose one canonical page (likely `docs/AUTO_UPDATE.md`) and:
  - Move operational/timer details into it (or link to a clearly “advanced ops” page)
  - Fix stale paths and service names
  - Remove or clearly label the non-canonical duplicate

Files involved:
- `docs/AUTO_UPDATE.md`
- `docs/operations/AUTO_UPDATE.md`

Status:
- Both documents updated to current UI naming and paths; optional future work is to consolidate into a single canonical page.

### Temperature monitoring docs: two sources with different “truth”

- `docs/TEMPERATURE_MONITORING.md` (sensor proxy focused, extensive)
- `docs/security/TEMPERATURE_MONITORING.md` (agent recommended, proxy “legacy”)

Action:
- Collapse into one canonical document with a clear decision tree, then:
  - Keep the other as a short redirect page, or delete it.

Files involved:
- `docs/TEMPERATURE_MONITORING.md`
- `docs/security/TEMPERATURE_MONITORING.md`

Status:
- `docs/TEMPERATURE_MONITORING.md` is now the canonical deep-dive, and `docs/security/TEMPERATURE_MONITORING.md` is a security/overview page.

### Adaptive polling docs disagree with defaults and file paths

- `docs/monitoring/ADAPTIVE_POLLING.md` claims adaptive polling is enabled by default and says env default is `true`.
- Code defaults `AdaptivePollingEnabled=false` and `docs/operations/ADAPTIVE_POLLING_ROLLOUT.md` references `/var/lib/pulse/system.json`.

Action:
- Make one canonical doc, fix defaults and paths, and ensure UI path matches current navigation.

Files involved:
- `docs/monitoring/ADAPTIVE_POLLING.md`
- `docs/operations/ADAPTIVE_POLLING_ROLLOUT.md`

Status:
- Defaults and paths updated to match current behavior.

## Stale / Incorrect Content (targeted findings)

### `docs/API.md`

Issues:
- AI endpoints mismatch current router paths (examples: `POST /api/ai/chat` vs current `/api/ai/execute`; settings update path differs).
- “complete REST API documentation” claim is optimistic. It’s a curated subset plus a “check router.go” note.

Action:
- Update AI section to match `internal/api/router.go`.
- Consider splitting into:
  - “Stable/public API” (guaranteed)
  - “Internal/subject to change” (documented but not stable)

### `docs/METRICS_HISTORY.md`

Issues:
- Documents `PULSE_METRICS_*_RETENTION_DAYS` env vars that do not appear to exist in the server config.
- Claims metrics are stored under `/etc/pulse/data/metrics/`, but the metrics store is SQLite (`metrics.db`) under the configured data directory.

Action:
- Rewrite this doc to match the tiered retention model and actual storage format/location.

### `docs/FAQ.md`

Issues:
- Recommends Docker as easiest install, conflicts with repo README.
- Password reset guidance does not mention the bootstrap token requirement that can appear after removing `.env`.
- Mentions `METRICS_RETENTION_DAYS` which does not appear to be a current server config knob (v5 uses tiered retention settings).

Action:
- Align install recommendation with v5 positioning.
- Update auth reset steps to include bootstrap token retrieval where applicable.
- Replace metrics retention knob guidance with current retention model and UI location.

### `docs/TROUBLESHOOTING.md` and `docs/DOCKER.md`

Issues:
- “Forgot password” flow implies you can just rerun the setup wizard after deleting `.env`, but first-time setup can require the bootstrap token.

Action:
- Update password reset steps and link to the bootstrap token section in `docs/INSTALL.md`.

### `docs/RELEASE_NOTES.md`

Issues:
- Entire document is v4.x release notes.

Action:
- Replace with v5 release notes (or move to `docs/releases/` and add v5.0.0 as the top section).
- For the v5 stable cut, include: breaking changes, migration notes, and versioned “what changed since v4”.

### `cmd/pulse-sensor-proxy/README.md`

Issues:
- Mentions downloading via `/download/pulse-sensor-proxy` but the server router does not expose this endpoint.
- “Deprecated” banner conflicts with current server behavior and security guidance.

Action:
- Either bring it in line with the chosen v5 temperature story, or clearly scope it as legacy.

### Broken local link

- `docs/TEMPERATURE_MONITORING.md` contains an absolute link to `/opt/pulse/cmd/pulse-sensor-proxy/README.md` which does not work in GitHub.

Action:
- Replace with a repo-relative link (or link to the canonical temperature doc).

### Widespread “runtime path” drift (`/opt/pulse/...`)

Several user-facing docs mix:
- repository paths (`/opt/pulse/...`) used in this dev workspace, and
- runtime paths used in real installs (`/etc/pulse`, `/data`, `/var/log/pulse`, systemd units).

This creates confusion and broken copy-paste commands.

Examples to review:
- `docs/ZFS_MONITORING.md` references `/opt/pulse/.env` and `/opt/pulse/pulse.log`.
- `docs/operations/*` references `/var/lib/pulse/system.json` rather than `/etc/pulse/system.json`.

Action:
- Adopt a consistent convention across docs:
  - **Runtime**: `/etc/pulse` (systemd/LXC), `/data` (Docker/Helm)
  - **Repo/dev**: `/opt/pulse` only in development docs
  - **Logs**: `journalctl -u pulse` (systemd) and `docker logs` (Docker), plus `/var/log/pulse/*` only if actually used in production images.

## Missing Docs for a v5 Stable Release (recommended additions)

### v5 upgrade guide (v4 → v5)

Add a single canonical page covering:
- “What changes in v5” in operator terms
- Any breaking changes and required actions
- Post-upgrade verification checklist (health endpoint, scheduler health, agents connected, temps, notifications)
- Rollback guidance for each deployment model (Docker, systemd/LXC, Helm)

Suggested path:
- `docs/UPGRADE_v5.md` (or `docs/MIGRATION_v5.md`)

### “Deployment model matrix”

Many docs implicitly assume a deployment type. Add a short matrix page that answers:
- What works on Docker vs Proxmox LXC vs systemd vs Helm
- How updates work per model
- Where config lives per model
- What “recommended” means (and why)

Suggested path:
- `docs/DEPLOYMENT_MODELS.md`

### AI safety and permissions

If v5 ships AI “execute/run-command” features:
- Document default safety posture
- What autonomous mode does
- Required scopes/roles
- Audit logging expectations
- Clear warning section for production

Suggested path:
- Expand `docs/AI.md` with a “Safety” section, or add `docs/AI_SAFETY.md`.

## Quick “Status” Inventory (what to touch for v5)

This is a fast triage list to help plan the doc refresh. Treat anything marked “Review” as “verify against current behavior”.

- Rewrite: `docs/AI.md`
- Rewrite: `docs/METRICS_HISTORY.md`
- Rewrite: `docs/RELEASE_NOTES.md` (or replace with v5 release notes)
- Update + align: `docs/INSTALL.md`, `docs/FAQ.md`, `docs/TROUBLESHOOTING.md`
- Update: `docs/API.md` (especially AI endpoints)
- Decide canonical + consolidate:
  - `docs/TEMPERATURE_MONITORING.md` vs `docs/security/TEMPERATURE_MONITORING.md`
  - `docs/AUTO_UPDATE.md` vs `docs/operations/AUTO_UPDATE.md`
  - `docs/monitoring/ADAPTIVE_POLLING.md` vs `docs/operations/ADAPTIVE_POLLING_ROLLOUT.md`
- Review (Helm): `docs/KUBERNETES.md`, `deploy/helm/pulse/README.md`
- Review (paths): `docs/ZFS_MONITORING.md` (and any other doc that uses `/opt/pulse/...` in user instructions)

## Suggested “Doc Refresh” Execution Order

1. Decide v5 canonical stories (agent vs proxy for temps, AI capabilities, Helm strategy).
2. Update the primary entrypoints:
   - `README.md`
   - `docs/README.md`
   - `docs/INSTALL.md`
3. Fix contradictions and remove duplicates (temperature, auto-update, adaptive polling).
4. Update `docs/API.md` to reflect current endpoints (especially AI).
5. Add v5 upgrade guide and deployment matrix.
6. Sweep FAQ + troubleshooting for the new canonical flows.
