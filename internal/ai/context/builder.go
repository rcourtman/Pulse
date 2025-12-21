package context

import (
	"strings"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

// MetricsHistoryProvider is the interface for accessing historical metrics
// This avoids importing the monitoring package directly
type MetricsHistoryProvider interface {
	GetNodeMetrics(nodeID string, metricType string, duration time.Duration) []MetricPoint
	GetGuestMetrics(guestID string, metricType string, duration time.Duration) []MetricPoint
	GetAllGuestMetrics(guestID string, duration time.Duration) map[string][]MetricPoint
	GetAllStorageMetrics(storageID string, duration time.Duration) map[string][]MetricPoint
}

// KnowledgeProvider provides user annotations and notes
type KnowledgeProvider interface {
	GetNotes(guestID string) []string
	FormatAllForContext() string
}

// FindingsProvider provides past findings for operational memory
type FindingsProvider interface {
	GetDismissedForContext() string
	GetPastFindingsForResource(resourceID string) []string
}

// BaselineProvider provides learned baselines for anomaly detection
type BaselineProvider interface {
	// CheckAnomaly returns severity, z-score, and baseline data
	// Severity is "", "low", "medium", "high", or "critical"
	CheckAnomaly(resourceID, metric string, value float64) (severity string, zScore float64, mean float64, stddev float64, ok bool)
	// GetBaseline returns the baseline for a resource/metric
	GetBaseline(resourceID, metric string) (mean float64, stddev float64, sampleCount int, ok bool)
}

// Builder constructs enriched AI context from multiple data sources
type Builder struct {
	// Data sources
	metricsHistory MetricsHistoryProvider
	knowledge      KnowledgeProvider
	findings       FindingsProvider
	baseline       BaselineProvider

	// Configuration
	trendWindow24h  time.Duration
	trendWindow7d   time.Duration
	includeHistory  bool
	includeTrends   bool
	includeBaseline bool
}

// NewBuilder creates a new context builder
func NewBuilder() *Builder {
	return &Builder{
		trendWindow24h:  24 * time.Hour,
		trendWindow7d:   7 * 24 * time.Hour,
		includeHistory:  true,
		includeTrends:   true,
		includeBaseline: true, // Enable when baseline provider is set
	}
}

// WithMetricsHistory sets the metrics history provider
func (b *Builder) WithMetricsHistory(mh MetricsHistoryProvider) *Builder {
	b.metricsHistory = mh
	return b
}

// WithKnowledge sets the knowledge provider for user notes
func (b *Builder) WithKnowledge(k KnowledgeProvider) *Builder {
	b.knowledge = k
	return b
}

// WithFindings sets the findings provider for operational memory
func (b *Builder) WithFindings(f FindingsProvider) *Builder {
	b.findings = f
	return b
}

// WithBaseline sets the baseline provider for anomaly detection
func (b *Builder) WithBaseline(bp BaselineProvider) *Builder {
	b.baseline = bp
	return b
}

// BuildForInfrastructure creates comprehensive context for the entire infrastructure
func (b *Builder) BuildForInfrastructure(state models.StateSnapshot) *InfrastructureContext {
	ctx := &InfrastructureContext{
		GeneratedAt: time.Now(),
	}

	// Process nodes
	for _, node := range state.Nodes {
		trends := b.computeNodeTrends(node.ID)
		resourceCtx := FormatNodeForContext(node, trends)
		b.enrichWithNotes(&resourceCtx)
		b.enrichWithAnomalies(&resourceCtx)
		ctx.Nodes = append(ctx.Nodes, resourceCtx)
	}

	// Process VMs
	for _, vm := range state.VMs {
		if vm.Template {
			continue
		}
		trends := b.computeGuestTrends(vm.ID)
		resourceCtx := FormatGuestForContext(
			vm.ID, vm.Name, vm.Node, "vm", vm.Status,
			vm.CPU, vm.Memory.Usage, vm.Disk.Usage,
			vm.Uptime, vm.LastBackup, trends,
		)
		// Add raw metric samples for LLM interpretation
		resourceCtx.MetricSamples = b.computeGuestMetricSamples(vm.ID)
		b.enrichWithNotes(&resourceCtx)
		b.enrichWithAnomalies(&resourceCtx)
		ctx.VMs = append(ctx.VMs, resourceCtx)
	}

	// Process containers (LXC and OCI)
	for _, ct := range state.Containers {
		if ct.Template {
			continue
		}
		trends := b.computeGuestTrends(ct.ID)
		
		// Determine container type - OCI containers are treated specially
		containerType := "container"
		if ct.IsOCI {
			containerType = "oci_container"
		}
		
		resourceCtx := FormatGuestForContext(
			ct.ID, ct.Name, ct.Node, containerType, ct.Status,
			ct.CPU, ct.Memory.Usage, ct.Disk.Usage,
			ct.Uptime, ct.LastBackup, trends,
		)
		
		// Add raw metric samples for LLM interpretation
		// This lets the LLM see actual patterns without pre-computed heuristics
		resourceCtx.MetricSamples = b.computeGuestMetricSamples(ct.ID)
		
		// Add OCI image info for AI context
		if ct.IsOCI && ct.OSTemplate != "" {
			if resourceCtx.Metadata == nil {
				resourceCtx.Metadata = make(map[string]interface{})
			}
			resourceCtx.Metadata["oci_image"] = ct.OSTemplate
		}
		
		b.enrichWithNotes(&resourceCtx)
		b.enrichWithAnomalies(&resourceCtx)
		ctx.Containers = append(ctx.Containers, resourceCtx)
	}

	// Process storage
	for _, storage := range state.Storage {
		trends := b.computeStorageTrends(storage.ID)
		resourceCtx := FormatStorageForContext(storage, trends)
		
		// Add capacity predictions for storage
		if predictions := b.computeStoragePredictions(storage, trends); len(predictions) > 0 {
			resourceCtx.Predictions = predictions
			ctx.Predictions = append(ctx.Predictions, predictions...)
		}
		
		ctx.Storage = append(ctx.Storage, resourceCtx)
	}

	// Process Docker hosts
	for _, dh := range state.DockerHosts {
		resourceCtx := b.buildDockerHostContext(dh)
		ctx.DockerHosts = append(ctx.DockerHosts, resourceCtx)
	}

	// Process agent hosts
	for _, host := range state.Hosts {
		resourceCtx := b.buildHostContext(host)
		ctx.Hosts = append(ctx.Hosts, resourceCtx)
	}

	// Calculate totals
	ctx.TotalResources = len(ctx.Nodes) + len(ctx.VMs) + len(ctx.Containers) +
		len(ctx.Storage) + len(ctx.DockerHosts) + len(ctx.Hosts)

	log.Debug().
		Int("nodes", len(ctx.Nodes)).
		Int("vms", len(ctx.VMs)).
		Int("containers", len(ctx.Containers)).
		Int("storage", len(ctx.Storage)).
		Int("predictions", len(ctx.Predictions)).
		Msg("Built enriched infrastructure context")

	return ctx
}

// computeNodeTrends computes trends for a node's metrics
func (b *Builder) computeNodeTrends(nodeID string) map[string]Trend {
	trends := make(map[string]Trend)
	
	if b.metricsHistory == nil || !b.includeTrends {
		return trends
	}

	// Compute 24h trends for key metrics
	for _, metric := range []string{"cpu", "memory"} {
		points := b.metricsHistory.GetNodeMetrics(nodeID, metric, b.trendWindow24h)
		if len(points) >= 3 {
			trend := ComputeTrend(points, metric, b.trendWindow24h)
			trends[metric+"_24h"] = trend
		}
	}

	// Also compute 7d trends for capacity planning
	for _, metric := range []string{"cpu", "memory"} {
		points := b.metricsHistory.GetNodeMetrics(nodeID, metric, b.trendWindow7d)
		if len(points) >= 10 {
			trend := ComputeTrend(points, metric, b.trendWindow7d)
			trends[metric+"_7d"] = trend
		}
	}

	return trends
}

// computeGuestTrends computes trends for a guest's metrics
func (b *Builder) computeGuestTrends(guestID string) map[string]Trend {
	trends := make(map[string]Trend)
	
	if b.metricsHistory == nil || !b.includeTrends {
		return trends
	}

	// Get all metrics at once for efficiency
	allMetrics := b.metricsHistory.GetAllGuestMetrics(guestID, b.trendWindow7d)
	
	for metric, points := range allMetrics {
		if len(points) < 3 {
			continue
		}
		
		// Compute 24h trend
		recent := filterRecentPoints(points, b.trendWindow24h)
		if len(recent) >= 3 {
			trend := ComputeTrend(recent, metric, b.trendWindow24h)
			trends[metric+"_24h"] = trend
		}
		
		// Compute 7d trend if enough data
		if len(points) >= 10 {
			trend := ComputeTrend(points, metric, b.trendWindow7d)
			trends[metric+"_7d"] = trend
		}
	}

	return trends
}

// computeStorageTrends computes trends for storage
func (b *Builder) computeStorageTrends(storageID string) map[string]Trend {
	trends := make(map[string]Trend)
	
	if b.metricsHistory == nil || !b.includeTrends {
		return trends
	}

	allMetrics := b.metricsHistory.GetAllStorageMetrics(storageID, b.trendWindow7d)
	
	// Focus on usage metric for storage
	if points, ok := allMetrics["usage"]; ok && len(points) >= 3 {
		recent := filterRecentPoints(points, b.trendWindow24h)
		if len(recent) >= 3 {
			trends["usage_24h"] = ComputeTrend(recent, "usage", b.trendWindow24h)
		}
		if len(points) >= 10 {
			trends["usage_7d"] = ComputeTrend(points, "usage", b.trendWindow7d)
		}
	}

	return trends
}

// computeStoragePredictions generates capacity predictions for storage
func (b *Builder) computeStoragePredictions(storage models.Storage, trends map[string]Trend) []Prediction {
	var predictions []Prediction

	// Use 7d trend for more stable prediction
	trend, ok := trends["usage_7d"]
	if !ok || trend.DataPoints < 10 {
		return predictions
	}

	// Only predict if growing
	if trend.Direction != TrendGrowing || trend.RatePerDay <= 0 {
		return predictions
	}

	// Current usage
	currentPct := storage.Usage
	if currentPct == 0 && storage.Total > 0 {
		currentPct = float64(storage.Used) / float64(storage.Total) * 100
	}

	// Calculate days until 90% (warning) and 100% (critical)
	for _, threshold := range []struct {
		pct   float64
		event string
	}{
		{90, "storage_warning_90pct"},
		{100, "storage_full"},
	} {
		if currentPct >= threshold.pct {
			continue // Already past this threshold
		}

		remaining := threshold.pct - currentPct
		daysUntil := remaining / trend.RatePerDay

		if daysUntil > 0 && daysUntil <= 30 { // Only predict within 30 days
			predictions = append(predictions, Prediction{
				ResourceID:  storage.ID,
				Metric:      "usage",
				Event:       threshold.event,
				ETA:         time.Now().Add(time.Duration(daysUntil*24) * time.Hour),
				DaysUntil:   daysUntil,
				Confidence:  trend.Confidence,
				Basis:       formatPredictionBasis(trend),
				GrowthRate:  trend.RatePerDay,
				CurrentPct:  currentPct,
			})
		}
	}

	return predictions
}

// formatPredictionBasis creates explanation for a prediction
func formatPredictionBasis(trend Trend) string {
	return "Growing " + formatRate(trend.RatePerDay) + " based on " + 
		formatDuration(trend.Period) + " of data"
}

// buildDockerHostContext creates context for a Docker host
func (b *Builder) buildDockerHostContext(host models.DockerHost) ResourceContext {
	displayName := host.Hostname
	if host.DisplayName != "" {
		displayName = host.DisplayName
	}

	ctx := ResourceContext{
		ResourceID:   host.ID,
		ResourceType: "docker_host",
		ResourceName: displayName,
		Status:       host.Status,
		Uptime:       time.Duration(host.UptimeSeconds) * time.Second,
	}

	// Note: Docker hosts don't have the same trend data as Proxmox resources
	// We could add container-level trends in the future

	return ctx
}

// buildHostContext creates context for an agent host
func (b *Builder) buildHostContext(host models.Host) ResourceContext {
	displayName := host.Hostname
	if host.DisplayName != "" {
		displayName = host.DisplayName
	}

	// Calculate CPU and memory from host data
	cpuPct := 0.0
	if len(host.LoadAverage) > 0 && host.CPUCount > 0 {
		cpuPct = host.LoadAverage[0] / float64(host.CPUCount) * 100
	}

	memPct := 0.0
	if host.Memory.Total > 0 {
		memPct = float64(host.Memory.Used) / float64(host.Memory.Total) * 100
	}

	ctx := ResourceContext{
		ResourceID:    host.ID,
		ResourceType:  "host",
		ResourceName:  displayName,
		CurrentCPU:    cpuPct,
		CurrentMemory: memPct,
		Status:        host.Status,
		Uptime:        time.Duration(host.UptimeSeconds) * time.Second,
	}

	return ctx
}

// enrichWithNotes adds user annotations to context
func (b *Builder) enrichWithNotes(ctx *ResourceContext) {
	if b.knowledge == nil {
		return
	}

	notes := b.knowledge.GetNotes(ctx.ResourceID)
	if len(notes) > 0 {
		ctx.UserNotes = notes
	}
}

// enrichWithAnomalies checks current values against baselines and adds anomalies
func (b *Builder) enrichWithAnomalies(ctx *ResourceContext) {
	if b.baseline == nil || !b.includeBaseline {
		return
	}

	// Check each metric type for anomalies
	metrics := map[string]float64{
		"cpu":    ctx.CurrentCPU,
		"memory": ctx.CurrentMemory,
		"disk":   ctx.CurrentDisk,
	}

	for metric, value := range metrics {
		if value == 0 {
			continue // Skip zeroes (usually means not reported)
		}

		severity, zScore, mean, stddev, ok := b.baseline.CheckAnomaly(ctx.ResourceID, metric, value)
		if !ok || severity == "" {
			continue // No anomaly or no baseline
		}

		direction := "above"
		if zScore < 0 {
			direction = "below"
		}

		anomaly := Anomaly{
			Metric:    metric,
			Current:   value,
			Expected:  mean,
			Deviation: zScore,
			Severity:  severity,
			Since:     time.Now(), // We don't track onset time yet
			Description: formatAnomalyDescription(metric, value, mean, stddev, severity, direction),
		}
		ctx.Anomalies = append(ctx.Anomalies, anomaly)
	}
}

// formatAnomalyDescription creates human-readable anomaly description
func formatAnomalyDescription(metric string, current, mean, stddev float64, severity, direction string) string {
	var sb strings.Builder
	sb.WriteString(strings.Title(metric))
	sb.WriteString(" is ")
	sb.WriteString(severity)
	sb.WriteString(" ")
	sb.WriteString(direction)
	sb.WriteString(" normal (")
	sb.WriteString(formatFloat(current, 1))
	sb.WriteString("% vs typical ")
	sb.WriteString(formatFloat(mean, 1))
	sb.WriteString("% ± ")
	sb.WriteString(formatFloat(stddev, 1))
	sb.WriteString("%)")
	return sb.String()
}

// computeGuestMetricSamples gets downsampled raw metrics for LLM interpretation
// Returns ~24 samples from the last 7 days, letting the LLM see patterns and determine if behavior is normal
// With modern context windows (128k+ tokens), this is a small cost for much better insights
func (b *Builder) computeGuestMetricSamples(guestID string) map[string][]MetricPoint {
	samples := make(map[string][]MetricPoint)

	if b.metricsHistory == nil {
		return samples
	}

	// Get 24 hours of data (matches in-memory retention)
	// This lets the LLM see recent patterns and changes
	allMetrics := b.metricsHistory.GetAllGuestMetrics(guestID, b.trendWindow24h)

	for metric, points := range allMetrics {
		if len(points) < 3 {
			continue
		}
		// Downsample to ~100 points (~15 min resolution over 24h)
		// Modern LLMs have 100k+ token contexts - we can afford detailed history
		sampled := DownsampleMetrics(points, 100)
		if len(sampled) >= 3 {
			samples[metric] = sampled
		}
	}

	// Debug: log if we're returning samples
	if len(samples) > 0 {
		log.Debug().
			Str("guestID", guestID).
			Int("metricCount", len(samples)).
			Msg("AI Context: Built metric samples for LLM")
	}

	return samples
}

// filterRecentPoints filters points to only include those within duration
func filterRecentPoints(points []MetricPoint, duration time.Duration) []MetricPoint {
	cutoff := time.Now().Add(-duration)
	result := make([]MetricPoint, 0, len(points))
	for _, p := range points {
		if p.Timestamp.After(cutoff) {
			result = append(result, p)
		}
	}
	return result
}

// MergeContexts combines context for targeted analysis with relevant infrastructure context
func (b *Builder) MergeContexts(target *ResourceContext, infrastructure *InfrastructureContext) string {
	// For targeted requests, highlight the target first, then add relevant related context
	var result strings.Builder
	
	result.WriteString("# Target Resource\n")
	result.WriteString(FormatResourceContext(*target))
	result.WriteString("\n")
	
	// Add related resources (same node, dependencies, etc.)
	// This could be expanded with dependency mapping in the future
	if target.Node != "" {
		result.WriteString("\n## Related Resources\n")
		// Find other resources on the same node
		for _, vm := range infrastructure.VMs {
			if vm.Node == target.Node && vm.ResourceID != target.ResourceID {
				result.WriteString(FormatResourceContext(vm))
			}
		}
		for _, ct := range infrastructure.Containers {
			if ct.Node == target.Node && ct.ResourceID != target.ResourceID {
				result.WriteString(FormatResourceContext(ct))
			}
		}
	}
	
	return result.String()
}
