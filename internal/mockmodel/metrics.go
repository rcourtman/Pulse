package mockmodel

import (
	"hash/fnv"
	"math"
	"strconv"
	"strings"
	"time"
)

type SeriesStyle int

const (
	StyleSpiky SeriesStyle = iota
	StylePlateau
	StyleFlat
)

type StorageCapacitySeries struct {
	Usage []float64
	Used  []float64
	Avail []float64
	Total []float64
}

type metricProfile int

const (
	profileCompute metricProfile = iota
	profileMemory
	profileDiskIO
	profileNetwork
	profileCapacity
	profileThermal
	profileFlat
)

type metricRoleModifiers struct {
	steadyStateScale  float64
	activityScale     float64
	diurnalScale      float64
	burstScale        float64
	maintenanceScale  float64
	memoryBaseScale   float64
	memoryDriftScale  float64
	storageDriftScale float64
	thermalScale      float64
}

func normalizeMetricRole(role string) string {
	role = strings.ToLower(strings.TrimSpace(role))
	if role == "" {
		return "general"
	}
	return role
}

func metricRoleProfile(role string) metricRoleModifiers {
	switch normalizeMetricRole(role) {
	case "web":
		return metricRoleModifiers{steadyStateScale: 0.95, activityScale: 0.95, diurnalScale: 1.35, burstScale: 1.15, maintenanceScale: 0.7, memoryBaseScale: 0.92, memoryDriftScale: 0.95, storageDriftScale: 0.88, thermalScale: 1.0}
	case "api":
		return metricRoleModifiers{steadyStateScale: 1.0, activityScale: 1.1, diurnalScale: 1.15, burstScale: 1.2, maintenanceScale: 0.85, memoryBaseScale: 1.0, memoryDriftScale: 1.05, storageDriftScale: 0.92, thermalScale: 1.04}
	case "database":
		return metricRoleModifiers{steadyStateScale: 1.22, activityScale: 0.82, diurnalScale: 0.7, burstScale: 0.8, maintenanceScale: 1.2, memoryBaseScale: 1.35, memoryDriftScale: 0.72, storageDriftScale: 1.12, thermalScale: 1.08}
	case "cache":
		return metricRoleModifiers{steadyStateScale: 0.82, activityScale: 1.02, diurnalScale: 0.95, burstScale: 1.35, maintenanceScale: 0.65, memoryBaseScale: 1.45, memoryDriftScale: 0.75, storageDriftScale: 0.72, thermalScale: 1.03}
	case "queue", "batch":
		return metricRoleModifiers{steadyStateScale: 0.9, activityScale: 0.95, diurnalScale: 0.85, burstScale: 1.45, maintenanceScale: 1.35, memoryBaseScale: 0.9, memoryDriftScale: 1.15, storageDriftScale: 1.0, thermalScale: 1.06}
	case "monitoring":
		return metricRoleModifiers{steadyStateScale: 1.05, activityScale: 0.8, diurnalScale: 0.85, burstScale: 0.75, maintenanceScale: 0.9, memoryBaseScale: 1.2, memoryDriftScale: 0.7, storageDriftScale: 1.05, thermalScale: 0.98}
	case "backup":
		return metricRoleModifiers{steadyStateScale: 0.58, activityScale: 0.45, diurnalScale: 0.5, burstScale: 0.7, maintenanceScale: 1.95, memoryBaseScale: 0.72, memoryDriftScale: 0.65, storageDriftScale: 1.2, thermalScale: 0.95}
	case "storage":
		return metricRoleModifiers{steadyStateScale: 1.08, activityScale: 0.65, diurnalScale: 0.55, burstScale: 0.6, maintenanceScale: 1.35, memoryBaseScale: 0.96, memoryDriftScale: 0.62, storageDriftScale: 1.28, thermalScale: 1.02}
	case "ingress":
		return metricRoleModifiers{steadyStateScale: 0.88, activityScale: 1.0, diurnalScale: 1.25, burstScale: 1.3, maintenanceScale: 0.65, memoryBaseScale: 0.82, memoryDriftScale: 0.88, storageDriftScale: 0.8, thermalScale: 1.0}
	case "ci":
		return metricRoleModifiers{steadyStateScale: 0.86, activityScale: 1.08, diurnalScale: 0.92, burstScale: 1.5, maintenanceScale: 1.25, memoryBaseScale: 1.02, memoryDriftScale: 1.22, storageDriftScale: 0.95, thermalScale: 1.08}
	case "media":
		return metricRoleModifiers{steadyStateScale: 1.0, activityScale: 1.05, diurnalScale: 1.45, burstScale: 1.28, maintenanceScale: 0.8, memoryBaseScale: 1.05, memoryDriftScale: 0.98, storageDriftScale: 1.0, thermalScale: 1.12}
	case "security":
		return metricRoleModifiers{steadyStateScale: 0.84, activityScale: 0.9, diurnalScale: 0.95, burstScale: 1.0, maintenanceScale: 0.85, memoryBaseScale: 0.9, memoryDriftScale: 0.72, storageDriftScale: 0.85, thermalScale: 1.0}
	default:
		return metricRoleModifiers{steadyStateScale: 1.0, activityScale: 1.0, diurnalScale: 1.0, burstScale: 1.0, maintenanceScale: 1.0, memoryBaseScale: 1.0, memoryDriftScale: 1.0, storageDriftScale: 1.0, thermalScale: 1.0}
	}
}

