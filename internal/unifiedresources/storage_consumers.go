package unifiedresources

import (
	"path/filepath"
	"sort"
	"strings"
)

const maxStorageTopConsumers = 5

type storageConsumerIndexKey struct {
	instance string
	name     string
}

type storageConsumerIndex struct {
	byName map[storageConsumerIndexKey][]*Resource
	byPath map[string][]*Resource
}

type storageConsumerSummary struct {
	ResourceID   string
	ResourceType ResourceType
	Name         string
	DiskCount    int
}

func (rr *ResourceRegistry) refreshStorageConsumersLocked() {
	index := buildStorageConsumerIndex(rr.resources)
	for _, resource := range rr.resources {
		if resource.Storage == nil {
			continue
		}
		resource.Storage.ConsumerCount = 0
		resource.Storage.ConsumerTypes = nil
		resource.Storage.TopConsumers = nil
	}
	if len(index.byName) == 0 && len(index.byPath) == 0 {
		return
	}

	consumersByStorage := make(map[string]map[string]*storageConsumerSummary)
	for _, resource := range rr.resources {
		if !isStorageConsumerResource(resource) {
			continue
		}
		matches := matchStorageConsumers(resource, index)
		for storageID, diskCount := range matches {
			if diskCount <= 0 {
				continue
			}
			storageConsumers := consumersByStorage[storageID]
			if storageConsumers == nil {
				storageConsumers = make(map[string]*storageConsumerSummary)
				consumersByStorage[storageID] = storageConsumers
			}
			summary := storageConsumers[resource.ID]
			if summary == nil {
				summary = &storageConsumerSummary{
					ResourceID:   resource.ID,
					ResourceType: resource.Type,
					Name:         strings.TrimSpace(resource.Name),
				}
				storageConsumers[resource.ID] = summary
			}
			summary.DiskCount += diskCount
		}
	}

	for storageID, storageConsumers := range consumersByStorage {
		resource := rr.resources[storageID]
		if resource == nil || resource.Storage == nil {
			continue
		}

		topConsumers := make([]StorageConsumerMeta, 0, len(storageConsumers))
		consumerTypes := make([]string, 0, len(storageConsumers))
		for _, consumer := range storageConsumers {
			topConsumers = append(topConsumers, StorageConsumerMeta{
				ResourceID:   consumer.ResourceID,
				ResourceType: consumer.ResourceType,
				Name:         consumer.Name,
				DiskCount:    consumer.DiskCount,
			})
			consumerTypes = append(consumerTypes, string(consumer.ResourceType))
		}

		sort.Slice(topConsumers, func(i, j int) bool {
			if topConsumers[i].DiskCount != topConsumers[j].DiskCount {
				return topConsumers[i].DiskCount > topConsumers[j].DiskCount
			}
			if topConsumers[i].ResourceType != topConsumers[j].ResourceType {
				return topConsumers[i].ResourceType < topConsumers[j].ResourceType
			}
			if topConsumers[i].Name != topConsumers[j].Name {
				return topConsumers[i].Name < topConsumers[j].Name
			}
			return topConsumers[i].ResourceID < topConsumers[j].ResourceID
		})
		if len(topConsumers) > maxStorageTopConsumers {
			topConsumers = topConsumers[:maxStorageTopConsumers]
		}

		resource.Storage.ConsumerCount = len(storageConsumers)
		resource.Storage.ConsumerTypes = uniqueStrings(consumerTypes)
		sort.Strings(resource.Storage.ConsumerTypes)
		resource.Storage.TopConsumers = topConsumers
	}
}

func buildStorageConsumerIndex(resources map[string]*Resource) storageConsumerIndex {
	index := storageConsumerIndex{
		byName: make(map[storageConsumerIndexKey][]*Resource),
		byPath: make(map[string][]*Resource),
	}

	for _, resource := range resources {
		if resource == nil || resource.Type != ResourceTypeStorage || resource.Storage == nil || resource.Proxmox == nil {
			continue
		}
		if !hasDataSource(resource.Sources, SourceProxmox) {
			continue
		}

		instance := strings.TrimSpace(resource.Proxmox.Instance)
		if instance == "" {
			continue
		}

		for _, name := range storageLookupNames(resource) {
			key := storageConsumerIndexKey{
				instance: instance,
				name:     normalizeStorageLookupName(name),
			}
			if key.name == "" {
				continue
			}
			index.byName[key] = append(index.byName[key], resource)
		}

		path := normalizeStoragePath(resource.Storage.Path)
		if path != "" {
			index.byPath[instance] = append(index.byPath[instance], resource)
		}
	}

	return index
}

