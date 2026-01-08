# 🚀 Pulse Pro (Technical Overview)

Pulse Pro unlocks advanced AI automation features on top of the free Pulse platform. It keeps the same self-hosted model while adding continuous, context-aware analysis and remediation workflows.

## What You Get

### Enterprise Audit Log
- Persistent audit trail with SQLite storage and HMAC signing.
- Queryable via `/api/audit` and verified per event in the Security → Audit Log UI.
- Supports filtering, verification badges, and signature checks for tamper detection.
- Configure with `PULSE_AUDIT_SIGNING_KEY`, `PULSE_AUDIT_RETENTION_DAYS`, and `PULSE_AUDIT_CLEANUP_INTERVAL_HOURS`.
- API reference: `docs/API.md`.
- If no signing key is set, events are stored without signatures and verification will fail.

### AI Patrol (LLM-Backed)
Scheduled background analysis that correlates live state + metrics history to produce actionable findings.

**Inputs:**
- Nodes, guests, storages, backups, containers, and Kubernetes resources.
- Metrics history trends and anomaly scores.
- Alert state and diagnostics.

**Outputs:**
- Findings with severity, category, and remediation hints.
- Trend-aware capacity warnings (e.g., "storage pool will be full in 10 days").
- Cross-system correlation (e.g., backups failing because a datastore is full).

### Pro-Only Automations
- **LLM-backed patrol analysis**: full AI analysis instead of heuristic-only findings.
- **Alert-triggered analysis**: on-demand deep analysis when alerts fire.
- **Autonomous mode**: optional diagnostic/fix commands through connected agents.
- **Auto-fix**: guarded remediations when enabled.
- **Kubernetes AI analysis**: deep cluster analysis beyond basic monitoring (Pro-only).
- **Agent Profiles**: centralized configuration profiles for fleets of agents.

### What Free Users Still Get
- **Heuristic Patrol**: local rule-based checks that surface common issues without any external AI provider.
- **AI Chat (BYOK)**: interactive troubleshooting with your own API keys.
- **Update alerts**: container/package update signals remain available in the free tier.

### What You See In The UI
- **Patrol findings**: a prioritized list with severity, evidence, and recommended fixes.
- **Alert timelines**: AI analysis events attached to the alert history for auditability.
- **Remediation controls**: explicit toggles for autonomous mode and auto-fix workflows.
- **Agent profiles**: create, edit, and assign profiles in **Settings → Agents → Agent Profiles**.

## Pro Feature Gates (License-Enforced)

Pulse Pro licenses enable specific server-side features. These are enforced at the API layer and in the UI:

- `ai_patrol`: LLM-backed patrol findings and live patrol stream.
- `ai_alerts`: alert-triggered analysis runs.
- `ai_autofix`: autonomous mode and auto-fix workflows.
- `kubernetes_ai`: AI analysis for Kubernetes clusters (not basic monitoring).
- `agent_profiles`: centralized agent configuration profiles.

## Why It Matters (Technical Value)

- **Cross-system correlation**: Patrol combines PVE, PBS, PMG, Docker, and Kubernetes signals into a single model context instead of isolated checks.
- **Trend-aware analysis**: Uses metrics history to detect slow-burn issues that static thresholds miss.
- **Noise control**: Suppression and dismissal memory prevent alert fatigue.
- **Actionable findings**: Each finding includes root-cause clues and next steps.
- **Auditability**: AI analysis is attached to alerts and stored with finding history, so decisions are traceable.
- **Fleet consistency**: Agent Profiles keep monitoring settings consistent across large deployments.

## Scheduling and Controls

- **Interval**: 10 minutes to 7 days (default 6 hours). Set to 0 to disable Patrol.
- **Scope**: Patrol only analyzes resources Pulse is already monitoring.
- **Safety**: Command execution and auto-fix are disabled by default and require explicit enablement.

## How Licensing Works

Pulse Pro is activated locally with a license key.

1. Go to **Settings → System → Pulse Pro**.
2. Paste your license key and click **Activate License**.
3. The key is validated locally (no license server required).

License status, expiry, and feature availability are visible in the same panel.

The license key is stored encrypted in `license.enc` under the Pulse config directory. It is not included in export/import backups, so re-activate after migrations.

### Feature Status API

You can inspect license feature gates via:

- `GET /api/license/features` (authenticated)

This returns a feature map like `ai_patrol`, `ai_alerts`, `ai_autofix`, and `kubernetes_ai` so you can automate Pro-only workflows safely.

## Under The Hood (Technical)

- **Patrol context**: patrol runs build a unified snapshot from live state + `metrics.db` history, then correlate alerts, diagnostics, and resource topology.
- **Findings storage**: findings persist in `ai_findings.json` with run history in `ai_patrol_runs.json`.
- **Alert-triggered analysis**: runs per alert event and writes analysis into the alert timeline for auditability.
- **Auto-fix safety**: requires explicit toggles and uses the same agent command scopes you configure for manual runs.

## Example Finding Payload (API)

`GET /api/ai/patrol/findings` returns structured findings you can integrate with external tooling:

```json
{
  "id": "finding-9f7c2f5e",
  "key": "storage-high-usage",
  "severity": "warning",
  "category": "capacity",
  "resource_id": "storage:local-lvm",
  "resource_name": "local-lvm",
  "resource_type": "storage",
  "node": "pve-1",
  "title": "Storage nearing capacity",
  "description": "local-lvm is at 87% and growing ~4%/day.",
  "recommendation": "Review VM disks on local-lvm or expand the volume within 7 days.",
  "evidence": "Used 1.74TB of 2.0TB; +4.1%/day over 7d.",
  "source": "ai-analysis",
  "detected_at": "2025-03-04T09:11:12Z",
  "last_seen_at": "2025-03-04T15:11:12Z",
  "alert_id": "alert-storage-usage-local-lvm",
  "times_raised": 2,
  "suppressed": false
}
```

Heuristic (free-tier) findings omit `source: "ai-analysis"` and include the same schema for consistent automations.

## Privacy and Data Handling

Patrol runs on your Pulse server. When Pro is enabled, only the minimal context needed for analysis is sent to the configured AI provider. No telemetry is sent to Pulse by default.

For a deeper AI walkthrough, see [AI.md](AI.md).
