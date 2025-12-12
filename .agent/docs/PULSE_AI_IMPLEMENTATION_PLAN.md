# Pulse AI Implementation Plan

This document outlines the concrete implementation steps to realize the Pulse AI vision.

---

## Current State Audit

### What We Have

| Component | Location | Status |
|-----------|----------|--------|
| Real-time state | `models.StateSnapshot` | ✅ Complete |
| Metrics collection | `monitoring.MetricsHistory` | ✅ Collecting, exposed to AI |
| Finding persistence | `ai.FindingsStore` | ✅ Works |
| Knowledge store | `ai/knowledge.Store` | ✅ Per-guest notes |
| Alert context | `ai.buildAlertContext()` | ✅ Current alerts only |
| User annotations | `buildUserAnnotationsContext()` | ✅ Basic |
| Base patrol | `patrol.go` | ✅ Heuristics + optional AI |
| **AI Context package** | `ai/context/` | ✅ **NEW - Phase 1** |
| **Trend computation** | `ai/context/trends.go` | ✅ **NEW - Linear regression** |
| **Context builder** | `ai/context/builder.go` | ✅ **NEW - Orchestration** |
| **Metrics adapter** | `ai/metrics_history_adapter.go` | ✅ **NEW - Wiring** |

### What's Missing

| Component | Impact | Priority | Status |
|-----------|--------|----------|--------|
| Historical context for AI | Core differentiator | P0 | ✅ Done |
| Trend computation | Predictive capability | P0 | ✅ Done |
| Baseline learning | Anomaly detection | P1 | 🔲 Next |
| Change detection | Root cause analysis | P1 | 🔲 Planned |
| Remediation logging | Operational memory | P2 | 🔲 Planned |
| Correlation engine | Advanced insights | P2 | 🔲 Future |
| Capacity forecasting | Proactive alerts | P1 | ⚡ Partial (storage predictions) |

---

## Phase 1: Foundation - AI Context Package

**Goal**: Create a clean abstraction for building AI context with historical data.

### 1.1 New Package Structure

```
internal/ai/context/
├── builder.go          # Main context builder orchestrator
├── current.go          # Current state formatting (refactor from patrol)
├── historical.go       # Historical metrics integration
├── trends.go           # Trend computation
├── insights.go         # Combined insights (anomalies, predictions)
├── formatter.go        # AI-friendly text formatting
└── types.go            # Shared types
```

### 1.2 Core Types

```go
// types.go

// ResourceContext contains all context for a single resource
type ResourceContext struct {
    ResourceID   string
    ResourceType string // "node", "vm", "container", "storage", "docker_host"
    ResourceName string
    
    // Current state
    Current CurrentState
    
    // Historical analysis
    Trends    map[string]Trend    // metric -> trend
    Baselines map[string]Baseline // metric -> baseline
    Anomalies []Anomaly
    
    // Operational memory
    PastFindings    []FindingSummary
    UserNotes       []string
    RecentChanges   []Change
    LastRemediation *RemediationRecord
}

// Trend represents the direction and rate of change for a metric
type Trend struct {
    Metric        string
    Direction     TrendDirection // stable, growing, declining, volatile
    RatePerHour   float64        // rate of change per hour
    RatePerDay    float64        // rate of change per day
    Current       float64
    Average24h    float64
    Average7d     float64
    Min24h        float64
    Max24h        float64
    DataPoints    int            // how much history we have
    Confidence    float64        // 0-1, based on data quality
}

type TrendDirection string
const (
    TrendStable    TrendDirection = "stable"
    TrendGrowing   TrendDirection = "growing"
    TrendDeclining TrendDirection = "declining"
    TrendVolatile  TrendDirection = "volatile"
)

// Baseline represents learned "normal" for a metric
type Baseline struct {
    Metric     string
    Mean       float64
    StdDev     float64
    P5         float64   // 5th percentile
    P95        float64   // 95th percentile
    SampleSize int
    LearnedAt  time.Time
}

// Anomaly represents a detected deviation from normal
type Anomaly struct {
    Metric       string
    Current      float64
    Expected     float64 // baseline mean
    Deviation    float64 // standard deviations from mean
    Severity     string  // "low", "medium", "high", "critical"
    Since        time.Time
    Description  string
}

// Prediction represents a forecasted event
type Prediction struct {
    ResourceID  string
    Metric      string
    Event       string    // "capacity_full", "oom", "pattern_repeat"
    ETA         time.Time
    Confidence  float64
    Basis       string    // explanation of prediction
}
```

