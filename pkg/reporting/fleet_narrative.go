package reporting

import (
	"context"
	"fmt"
	"sort"
)

// FleetNarrativeInput is the cross-resource view passed to a FleetNarrator.
// It is denormalised from MultiReportData so AI implementations do not
// need to traverse internal structures.
type FleetNarrativeInput struct {
	Title       string
	Period      TimeRange
	PriorPeriod *TimeRange
	Resources   []FleetResourceSummary
	Aggregate   FleetAggregate
}

// FleetResourceSummary is a compact per-resource snapshot — the same
// numbers the deterministic fleet table renders, plus alert/finding
// counts. Heavy time-series data is deliberately not included; the
// renderer keeps the chart data, the narrator only needs the rollups.
type FleetResourceSummary struct {
	ResourceID       string
	ResourceName     string
	ResourceType     string
	Status           string
	AvgCPU           float64
	MaxCPU           float64
	AvgMemory        float64
	MaxMemory        float64
	AvgDisk          float64
	MaxDisk          float64
	ActiveAlerts     int
	CriticalAlerts   int
	ResolvedAlerts   int
	UnhealthyDisks   int
	StoragePoolsHigh int
	Findings         int
}

// FleetAggregate is the fleet-wide rollup. Means are means-of-means:
// each resource contributes one observation per metric, regardless of
// its weight or sample count. Sysadmins reading a fleet report want
// "is anything in this fleet hot?" not a sample-weighted mean.
type FleetAggregate struct {
	ResourceCount       int
	TotalActiveAlerts   int
	TotalCriticalAlerts int
	TotalResolvedAlerts int
	TotalFindings       int
	AvgCPUMean          float64
	AvgMemoryMean       float64
	AvgDiskMean         float64
	MaxCPUSeen          float64
	MaxMemorySeen       float64
	MaxDiskSeen         float64
}

// FleetNarrative is the cross-resource interpretation rendered into
// the fleet summary cover. Source values mirror Narrative.
type FleetNarrative struct {
	Source           string
	HealthStatus     string
	HealthMessage    string
	ExecutiveSummary string
	Outliers         []FleetOutlier
	Patterns         []NarrativeBullet
	Recommendations  []string
	PeriodComparison string
	Disclaimer       string
}

// FleetOutlier names a single resource the operator should pay
// attention to and why. The Reason is rendered verbatim in the PDF.
type FleetOutlier struct {
	ResourceID   string
	ResourceName string
	Reason       string
	Severity     string
}

// FleetNarrator produces a FleetNarrative from a FleetNarrativeInput.
type FleetNarrator interface {
	NarrateFleet(ctx context.Context, in FleetNarrativeInput) (FleetNarrative, error)
}

// narrateFleet is the helper used by the engine: tries the supplied
// fleet narrator (typically AI), falls back to the heuristic on
// nil/error so a fleet report always has narrative.
func narrateFleet(ctx context.Context, n FleetNarrator, in FleetNarrativeInput) FleetNarrative {
	if n != nil {
		out, err := n.NarrateFleet(ctx, in)
		if err == nil {
			if out.Source == "" {
				out.Source = NarrativeSourceAI
			}
			return out
		}
	}
	heuristic := HeuristicFleetNarrator{}
	out, _ := heuristic.NarrateFleet(ctx, in)
	out.Source = NarrativeSourceHeuristic
	return out
}

// HeuristicFleetNarrator is the deterministic fleet fallback. It does
// not invent synthesis — it picks the worst outliers by hard threshold,
// counts cross-cutting patterns, and emits aggregated recommendations.
type HeuristicFleetNarrator struct{}

// NarrateFleet implements FleetNarrator. It never returns an error.
func (HeuristicFleetNarrator) NarrateFleet(_ context.Context, in FleetNarrativeInput) (FleetNarrative, error) {
	return FleetNarrative{
		Source:          NarrativeSourceHeuristic,
		HealthStatus:    fleetHeuristicHealthStatus(in),
		HealthMessage:   fleetHeuristicHealthMessage(in),
		Outliers:        fleetHeuristicOutliers(in),
		Patterns:        fleetHeuristicPatterns(in),
		Recommendations: fleetHeuristicRecommendations(in),
	}, nil
}

