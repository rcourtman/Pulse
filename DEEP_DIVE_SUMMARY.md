# Pulse Deep Dive Summary
**Status:** ✅ Complete
**Updated:** 2026-01-02

---

## 📡 Area: Adaptive Polling (`internal/monitoring`)
**Verified Files:** `scheduler.go`, `circuit_breaker.go`, `monitor.go`, `task_queue.go`

### ✅ Verification Checklist
- [x] **Logic Accuracy**: Verified that `BuildPlan` correctly calculates `NextRun` times based on staleness and error history.
- [x] **Jitter & Smoothing**: Confirmed that the `adaptiveIntervalSelector` adds noise to prevent "thundering herd" patterns.
- [x] **Circuit Breaker**: Verified the exponential backoff (min 5s, max 5m) for failing nodes.
- [x] **Resource Protection**: Verified that the Worker Pool (default 10) prevents polling from overwhelming the host.
- [x] **Error Handling**: Confirmed that permanent errors bypass retries and go to the Dead Letter Queue.

---

## 🤖 Area: AI Patrol & Findings (`internal/ai`)
**Verified Files:** `patrol.go`, `findings.go`, `service.go`, `config/ai.go`

### ✅ Verification Checklist
- [x] **Prompt Logic**: Verified that dismissed findings are injected into LLM prompts as "Operational Memory".
- [x] **Escalation Logic**: Confirmed the system overrides dismissals if issue severity worsens.
- [x] **Default Auditing**: Identified and updated the documentation to reflect the 6-hour default patrol interval.
- [x] **Free Tier Fallback**: Verified that "Heuristic Patrol" provides value without requiring a Pro LLM.
- [x] **Persistence**: Confirmed findings are stored in `ai_findings.enc` and survive service restarts.

---

## ⚡ Area: Real-time Engine (`internal/websocket`)
**Verified Files:** `hub.go`, `router.go`, `state_snapshot.go`

### ✅ Verification Checklist
- [x] **Coalescing**: Verified that rapid state updates are squashed into a 100ms window to save client bandwidth.
- [x] **Concurrency Safety**: Confirmed deep cloning of alerts (`cloneAlertData`) to prevent data races during broadcast.
- [x] **Initial Sync**: Verified the "Welcome -> InitialState" handshake sequence for new connections.
- [x] **Proxy Awareness**: Confirmed support for `X-Forwarded-*` headers for robust origin validation.
- [x] **Sanitization**: Verified recursive NaN/Inf cleanup to prevent JSON marshalling errors on numeric metrics.

---

## 📉 Area: Metrics Persistence (`internal/metrics`)
**Verified Files:** `store.go`, `docs/METRICS_HISTORY.md`

### ✅ Verification Checklist
- [x] **Tiered Storage**: Verified that the system correctly rolls up data into Raw, Minute, Hourly, and Daily resolutions.
- [x] **Automatic Tier Selection**: Confirmed that queries automatically select the optimal granularity based on the requested time range.
- [x] **Batch Logging**: Verified that writes are buffered (100 records or 5s) and committed via WAL mode for performance.
- [x] **Data Integrity**: Verified that `Min`, `Max`, and `AVG` are preserved during rollups, allowing for high-quality historical charts.
- [x] **Retention Pruning**: Confirmed the background worker hourly cleanup process works as expected.

---

## 🔔 Area: Alerting Framework (`internal/alerts`)
**Verified Files:** `alerts.go`, `CONFIGURATION.md`

### ✅ Verification Checklist
- [x] **Hysteresis**: Verified that separately configurable Trigger/Clear thresholds prevent "jitter" alerts.
- [x] **Time Thresholds**: Confirmed that metrics must exceed thresholds for a sustained period (default 5s) before firing.
- [x] **Flapping Detection**: Verified the state-change tracking logic (5 changes in 5 min) that silences noisy alerts.
- [x] **Escalation & Quiet Hours**: Confirmed that quiet hours can selectively target performance vs. offline alerts, and escalations trigger correctly on timers.
- [x] **Rate Limiting**: Verified the 10-alerts-per-hour limit and 2% minimum delta suppression logic.

---

## 🏠 Area: Host Agent Integration (`internal/api/host_agents.go`)
**Verified Files:** `host_agents.go`, `UNIFIED_AGENT.md`, `AGENT_SECURITY.md`

### ✅ Verification Checklist
- [x] **Zero-Config Pairing**: Verified the hostname-based lookup logic that allows agents to auto-bind to existing PVE nodes.
- [x] **Bi-Directional Communication**: Confirmed that agents receive configuration overrides (like `commandsEnabled`) in the response to their metric reports.
- [x] **Security Scoping**: Verified that agents are restricted to their own host records via API token validation.
- [x] **Smart Deduplication**: Confirmed that the system prefers Agent metrics over Proxmox API metrics when both are available (for higher accuracy).
- [x] **Self-Unregistration**: Verified the `/uninstall` endpoint allowing for clean removal of infrastructure records.

---

## 🔐 Area: Security & Auth (`internal/api/auth.go`)
**Verified Files:** `auth.go`, `OIDC.md`, `PROXY_AUTH.md`

