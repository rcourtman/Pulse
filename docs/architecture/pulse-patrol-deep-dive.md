# Pulse Patrol: Technical Deep Dive

This document provides an in-depth look at the engineering behind Pulse Patrol — a context-aware, learning AI analysis system that goes far beyond traditional threshold-based monitoring.

---

## Executive Summary

Pulse Patrol is not a simple alerting system. It's a **multi-layered intelligence platform** that:

1. **Learns** what's normal for your environment
2. **Predicts** issues before they become critical
3. **Correlates** events across your entire infrastructure
4. **Remembers** past incidents and successful remediations
5. **Investigates** issues autonomously when configured
6. **Verifies** fixes and tracks remediation effectiveness

All while running entirely on your infrastructure with BYOK (Bring Your Own Key) for complete privacy.

---

## Architecture Overview

```
   ┌─────────────────────────────────────────────────────────────────────┐
   │                      PULSE PATROL SERVICE                          │
   │                                                                     │
   │  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐              │
   │  │   Baseline   │  │   Pattern    │  │ Correlation  │              │
   │  │    Store     │  │  Detector    │  │   Detector   │              │
   │  │  (Learning)  │  │ (Prediction) │  │ (Root Cause) │              │
   │  └──────────────┘  └──────────────┘  └──────────────┘              │
   │          │                 │                 │                      │
   │          └─────────────────┼─────────────────┘                      │
   │                            │                                        │
   │                    ┌───────▼───────┐                               │
   │                    │  Intelligence │                               │
   │                    │  Orchestrator │                               │
   │                    └───────┬───────┘                               │
   │                            │                                        │
   │  ┌──────────────┐  ┌──────▼───────┐  ┌──────────────┐              │
   │  │   Incident   │  │   Forecast   │  │  Knowledge   │              │
   │  │    Store     │  │   Service    │  │    Store     │              │
   │  │  (History)   │  │ (Prediction) │  │  (Learning)  │              │
   │  └──────────────┘  └──────────────┘  └──────────────┘              │
   │                            │                                        │
   │                    ┌───────▼───────┐                               │
   │                    │    Patrol     │                               │
   │                    │   Service     │                               │
   │                    └───────┬───────┘                               │
   │                            │                                        │
   │         ┌──────────────────┼──────────────────┐                    │
   │         │                  │                  │                    │
   │   ┌─────▼─────┐     ┌─────▼─────┐     ┌─────▼─────┐               │
   │   │  Signal   │     │ Agentic   │     │ Evaluation│               │
   │   │ Detection │     │   Loop    │     │   Pass    │               │
   │   │(Determin.)│     │  (LLM)    │     │  (LLM)    │               │
   │   └───────────┘     └───────────┘     └───────────┘               │
   │                            │                                        │
   │                    ┌───────▼───────┐                               │
   │                    │ Investigation │                               │
   │                    │  Orchestrator │                               │
   │                    └───────────────┘                               │
   └─────────────────────────────────────────────────────────────────────┘
```

---

## 1. Baseline Learning Engine

**📁 Location:** `internal/ai/baseline/store.go`

### What It Does

The baseline engine learns what "normal" looks like for each resource in your environment. Rather than using static thresholds, it builds statistical models from your actual metrics history.

### How It Works

```go
type MetricBaseline struct {
    Mean          float64           // Average value
    StdDev        float64           // Standard deviation
    Min           float64           // Minimum observed
    Max           float64           // Maximum observed
    SampleCount   int               // Number of data points
    HourlySamples map[int][]float64 // Samples bucketed by hour (0-23)
}
```

**Key Features:**

- **Time-of-day awareness**: Tracks hourly buckets to understand that 3pm CPU usage differs from 3am
- **Z-score anomaly detection**: Flags deviations > 2.0 standard deviations from baseline
- **Anomaly severity classification**:
  - `normal`: z-score < 2.0
  - `mild`: 2.0 ≤ z-score < 2.5
  - `moderate`: 2.5 ≤ z-score < 3.0
  - `severe`: 3.0 ≤ z-score < 4.0
  - `extreme`: z-score ≥ 4.0