func fleetHeuristicHealthStatus(in FleetNarrativeInput) string {
	switch {
	case in.Aggregate.TotalCriticalAlerts > 0:
		return "CRITICAL"
	case in.Aggregate.TotalActiveAlerts > 0:
		return "WARNING"
	default:
		return "HEALTHY"
	}
}

func fleetHeuristicHealthMessage(in FleetNarrativeInput) string {
	a := in.Aggregate
	switch {
	case a.TotalCriticalAlerts == 1:
		return "1 critical alert across the fleet requires immediate attention"
	case a.TotalCriticalAlerts > 1:
		return fmt.Sprintf("%d critical alerts across the fleet require immediate attention", a.TotalCriticalAlerts)
	case a.TotalActiveAlerts == 1:
		return "1 active alert across the fleet — review recommended"
	case a.TotalActiveAlerts > 1:
		return fmt.Sprintf("%d active alerts across the fleet — review recommended", a.TotalActiveAlerts)
	case a.ResourceCount == 0:
		return "No resources in this fleet report"
	default:
		return fmt.Sprintf("All %d resources operating normally", a.ResourceCount)
	}
}

// fleetHeuristicOutliers picks up to fleetMaxOutliers resources that
// most warrant operator attention, ordered by severity then magnitude.
const fleetMaxOutliers = 5

func fleetHeuristicOutliers(in FleetNarrativeInput) []FleetOutlier {
	type scored struct {
		outlier FleetOutlier
		rank    int // higher = render first
	}
	var pool []scored

	for _, r := range in.Resources {
		if r.CriticalAlerts > 0 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("%d critical alert(s) active", r.CriticalAlerts),
					Severity:     NarrativeSeverityCritical,
				},
				rank: 1000 + r.CriticalAlerts,
			})
		}
		if r.UnhealthyDisks > 0 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("%d disk(s) failing or near end of life", r.UnhealthyDisks),
					Severity:     NarrativeSeverityCritical,
				},
				rank: 900 + r.UnhealthyDisks,
			})
		}
		if r.StoragePoolsHigh > 0 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("%d storage pool(s) above 90%% capacity", r.StoragePoolsHigh),
					Severity:     NarrativeSeverityWarning,
				},
				rank: 700 + r.StoragePoolsHigh,
			})
		}
		if r.AvgMemory > 85 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("Memory averaging %.1f%% — sustained pressure", r.AvgMemory),
					Severity:     NarrativeSeverityWarning,
				},
				rank: 500 + int(r.AvgMemory),
			})
		}
		if r.MaxCPU > 90 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("CPU peaked at %.1f%%", r.MaxCPU),
					Severity:     NarrativeSeverityWarning,
				},
				rank: 400 + int(r.MaxCPU),
			})
		}
		if r.AvgDisk > 85 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("Disk usage averaging %.1f%%", r.AvgDisk),
					Severity:     NarrativeSeverityWarning,
				},
				rank: 300 + int(r.AvgDisk),
			})
		}
		if r.ActiveAlerts > 0 && r.CriticalAlerts == 0 {
			pool = append(pool, scored{
				outlier: FleetOutlier{
					ResourceID:   r.ResourceID,
					ResourceName: displayResourceName(r),
					Reason:       fmt.Sprintf("%d non-critical alert(s) active", r.ActiveAlerts),
					Severity:     NarrativeSeverityInfo,
				},
				rank: 100 + r.ActiveAlerts,
			})
		}
	}

	sort.SliceStable(pool, func(i, j int) bool { return pool[i].rank > pool[j].rank })
	if len(pool) > fleetMaxOutliers {
		pool = pool[:fleetMaxOutliers]
	}
	out := make([]FleetOutlier, 0, len(pool))
	seen := make(map[string]struct{}, len(pool))
	for _, s := range pool {
		key := s.outlier.ResourceID + "|" + s.outlier.Reason
		if _, dup := seen[key]; dup {
			continue
		}
		seen[key] = struct{}{}
		out = append(out, s.outlier)
	}
	return out
}