### 1.3 Context Builder

```go
// builder.go

type ContextBuilder struct {
    stateProvider   StateProvider
    metricsHistory  *monitoring.MetricsHistory
    findingsStore   *FindingsStore
    knowledgeStore  *knowledge.Store
    baselineStore   *BaselineStore
    
    // Configuration
    includeTrends     bool
    includeBaselines  bool
    includeHistory    bool
    historicalWindow  time.Duration
}

// BuildForResource creates comprehensive context for a single resource
func (b *ContextBuilder) BuildForResource(resourceID string) (*ResourceContext, error)

// BuildForInfrastructure creates summarized context for all infrastructure
func (b *ContextBuilder) BuildForInfrastructure() (*InfrastructureContext, error)

// FormatForAI converts context to AI-consumable markdown
func (b *ContextBuilder) FormatForAI(ctx *ResourceContext) string

// FormatInfrastructureForAI converts full infrastructure context
func (b *ContextBuilder) FormatInfrastructureForAI(ctx *InfrastructureContext) string
```

### 1.4 Trend Computation

```go
// trends.go

// ComputeTrend calculates trend from historical data points
func ComputeTrend(points []monitoring.MetricPoint, window time.Duration) Trend {
    if len(points) < 2 {
        return Trend{Confidence: 0}
    }
    
    // Calculate basic statistics
    avg, min, max, stddev := computeStats(points)
    
    // Linear regression for direction and rate
    slope, r2 := linearRegression(points)
    
    // Classify direction
    direction := classifyTrend(slope, stddev, avg)
    
    // Rate per hour/day
    ratePerHour := slope * 3600 // slope is per second
    ratePerDay := ratePerHour * 24
    
    return Trend{
        Direction:   direction,
        RatePerHour: ratePerHour,
        RatePerDay:  ratePerDay,
        Current:     points[len(points)-1].Value,
        Average24h:  avg,
        Min24h:      min,
        Max24h:      max,
        DataPoints:  len(points),
        Confidence:  r2,
    }
}

func classifyTrend(slope, stddev, avg float64) TrendDirection {
    // Normalize slope relative to value magnitude
    if avg == 0 {
        avg = 1 // avoid division by zero
    }
    normalizedSlope := (slope * 3600) / avg // hourly change as fraction of avg
    
    // Threshold based on volatility
    threshold := 0.01 // 1% per hour is significant
    
    if stddev/avg > 0.2 {
        return TrendVolatile
    }
    if normalizedSlope > threshold {
        return TrendGrowing
    }
    if normalizedSlope < -threshold {
        return TrendDeclining
    }
    return TrendStable
}
```

### 1.5 Integration with Existing Code

```go
// In patrol.go, replace buildInfrastructureSummary:

func (p *PatrolService) buildEnrichedContext(state models.StateSnapshot) string {
    builder := context.NewBuilder(
        p.stateProvider,
        p.metricsHistory,
        p.findings,
        p.knowledgeStore,
        p.baselineStore,
    )
    
    infraCtx, err := builder.BuildForInfrastructure()
    if err != nil {
        log.Warn().Err(err).Msg("Failed to build enriched context, falling back")
        return p.buildBasicSummary(state)
    }
    
    return builder.FormatInfrastructureForAI(infraCtx)
}
```

---

## Phase 2: Baseline Learning

**Goal**: Learn what "normal" looks like for each resource so we can detect anomalies.

