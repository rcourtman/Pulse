package unifiedresources

import (
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
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

type pbsDatastoreIndexKey struct {
	instance string
	name     string
}

type pbsBackupWorkloadIndex struct {
	byVMID        map[int][]*Resource
	vmidAmbiguous map[int]bool
}

func (rr *ResourceRegistry) refreshStorageConsumersLocked() {
	for _, resource := range rr.resources {
		if resource.Storage == nil {
			continue
		}
		resource.Storage.ConsumerCount = 0
		resource.Storage.ConsumerTypes = nil
		resource.Storage.TopConsumers = nil
	}

	consumersByStorage := make(map[string]map[string]*storageConsumerSummary)

	proxmoxIndex := buildStorageConsumerIndex(rr.resources)
	if len(proxmoxIndex.byName) > 0 || len(proxmoxIndex.byPath) > 0 {
		addProxmoxStorageConsumers(consumersByStorage, rr.resources, proxmoxIndex)
	}

	addPBSStorageConsumers(consumersByStorage, rr.resources, rr.pbsBackups)
	applyStorageConsumers(rr.resources, consumersByStorage)
}

func addProxmoxStorageConsumers(consumersByStorage map[string]map[string]*storageConsumerSummary, resources map[string]*Resource, index storageConsumerIndex) {
	for _, resource := range resources {
		if !isStorageConsumerResource(resource) {
			continue
		}
		matches := matchStorageConsumers(resource, index)
		for storageID, diskCount := range matches {
			addStorageConsumerSummary(consumersByStorage, storageID, resource, diskCount)
		}
	}
}

func addPBSStorageConsumers(consumersByStorage map[string]map[string]*storageConsumerSummary, resources map[string]*Resource, backups []models.PBSBackup) {
	if len(backups) == 0 {
		return
	}

	datastoreIndex := buildPBSDatastoreIndex(resources)
	workloadIndex := buildPBSBackupWorkloadIndex(resources)
	if len(datastoreIndex) == 0 || len(workloadIndex.byVMID) == 0 {
		return
	}

	for _, backup := range backups {
		vmid, err := strconv.Atoi(strings.TrimSpace(backup.VMID))
		if err != nil || vmid <= 0 {
			continue
		}
		datastore := selectPBSDatastoreResource(datastoreIndex, backup)
		if datastore == nil {
			continue
		}
		workload := resolvePBSBackupWorkload(workloadIndex, backup, vmid)
		if workload == nil {
			continue
		}
		addStorageConsumerSummary(consumersByStorage, datastore.ID, workload, 1)
	}
}

func addStorageConsumerSummary(consumersByStorage map[string]map[string]*storageConsumerSummary, storageID string, resource *Resource, count int) {
	if count <= 0 || resource == nil || strings.TrimSpace(storageID) == "" {
		return
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
	summary.DiskCount += count
}

func applyStorageConsumers(resources map[string]*Resource, consumersByStorage map[string]map[string]*storageConsumerSummary) {
	for storageID, storageConsumers := range consumersByStorage {
		resource := resources[storageID]
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

func buildPBSDatastoreIndex(resources map[string]*Resource) map[pbsDatastoreIndexKey][]*Resource {
	index := make(map[pbsDatastoreIndexKey][]*Resource)
	for _, resource := range resources {
		if !isPBSDatastoreResource(resource) {
			continue
		}
		datastoreName := normalizePBSLookupName(resource.Name)
		if datastoreName == "" {
			continue
		}
		for _, alias := range pbsDatastoreInstanceAliases(resource, resources) {
			key := pbsDatastoreIndexKey{
				instance: normalizePBSLookupName(alias),
				name:     datastoreName,
			}
			if key.instance == "" {
				continue
			}
			index[key] = append(index[key], resource)
		}
	}
	return index
}

func buildPBSBackupWorkloadIndex(resources map[string]*Resource) pbsBackupWorkloadIndex {
	index := pbsBackupWorkloadIndex{
		byVMID:        make(map[int][]*Resource),
		vmidAmbiguous: make(map[int]bool),
	}
	vmidInstances := make(map[int]map[string]struct{})
	for _, resource := range resources {
		if resource == nil || resource.Proxmox == nil {
			continue
		}
		switch resource.Type {
		case ResourceTypeVM, ResourceTypeSystemContainer:
		default:
			continue
		}
		vmid := resource.Proxmox.VMID
		if vmid <= 0 {
			continue
		}
		index.byVMID[vmid] = append(index.byVMID[vmid], resource)
		instances := vmidInstances[vmid]
		if instances == nil {
			instances = make(map[string]struct{})
			vmidInstances[vmid] = instances
		}
		instances[strings.TrimSpace(resource.Proxmox.Instance)] = struct{}{}
	}
	for vmid, instances := range vmidInstances {
		if len(instances) > 1 {
			index.vmidAmbiguous[vmid] = true
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

func isPBSDatastoreResource(resource *Resource) bool {
	return resource != nil &&
		resource.Type == ResourceTypeStorage &&
		resource.Storage != nil &&
		strings.EqualFold(strings.TrimSpace(resource.Storage.Platform), "pbs") &&
		strings.EqualFold(strings.TrimSpace(resource.Storage.Topology), "datastore")
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

func pbsDatastoreInstanceAliases(resource *Resource, resources map[string]*Resource) []string {
	if resource == nil {
		return nil
	}
	aliases := []string{resource.Name}
	if resource.ParentID != nil {
		if parent := resources[strings.TrimSpace(*resource.ParentID)]; parent != nil {
			aliases = append(aliases, parent.Name)
			if parent.PBS != nil {
				aliases = append(aliases, parent.PBS.InstanceID, parent.PBS.Hostname)
			}
		}
	}
	return uniqueStrings(aliases)
}

func selectPBSDatastoreResource(index map[pbsDatastoreIndexKey][]*Resource, backup models.PBSBackup) *Resource {
	key := pbsDatastoreIndexKey{
		instance: normalizePBSLookupName(backup.Instance),
		name:     normalizePBSLookupName(backup.Datastore),
	}
	if key.instance == "" || key.name == "" {
		return nil
	}
	candidates := index[key]
	if len(candidates) != 1 {
		return nil
	}
	return candidates[0]
}

func resolvePBSBackupWorkload(index pbsBackupWorkloadIndex, backup models.PBSBackup, vmid int) *Resource {
	candidates := index.byVMID[vmid]
	if len(candidates) == 0 {
		return nil
	}
	candidates = filterPBSBackupCandidatesByType(candidates, backup.BackupType)
	if len(candidates) == 0 {
		return nil
	}
	if matched := filterPBSBackupCandidatesByNamespace(candidates, backup.Namespace); len(matched) == 1 {
		return matched[0]
	} else if len(matched) > 1 {
		return nil
	}
	if index.vmidAmbiguous[vmid] {
		return nil
	}
	if len(candidates) == 1 {
		return candidates[0]
	}
	return nil
}

func filterPBSBackupCandidatesByType(candidates []*Resource, backupType string) []*Resource {
	backupType = strings.ToLower(strings.TrimSpace(backupType))
	if backupType == "" {
		return candidates
	}
	filtered := make([]*Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil {
			continue
		}
		switch backupType {
		case "vm":
			if candidate.Type == ResourceTypeVM {
				filtered = append(filtered, candidate)
			}
		case "ct", "container":
			if candidate.Type == ResourceTypeSystemContainer {
				filtered = append(filtered, candidate)
			}
		default:
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func filterPBSBackupCandidatesByNamespace(candidates []*Resource, namespace string) []*Resource {
	namespace = strings.TrimSpace(namespace)
	if namespace == "" {
		return nil
	}
	filtered := make([]*Resource, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == nil || candidate.Proxmox == nil {
			continue
		}
		if pbsNamespaceMatchesInstance(namespace, candidate.Proxmox.Instance) {
			filtered = append(filtered, candidate)
		}
	}
	return filtered
}

func normalizePBSLookupName(value string) string {
	return strings.ToLower(strings.TrimSpace(value))
}

func pbsNamespaceMatchesInstance(namespace, instance string) bool {
	namespace = normalizeStorageLookupName(namespace)
	instance = normalizeStorageLookupName(instance)
	if namespace == "" || instance == "" {
		return false
	}
	if namespace == instance {
		return true
	}
	return strings.HasSuffix(instance, namespace) || strings.HasSuffix(namespace, instance)
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
	if normalizeStorageNodeName(resource.Proxmox.NodeName) == node {
		return true
	}
	for _, candidate := range resource.Storage.Nodes {
		if normalizeStorageNodeName(candidate) == node {
			return true
		}
	}
	return resource.Storage.Shared
}

func storageLookupNames(resource *Resource) []string {
	if resource == nil {
		return nil
	}
	names := []string{
		resource.Name,
	}
	if resource.Storage != nil {
		names = append(names, resource.Storage.Type, resource.Storage.Content)
	}
	return uniqueStrings(names)
}

func proxmoxStorageNameFromDevice(device string) string {
	device = strings.TrimSpace(device)
	if device == "" {
		return ""
	}
	if idx := strings.IndexRune(device, ':'); idx > 0 {
		return device[:idx]
	}
	return ""
}

func normalizeStorageLookupName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	if name == "" {
		return ""
	}
	return strings.Trim(name, "/")
}

func normalizeStoragePath(path string) string {
	path = filepath.Clean(strings.TrimSpace(path))
	if path == "." {
		return ""
	}
	return path
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

func normalizeStorageNodeName(name string) string {
	name = strings.TrimSpace(strings.ToLower(name))
	name = strings.TrimSuffix(name, ".local")
	return name
}