```go
// Anomaly detection with hourly context
func (s *Store) IsAnomaly(resourceID, metric string, value float64) (bool, float64) {
    baseline := s.baselines[resourceID]
    // Use hourly mean if we have enough samples for current hour
    hourlyMean, usedHourly := baseline.Metrics[metric].GetHourlyMean(time.Now().Hour())
    
    if usedHourly {
        zScore = (value - hourlyMean) / baseline.StdDev
    } else {
        zScore = (value - baseline.Mean) / baseline.StdDev
    }
    
    return math.Abs(zScore) > anomalyThreshold, zScore
}
```

**Callback System:**
```go
// Event-driven anomaly response
func (s *Store) SetAnomalyCallback(callback AnomalyCallback) {
    // Called when anomaly detected — can trigger targeted patrol
}
```

---

## 2. Pattern Detection & Prediction

**📁 Location:** `internal/ai/patterns/detector.go`

### What It Does

Tracks historical events and identifies **recurring patterns** to **predict future failures**.

### Event Types Tracked

```go
const (
    EventHighMemory   = "high_memory"   // Memory exceeded threshold
    EventHighCPU      = "high_cpu"      // CPU exceeded threshold
    EventDiskFull     = "disk_full"     // Disk space critical
    EventOOMKill      = "oom_kill"      // Out-of-memory kill
    EventServiceCrash = "service_crash" // Service crashed
    EventUnresponsive = "unresponsive"  // Resource became unresponsive
    EventBackupFailed = "backup_failed" // Backup job failed
)
```

### Pattern Structure

```go
type Pattern struct {
    ResourceID      string
    EventType       EventType
    Occurrences     int           // How many times this has happened
    AverageInterval time.Duration // Average time between occurrences
    LastOccurrence  time.Time
    NextPredicted   time.Time     // When we expect this to happen again
    Confidence      float64       // 0.0 to 1.0
    AverageDuration time.Duration // How long events typically last
}
```

### Prediction Algorithm

1. **Collect events** by resource and type
2. **Calculate inter-arrival times** between occurrences
3. **Apply exponential smoothing** for prediction
4. **Assign confidence** based on consistency and sample size

```go
type FailurePrediction struct {
    ResourceID  string
    EventType   EventType
    PredictedAt time.Time  // When we expect the failure
    DaysUntil   float64    // Days until predicted failure
    Confidence  float64    // How confident we are
    Basis       string     // "Based on 7 occurrences over 30 days"
    Pattern     *Pattern   // The underlying pattern
}
```

**Example Output:**
> "VM `prod-web-1` has experienced `high_memory` events 5 times in the past 14 days with an average interval of 2.8 days. Next occurrence predicted in ~1.5 days (confidence: 0.72)."

---

## 3. Correlation & Root Cause Analysis

**📁 Location:** `internal/ai/correlation/`

### Correlation Detector (`detector.go`)

Tracks when events on one resource are **followed by events on another**, enabling cascade failure prediction.

```go
type Correlation struct {
    SourceID     string        // Resource that has events first
    SourceName   string
    TargetID     string        // Resource that has events after
    TargetName   string
    EventType    EventType
    Occurrences  int           // How many times this sequence occurred
    AvgDelay     time.Duration // Average time from source to target event
    Confidence   float64
    Description  string        // "When storage-1 has disk_full, web-1 crashes within 5 minutes"
}
```

**Key Methods:**

```go
// Get what depends on this resource
func (d *Detector) GetDependencies(resourceID string) []string

// Get what this resource depends on
func (d *Detector) GetDependsOn(resourceID string) []string

// Predict cascade effects
func (d *Detector) PredictCascade(resourceID string, eventType EventType) []CascadePrediction
```

**Cascade Prediction Example:**
> "If `storage-1` experiences `disk_full`, expect `web-1` to become `unresponsive` within 3-7 minutes (confidence: 0.85), and `database-1` to experience `service_crash` within 10-15 minutes (confidence: 0.67)."

### Root Cause Engine (`rootcause.go`)

Goes beyond correlation to identify the **underlying cause** of related issues.

