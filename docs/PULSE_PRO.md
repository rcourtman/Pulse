# 🚀 Pulse Pro (Technical Overview)

Pulse Pro unlocks advanced AI automation features on top of the free Pulse platform. Pulse Patrol is available to all users with BYOK, while Pro adds auto-fix, autonomy, and deeper analysis.

## What You Get

### Audit Log
- Persistent audit trail with SQLite storage and HMAC signing.
- Queryable via `/api/audit` and verified per event in the Security → Audit Log UI.
- Supports filtering, verification badges, and signature checks for tamper detection.
- Signing uses an auto-generated HMAC key stored (encrypted) at `.audit-signing.key` in the Pulse data directory.
- Retention defaults to 90 days (not currently configurable via environment variables).
- API reference: `docs/API.md`.
- If signing is disabled (for example, encryption is unavailable), events are stored without signatures and verification will fail.

### Audit Webhooks
- real-time delivery of audit events to external endpoints (SIEM, ELK, etc.).
- Asynchronous dispatch to ensure zero impact on system latency.
- Signature verification on ingest for secure integration.
- Configurable via **Settings → Security → Webhooks**.

### Advanced Reporting
- Generate comprehensive PDF/CSV reports for nodes, VMs, containers, and storage.
- Includes key statistics, trends, and capacity projections.
- Customizable time ranges and metric aggregation.
- Access via **Settings → System → Reporting**.

### Pulse Patrol (BYOK)
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
- **Alert-triggered analysis**: on-demand deep analysis when alerts fire.
- **Auto-fix mode**: automatic remediation with verification loops (see Autonomy Levels below).
- **Full autonomy unlock**: auto-fix for critical findings without requiring approval.
- **Kubernetes AI analysis**: deep cluster analysis beyond basic monitoring.
- **Audit-triggered webhooks**: real-time delivery of security events to external systems.
- **Advanced Reporting**: scheduled or on-demand PDF/CSV infrastructure health reports.
- **Agent Profiles**: centralized configuration profiles for fleets of agents.
- **Multi-Tenant Organizations (Enterprise)**: isolate infrastructure by organization with per-org state, config, and RBAC. Create organizations, manage members, share resources across orgs. See [MULTI_TENANT.md](MULTI_TENANT.md).

### Autonomy Levels

Patrol and the Assistant support tiered autonomy:

| Mode | Behavior | License |
|------|----------|--------|
| **Monitor** | Detect issues only. No investigation or fixes. | Free (BYOK) |
| **Investigate** | Investigates findings and proposes fixes. All fixes require approval. | Free (BYOK) |
| **Auto-fix** | Automatically fixes issues and verifies. Critical findings require approval by default. | **Pro** |
| **Full autonomy** | Auto-fix for all findings including critical, without approval. | **Pro** (explicit toggle) |

### Investigation Orchestration

When Patrol creates a finding, the investigation orchestrator can:

1. **Create a chat session** dedicated to the finding.
2. **AI analyzes** the issue using available tools (metrics, logs, storage, etc.).
3. **Propose a fix** with risk assessment (low/medium/high/critical).
4. **Queue for approval** or **auto-execute** based on autonomy level.
5. **Verify the fix** with a follow-up read after execution.

Investigation outcomes include:
- `resolved` — Issue resolved during investigation
- `fix_queued` — Fix proposed, awaiting approval
- `fix_executed` — Fix auto-executed successfully
- `fix_verified` — Fix worked, issue confirmed resolved
- `needs_attention` — Requires human intervention
- `cannot_fix` — Issue cannot be automatically fixed

### What Free Users Still Get
- **Pulse Patrol (BYOK)**: background findings and investigation proposals with your own provider.
- **AI Chat (BYOK)**: interactive troubleshooting with your own API keys.
- **Update alerts**: container/package update signals remain available in the free tier.

### What You See In The UI
- **Patrol findings**: a prioritized list with severity, evidence, and recommended fixes.
- **Investigation status**: progress indicators showing investigation state and outcome.
- **Approval cards**: pending fixes await your review with one-click approve/deny.
- **Alert timelines**: AI analysis events attached to the alert history for auditability.
- **Remediation controls**: explicit toggles for autonomy mode in Patrol settings.
- **Agent profiles**: create, edit, and assign profiles in **Settings → Agents → Agent Profiles**.

## Pro Feature Gates (License-Enforced)

Pulse Pro licenses enable specific server-side features. These are enforced at the API layer and in the UI:

- `ai_alerts`: alert-triggered analysis runs.
- `ai_autofix`: autonomous mode and auto-fix workflows.
- `kubernetes_ai`: AI analysis for Kubernetes clusters (not basic monitoring).
- `agent_profiles`: centralized agent configuration profiles.
- `advanced_reporting`: infrastructure health report generation (PDF/CSV).
- `audit_logging`: persistent audit trail and real-time webhook delivery.
- `long_term_metrics`: 30-day and 90-day metrics history (7-day history is free).
- `multi_tenant`: organization-based infrastructure isolation (Enterprise license required).

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

This returns a feature map including `ai_alerts`, `ai_autofix`, `kubernetes_ai`, and `multi_tenant` so you can automate Pro-only and Enterprise workflows safely.

## Under The Hood (Technical)

- **Patrol context**: patrol runs build a unified snapshot from live state + `metrics.db` history, then correlate alerts, diagnostics, and resource topology.
- **Findings storage**: findings persist in `ai_findings.json` with run history in `ai_patrol_runs.json`.
- **Alert-triggered analysis**: runs per alert event and writes analysis into the alert timeline for auditability.
- **Auto-fix safety**: requires explicit toggles and uses the same agent command scopes you configure for manual runs.

📖 **For complete technical details on the AI subsystems:**
- [Pulse Patrol Deep Dive](architecture/pulse-patrol-deep-dive.md) — Baseline learning, pattern detection, forecasting, correlation analysis, incident memory
- [Pulse Assistant Deep Dive](architecture/pulse-assistant-deep-dive.md) — Context prefetching, FSM enforcement, knowledge accumulation, safety gates

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

Findings include `source: "ai-analysis"` when AI is enabled (BYOK).

## Privacy and Data Handling

Patrol runs on your Pulse server. When AI is enabled, only the minimal context needed for analysis is sent to the configured AI provider. No telemetry is sent to Pulse by default.

For a deeper AI walkthrough, see [AI.md](AI.md).