// fleetHeuristicPatterns expresses cross-cutting trends as count-based
// observations — "X of Y resources show memory pressure," not synthesis.
// These are the kind of summary lines the AI narrator can do better,
// but the heuristic must produce something useful when AI is off.
func fleetHeuristicPatterns(in FleetNarrativeInput) []NarrativeBullet {
	if in.Aggregate.ResourceCount == 0 {
		return []NarrativeBullet{{
			Text:     "No resources reported metrics in the selected window",
			Severity: NarrativeSeverityInfo,
		}}
	}
	var out []NarrativeBullet

	memHigh := 0
	cpuHigh := 0
	diskHigh := 0
	for _, r := range in.Resources {
		if r.AvgMemory > 85 {
			memHigh++
		}
		if r.MaxCPU > 90 {
			cpuHigh++
		}
		if r.AvgDisk > 85 {
			diskHigh++
		}
	}
	if memHigh > 0 {
		out = append(out, NarrativeBullet{
			Text:     fmt.Sprintf("%d of %d resources show sustained memory pressure (>85%% avg)", memHigh, in.Aggregate.ResourceCount),
			Severity: severityFromCount(memHigh, in.Aggregate.ResourceCount),
		})
	}
	if cpuHigh > 0 {
		out = append(out, NarrativeBullet{
			Text:     fmt.Sprintf("%d of %d resources hit CPU peaks above 90%%", cpuHigh, in.Aggregate.ResourceCount),
			Severity: severityFromCount(cpuHigh, in.Aggregate.ResourceCount),
		})
	}
	if diskHigh > 0 {
		out = append(out, NarrativeBullet{
			Text:     fmt.Sprintf("%d of %d resources are above 85%% disk usage", diskHigh, in.Aggregate.ResourceCount),
			Severity: severityFromCount(diskHigh, in.Aggregate.ResourceCount),
		})
	}
	if in.Aggregate.TotalResolvedAlerts > 0 {
		out = append(out, NarrativeBullet{
			Text:     fmt.Sprintf("%d alerts triggered and resolved across the fleet during this period", in.Aggregate.TotalResolvedAlerts),
			Severity: NarrativeSeverityInfo,
		})
	}
	if len(out) == 0 {
		out = append(out, NarrativeBullet{
			Text:     fmt.Sprintf("Fleet of %d resources operating within nominal thresholds", in.Aggregate.ResourceCount),
			Severity: NarrativeSeverityOK,
		})
	}
	return out
}

func severityFromCount(hot, total int) string {
	if total <= 0 {
		return NarrativeSeverityInfo
	}
	ratio := float64(hot) / float64(total)
	switch {
	case ratio >= 0.5:
		return NarrativeSeverityCritical
	case ratio >= 0.25:
		return NarrativeSeverityWarning
	default:
		return NarrativeSeverityInfo
	}
}

func fleetHeuristicRecommendations(in FleetNarrativeInput) []string {
	var recs []string
	a := in.Aggregate
	if a.TotalCriticalAlerts > 0 {
		recs = append(recs, "Investigate and resolve critical alerts across the fleet immediately")
	}
	memHigh := 0
	cpuHigh := 0
	diskHigh := 0
	disksHigh := 0
	for _, r := range in.Resources {
		if r.AvgMemory > 85 {
			memHigh++
		}
		if r.MaxCPU > 90 {
			cpuHigh++
		}
		if r.AvgDisk > 85 {
			diskHigh++
		}
		if r.UnhealthyDisks > 0 {
			disksHigh++
		}
	}
	if memHigh > 0 {
		recs = append(recs, "Review memory allocation on resources with sustained pressure (>85% avg)")
	}
	if cpuHigh > 0 {
		recs = append(recs, "Profile CPU-intensive workloads on resources peaking above 90%")
	}
	if diskHigh > 0 {
		recs = append(recs, "Plan capacity expansion for resources above 85% disk usage")
	}
	if disksHigh > 0 {
		recs = append(recs, "Replace failing or end-of-life disks before they cause outage")
	}
	if len(recs) == 0 {
		recs = append(recs, "No fleet-wide action required — continue routine monitoring")
	}
	return recs
}