```go
type ResourceRelationship struct {
    SourceID     string
    TargetID     string
    Relationship RelationshipType // runs_on, uses_storage, uses_network, depends_on, etc.
}

type RootCauseAnalysis struct {
    ID            string
    TriggerEvent  RelatedEvent     // What started the incident
    RootCause     *RelatedEvent    // The actual root cause
    RelatedEvents []RelatedEvent   // All related events
    CausalChain   []string         // "storage-1 (disk_full) → db-1 (slow) → web-1 (timeout)"
    Confidence    float64
    Explanation   string           // Human-readable explanation
}
```

**Relationship Types:**
```go
const (
    RelationshipRunsOn      = "runs_on"      // VM runs on Node
    RelationshipUsesStorage = "uses_storage" // VM uses Storage pool
    RelationshipUsesNetwork = "uses_network" // Guest uses Network
    RelationshipDependsOn   = "depends_on"   // Generic dependency
    RelationshipHosted      = "hosted"       // Container hosted on Docker
)
```

---

## 4. Forecast Service

**📁 Location:** `internal/ai/forecast/service.go`

### What It Does

Extrapolates trends to predict when resources will exhaust capacity.

### Trend Analysis

```go
type Trend struct {
    Direction    TrendDirection // stable, increasing, decreasing, volatile
    RatePerHour  float64        // Change per hour
    RatePerDay   float64        // Change per day
    Acceleration float64        // Is the rate changing?
    Seasonality  *Seasonality   // Daily/weekly patterns
}

type Seasonality struct {
    HasDaily  bool
    HasWeekly bool
    PeakHours []int // e.g., [9, 10, 11, 14, 15, 16]
    PeakDays  []int // e.g., [1, 2, 3, 4, 5] (Mon-Fri)
}
```

### Forecasting

```go
type Forecast struct {
    ResourceID      string
    Metric          string         // cpu, memory, disk
    CurrentValue    float64
    PredictedValue  float64        // Value at horizon
    Trend           Trend
    TimeToThreshold *time.Duration // Time until critical threshold
    ThresholdValue  float64
    Description     string         // "Disk will be full in 12 days at current rate"
}
```

**Example Output:**
> "Storage pool `local-zfs` is at 78% and growing +2.3%/day. At this rate, it will reach 95% (critical) in 7.4 days. Weekly pattern detected: higher growth on weekdays. Recommend action by end of week."

---

## 5. Incident Memory System

**📁 Location:** `internal/ai/memory/`

### Incident Store (`incidents.go`)

Maintains full incident timelines with auditability.

```go
type Incident struct {
    ID           string
    AlertID      string
    AlertType    string
    ResourceID   string
    ResourceName string
    Severity     string
    StartedAt    time.Time
    ResolvedAt   *time.Time
    Duration     time.Duration
    Acknowledged bool
    AckUser      string
    AckTime      *time.Time
    Events       []IncidentEvent  // Full timeline
}

type IncidentEvent struct {
    ID        string
    Type      IncidentEventType  // alert_fired, acknowledged, analysis, command, resolved
    Timestamp time.Time
    Summary   string
    Details   map[string]interface{}
}
```

**Event Types:**
- `alert_fired` — Initial alert trigger
- `alert_acknowledged` — User acknowledged the alert
- `analysis` — AI analyzed the issue
- `command` — Command was executed
- `runbook` — Runbook was triggered
- `note` — User added a note
- `resolved` — Alert was resolved

### Remediation Log (`remediation.go`)

Tracks every remediation action for learning and rollback.

```go
type RemediationRecord struct {
    ID           string
    Timestamp    time.Time
    ResourceID   string
    ResourceType string
    ResourceName string
    FindingID    string        // Linked to the finding
    Problem      string        // What was wrong
    Action       string        // What was done
    Command      string        // Actual command executed
    Output       string        // Command output
    Outcome      Outcome       // resolved, partial, failed
    Duration     time.Duration
    Automatic    bool          // Was this auto-fix or manual?
    Rollback     *RollbackInfo // Rollback capability
}

type RollbackInfo struct {
    Reversible   bool
    RollbackCmd  string      // Command to undo
    PreState     string      // State before action
    RolledBack   bool
    RolledBackAt *time.Time
    RolledBackBy string
    RollbackID   string
}
```

