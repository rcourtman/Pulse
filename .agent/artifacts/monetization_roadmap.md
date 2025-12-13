# Pulse Monetization Readiness Roadmap

> Created: 2025-12-13
> Status: Planning → Implementation

## Executive Summary

Based on GitHub insights (3,068 stars, 994 cloners/2wks, 458k downloads latest release) and user feedback, Pulse has strong organic adoption. The codebase already has solid foundations for monetization, but several enhancements will make tier-based pricing natural.

## Current State ✅

### What's Already Built
- **Historical metrics storage** (SQLite with tiered rollups: raw→minute→hourly→daily)
- **Unified resource model** (platform-agnostic Resource type)
- **Multi-platform support** (PVE, PBS, PMG, Docker, K8s, TrueNAS, Host-Agent)
- **Cross-source deduplication** (ResourceIdentity)
- **AI opt-in with BYO key** (no cost burden on Pulse)
- **Cluster awareness** (ClusterEndpoints, multi-node)

### Default Retention (hardcoded in `internal/metrics/store.go`)
- Raw: 2 hours
- Minute: 24 hours  
- Hourly: 7 days
- Daily: 90 days

---

## Phase 1: Configurable Foundations (This Sprint)

### 1.1 Configurable Metrics Retention ✅ COMPLETE
**Goal:** Allow users to configure retention periods via settings.

**Why it matters for monetization:**
- Natural tier split: "Free = 7 days, Paid = 90 days"
- Users who want longer history have a clear upgrade path

**Implementation (Completed 2025-12-13):**
- [x] Added `MetricsRetention*` fields to `SystemSettings` in `config/persistence.go`
- [x] Added corresponding fields to `Config` in `config/config.go`
- [x] Set sensible defaults (Raw: 2h, Minute: 24h, Hourly: 7d, Daily: 90d)
- [x] Wired config to `metrics.StoreConfig` in `monitor.go`
- [x] Added debug logging showing active retention values

**Files modified:**
- `internal/config/persistence.go` - Added 4 new fields to SystemSettings
- `internal/config/config.go` - Added 4 new fields to Config, loading logic, defaults
- `internal/monitoring/monitor.go` - Wire config values to metrics store

**Next steps for this feature:**
- [ ] Add retention settings UI in Settings page
- [ ] Expose via API for programmatic configuration

### 1.2 Metrics History API Endpoint
**Goal:** Expose historical metrics via REST API for frontend charts.

**Why it matters:**
- Users can see value of stored history
- Charts showing 7d/30d/90d trends become possible
- Makes the retention config visible to users

**Implementation:**
- [ ] Add `/api/metrics/history/:resourceType/:resourceId` endpoint
- [ ] Support query params: `start`, `end`, `metric` (cpu, memory, disk, network)
- [ ] Auto-select appropriate tier based on time range

---

## Phase 2: Multi-Tenant Foundations (Next Sprint)

### 2.1 Tenant/Organization Model
**Goal:** Add optional tenant isolation for MSP use cases.

**Why it matters:**
- MSPs are the primary paying audience
- Each client = one tenant, isolated data
- Per-tenant billing becomes possible

**Implementation:**
- [ ] Add `TenantID` field to Resource model (optional, empty = single-tenant)
- [ ] Add tenant filtering to Store queries
- [ ] Add tenant selector to UI (hidden in single-tenant mode)

### 2.2 RBAC Enhancements
**Goal:** Role-based access control per tenant.

**Why it matters:**
- Enterprise requirement
- MSP operators vs client viewers

---

## Phase 3: Reports & Exports (Future)

### 3.1 Backup Compliance Reports
**Goal:** Weekly/monthly PDF/email reports on backup status.

**Why it matters:**
- Top user request ("List of unbacked-up VMs")
- Obvious paid feature
- Sticky habit (scheduled reports)

### 3.2 Capacity Planning Reports
**Goal:** Trend-based forecasting for storage and compute.

**Why it matters:**
- Uses historical data (paid tier dependency)
- High value for ops teams

---

## Phase 4: AI Enhancements (Future)

### 4.1 Grounded Incident Summaries
**Goal:** "Explain this alert with evidence" + "Draft incident report"

**Why it matters:**
- Only valuable because of Pulse context
- Can't be replicated by Claude Code alone

### 4.2 What-Changed Analysis
**Goal:** "What changed in the last 24h that might explain this issue?"

**Why it matters:**
- Requires historical data (ties to paid retention)
- High-value for triage

---

## Proposed Tier Structure (Future Reference)

| Feature | Free | Pro | Enterprise |
|---------|------|-----|------------|
| Clusters | 1 | Unlimited | Unlimited |
| Retention | 7 days | 90 days | 1 year |
| Alert channels | 1 | 5 | Unlimited |
| Backup reports | - | Weekly | Custom |
| RBAC | - | - | ✓ |
| Multi-tenant | - | - | ✓ |
| AI features | Basic | Full | Full + Custom |
| Support | Community | Email | Priority |

---

## Implementation Priority

1. **Configurable retention** (enables tier split)
2. **Metrics history API** (makes history visible)
3. **Frontend history charts** (user-visible value)
4. **Multi-tenant model** (MSP readiness)
5. **Reports** (paid feature)

Starting with #1 now.