func displayResourceName(r FleetResourceSummary) string {
	if r.ResourceName != "" {
		return r.ResourceName
	}
	return r.ResourceID
}

// buildFleetNarrativeInput collapses MultiReportData into the compact
// FleetNarrativeInput the narrator consumes. Per-resource MetricStats
// are reduced to the means/maxes the renderer surfaces; raw time-series
// stays in MultiReportData for the deterministic charts.
func buildFleetNarrativeInput(data *MultiReportData) FleetNarrativeInput {
	if data == nil {
		return FleetNarrativeInput{}
	}
	in := FleetNarrativeInput{
		Title:     data.Title,
		Period:    TimeRange{Start: data.Start, End: data.End},
		Resources: make([]FleetResourceSummary, 0, len(data.Resources)),
	}

	var sumCPU, sumMem, sumDisk float64
	var countCPU, countMem, countDisk int

	for _, rd := range data.Resources {
		if rd == nil {
			continue
		}
		summary := FleetResourceSummary{
			ResourceID:   rd.ResourceID,
			ResourceType: rd.ResourceType,
			Findings:     len(rd.Findings),
		}
		if rd.Resource != nil {
			summary.ResourceName = rd.Resource.Name
			summary.Status = rd.Resource.Status
		}

		if stats, ok := rd.Summary.ByMetric["cpu"]; ok {
			summary.AvgCPU = stats.Avg
			summary.MaxCPU = stats.Max
			sumCPU += stats.Avg
			countCPU++
			if stats.Max > in.Aggregate.MaxCPUSeen {
				in.Aggregate.MaxCPUSeen = stats.Max
			}
		}
		if stats, ok := rd.Summary.ByMetric["memory"]; ok {
			summary.AvgMemory = stats.Avg
			summary.MaxMemory = stats.Max
			sumMem += stats.Avg
			countMem++
			if stats.Max > in.Aggregate.MaxMemorySeen {
				in.Aggregate.MaxMemorySeen = stats.Max
			}
		}
		diskKey := "disk"
		if _, ok := rd.Summary.ByMetric["disk"]; !ok {
			diskKey = "usage"
		}
		if stats, ok := rd.Summary.ByMetric[diskKey]; ok {
			summary.AvgDisk = stats.Avg
			summary.MaxDisk = stats.Max
			sumDisk += stats.Avg
			countDisk++
			if stats.Max > in.Aggregate.MaxDiskSeen {
				in.Aggregate.MaxDiskSeen = stats.Max
			}
		}

		for _, alert := range rd.Alerts {
			if alert.ResolvedTime != nil {
				summary.ResolvedAlerts++
				in.Aggregate.TotalResolvedAlerts++
				continue
			}
			summary.ActiveAlerts++
			in.Aggregate.TotalActiveAlerts++
			if alert.Level == "critical" {
				summary.CriticalAlerts++
				in.Aggregate.TotalCriticalAlerts++
			}
		}
		for _, disk := range rd.Disks {
			if disk.Health == "FAILED" || (disk.WearLevel > 0 && disk.WearLevel <= 30) {
				summary.UnhealthyDisks++
			}
		}
		for _, st := range rd.Storage {
			if st.UsagePerc >= 90 {
				summary.StoragePoolsHigh++
			}
		}

		in.Aggregate.TotalFindings += summary.Findings
		in.Resources = append(in.Resources, summary)
	}

	in.Aggregate.ResourceCount = len(in.Resources)
	if countCPU > 0 {
		in.Aggregate.AvgCPUMean = sumCPU / float64(countCPU)
	}
	if countMem > 0 {
		in.Aggregate.AvgMemoryMean = sumMem / float64(countMem)
	}
	if countDisk > 0 {
		in.Aggregate.AvgDiskMean = sumDisk / float64(countDisk)
	}
	return in
}
