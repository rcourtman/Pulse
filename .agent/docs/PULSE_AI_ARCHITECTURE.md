# Pulse AI Architecture: Long-Term Vision

## The Core Problem

Pulse AI currently provides "AI that can talk to your infrastructure." But this is becoming commodity. Any user can:
1. Install Claude Code / Cursor / Windsurf
2. Give it SSH access to their Proxmox nodes
3. Ask "What's wrong with my infrastructure?"

**We need to provide value that a stateless AI session cannot.**

---

## The Fundamental Insight

A stateless AI with SSH access can answer: **"What is the current state?"**

Pulse, with its continuous monitoring, can answer:
- **"How has this changed over time?"**
- **"What does 'normal' look like for YOUR infrastructure?"**
- **"What's about to go wrong?"**
- **"Have we seen this pattern before?"**
- **"What did you do last time this happened?"**

These require **persistent context** that accumulates over time. This is our moat.

---

## Architecture Principles

### 1. Context is King

The AI is only as useful as the context we provide. We should think of Pulse as a **context accumulation engine** that happens to have an AI interface.

Every piece of data Pulse collects should be available to the AI in a digestible form:
- Real-time metrics
- Historical trends
- User annotations
- Alert history
- Previous AI findings
- Configuration changes
- Remediation history

### 2. Time-Aware Intelligence

The AI should always know:
- What's happening **now**
- What happened **before** (trends, history)
- What will likely happen **next** (forecasts)
- What's **different** from normal (anomalies)

### 3. Learning From Operations

Every interaction with Pulse teaches it about the user's infrastructure:
- Dismissed findings → "This is expected behavior"
- User notes → "This VM runs the critical database"
- Alert patterns → "This resource is flaky on Tuesdays"
- Remediation actions → "Last time this happened, we restarted the service"

### 4. Proactive, Not Just Reactive

The goal isn't just to answer questions. It's to:
- Surface problems before users ask
- Predict capacity issues weeks in advance
- Notice patterns humans would miss
- Remember what humans would forget

---

## Data Architecture

### Layer 1: Real-Time State (Already Have)

```
StateSnapshot
├── Nodes[]
├── VMs[]
├── Containers[]
├── Storage[]
├── DockerHosts[]
├── PBSInstances[]
├── Hosts[]
└── PMGInstances[]
```

This is what we send to the AI today. Point-in-time. Commodity.

### Layer 2: Historical Metrics (Partially Have)

```
MetricsHistory
├── NodeMetrics[nodeID] → {CPU[], Memory[], Disk[]} over time
├── GuestMetrics[guestID] → {CPU[], Memory[], Network[]} over time
└── StorageMetrics[storageID] → {Usage[], Used[], Total[]} over time
```

We collect this for the frontend trendlines, but **don't expose it to the AI**.

### Layer 3: Computed Insights (Need to Build)

```
InsightsStore
├── Trends[resourceID] → {direction, rate_of_change, forecast}
├── Baselines[resourceID] → {normal_cpu_range, normal_memory_range, typical_patterns}
├── Anomalies[resourceID] → {current_deviations, severity}
├── Correlations[] → {resource_a, resource_b, relationship}
└── Predictions[] → {resource, metric, predicted_event, eta}
```

This is computed from historical data and provides **derived intelligence**.

### Layer 4: Operational Memory (Partially Have)

```
OperationalMemory
├── Findings[findingID] → {status, user_response, resolution}
├── Knowledge[guestID] → {user_notes, learned_facts}
├── AlertHistory[] → {alert, duration, resolution, user_action}
├── RemediationLog[] → {problem, action_taken, outcome, timestamp}
└── ChangeLog[] → {resource, what_changed, when, detected_impact}
```

This captures **what happened and how it was handled**.

---

## The AI Context Pipeline

When the AI needs context (for chat, patrol, or alert analysis), we build it in layers:

```
┌─────────────────────────────────────────────────────────────┐
│                    CONTEXT ASSEMBLY                         │
├─────────────────────────────────────────────────────────────┤
│                                                             │
│  1. CURRENT STATE (required)                                │
│     - Real-time metrics for relevant resources              │
│     - Current alerts and their status                       │
│                                                             │
│  2. HISTORICAL CONTEXT (high value)                         │
│     - Trends: "Memory has been growing 3%/day for 5 days"   │
│     - Baselines: "Normal CPU for this VM is 5-15%"          │
│     - Anomalies: "Current 45% is 3σ above normal"           │
│                                                             │
│  3. OPERATIONAL CONTEXT (essential for continuity)          │
│     - Previous findings for this resource                   │
│     - User notes: "This is the production database"         │
│     - Past remediations: "We increased RAM last month"      │
│                                                             │
│  4. PREDICTIVE CONTEXT (proactive value)                    │
│     - Forecasts: "At current rate, disk full in 12 days"    │
│     - Pattern alerts: "This usually fails after X"          │
│     - Correlations: "When A spikes, B usually follows"      │
│                                                             │
│  5. USER CONTEXT (personalization)                          │
│     - Infrastructure notes: "This is a homelab"             │
│     - Preferences: "I prefer conservative recommendations"  │
│     - Expertise level: "User is comfortable with CLI"       │
│                                                             │
└─────────────────────────────────────────────────────────────┘
```

---

## Implementation Status

### ✅ Phase 1: Historical Context Integration (COMPLETE)

**Implemented in `internal/ai/context/` package:**

- `builder.go` - Context builder with trend and prediction integration
- `formatter.go` - Format resources with metrics for AI consumption
- `trends.go` - Linear regression for trend direction and rate of change

**Features:**
- Trend computation (growing/declining/stable/volatile)
- 24h and 7d trend summaries
- Rate of change calculations
- Integrated into patrol and chat via `buildEnrichedContext()`

### ✅ Phase 2: Anomaly Detection (COMPLETE)

**Implemented in `internal/ai/baseline/` package:**

- `store.go` - Statistical baseline learning and anomaly detection

**Features:**
- Rolling statistics per resource (mean, stddev, percentiles)
- Z-score based anomaly severity (low/medium/high/critical)
- Persists baselines to disk (`ai_baselines.json`)
- Background learning loop (hourly updates)
- 7-day learning window with minimum sample requirements

### ✅ Phase 3: Operational Memory (COMPLETE)

**Implemented in `internal/ai/memory/` package:**

- `changes.go` - Change detection for infrastructure changes
- `remediation.go` - Remediation action logging

**Change Detection tracks:**
- Resource creation/deletion
- Status changes (started, stopped)
- VM/container migrations between nodes
- CPU/memory configuration changes
- Backup completions

**Remediation logging records:**
- Command executed and output
- Problem being addressed
- Linked finding ID (if any)
- Outcome (resolved/partial/failed/unknown)
- Automatic vs manual distinction

### ✅ Phase 4: Remediation Integration (COMPLETE)

**AI now learns from past fixes:**

- Commands logged to remediation log after execution
- System prompts include "Past Successful Fixes for Similar Issues"
- System prompts include "Remediation History for This Resource"
- Keyword matching finds relevant past solutions

**Example AI context now includes:**
```markdown
## Past Successful Fixes for Similar Issues
These actions worked for similar problems before:
- **High memory usage causing slo...**: `apt clean && apt autoremove` (resolved)

## Remediation History for This Resource
- 2 hours ago: Memory at 95% → `systemctl restart nginx` (resolved)
- 1 day ago: Disk full warning → `journalctl --vacuum-time=1d` (resolved)
```

---

## Next Steps

### ✅ Phase 5: Predictive Intelligence (COMPLETE)

**Implemented in `internal/ai/patterns/` package:**

- `detector.go` - Pattern detector for failure prediction

**Features:**
1. **Capacity Forecasting** ✅
   - Extrapolate growth trends
   - "Storage will be full in X days at current rate"

2. **Failure Prediction** ✅
   - Track historical events (high memory, OOM, restarts, etc.)
   - Detect recurring patterns with interval analysis
   - Calculate confidence based on pattern consistency
   - Predict next occurrence time
   - Persists to `ai_patterns.json`

3. **Alert History Integration** ✅
   - Callback system in `alerts.HistoryManager`
   - Every alert is recorded as a historical event
   - Pattern detector learns from production alerts