**Key Capabilities:**

```go
// Find remediations that worked for similar problems
func (r *RemediationLog) GetSuccessfulRemediations(problem string, limit int) []RemediationRecord

// Get remediations that can be undone
func (r *RemediationLog) GetRollbackable(limit int) []RemediationRecord

// Mark a remediation as rolled back
func (r *RemediationLog) MarkRolledBack(id, rollbackID, username string) error
```

---

## 6. Knowledge Store

**📁 Location:** `internal/ai/knowledge/store.go`

### What It Does

Stores **persistent, per-resource knowledge** that the AI learns over time, encrypted at rest.

### Note Categories

```go
const (
    CategoryService    = "service"        // What services run here
    CategoryConfig     = "config"         // Configuration notes
    CategoryLearning   = "learning"       // AI-learned facts
    CategoryHistory    = "history"        // Historical context
    CategoryInfra      = "infrastructure" // Auto-discovered facts
)
```

### Structure

```go
type GuestKnowledge struct {
    GuestID   string
    GuestName string
    GuestType string
    Notes     []Note
    UpdatedAt time.Time
}

type Note struct {
    ID        string
    Category  string
    Title     string
    Content   string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### Discovery Context Integration

```go
// Inject discovery context (versions, ports, config paths) into investigations
func (s *Store) SetDiscoveryContextProvider(provider func() string)

// Scoped context for specific resources
func (s *Store) GetInfrastructureContextForResources(resourceIDs []string) string
```

**Example Knowledge:**
> **Service**: "Runs Jellyfin media server on port 8096, Caddy reverse proxy on 443"
> **Config**: "Config at /opt/jellyfin/config, database at /var/lib/jellyfin"  
> **Learning**: "High memory usage expected during transcoding — threshold 90% is normal"

---

## 7. Deterministic Signal Detection

**📁 Location:** `internal/ai/patrol_signals.go`

### Philosophy

Patrol combines **LLM judgment** with **deterministic detection** to ensure no issues are missed, even if the LLM overlooks something.

### Signal Types

```go
const (
    SignalSMARTFailure = "smart_failure" // SMART health check failed
    SignalHighCPU      = "high_cpu"      // CPU exceeded threshold
    SignalHighMemory   = "high_memory"   // Memory exceeded threshold
    SignalHighDisk     = "high_disk"     // Storage pool filling up
    SignalBackupFailed = "backup_failed" // Backup task failed
    SignalBackupStale  = "backup_stale"  // No backup in 48+ hours
    SignalActiveAlert  = "active_alert"  // Critical/warning alert present
)
```

### Configurable Thresholds

```go
type SignalThresholds struct {
    StorageWarningPercent  float64       // Default: 75%
    StorageCriticalPercent float64       // Default: 95%
    HighCPUPercent         float64       // Default: 70%
    HighMemoryPercent      float64       // Default: 80%
    BackupStaleThreshold   time.Duration // Default: 48 hours
}