func NormalizeBlendWeight(weight float64, step, reference time.Duration) float64 {
	if weight <= 0 {
		return 0.01
	}
	if weight >= 1 {
		return 1
	}
	if step <= 0 || reference <= 0 || step == reference {
		return weight
	}

	ratio := float64(step) / float64(reference)
	normalized := 1 - math.Pow(1-weight, ratio)
	return clampFloat(normalized, 0.0005, 0.999)
}

func StyleSpeed(style SeriesStyle) float64 {
	switch style {
	case StylePlateau:
		return 0.5
	case StyleFlat:
		return 0.12
	default:
		return 1.0
	}
}

func ValueAt(seed uint64, min, max float64, speed float64, at time.Time) float64 {
	return valueAtProfile(seed, min, max, profileFromSpeed(speed), at)
}

func ValueAtMetric(seed uint64, min, max float64, metric string, speed float64, at time.Time) float64 {
	return ValueAtMetricWithRole(seed, min, max, metric, speed, "", at)
}

func ValueAtMetricWithRole(
	seed uint64,
	min, max float64,
	metric string,
	speed float64,
	role string,
	at time.Time,
) float64 {
	return valueAtProfileWithRole(seed, min, max, profileFromMetric(metric, profileFromSpeed(speed)), role, at)
}

func SeriesForTimestamps(current float64, timestamps []time.Time, seed uint64, min, max float64, style SeriesStyle) []float64 {
	return seriesForProfile(current, timestamps, seed, min, max, profileFromStyle(style))
}

func SeriesForMetricTimestamps(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	metric string,
	style SeriesStyle,
) []float64 {
	return SeriesForMetricTimestampsWithRole(current, timestamps, seed, min, max, metric, style, "")
}

func SeriesForMetricTimestampsWithRole(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	metric string,
	style SeriesStyle,
	role string,
) []float64 {
	return seriesForProfileWithRole(current, timestamps, seed, min, max, profileFromMetric(metric, profileFromStyle(style)), role)
}

func StorageCapacitySeriesForTimestamps(
	currentUsed float64,
	currentTotal float64,
	timestamps []time.Time,
	seed uint64,
) StorageCapacitySeries {
	return StorageCapacitySeriesForTimestampsWithRole(currentUsed, currentTotal, timestamps, seed, "")
}