**Example AI context now includes:**
```markdown
## ⏰ Failure Predictions
Based on historical patterns:
- high memory usage typically occurs every ~7 days (next expected in ~3 days)
- OOM events typically occurs every ~14 days (last: 12 days ago, overdue)
```

### Phase 6: Multi-Resource Correlation (PLANNED)

**Goal**: Understand relationships between resources.

1. **Automatic Correlation Detection**
   - When A spikes, does B spike?
   - When A restarts, does B show errors?
   - Statistical correlation over time

2. **Dependency Mapping**
   - User-provided: "This VM depends on that NFS storage"
   - Inferred: "These 3 containers always restart together"

3. **Cascade Analysis**
   - "If node X goes down, these 5 critical VMs are affected"
   - "Storage Y failing would impact 12 backup jobs"

---

## AI Prompt Structure

With this architecture, a typical AI prompt would look like:

```markdown
# Infrastructure Analysis Request

## Target Resource
VM 'database' (ID: 102, Node: pve-main)

## Current State
- Status: running
- CPU: 78% (normal: 15-30%)
- Memory: 92% (normal: 60-75%)
- Disk: 67% (stable)
- Uptime: 45 days

## Historical Context (7 days)
- Memory: Growing +2.1%/day (was 77% 7 days ago)
- CPU: Elevated since 3 days ago (was 20%)
- Pattern: No daily cycles detected, continuous growth

## Anomaly Score: HIGH
- Memory 2.8σ above baseline
- CPU 3.1σ above baseline
- Combined anomaly score: 87/100

## Operational History
- Last issue: 3 months ago, high memory (user added swap, resolved)
- User notes: "Production PostgreSQL, critical, no downtime allowed"
- Related resources: Depends on storage 'ceph-ssd', accessed by VMs 105, 107, 112

## Recent Changes
- 4 days ago: VM 105 ('app-server') was updated
- 3 days ago: This VM's CPU started increasing

## Predictions
- At current rate, memory will hit 100% in ~4 days
- Similar pattern to last incident (high memory leading to OOM)

## User Question
"Why is my database server slow?"
```

**This context is impossible to replicate with a stateless SSH session.**

---

## Success Metrics

How do we know Pulse AI is providing value?

1. **Predictive Accuracy**
   - Did our capacity forecasts come true?
   - Did predicted failures occur?

2. **Time to Resolution**
   - How long from problem detection to resolution?
   - Compare AI-assisted vs. manual

3. **Proactive Catches**
   - Problems found by patrol before user noticed
   - Predictions that led to preventive action

4. **User Engagement**
   - Are users adding notes? (means they trust the system)
   - Are they dismissing findings with reasons? (feedback loop)
   - Repeat usage of chat feature

5. **Context Utilization**
   - Is the AI using historical context in responses?
   - Are predictions being cited in findings?

---

## Technical Considerations

### Data Retention
- Short-term (24h): High-resolution metrics for immediate analysis
- Medium-term (7-30d): Hourly aggregates for trend analysis
- Long-term (90d+): Daily summaries for baseline/pattern learning

### Performance
- Context building must be fast (<100ms)
- Precompute expensive analytics (trends, baselines) on schedule
- Cache formatted context, invalidate on significant changes

### Storage
- Baselines and insights are small, store in SQLite or JSON
- Historical metrics can grow; implement rollup/aggregation
- Consider time-series database for scale (InfluxDB, TimescaleDB)

### Privacy
- All data stays local (no cloud sync of infrastructure data)
- AI context is built locally, only prompts go to API
- User controls what context is included

---

## Summary

The path to differentiating Pulse AI:

| Today | Tomorrow |
|-------|----------|
| "Here's your current state" | "Here's what's changed and why it matters" |
| "This metric is high" | "This is unusual for YOUR infrastructure" |
| "You should check X" | "Last time this happened, you did Y and it worked" |
| "Something might be wrong" | "X will fail in 5 days if this continues" |
| Stateless queries | Accumulated operational intelligence |

**The AI becomes more valuable the longer Pulse runs.** This is the moat.