### 2.1 Baseline Store

```go
// internal/ai/baseline/store.go

type Store struct {
    mu        sync.RWMutex
    baselines map[string]*ResourceBaseline // resourceID -> baselines
    
    persistence Persistence
    
    // Configuration
    learningWindow  time.Duration // how far back to learn from (default: 7 days)
    minSamples      int           // minimum samples needed (default: 100)
    updateInterval  time.Duration // how often to recompute (default: 1 hour)
}

type ResourceBaseline struct {
    ResourceID  string
    LastUpdated time.Time
    
    Metrics map[string]*MetricBaseline // metric name -> baseline
}

type MetricBaseline struct {
    Mean       float64
    StdDev     float64
    Percentiles map[int]float64 // 5, 25, 50, 75, 95
    SampleCount int
    
    // Time-of-day patterns (optional, phase 2+)
    HourlyMeans [24]float64
}

// Learn computes baselines from historical data
func (s *Store) Learn(resourceID string, history *monitoring.MetricsHistory) error

// GetBaseline returns the baseline for a resource/metric
func (s *Store) GetBaseline(resourceID, metric string) (*MetricBaseline, bool)

// IsAnomaly checks if a value is anomalous given the baseline
func (s *Store) IsAnomaly(resourceID, metric string, value float64) (bool, float64)
```

### 2.2 Background Learning Loop

```go
// Run as part of patrol service or separate goroutine

func (s *Store) StartLearningLoop(ctx context.Context, interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ctx.Done():
            return
        case <-ticker.C:
            s.updateAllBaselines()
        }
    }
}

func (s *Store) updateAllBaselines() {
    // Get list of all resources with metrics
    resources := s.metricsHistory.GetResourceIDs()
    
    for _, resourceID := range resources {
        if err := s.Learn(resourceID, s.metricsHistory); err != nil {
            log.Warn().Err(err).Str("resource", resourceID).Msg("Failed to update baseline")
        }
    }
    
    // Persist updated baselines
    s.save()
}
```

### 2.3 Anomaly Detection

```go
// internal/ai/anomaly/detector.go

type Detector struct {
    baselineStore *baseline.Store
    
    // Thresholds
    warningThreshold  float64 // default: 2.0 std devs
    criticalThreshold float64 // default: 3.0 std devs
}

type Detection struct {
    ResourceID   string
    Metric       string
    CurrentValue float64
    ExpectedMean float64
    StdDev       float64
    ZScore       float64
    Severity     AnomalySeverity
    DetectedAt   time.Time
}

func (d *Detector) Check(resourceID, metric string, value float64) *Detection {
    baseline, ok := d.baselineStore.GetBaseline(resourceID, metric)
    if !ok || baseline.SampleCount < 50 {
        return nil // not enough data yet
    }
    
    zScore := (value - baseline.Mean) / baseline.StdDev
    absZ := math.Abs(zScore)
    
    if absZ < d.warningThreshold {
        return nil // within normal range
    }
    
    severity := AnomalyWarning
    if absZ >= d.criticalThreshold {
        severity = AnomalyCritical
    }
    
    return &Detection{
        ResourceID:   resourceID,
        Metric:       metric,
        CurrentValue: value,
        ExpectedMean: baseline.Mean,
        StdDev:       baseline.StdDev,
        ZScore:       zScore,
        Severity:     severity,
        DetectedAt:   time.Now(),
    }
}
```

---

## Phase 3: Operational Memory

**Goal**: Remember what happened, what users said, and what worked.

### 3.1 Change Detection