func StorageCapacitySeriesForTimestampsWithRole(
	currentUsed float64,
	currentTotal float64,
	timestamps []time.Time,
	seed uint64,
	role string,
) StorageCapacitySeries {
	if len(timestamps) == 0 {
		return StorageCapacitySeries{}
	}

	total := math.Max(0, currentTotal)
	if total <= 0 {
		zeros := make([]float64, len(timestamps))
		return StorageCapacitySeries{
			Usage: zeros,
			Used:  append([]float64(nil), zeros...),
			Avail: append([]float64(nil), zeros...),
			Total: append([]float64(nil), zeros...),
		}
	}

	used := clampFloat(currentUsed, 0, total)
	currentUsage := clampFloat((used/total)*100, 0, 100)
	usageSeries := SeriesForMetricTimestampsWithRole(currentUsage, timestamps, seed, 0, 100, "usage", StyleFlat, role)

	usedSeries := make([]float64, len(timestamps))
	availSeries := make([]float64, len(timestamps))
	totalSeries := make([]float64, len(timestamps))
	for i, usage := range usageSeries {
		totalSeries[i] = total
		occupancy := clampFloat(usage/100, 0, 1)
		usedSeries[i] = total * occupancy
		availSeries[i] = total - usedSeries[i]
	}

	last := len(timestamps) - 1
	usageSeries[last] = currentUsage
	usedSeries[last] = used
	availSeries[last] = total - used
	totalSeries[last] = total

	return StorageCapacitySeries{
		Usage: usageSeries,
		Used:  usedSeries,
		Avail: availSeries,
		Total: totalSeries,
	}
}

func seriesForProfile(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	profile metricProfile,
) []float64 {
	return seriesForProfileWithRole(current, timestamps, seed, min, max, profile, "")
}

func seriesForProfileWithRole(
	current float64,
	timestamps []time.Time,
	seed uint64,
	min float64,
	max float64,
	profile metricProfile,
	role string,
) []float64 {
	if len(timestamps) == 0 {
		return nil
	}

	current = clampFloat(current, min, max)
	values := make([]float64, len(timestamps))
	for i, ts := range timestamps {
		values[i] = valueAtProfileWithRole(seed, min, max, profile, role, ts)
	}

	now := timestamps[len(timestamps)-1]
	shift := current - values[len(values)-1]
	halfLife := anchorHalfLife(profile)
	for i, ts := range timestamps {
		age := now.Sub(ts)
		if age < 0 {
			age = 0
		}
		values[i] = clampFloat(values[i]+shift*anchorWeight(age, halfLife), min, max)
	}
	values[len(values)-1] = current
	return values
}

func valueAtProfile(seed uint64, min, max float64, profile metricProfile, at time.Time) float64 {
	return valueAtProfileWithRole(seed, min, max, profile, "", at)
}

func valueAtProfileWithRole(seed uint64, min, max float64, profile metricProfile, role string, at time.Time) float64 {
	span := math.Max(1, max-min)
	modifiers := metricRoleProfile(role)
	switch profile {
	case profileMemory:
		return clampFloat(memoryValue(seed, min, span, modifiers, at), min, max)
	case profileDiskIO:
		return clampFloat(diskIOValue(seed, min, span, modifiers, at), min, max)
	case profileNetwork:
		return clampFloat(networkValue(seed, min, span, modifiers, at), min, max)
	case profileCapacity:
		return clampFloat(capacityValue(seed, min, span, modifiers, at), min, max)
	case profileThermal:
		return clampFloat(thermalValue(seed, min, span, modifiers, at), min, max)
	case profileFlat:
		return clampFloat(flatValue(seed, min, span, modifiers, at), min, max)
	default:
		return clampFloat(computeValue(seed, min, span, modifiers, at), min, max)
	}
}

func computeValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	activity := clamp01(scheduledActivity(seed, at) * modifiers.activityScale)
	utilBias := 0.04 + unitFloat(seed, "cpu-util", 0)*0.12
	platformBias := 0.03 + unitFloat(seed, "cpu-platform", 0)*0.08
	sustained := span * modifiers.steadyStateScale * (utilBias + platformBias + activity*(0.10+unitFloat(seed, "cpu-active", 0)*0.24))
	diurnal := span * modifiers.diurnalScale * scheduledWave(seed, "cpu-diurnal", at, 0.04)
	trend := span * slowTrend(seed, "cpu-trend", at, 6*time.Hour, 24*time.Hour, 0.08)
	chatter := span * (0.008 + activity*0.018) * positiveFloat(seed, "cpu-chatter", at.Unix(), 5*60)
	spikes := span * modifiers.burstScale * (0.08 + activity*0.22 + unitFloat(seed, "cpu-spike-amp", 0)*0.10) *
		eventPulse(seed, "cpu-spike", at, 8*time.Minute, 0.02+activity*0.06)
	batch := span * modifiers.burstScale * (0.05 + activity*0.18) *
		eventPulse(seed, "cpu-batch", at, 36*time.Minute, 0.012+activity*0.03)
	noise := span * 0.004 * interpolatedCenteredFloat(seed, "cpu-noise", at.Unix(), 90)

	return min + sustained + diurnal + trend + chatter + spikes + batch + noise
}

func memoryValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	activity := clamp01(scheduledActivity(seed, at) * modifiers.activityScale)
	base := span * modifiers.memoryBaseScale * (0.20 + unitFloat(seed, "mem-base", 0)*0.42)
	workingSet := span * (0.03 + activity*0.07*modifiers.activityScale)
	plateau := span * 0.11 * modifiers.memoryDriftScale * plateauComponent(seed, "mem-plateau", at, 6*time.Hour, 0.16)
	trend := span * 0.05 * modifiers.memoryDriftScale * plateauComponent(seed, "mem-trend", at, 24*time.Hour, 0.10)
	hotset := span * (0.006 + activity*0.018) * positiveFloat(seed, "mem-hotset", at.Unix(), 20*60)
	reclaim := span * (0.02 + unitFloat(seed, "mem-reclaim", 0)*0.05) *
		eventPulse(seed, "mem-reclaim", at, 12*time.Hour, 0.04)
	noise := span * 0.0015 * interpolatedCenteredFloat(seed, "mem-noise", at.Unix(), 5*60)

	return min + base + workingSet + plateau + trend + hotset - reclaim + noise
}

func diskIOValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	activity := clamp01(scheduledActivity(seed, at) * modifiers.activityScale)
	backup := maintenanceWindow(seed, at) * modifiers.maintenanceScale
	base := span * modifiers.steadyStateScale * (0.015 + unitFloat(seed, "io-base", 0)*0.035)
	sustained := span * activity * modifiers.steadyStateScale * (0.03 + unitFloat(seed, "io-active", 0)*0.07)
	backupLoad := span * backup * (0.05 + unitFloat(seed, "io-backup", 0)*0.10)
	bursts := span * modifiers.burstScale * (0.10 + activity*0.12 + backup*0.18) *
		eventPulse(seed, "io-burst", at, 12*time.Minute, 0.04+activity*0.05+backup*0.03)
	sweeps := span * modifiers.maintenanceScale * (0.06 + backup*0.18) *
		eventPulse(seed, "io-sweep", at, 48*time.Minute, 0.02+backup*0.05)
	chatter := span * (0.006 + activity*0.010) * positiveFloat(seed, "io-chatter", at.Unix(), 8*60)
	noise := span * 0.003 * interpolatedCenteredFloat(seed, "io-noise", at.Unix(), 120)

	return min + base + sustained + backupLoad + bursts + sweeps + chatter + noise
}

func networkValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	activity := clamp01(scheduledActivity(seed, at) * modifiers.activityScale)
	backup := maintenanceWindow(seed+17, at) * 0.45 * modifiers.maintenanceScale
	base := span * modifiers.steadyStateScale * (0.012 + unitFloat(seed, "net-base", 0)*0.04)
	flow := span * activity * modifiers.steadyStateScale * (0.05 + unitFloat(seed, "net-flow", 0)*0.09)
	diurnal := span * modifiers.diurnalScale * scheduledWave(seed, "net-diurnal", at, 0.05)
	bursts := span * modifiers.burstScale * (0.08 + activity*0.16 + backup*0.12) *
		eventPulse(seed, "net-burst", at, 7*time.Minute, 0.05+activity*0.07)
	sessions := span * modifiers.burstScale * (0.05 + activity*0.14) *
		eventPulse(seed, "net-session", at, 28*time.Minute, 0.025+activity*0.03+backup*0.02)
	chatter := span * (0.008 + activity*0.012) * positiveFloat(seed, "net-chatter", at.Unix(), 4*60)
	noise := span * 0.003 * interpolatedCenteredFloat(seed, "net-noise", at.Unix(), 90)

	return min + base + flow + diurnal + bursts + sessions + chatter + noise
}

func capacityValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	base := span * modifiers.steadyStateScale * (0.20 + unitFloat(seed, "capacity-base", 0)*0.58)
	cycle := capacityCycle(seed, at)
	drift := span * 0.05 * modifiers.storageDriftScale * plateauComponent(seed, "capacity-drift", at, 36*time.Hour, 0.08)
	usageWave := span * modifiers.storageDriftScale * (0.07 + unitFloat(seed, "capacity-amp", 0)*0.10) * (cycle - 0.40)
	noise := span * 0.001 * interpolatedCenteredFloat(seed, "capacity-noise", at.Unix(), 30*60)

	return min + base + usageWave + drift + noise
}

func flatValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	base := span * modifiers.steadyStateScale * (0.24 + unitFloat(seed, "flat-base", 0)*0.44)
	drift := span * 0.05 * modifiers.storageDriftScale * plateauComponent(seed, "flat-drift", at, 18*time.Hour, 0.12)
	micro := span * 0.012 * interpolatedCenteredFloat(seed, "flat-micro", at.Unix(), 30*60)
	noise := span * 0.001 * interpolatedCenteredFloat(seed, "flat-noise", at.Unix(), 10*60)

	return min + base + drift + micro + noise
}

func thermalValue(seed uint64, min, span float64, modifiers metricRoleModifiers, at time.Time) float64 {
	base := span * modifiers.thermalScale * (0.18 + unitFloat(seed, "thermal-base", 0)*0.22)
	drift := span * 0.02 * plateauComponent(seed, "thermal-drift", at, 24*time.Hour, 0.06)
	diurnal := span * 0.012 * scheduledWave(seed, "thermal-diurnal", at, 1.0)
	chatter := span * 0.004 * interpolatedCenteredFloat(seed, "thermal-chatter", at.Unix(), 45*60)
	noise := span * 0.0007 * interpolatedCenteredFloat(seed, "thermal-noise", at.Unix(), 15*60)

	return min + base + drift + diurnal + chatter + noise
}

func profileFromMetric(metric string, fallback metricProfile) metricProfile {
	switch strings.ToLower(strings.TrimSpace(metric)) {
	case "cpu":
		return profileCompute
	case "memory":
		return profileMemory
	case "diskread", "diskwrite", "diskio", "iops":
		return profileDiskIO
	case "netin", "netout", "network", "rx", "tx":
		return profileNetwork
	case "disk", "usage", "used", "avail", "total", "free", "storage", "capacity":
		return profileCapacity
	case "temperature", "temp", "smart_temp":
		return profileThermal
	default:
		return fallback
	}
}

func profileFromStyle(style SeriesStyle) metricProfile {
	switch style {
	case StylePlateau:
		return profileMemory
	case StyleFlat:
		return profileCapacity
	default:
		return profileCompute
	}
}

func profileFromSpeed(speed float64) metricProfile {
	switch {
	case speed >= 0.8:
		return profileCompute
	case speed >= 0.3:
		return profileMemory
	default:
		return profileCapacity
	}
}

