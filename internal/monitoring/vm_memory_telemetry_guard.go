package monitoring

import (
	"fmt"
	"sort"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rs/zerolog/log"
)

const (
	minRunningVMsForRepeatedMemoryCheck = 4
	minRepeatedVMsForSuspicion          = 3
	minRepeatedMemoryShare              = 0.60
	maxRepeatedMemorySampleNames        = 5
)

type repeatedVMMemoryUsage struct {
	suspicious      bool
	signature       string
	runningCount    int
	repeatedCount   int
	repeatedMemUsed int64
	sourceBreakdown []string
	sampleVMNames   []string
}

type repeatedMemoryGroup struct {
	count   int
	sources map[string]int
	names   []string
}

func filterVMsByInstance(vms []models.VM, instanceName string) []models.VM {
	filtered := make([]models.VM, 0, len(vms))
	for _, vm := range vms {
		if vm.Instance == instanceName {
			filtered = append(filtered, vm)
		}
	}
	return filtered
}

func detectRepeatedVMMemoryUsage(vms []models.VM) repeatedVMMemoryUsage {
	groups := make(map[int64]*repeatedMemoryGroup)
	runningCount := 0

	for _, vm := range vms {
		if vm.Type != "qemu" || vm.Status != "running" || vm.Memory.Total <= 0 || vm.Memory.Used <= 0 {
			continue
		}
		runningCount++

		group, ok := groups[vm.Memory.Used]
		if !ok {
			group = &repeatedMemoryGroup{
				sources: make(map[string]int),
			}
			groups[vm.Memory.Used] = group
		}
		group.count++
		source := strings.TrimSpace(vm.MemorySource)
		if source == "" {
			source = "unknown"
		}
		group.sources[source]++
		if len(group.names) < maxRepeatedMemorySampleNames {
			name := strings.TrimSpace(vm.Name)
			if name == "" {
				name = vm.ID
			}
			group.names = append(group.names, name)
		}
	}

	if runningCount < minRunningVMsForRepeatedMemoryCheck {
		return repeatedVMMemoryUsage{runningCount: runningCount}
	}

	var topUsed int64
	var topGroup *repeatedMemoryGroup
	for used, group := range groups {
		if topGroup == nil || group.count > topGroup.count {
			topUsed = used
			topGroup = group
		}
	}
	if topGroup == nil {
		return repeatedVMMemoryUsage{runningCount: runningCount}
	}

	share := float64(topGroup.count) / float64(runningCount)
	if topGroup.count < minRepeatedVMsForSuspicion || share < minRepeatedMemoryShare {
		return repeatedVMMemoryUsage{runningCount: runningCount}
	}

	breakdown := formatMemorySourceBreakdown(topGroup.sources)
	sampleNames := append([]string(nil), topGroup.names...)
	sort.Strings(sampleNames)

	return repeatedVMMemoryUsage{
		suspicious:      true,
		signature:       fmt.Sprintf("%d:%d:%s", topUsed, topGroup.count, strings.Join(breakdown, ",")),
		runningCount:    runningCount,
		repeatedCount:   topGroup.count,
		repeatedMemUsed: topUsed,
		sourceBreakdown: breakdown,
		sampleVMNames:   sampleNames,
	}
}

func formatMemorySourceBreakdown(counts map[string]int) []string {
	type sourceCount struct {
		source string
		count  int
	}
	entries := make([]sourceCount, 0, len(counts))
	for source, count := range counts {
		entries = append(entries, sourceCount{source: source, count: count})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count == entries[j].count {
			return entries[i].source < entries[j].source
		}
		return entries[i].count > entries[j].count
	})

	breakdown := make([]string, 0, len(entries))
	for _, entry := range entries {
		breakdown = append(breakdown, fmt.Sprintf("%s:%d", entry.source, entry.count))
	}
	return breakdown
}

func (m *Monitor) logSuspiciousRepeatedVMMemoryUsage(instanceName string, currentVMs []models.VM, previousVMs []models.VM) {
	current := detectRepeatedVMMemoryUsage(currentVMs)
	if !current.suspicious {
		return
	}

	previous := detectRepeatedVMMemoryUsage(previousVMs)
	if previous.suspicious && previous.signature == current.signature {
		return
	}

	log.Warn().
		Str("instance", instanceName).
		Int("runningVMs", current.runningCount).
		Int("repeatedVMs", current.repeatedCount).
		Int64("repeatedMemoryUsedBytes", current.repeatedMemUsed).
		Strs("memorySources", current.sourceBreakdown).
		Strs("sampleVMs", current.sampleVMNames).
		Msg("Suspicious repeated VM memory-used pattern detected")
}