```go
// internal/ai/memory/changes.go

type ChangeDetector struct {
    previousState map[string]ResourceSnapshot
    mu           sync.RWMutex
    
    changes     []Change
    maxChanges  int
    persistence Persistence
}

type Change struct {
    ID          string
    ResourceID  string
    ChangeType  ChangeType
    Before      interface{}
    After       interface{}
    DetectedAt  time.Time
    Description string
}

type ChangeType string
const (
    ChangeCreated     ChangeType = "created"
    ChangeDeleted     ChangeType = "deleted"
    ChangeConfig      ChangeType = "config"      // RAM, CPU allocation changed
    ChangeStatus      ChangeType = "status"      // started, stopped
    ChangeMigrated    ChangeType = "migrated"    // moved to different node
)

func (d *ChangeDetector) Detect(current models.StateSnapshot) []Change {
    // Compare current state to previous
    // Detect new resources, deleted resources, config changes
    // Store changes and return new ones
}
```

### 3.2 Remediation Logging

```go
// internal/ai/memory/remediation.go

type RemediationLog struct {
    mu      sync.RWMutex
    records []RemediationRecord
    
    persistence Persistence
}

type RemediationRecord struct {
    ID           string
    Timestamp    time.Time
    ResourceID   string
    FindingID    string    // linked AI finding if any
    Problem      string    // what was wrong
    Action       string    // what was done
    Outcome      Outcome   // did it work?
    Duration     time.Duration // how long until resolved
    Note         string    // optional user/AI note
}

type Outcome string
const (
    OutcomeResolved    Outcome = "resolved"
    OutcomePartial     Outcome = "partial"
    OutcomeFailed      Outcome = "failed"
    OutcomeUnknown     Outcome = "unknown"
)

// Log records a remediation action
func (r *RemediationLog) Log(record RemediationRecord) error

// GetForResource returns remediation history for a resource
func (r *RemediationLog) GetForResource(resourceID string, limit int) []RemediationRecord

// GetSimilar finds similar past remediations
func (r *RemediationLog) GetSimilar(problem string, limit int) []RemediationRecord
```

### 3.3 Integration Points

When the AI executes a command:
```go
func (s *Service) onToolComplete(toolID, command, output string, success bool) {
    // Log the remediation attempt
    s.remediationLog.Log(RemediationRecord{
        ID:        uuid.New().String(),
        Timestamp: time.Now(),
        ResourceID: s.currentContext.TargetID,
        FindingID:  s.currentContext.FindingID,
        Problem:    s.currentContext.Problem,
        Action:     command,
        Outcome:    outcomeFromSuccess(success),
    })
}
```

When a finding is resolved:
```go
func (s *FindingsStore) Resolve(findingID string, auto bool) bool {
    // Link to any remediation actions
    // Record what was done
}
```

---

## Phase 4: Capacity Forecasting

**Goal**: Predict when resources will run out.

### 4.1 Forecaster

```go
// internal/ai/forecast/capacity.go

type CapacityForecaster struct {
    metricsHistory *monitoring.MetricsHistory
    minDataPoints  int // minimum points needed for forecast
}

type CapacityForecast struct {
    ResourceID   string
    Metric       string
    CurrentUsage float64
    Limit        float64
    
    GrowthRate   float64       // per day
    ETA          time.Time     // when it hits limit
    DaysLeft     float64
    Confidence   float64       // 0-1
    
    // Projection points for visualization
    Projection   []ProjectionPoint
}

func (f *CapacityForecaster) Forecast(resourceID, metric string, limit float64) (*CapacityForecast, error) {
    points := f.metricsHistory.GetMetrics(resourceID, metric, 7*24*time.Hour)
    if len(points) < f.minDataPoints {
        return nil, ErrInsufficientData
    }
    
    // Linear regression for growth rate
    slope, r2 := linearRegression(points)
    if slope <= 0 {
        return nil, nil // not growing
    }
    
    current := points[len(points)-1].Value
    remaining := limit - current
    hoursUntilFull := remaining / (slope * 3600)
    
    if hoursUntilFull <= 0 {
        return nil, nil // already at limit
    }
    
    eta := time.Now().Add(time.Duration(hoursUntilFull) * time.Hour)
    
    return &CapacityForecast{
        ResourceID:   resourceID,
        Metric:       metric,
        CurrentUsage: current,
        Limit:        limit,
        GrowthRate:   slope * 86400, // per day
        ETA:          eta,
        DaysLeft:     hoursUntilFull / 24,
        Confidence:   r2,
    }, nil
}
```