// Thresholds can sync with user-configured alert settings
func SignalThresholdsFromPatrol(pt PatrolThresholds) SignalThresholds
```

### Detection Flow

1. **Tool calls complete** during patrol
2. **DetectSignals()** parses tool outputs for known patterns
3. **UnmatchedSignals()** compares against findings the LLM reported
4. **Evaluation pass** — if signals were missed, a focused LLM call reviews them
5. **Fallback creation** — if still unmatched, deterministic findings are created

---

## 8. Investigation Orchestrator

**📁 Location:** `internal/ai/investigation/`

### Investigation Session

```go
type InvestigationSession struct {
    ID             string
    FindingID      string
    SessionID      string     // Chat session ID
    Status         Status     // pending, running, completed, failed, needs_attention
    StartedAt      time.Time
    CompletedAt    *time.Time
    TurnCount      int        // Agentic turns used
    Outcome        Outcome
    ProposedFix    *Fix
    ApprovalID     string     // If queued for approval
    ToolsAvailable []string
    ToolsUsed      []string
    EvidenceIDs    []string
    Summary        string
    Error          string
}
```

### Configuration

```go
type InvestigationConfig struct {
    MaxTurns                int           // Default: 15
    Timeout                 time.Duration // Default: 10 minutes
    MaxConcurrent           int           // Default: 3
    MaxAttemptsPerFinding   int           // Default: 3
    CooldownDuration        time.Duration // Default: 1 hour
    TimeoutCooldownDuration time.Duration // Default: 10 minutes (shorter for timeouts)
    VerificationDelay       time.Duration // Default: 30 seconds
}
```

### Fix Structure

```go
type Fix struct {
    ID          string
    Description string
    Commands    []string
    RiskLevel   string   // low, medium, high, critical
    Destructive bool     // Flagged by pattern matching
    TargetHost  string
    Rationale   string
}
```

### Destructive Command Detection

Commands are scanned for dangerous patterns:
- `rm -rf`, `dd if=`, `mkfs.`
- `shutdown`, `reboot`, `poweroff`
- `iptables -F`, `ufw disable`
- Database drops, config wipes

---

## 9. Agentic Patrol Loop

**📁 Location:** `internal/ai/patrol_ai.go`

### Dynamic Turn Budget

```go
const (
    patrolMinTurns          = 20
    patrolMaxTurnsLimit     = 80
    patrolTurnsPer50Devices = 5  // +5 turns per 50 devices
    patrolQuickMinTurns     = 10 // Scoped patrols are faster
    patrolQuickMaxTurns     = 30
)

func computePatrolMaxTurns(resourceCount int, scope *PatrolScope) int {
    // Scales with environment size
}
```

### Patrol Phases

1. **Seed Context Building** — Inventory + thresholds + active findings + notes
2. **Streaming Analysis** —  LLM investigates using MCP tools
3. **Signal Detection** — Deterministic check on tool outputs
4. **Evaluation Pass** — Focused review of missed signals
5. **Stale Finding Reconciliation** — Resolve findings whose issues cleared
6. **Investigation Triggering** — Queue findings for deep investigation

### Thinking Token Cleanup

Responses from DeepSeek and other models may include internal reasoning markers. Patrol strips these:

```go
func CleanThinkingTokens(content string) string {
    // Removes:
    // - <think>...</think>
    // - <｜end▁of▁thinking｜>
    // - <｜DSML｜...> (DeepSeek internal format)
    // - Internal reasoning lines ("Now, ", "Let me ", etc.)
}
```

---

## 10. Seed Context (What Patrol Sees)

Every patrol run builds comprehensive context:

| Component | What's Included |
|-----------|-----------------|
| **Nodes** | Status, load, uptime, 24h/7d trends, anomaly flags |
| **VMs/LXCs** | Full metrics, backup status, OCI images, trend direction |
| **Storage** | Usage %, growth rate, days-to-full prediction |
| **Docker** | Container counts, health states, update availability |
| **Kubernetes** | Nodes, pods, deployments, services, namespaces |
| **PBS** | Datastore status, job outcomes, verification results |
| **PMG** | Mail queue depths, spam stats, delivery rates |
| **Agents** | Connection status, permissions, scope restrictions |
| **Ceph** | Cluster health, OSD states, PG status |
| **Baselines** | Per-resource learned normal ranges |
| **Patterns** | Detected recurring issues and predictions |
| **Correlations** | Known dependencies and cascade risks |
| **Forecasts** | Capacity predictions for at-risk resources |
| **Active Findings** | Existing issues being tracked |
| **User Notes** | Your annotations explaining expected behavior |
| **Suppression Rules** | What you've dismissed as not-an-issue |
| **Recent Remediations** | What worked (or failed) recently |

---

## 11. Findings Lifecycle

```
┌─────────────────────────────────────────────────────────┐
│                    Finding Created                      │
│  (by LLM via patrol_report_finding or deterministic)    │
└───────────────────────────┬─────────────────────────────┘
                            │
                            ▼
            ┌───────────────────────────────┐
            │    Threshold Validation       │
            │  (Must exceed user thresholds)│
            └───────────────┬───────────────┘
                            │
           Pass ◄───────────┼───────────► Reject (filtered out)
                            │
                            ▼
            ┌───────────────────────────────┐
            │   Semantic Deduplication      │
            │ (Similar finding already open?)│
            └───────────────┬───────────────┘
                            │
           New ◄────────────┼────────────► Merge (bump count)
                            │
                            ▼
            ┌───────────────────────────────┐
            │       Finding Stored          │
            │   (ai_findings.json)          │
            └───────────────┬───────────────┘
                            │
            ┌───────────────┼───────────────┐
            ▼               ▼               ▼
         ┌──────┐     ┌──────────┐   ┌────────────┐
         │Active│     │Investigate│   │ Auto-fix   │
         │(idle)│     │(approval) │   │(autonomous)│
         └──────┘     └─────┬─────┘   └─────┬──────┘
                            │               │
                            ▼               ▼
                    ┌───────────────────────────┐
                    │  Fix Proposed / Executed  │
                    └───────────┬───────────────┘
                                │
                                ▼
                    ┌───────────────────────────┐
                    │   Verification Delay      │
                    │      (30 seconds)         │
                    └───────────┬───────────────┘
                                │
                                ▼
                    ┌───────────────────────────┐
                    │   Verification Check      │
                    │  (Is the issue resolved?) │
                    └───────────┬───────────────┘
                                │
         ┌──────────────────────┼──────────────────────┐
         ▼                      ▼                      ▼
    ┌─────────┐          ┌────────────┐        ┌──────────────┐
    │Resolved │          │  Persists  │        │Needs Attention│
    │(closed) │          │(retry later)│       │ (escalate)    │
    └─────────┘          └────────────┘        └──────────────┘