func anchorHalfLife(profile metricProfile) time.Duration {
	switch profile {
	case profileMemory:
		return 18 * time.Hour
	case profileCapacity:
		return 72 * time.Hour
	case profileThermal, profileFlat:
		return 96 * time.Hour
	case profileDiskIO, profileNetwork:
		return 10 * time.Hour
	default:
		return 6 * time.Hour
	}
}

func anchorWeight(age, halfLife time.Duration) float64 {
	if age <= 0 || halfLife <= 0 {
		return 1
	}
	return math.Exp(-math.Ln2 * float64(age) / float64(halfLife))
}

func scheduledActivity(seed uint64, at time.Time) float64 {
	utc := at.UTC()
	hour := float64(utc.Hour()) + float64(utc.Minute())/60 + float64(utc.Second())/3600
	weekday := utc.Weekday()
	mode := seed % 6
	intensity := 0.82 + unitFloat(seed, "schedule-intensity", 0)*0.36
	weekendScale := 0.35 + unitFloat(seed, "weekend-scale", 0)*0.45

	var activity float64
	switch mode {
	case 0: // office-heavy
		activity = 0.05 + 0.72*wrappedPeak(hour, 10.0, 2.2) + 0.58*wrappedPeak(hour, 14.5, 2.6)
		if weekday == time.Saturday || weekday == time.Sunday {
			activity *= weekendScale
		}
	case 1: // always-on SaaS
		activity = 0.38 + 0.18*wrappedPeak(hour, 11.0, 4.5) + 0.14*wrappedPeak(hour, 20.0, 3.6)
	case 2: // evening-heavy
		activity = 0.07 + 0.24*wrappedPeak(hour, 12.5, 3.0) + 0.72*wrappedPeak(hour, 20.0, 2.8)
		if weekday == time.Saturday || weekday == time.Sunday {
			activity *= 1.15
		}
	case 3: // batch/night
		activity = 0.09 + 0.64*wrappedPeak(hour, 1.5, 2.1) + 0.34*wrappedPeak(hour, 5.0, 2.0)
	case 4: // follow-the-sun
		activity = 0.26 + 0.22*wrappedPeak(hour, 8.0, 4.0) + 0.20*wrappedPeak(hour, 16.0, 4.0) + 0.12*wrappedPeak(hour, 22.0, 3.3)
	default: // weekend/media
		activity = 0.08 + 0.28*wrappedPeak(hour, 11.5, 3.3) + 0.62*wrappedPeak(hour, 21.0, 3.0)
		if weekday == time.Saturday || weekday == time.Sunday {
			activity *= 1.22
		} else {
			activity *= 0.85
		}
	}

	return clamp01(activity * intensity)
}

func scheduledWave(seed uint64, label string, at time.Time, amplitude float64) float64 {
	activity := scheduledActivity(seed, at)
	return amplitude * (activity - 0.42)
}

func maintenanceWindow(seed uint64, at time.Time) float64 {
	hour := float64(at.UTC().Hour()) + float64(at.UTC().Minute())/60
	center := unitFloat(seed, "maintenance-hour", 0) * 24
	width := 0.9 + unitFloat(seed, "maintenance-width", 0)*1.7
	strength := 0.35 + unitFloat(seed, "maintenance-strength", 0)*0.65
	return wrappedPeak(hour, center, width) * strength
}

func slowTrend(seed uint64, label string, at time.Time, shortWindow, longWindow time.Duration, amplitude float64) float64 {
	short := interpolatedCenteredFloat(seed, label+"-short", at.Unix(), int64(shortWindow/time.Second))
	long := interpolatedCenteredFloat(seed, label+"-long", at.Unix(), int64(longWindow/time.Second))
	return amplitude * (short*0.45 + long*0.55)
}