### 4.2 Integration with Patrol

```go
func (p *PatrolService) generateForecasts(state models.StateSnapshot) []Prediction {
    var predictions []Prediction
    
    // Forecast storage capacity
    for _, storage := range state.Storage {
        if storage.Total == 0 {
            continue
        }
        forecast, err := p.forecaster.Forecast(storage.ID, "used", float64(storage.Total))
        if err != nil || forecast == nil {
            continue
        }
        
        if forecast.DaysLeft < 30 && forecast.Confidence > 0.5 {
            predictions = append(predictions, Prediction{
                ResourceID: storage.ID,
                Metric:     "storage_capacity",
                Event:      "capacity_full",
                ETA:        forecast.ETA,
                Confidence: forecast.Confidence,
                Basis:      fmt.Sprintf("Growing %.1f GB/day", forecast.GrowthRate/1e9),
            })
        }
    }
    
    // Forecast VM memory (could predict OOM)
    // Forecast backup storage growth
    // etc.
    
    return predictions
}
```

---

## File System Layout (Final)

```
internal/ai/
├── context/
│   ├── builder.go          # Main orchestrator
│   ├── current.go          # Current state extraction
│   ├── historical.go       # Historical data integration
│   ├── trends.go           # Trend computation
│   ├── formatter.go        # AI-friendly formatting
│   └── types.go            # Shared types
├── baseline/
│   ├── store.go            # Baseline storage and learning
│   ├── persistence.go      # Disk persistence
│   └── learning.go         # Statistical learning
├── anomaly/
│   ├── detector.go         # Anomaly detection
│   └── types.go
├── forecast/
│   ├── capacity.go         # Capacity forecasting
│   └── patterns.go         # Pattern-based prediction
├── memory/
│   ├── changes.go          # Change detection
│   ├── remediation.go      # Remediation logging
│   └── persistence.go
├── knowledge/              # (existing)
│   ├── store.go
│   └── store_test.go
├── providers/              # (existing)
├── findings.go             # (existing)
├── patrol.go               # (existing, will use new context/)
├── service.go              # (existing, will use new context/)
└── routing.go              # (existing)
```

---

## Migration Strategy

### Step 1: Add without changing

Create new packages (`context/`, `baseline/`, etc.) that work alongside existing code. Don't break anything.

### Step 2: Wire up to MetricsHistory

Pass `*monitoring.MetricsHistory` to the AI service at startup. Required for historical context.

### Step 3: Switch patrol to enriched context

Replace `buildInfrastructureSummary` with `buildEnrichedContext` behind a feature flag.

### Step 4: Add baseline learning

Start computing baselines in background. Initially just store, don't act.

### Step 5: Enable anomaly annotations

Add anomaly context to AI prompts. Let AI mention anomalies in findings.

### Step 6: Add forecasts

Enable capacity forecasting. Create new finding types for predicted issues.

### Step 7: Phase out old code

Remove deprecated methods once new system is stable.

---

## Testing Strategy

1. **Unit tests** for trend computation, baseline learning, anomaly detection
2. **Integration tests** with mock metrics history
3. **Golden file tests** for AI context formatting (ensure consistent output)
4. **Baseline learning tests** with synthetic time-series data
5. **Forecast accuracy tests** with historical data validation

---

## Success Criteria

Phase 1 complete when:
- AI prompts include historical trends for all resources
- "24h trend" visible in patrol output

Phase 2 complete when:
- Baselines computed automatically
- Anomalies flagged in AI context
- "X is unusual" appearing in findings

Phase 3 complete when:
- Changes detected and logged
- Remediation history queryable
- "Last time this happened..." in AI responses

Phase 4 complete when:
- Capacity forecasts generated
- "Full in X days" predictions accurate
- Predictive findings created before issues occur
