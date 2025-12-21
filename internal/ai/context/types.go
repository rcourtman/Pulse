// Package context provides AI context building with historical data integration.
// This package transforms raw metrics and state into meaningful, time-aware context
// that differentiates Pulse AI from stateless AI assistants.
package context

import (
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/types"
)

// MetricPoint is an alias for the shared type
type MetricPoint = types.MetricPoint

// TrendDirection indicates whether a metric is growing, stable, or declining
type TrendDirection string

const (
	TrendStable    TrendDirection = "stable"    // No significant change
	TrendGrowing   TrendDirection = "growing"   // Increasing over time
	TrendDeclining TrendDirection = "declining" // Decreasing over time
	TrendVolatile  TrendDirection = "volatile"  // Fluctuating significantly
)

// Trend represents the direction and rate of change for a metric
type Trend struct {
	Metric      string         // Name of the metric (cpu, memory, disk)
	Direction   TrendDirection // Overall direction
	RatePerHour float64        // Change per hour (in metric units, e.g., percentage points)
	RatePerDay  float64        // Change per day
	Current     float64        // Most recent value
	Average     float64        // Average over the period
	Min         float64        // Minimum value
	Max         float64        // Maximum value
	StdDev      float64        // Standard deviation
	DataPoints  int            // Number of data points used
	Period      time.Duration  // Time period analyzed
	Confidence  float64        // 0-1 confidence based on data quality (R² for linear fit)
}

// Baseline represents learned "normal" behavior for a metric
type Baseline struct {
	Metric      string    // Name of the metric
	Mean        float64   // Average value
	StdDev      float64   // Standard deviation
	P5          float64   // 5th percentile (low boundary)
	P50         float64   // Median
	P95         float64   // 95th percentile (high boundary)
	Min         float64   // Observed minimum
	Max         float64   // Observed maximum
	SampleCount int       // Number of samples used
	LearnedAt   time.Time // When baseline was computed
}

// Anomaly represents a detected deviation from normal behavior
type Anomaly struct {
	Metric      string    // Which metric is anomalous
	Current     float64   // Current value
	Expected    float64   // Expected value (baseline mean)
	Deviation   float64   // Number of standard deviations from mean
	Severity    string    // "low", "medium", "high", "critical"
	Since       time.Time // When the anomaly started (if known)
	Description string    // Human-readable description
}

// Prediction represents a forecasted future event
type Prediction struct {
	ResourceID  string        // Which resource this prediction is for
	Metric      string        // Which metric
	Event       string        // Type of predicted event (capacity_full, oom, etc.)
	ETA         time.Time     // When the event is predicted to occur
	DaysUntil   float64       // Days until event
	Confidence  float64       // 0-1 confidence level
	Basis       string        // Explanation of how prediction was made
	GrowthRate  float64       // Rate of change used for projection
	CurrentPct  float64       // Current usage percentage
}

// Change represents a detected configuration or state change
type Change struct {
	ResourceID   string      // Which resource changed
	ResourceName string      // Display name
	ChangeType   ChangeType  // Type of change
	Before       interface{} // Previous value (nil for creation)
	After        interface{} // New value (nil for deletion)
	DetectedAt   time.Time   // When change was detected
	Description  string      // Human-readable description
}

// ChangeType categorizes types of changes
type ChangeType string

const (
	ChangeCreated    ChangeType = "created"    // New resource appeared
	ChangeDeleted    ChangeType = "deleted"    // Resource disappeared
	ChangeConfig     ChangeType = "config"     // Configuration change (RAM, CPU)
	ChangeStatus     ChangeType = "status"     // Status change (started, stopped)
	ChangeMigrated   ChangeType = "migrated"   // Moved to different node
	ChangePerformance ChangeType = "performance" // Significant performance shift
)

// ResourceTrends contains all trend data for a single resource
type ResourceTrends struct {
	ResourceID   string           // Unique identifier
	ResourceType string           // node, vm, container, storage, docker_host
	ResourceName string           // Display name
	Trends       map[string]Trend // Metric name -> trend data
	DataAvailable bool            // Whether we have historical data for this resource
	OldestData   time.Time        // Timestamp of oldest data point
	NewestData   time.Time        // Timestamp of newest data point
}

// ResourceContext contains all context for a single resource
type ResourceContext struct {
	ResourceID   string
	ResourceType string // "node", "vm", "container", "oci_container", "storage", "docker_host"
	ResourceName string
	Node         string // Parent node (for guests)

	// Current state (point-in-time)
	CurrentCPU     float64
	CurrentMemory  float64
	CurrentDisk    float64
	Status         string
	Uptime         time.Duration

	// Historical analysis
	Trends    map[string]Trend    // metric -> trend (24h and 7d)
	Baselines map[string]Baseline // metric -> baseline
	Anomalies []Anomaly           // Current anomalies

	// Raw metric samples - downsampled for LLM interpretation
	// Key is metric name (cpu, memory, disk), value is sampled points
	// This lets the LLM see actual patterns without pre-computed heuristics
	MetricSamples map[string][]MetricPoint

	// Predictions
	Predictions []Prediction

	// Operational memory
	UserNotes       []string  // User-provided annotations
	PastIssues      []string  // Summary of past findings
	LastRemediation string    // What was done last time
	RecentChanges   []Change  // Recent configuration changes

	// Additional metadata (e.g., OCI image for OCI containers)
	Metadata map[string]interface{}
}

// InfrastructureContext contains summarized context for the entire infrastructure
type InfrastructureContext struct {
	// Timestamp of this context snapshot
	GeneratedAt time.Time
	
	// Summary statistics
	TotalResources     int
	ResourcesWithData  int // Resources with historical data available
	
	// Categorized resources with their context
	Nodes       []ResourceContext
	VMs         []ResourceContext
	Containers  []ResourceContext
	Storage     []ResourceContext
	DockerHosts []ResourceContext
	Hosts       []ResourceContext
	
	// Global insights
	Anomalies   []Anomaly    // Cross-infrastructure anomalies
	Predictions []Prediction // Capacity and failure predictions
	Changes     []Change     // Recent changes across infrastructure
}

// Stats contains summary statistics for a metric
type Stats struct {
	Count   int
	Min     float64
	Max     float64
	Sum     float64
	Mean    float64
	StdDev  float64
}

// LinearRegressionResult contains the results of linear regression
type LinearRegressionResult struct {
	Slope     float64 // Rate of change per second
	Intercept float64 // Y-intercept
	R2        float64 // Coefficient of determination (0-1)
}