```

---

## 12. What Makes Patrol Different

| Traditional Alerting | Pulse Patrol |
|---------------------|--------------|
| Static thresholds | Learned baselines + context |
| Single metric | Cross-system correlation |
| Instant alerts | Trend-aware predictions |
| No memory | Incident history + pattern learning |
| Manual investigation | Autonomous investigation |
| Manual fixes | Verified auto-remediation |
| Alert fatigue | Noise-controlled findings |
| Siloed tools | Unified intelligence |

---

## 13. Privacy & Security

- **BYOK**: All AI calls use your API keys to your chosen provider
- **On-premises**: All processing happens on your Pulse server
- **Encrypted storage**: Sensitive data (keys, licenses) stored encrypted
- **Minimal context**: Only necessary data sent to AI providers
- **No telemetry**: No data sent to Pulse by default
- **Audit trail**: All actions logged with timestamps and user attribution

---

## Files Reference

| Directory/File | Purpose |
|----------------|---------|
| `internal/ai/patrol.go` | Core PatrolService definition, interfaces |
| `internal/ai/patrol_run.go` | Patrol loop, scoped runs, lifecycle |
| `internal/ai/patrol_ai.go` | LLM integration, agentic loop, context building |
| `internal/ai/patrol_signals.go` | Deterministic signal detection |
| `internal/ai/patrol_findings.go` | Finding CRUD, investigation triggers |
| `internal/ai/patrol_triggers.go` | Event-driven patrol triggers |
| `internal/ai/intelligence.go` | Unified intelligence orchestrator |
| `internal/ai/baseline/` | Baseline learning and anomaly detection |
| `internal/ai/patterns/` | Pattern detection and failure prediction |
| `internal/ai/correlation/` | Correlation detection and root cause analysis |
| `internal/ai/forecast/` | Trend extrapolation and capacity forecasting |
| `internal/ai/memory/` | Incidents, remediations, context tracking |
| `internal/ai/knowledge/` | Persistent per-resource knowledge store |
| `internal/ai/investigation/` | Investigation orchestrator and sessions |
| `internal/ai/tools/` | MCP tool implementations (50+ tools) |

---

## Summary

Pulse Patrol represents a comprehensive approach to infrastructure intelligence:

1. **It learns** — Baselines, patterns, correlations
2. **It predicts** — Forecasts, failure predictions, cascade analysis
3. **It remembers** — Incidents, remediations, knowledge
4. **It investigates** — Autonomous diagnosis with tool access
5. **It fixes** — Verified remediation with rollback capability
6. **It improves** — Tracks what works and learns from outcomes

All running on your infrastructure, with your AI keys, with complete transparency.