func plateauComponent(seed uint64, label string, at time.Time, window time.Duration, transitionFraction float64) float64 {
	seconds := int64(window / time.Second)
	if seconds <= 0 {
		return centeredFloat(seed, label, 0)
	}
	if transitionFraction <= 0 {
		transitionFraction = 0.1
	}
	if transitionFraction >= 1 {
		transitionFraction = 0.95
	}

	bucket := floorDiv(at.Unix(), seconds)
	progress := float64(mod(at.Unix(), seconds)) / float64(seconds)
	current := centeredFloat(seed, label, bucket)
	startTransition := 1 - transitionFraction
	if progress <= startTransition {
		return current
	}

	next := centeredFloat(seed, label, bucket+1)
	t := smoothstep((progress - startTransition) / transitionFraction)
	return current*(1-t) + next*t
}

func capacityCycle(seed uint64, at time.Time) float64 {
	cycleHours := 42 + unitFloat(seed, "capacity-cycle", 0)*120
	offset := unitFloat(seed, "capacity-offset", 0) * cycleHours
	phase := math.Mod(float64(at.Unix())/3600+offset, cycleHours) / cycleHours
	if phase < 0.9 {
		return phase / 0.9
	}
	return 1 - smoothstep((phase-0.9)/0.1)
}

func eventPulse(seed uint64, label string, at time.Time, window time.Duration, probability float64) float64 {
	seconds := int64(window / time.Second)
	if seconds <= 0 {
		return 0
	}
	probability = clampFloat(probability, 0, 0.92)
	bucket := floorDiv(at.Unix(), seconds)
	if unitFloat(seed, label+"-gate", bucket) >= probability {
		return 0
	}

	progress := float64(mod(at.Unix(), seconds)) / float64(seconds)
	riseFraction := 0.08 + unitFloat(seed, label+"-rise", bucket)*0.18
	if progress <= riseFraction {
		return progress / riseFraction
	}

	decay := (progress - riseFraction) / math.Max(1e-6, 1-riseFraction)
	power := 1.2 + unitFloat(seed, label+"-power", bucket)*2.2
	return math.Pow(1-decay, power)
}

func positiveFloat(seed uint64, label string, unixSeconds, window int64) float64 {
	return math.Max(0, interpolatedCenteredFloat(seed, label, unixSeconds, window))
}

func wrappedPeak(hour, center, width float64) float64 {
	if width <= 0 {
		width = 1
	}
	delta := math.Abs(hour - center)
	if delta > 12 {
		delta = 24 - delta
	}
	x := delta / width
	return math.Exp(-0.5 * x * x)
}

func clamp01(value float64) float64 {
	return clampFloat(value, 0, 1)
}

func interpolatedCenteredFloat(seed uint64, label string, unixSeconds, window int64) float64 {
	if window <= 0 {
		return centeredFloat(seed, label, 0)
	}

	bucket := floorDiv(unixSeconds, window)
	progress := float64(mod(unixSeconds, window)) / float64(window)
	a := centeredFloat(seed, label, bucket)
	b := centeredFloat(seed, label, bucket+1)
	t := smoothstep(progress)
	return a*(1-t) + b*t
}

func centeredFloat(seed uint64, label string, bucket int64) float64 {
	return unitFloat(seed, label, bucket)*2 - 1
}

func unitFloat(seed uint64, label string, bucket int64) float64 {
	hash := hashKey(seed, label, bucket)
	const mask = uint64(1<<53) - 1
	return float64(hash&mask) / float64(mask)
}

func hashKey(seed uint64, label string, bucket int64) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(strconv.FormatUint(seed, 10)))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(label))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(strconv.FormatInt(bucket, 10)))
	return h.Sum64()
}

func smoothstep(t float64) float64 {
	if t <= 0 {
		return 0
	}
	if t >= 1 {
		return 1
	}
	return t * t * (3 - 2*t)
}

func floorDiv(value, divisor int64) int64 {
	if divisor == 0 {
		return 0
	}
	quotient := value / divisor
	remainder := value % divisor
	if remainder != 0 && ((remainder < 0) != (divisor < 0)) {
		quotient--
	}
	return quotient
}

func mod(value, divisor int64) int64 {
	if divisor == 0 {
		return 0
	}
	remainder := value % divisor
	if remainder < 0 {
		remainder += divisor
	}
	return remainder
}

func clampFloat(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
