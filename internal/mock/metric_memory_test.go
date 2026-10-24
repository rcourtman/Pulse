package mock

import (
	"fmt"
	"math"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
)

func TestMemoryMetricRoleBoundsStayBelowCapacity(t *testing.T) {
	classes := []string{"node", "vm", "container", "agent", "dockerHost", "dockerContainer", "pbs", "pbsDatastore", "unknown"}
	roles := []string{metricRoleGeneral, metricRoleWeb, metricRoleAPI, metricRoleDatabase, metricRoleCache, metricRoleQueue, metricRoleMonitoring, metricRoleBackup, metricRoleStorage, metricRoleIngress, metricRoleCI, metricRoleMedia, metricRoleSecurity, metricRoleBatch}
	for _, class := range classes {
		for _, role := range roles {
			t.Run(class+"/"+role, func(t *testing.T) {
				min, max := metricBoundsForRole(class, "memory", role)
				if math.IsNaN(min) || math.IsNaN(max) || min < 0 || max >= 100 || max <= min {
					t.Fatalf("memory bounds [%v, %v] must leave room for reclaimable cache and free pages", min, max)
				}
			})
		}
	}

	// The repair is memory-only. Multi-core Docker CPU may legitimately
	// exceed 100%, and unamplified agent memory retains its existing range.
	if _, max := metricBoundsForRole("dockerContainer", "cpu", metricRoleCI); max <= 100 {
		t.Fatalf("multi-core Docker CPU ceiling was reduced to %v", max)
	}
	if min, max := metricBoundsForRole("agent", "memory", metricRoleGeneral); min != 12 || max != 96 {
		t.Fatalf("ordinary agent memory bounds changed: [%v, %v]", min, max)
	}
}

func TestMemoryMetricSamplersKeepReclaimableHeadroom(t *testing.T) {
	previousRegistry := currentMetricRoleRegistry()
	t.Cleanup(func() { setMetricRoleRegistry(previousRegistry) })

	// Fixed week, identity and personas: the old amplified agent/cache bounds
	// admit >100% samples without relying on wall time or repeated random runs.
	start := time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)
	timestamps := make([]time.Time, 7*24*4)
	for i := range timestamps {
		timestamps[i] = start.Add(time.Duration(i) * 15 * time.Minute)
	}
	for _, class := range []string{"agent", "dockerHost", "dockerContainer"} {
		for _, role := range []string{metricRoleGeneral, metricRoleDatabase, metricRoleCache, metricRoleMonitoring} {
			t.Run(class+"/"+role, func(t *testing.T) {
				id := "host-linux-1"
				roles := map[string]string{metricRoleRegistryKey(class, id): role}
				setMetricRoleRegistry(roles)
				sampler := MetricSampler{roles: roles}
				series := SampleMetricSeries(class, id, "memory", timestamps)
				boundSeries := sampler.SampleMetricSeries(class, id, "memory", timestamps)
				for i, at := range timestamps {
					point := SampleMetric(class, id, "memory", at)
					boundPoint := sampler.SampleMetric(class, id, "memory", at)
					if point != series[i] || point != boundPoint || point != boundSeries[i] {
						t.Fatalf("point/global/bound series diverged at %s: %v %v %v %v", at, point, series[i], boundPoint, boundSeries[i])
					}
					total := int64(26 << 30) // capacity in the original CI failure
					used := int64(float64(total) * point / 100)
					cache := (total - used) / 3
					free := total - used - cache
					if math.IsNaN(point) || math.IsInf(point, 0) || point < 0 || point >= 100 || used < 0 || cache <= 0 || free < 0 || used+cache+free != total {
						t.Fatalf("invalid memory at %s: usage=%v total=%d used=%d cache=%d free=%d", at, point, total, used, cache, free)
					}
				}
			})
		}
	}
}

func TestGeneratedHostsKeepMemoryAccountingThroughRoleChanges(t *testing.T) {
	previousRegistry := currentMetricRoleRegistry()
	t.Cleanup(func() { setMetricRoleRegistry(previousRegistry) })
	cfg := DefaultConfig
	cfg.GenericHostCount = 5
	cfg.RandomMetrics = true

	roles := make(map[string]string)
	for i := 0; i < cfg.GenericHostCount; i++ {
		profile := genericHostProfiles[mockStableChoice(len(genericHostProfiles), fmt.Sprintf("%d", i), "generic-host-profile")]
		roles[metricRoleRegistryKey("agent", fmt.Sprintf("host-%s-%d", profile.Platform, i+1))] = metricRoleCache
	}
	setMetricRoleRegistry(roles)
	data := models.StateSnapshot{Hosts: generateHosts(cfg)}
	check := func(at string) {
		t.Helper()
		for _, host := range data.Hosts {
			mem := host.Memory
			if mem.Total <= 0 || mem.Used < 0 || mem.Cache <= 0 || mem.Free < 0 || mem.Used+mem.Cache+mem.Free != mem.Total || mem.Usage < 0 || mem.Usage >= 100 || math.IsNaN(mem.Usage) {
				t.Fatalf("%s host %s has invalid memory accounting: %+v", at, host.ID, mem)
			}
			if delta := math.Abs(mem.Usage - 100*float64(mem.Used)/float64(mem.Total)); delta > 100/float64(mem.Total) {
				t.Fatalf("%s host %s percentage does not match used bytes: %+v", at, host.ID, mem)
			}
		}
	}
	check("initial generation")
	for i := range data.Hosts {
		data.Hosts[i].Status = "online"
	}
	start := time.Date(2026, time.October, 23, 0, 0, 0, 0, time.UTC)
	for tick := 0; tick < 56; tick++ {
		role := []string{metricRoleCache, metricRoleDatabase, metricRoleBackup, metricRoleGeneral}[tick%4]
		for id := range roles {
			roles[id] = role
		}
		setMetricRoleRegistry(roles)
		at := start.Add(time.Duration(tick) * 3 * time.Hour)
		updateHosts(&data, cfg, at)
		check(at.String())
	}
}