### ✅ Verification Checklist
- [x] **Sliding Sessions**: Verified that active dashboard use extends session life via `ValidateAndExtendSession`.
- [x] **OIDC Background Refresh**: Confirmed that OIDC tokens are automatically refreshed in the background 5 minutes before expiry.
- [x] **Proxy Authentication**: Verified that Pulse can trust external SSO headers like `X-Proxy-Secret` and map roles to admin privileges.
- [x] **Brute-Force Protection**: Confirmed built-in IP and User lockout logic (15min cooldown) and rate limiting.
- [x] **Cookie Intelligence**: Verified that Pulse automatically adjusts `SameSite` and `Secure` flags based on proxy/HTTPS detection (e.g., Cloudflare Tunnel support).

---

## 🔌 Area: Proxmox Client (`pkg/proxmox/client.go`)
**Verified Files:** `client.go`, `cluster_client.go`

### ✅ Verification Checklist
- [x] **Defensive Parsing**: Verified that `FlexInt` and `coerceUint64` handle Proxmox API inconsistencies (strings vs numbers, and scientific notation).
- [x] **Auth Fallback**: Confirmed that the client automatically falls back from JSON to Form-Encoded auth for older PVE versions.
- [x] **Smart Ticket Refresh**: Verified that session tickets are automatically refreshed every 2 hours without interrupting polling.
- [x] **Memory Correction**: Confirmed the `EffectiveAvailable()` calculation accurately reflects reclaimable memory (Free + Buffers + Cached).
- [x] **Error Guidance**: Verified that the client parses 403/595 errors into actionable advice for users (e.g., reminding them to set permissions on the User, not just the Token).

---

## 🐳 Area: Docker Agent (`internal/dockeragent`)
**Verified Files:** `collect.go`, `container_update.go`, `registry.go`

### ✅ Verification Checklist
- [x] **Memory Accuracy**: Verified subtraction of reclaimable cache (cgroup v1/v2) to match `docker stats`.
- [x] **Safe Updates**: Confirmed the "Rename -> Pull -> Create -> Health Check" update lifecycle with automatic 5s rollback guard.
- [x] **Registry Intelligence**: Verified multi-arch manifest resolution and anonymous token handling for Docker Hub/GHCR.
- [x] **Mode Awareness**: Confirmed that `Unified` mode correctly aligns machine IDs with the host agent to prevent token conflicts.

---

## 🔄 Area: Update System (`internal/updates`)
**Verified Files:** `manager.go`, `adapter_installsh.go`, `version.go`

### ✅ Verification Checklist
- [x] **Reliable Discovery**: Verified the GitHub API + RSS/Atom feed fallback for update discovery.
- [x] **Atomic Updates**: Confirmed the secure pipe-delivery of `install.sh` and verification of its SHA256 checksum.
- [x] **Health-Aware Deployment**: Verified that updates are only considered successful if the `/api/health` endpoint recovers within 30s.
- [x] **History & Recovery**: Confirmed that backup paths are persisted in history, allowing for full state rollback of both binary and config.

---

## 📢 Area: Notification System (`internal/notifications`)
**Verified Files:** `notifications.go`, `email_template.go`, `queue.go`

### ✅ Verification Checklist
- [x] **SSRF Protection**: Verified that `ValidateWebhookURL` blocks loopback, private IP ranges, and dangerous redirects.
- [x] **Smart Grouping**: Confirmed `groupWindow` logic correctly bundles multiple alerts into a single delivery.
- [x] **Contextual Cooldown**: Verified that cooldown uses `AlertID` + `StartTime`, allowing immediate re-notifications for new events while suppressing noise for ongoing ones.
- [x] **Persistent Queue**: Confirmed that notifications survive service restarts via the background disk-backed queue.

---

## 🔍 Area: Network Discovery (`pkg/discovery`)
**Verified Files:** `discovery.go`, `envdetect/detect.go`

### ✅ Verification Checklist
- [x] **Environment Awareness**: Confirmed that the scanner automatically detects if it's in Docker/LXC to adjust scan targets.
- [x] **Confidence-Based Phases**: Verified that lower-likelihood subnets are scanned with lower priority and skipped if time budget is low.
- [x] **Fingerprint Accuracy**: Confirmed that the probe goes beyond port checking, using TLS and API fingerprinting with a weighted scoring engine (0.7+ threshold).

---

## 🤖 Area: AI Agent & Command Execution (`internal/agentexec`)
**Verified Files:** `policy.go`, `server.go`

### ✅ Verification Checklist
- [x] **Security Boundary**: Verified the `CommandPolicy` engine correctly categorizes commands into Auto-Approve, Require-Approval, and Blocked.
- [x] **Evasion Resistance**: Confirmed `sudo` normalization prevents simple policy bypass attempts.
- [x] **RPC Reliability**: Verified the 5s heartbeat and "3-strike" policy for managing agent connection lifecycles.

---

## 🏁 Overall Conclusion
The Pulse architecture is highly optimized for performance and reliability. The monitoring engine uses adaptive logic to protect resources, the AI system maintains a long-term memory to reduce noise, and the WebSocket hub ensures the frontend stays responsive without flooding the network.

Documentation is generally very strong, with minor discrepancies identified in areas where defaults were recently changed for token efficiency or performance.
