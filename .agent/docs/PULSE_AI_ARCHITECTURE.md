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

## Implementation Roadmap

### Phase 1: Historical Context Integration

**Goal**: Make the AI aware of trends and history, not just current state.

1. **Create `internal/ai/context/` package**
   - `historical.go` - Pull data from MetricsHistory
   - `trends.go` - Compute trend direction, rate of change
   - `formatter.go` - Format for AI consumption

2. **Trend Computation**
   - Simple linear regression for direction
   - Rate of change calculation
   - Stability classification (stable/growing/declining/volatile)

3. **Integrate into Patrol and Chat**
   - `buildEnrichedContext()` replaces `buildInfrastructureSummary()`
   - Include "Last 24h" and "Last 7d" summaries

**Example output:**
```markdown
## VM: webserver (node: minipc)
Current: CPU=12%, Memory=67%, Disk=45%
24h Trend: CPU stable (8-15%), Memory growing +1.2%/hr, Disk stable
7d Trend: Memory +15% total (was 52% a week ago)
Baseline: CPU normal=5-20%, Memory normal=45-60% (currently elevated)
```

### Phase 2: Anomaly Detection

**Goal**: Automatically detect when something is "unusual" for this specific infrastructure.

1. **Baseline Learning**
   - Track rolling statistics per resource (mean, std dev, percentiles)
   - Time-of-day / day-of-week patterns
   - Persist baselines across restarts

2. **Anomaly Scoring**
   - Statistical deviation from baseline
   - Pattern breaks (e.g., usually low at night, now high)
   - Sudden changes vs. gradual drift

3. **Anomaly Context for AI**
   - "This is unusual" annotations
   - Confidence levels
   - Similar past anomalies and outcomes

**Example output:**
```markdown
⚠️ ANOMALY: VM 'database' memory at 89%
- Baseline for this time: 45-55%
- Current value is 4.2σ above normal
- Similar anomaly 2 weeks ago led to OOM (resolved by restart)
```

### Phase 3: Operational Memory

**Goal**: The AI remembers what happened and what worked.

1. **Remediation Logging**
   - When AI suggests/executes a fix, log it
   - Track outcome (did it work? for how long?)
   - Link to findings

2. **Change Detection**
   - Detect configuration changes (new VMs, resource changes)
   - Correlate changes with subsequent issues
   - "This problem started 2 days after you added GPU passthrough"

3. **Solution Database**
   - Index past problems and solutions
   - "We've seen this before: [link to past finding]"
   - "Last time, restarting the service fixed it"

**Example output:**
```markdown
## Historical Context for VM 'webserver'
- Created: 6 months ago
- Last modified: 2 weeks ago (RAM increased 4GB→8GB)
- Past issues:
  - 2 weeks ago: High memory (resolved by RAM increase)
  - 1 month ago: Disk full (resolved by log rotation)
- User note: "Runs production web app, critical 9-5"
```

### Phase 4: Predictive Intelligence

**Goal**: Warn users before problems occur.

1. **Capacity Forecasting**
   - Extrapolate growth trends
   - "Storage will be full in X days at current rate"
   - Account for patterns (e.g., weekly backup spikes)

2. **Failure Prediction**
   - Resources that fail periodically (e.g., OOM every 2 weeks)
   - Predict next occurrence
   - "This container typically OOMs every ~10 days, last was 8 days ago"

3. **Correlation-Based Alerts**
   - "When VM A memory exceeds 80%, VM B usually crashes within 2 hours"
   - Learn these from historical data

**Example output:**
```markdown
## Predictions
⏰ Storage 'local-zfs': Full in ~18 days at current growth rate
⏰ Container 'logstash': Historically OOMs every 10-14 days (last: 9 days ago)
⏰ Backup jobs: Growing 5% per week, will exceed window in ~6 weeks
```

### Phase 5: Multi-Resource Correlation

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