func isStorageConsumerResource(resource *Resource) bool {
	if resource == nil || resource.Proxmox == nil {
		return false
	}
	switch resource.Type {
	case ResourceTypeVM, ResourceTypeSystemContainer:
		return true
	default:
		return false
	}
}

func matchStorageConsumers(resource *Resource, index storageConsumerIndex) map[string]int {
	instance := strings.TrimSpace(resource.Proxmox.Instance)
	if instance == "" {
		return nil
	}

	matches := make(map[string]int)
	for _, disk := range resource.Proxmox.Disks {
		device := strings.TrimSpace(disk.Device)
		if device == "" {
			continue
		}

		if storageName := proxmoxStorageNameFromDevice(device); storageName != "" {
			candidates := index.byName[storageConsumerIndexKey{
				instance: instance,
				name:     normalizeStorageLookupName(storageName),
			}]
			if candidate := selectStorageConsumerCandidate(candidates, resource.Proxmox.NodeName); candidate != nil {
				matches[candidate.ID]++
				continue
			}
		}

		if candidate := selectStorageConsumerPathCandidate(index.byPath[instance], resource.Proxmox.NodeName, device); candidate != nil {
			matches[candidate.ID]++
		}
	}

	return matches
}

func selectStorageConsumerCandidate(candidates []*Resource, guestNode string) *Resource {
	filtered := filterStorageCandidatesForNode(candidates, guestNode)
	if len(filtered) == 1 {
		return filtered[0]
	}
	return nil
}

func selectStorageConsumerPathCandidate(candidates []*Resource, guestNode, device string) *Resource {
	devicePath := normalizeStoragePath(device)
	if devicePath == "" {
		return nil
	}

	filtered := filterStorageCandidatesForNode(candidates, guestNode)
	bestLength := 0
	var matched *Resource
	for _, candidate := range filtered {
		path := normalizeStoragePath(candidate.Storage.Path)
		if !storagePathMatchesDevice(path, devicePath) {
			continue
		}
		if len(path) < bestLength {
			continue
		}
		if len(path) == bestLength {
			if matched != nil && matched.ID != candidate.ID {
				matched = nil
			}
			continue
		}
		bestLength = len(path)
		matched = candidate
	}
	return matched
}

func filterStorageCandidatesForNode(candidates []*Resource, guestNode string) []*Resource {
	if len(candidates) == 0 {
		return nil
	}
	filtered := make([]*Resource, 0, len(candidates))
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Storage == nil || candidate.Proxmox == nil {
			continue
		}
		if !storageSupportsNode(candidate, guestNode) {
			continue
		}
		if _, ok := seen[candidate.ID]; ok {
			continue
		}
		seen[candidate.ID] = struct{}{}
		filtered = append(filtered, candidate)
	}
	return filtered
}

func storageSupportsNode(resource *Resource, guestNode string) bool {
	node := normalizeStorageNodeName(guestNode)
	if node == "" {
		return true
	}
	if resource == nil || resource.Storage == nil || resource.Proxmox == nil {
		return false
	}
	if resource.Storage.Shared {
		if len(resource.Storage.Nodes) == 0 {
			return true
		}
		for _, candidate := range resource.Storage.Nodes {
			if normalizeStorageNodeName(candidate) == node {
				return true
			}
		}
		return false
	}
	return normalizeStorageNodeName(resource.Proxmox.NodeName) == node
}

func storageLookupNames(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	return uniqueStrings([]string{
		strings.TrimSpace(resource.Name),
	})
}

func proxmoxStorageNameFromDevice(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	idx := strings.Index(device, ":")
	if idx <= 0 {
		return ""
	}
	return strings.TrimSpace(device[:idx])
}

func normalizeStorageLookupName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func normalizeStorageNodeName(node string) string {
	node = strings.TrimSpace(node)
	if node == "" {
		return ""
	}
	normalized := NormalizeHostname(node)
	if normalized != "" {
		return normalized
	}
	return strings.ToLower(node)
}

func normalizeStoragePath(path string) string {
	path = strings.TrimSpace(path)
	if path == "" || !strings.HasPrefix(path, "/") {
		return ""
	}
	return filepath.Clean(path)
}

func storagePathMatchesDevice(storagePath, devicePath string) bool {
	if storagePath == "" || devicePath == "" {
		return false
	}
	if devicePath == storagePath {
		return true
	}
	return strings.HasPrefix(devicePath, storagePath+"/")
}
