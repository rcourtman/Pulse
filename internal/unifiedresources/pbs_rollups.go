package unifiedresources

import (
	"sort"
	"strconv"
	"strings"
)

const maxPBSTopProtectedWorkloads = 5

type pbsProtectedWorkloadSummary struct {
	ResourceID   string
	ResourceType ResourceType
	Name         string
}

func (rr *ResourceRegistry) refreshPBSRollupsLocked() {
	for _, resource := range rr.resources {
		if resource == nil || resource.PBS == nil {
			continue
		}
		resource.PBS.AffectedDatastoreCount = 0
		resource.PBS.AffectedDatastores = nil
		resource.PBS.AffectedDatastoreSummary = ""
		resource.PBS.ProtectedWorkloadCount = 0
		resource.PBS.ProtectedWorkloadTypes = nil
		resource.PBS.ProtectedWorkloadNames = nil
		resource.PBS.ProtectedWorkloadSummary = ""
		resource.PBS.PostureSummary = ""
	}

	affectedDatastores := make(map[string][]string)
	for _, resource := range rr.resources {
		if !isPBSDatastoreResource(resource) || resource.ParentID == nil {
			continue
		}
		parentID := strings.TrimSpace(*resource.ParentID)
		parent := rr.resources[parentID]
		if parent == nil || parent.PBS == nil {
			continue
		}
		if !pbsDatastoreAffectsPosture(resource) {
			continue
		}
		affectedDatastores[parentID] = append(affectedDatastores[parentID], strings.TrimSpace(resource.Name))
	}

	protectedByPBS := make(map[string]map[string]*pbsProtectedWorkloadSummary)
	if len(rr.pbsBackups) > 0 {
		instanceIndex := buildPBSInstanceIndex(rr.resources)
		workloadIndex := buildPBSBackupWorkloadIndex(rr.resources)
		for _, backup := range rr.pbsBackups {
			pbsResource := selectPBSInstanceResource(instanceIndex, backup.Instance)
			if pbsResource == nil || pbsResource.PBS == nil {
				continue
			}
			vmid, err := strconv.Atoi(strings.TrimSpace(backup.VMID))
			if err != nil || vmid <= 0 {
				continue
			}
			workload := resolvePBSBackupWorkload(workloadIndex, backup, vmid)
			if workload == nil {
				continue
			}
			workloads := protectedByPBS[pbsResource.ID]
			if workloads == nil {
				workloads = make(map[string]*pbsProtectedWorkloadSummary)
				protectedByPBS[pbsResource.ID] = workloads
			}
			if _, exists := workloads[workload.ID]; exists {
				continue
			}
			workloads[workload.ID] = &pbsProtectedWorkloadSummary{
				ResourceID:   workload.ID,
				ResourceType: workload.Type,
				Name:         strings.TrimSpace(workload.Name),
			}
		}
	}

	for _, resource := range rr.resources {
		if resource == nil || resource.PBS == nil {
			continue
		}

		names := uniqueStrings(affectedDatastores[resource.ID])
		sort.Strings(names)
		resource.PBS.AffectedDatastores = append([]string(nil), names...)
		resource.PBS.AffectedDatastoreCount = len(names)
		resource.PBS.AffectedDatastoreSummary = summarizePBSAffectedDatastores(resource.PBS.AffectedDatastores)

		workloads := protectedByPBS[resource.ID]
		if len(workloads) == 0 {
			resource.PBS.PostureSummary = summarizePBSPosture(resource.PBS)
			continue
		}

		summaries := make([]pbsProtectedWorkloadSummary, 0, len(workloads))
		types := make([]string, 0, len(workloads))
		names = names[:0]
		for _, workload := range workloads {
			summaries = append(summaries, *workload)
			types = append(types, string(workload.ResourceType))
		}
		sort.Slice(summaries, func(i, j int) bool {
			if summaries[i].ResourceType != summaries[j].ResourceType {
				return summaries[i].ResourceType < summaries[j].ResourceType
			}
			if summaries[i].Name != summaries[j].Name {
				return summaries[i].Name < summaries[j].Name
			}
			return summaries[i].ResourceID < summaries[j].ResourceID
		})

		protectedNames := make([]string, 0, len(summaries))
		for _, summary := range summaries {
			if summary.Name == "" {
				continue
			}
			protectedNames = append(protectedNames, summary.Name)
			if len(protectedNames) == maxPBSTopProtectedWorkloads {
				break
			}
		}

		resource.PBS.ProtectedWorkloadCount = len(summaries)
		resource.PBS.ProtectedWorkloadTypes = uniqueStrings(types)
		sort.Strings(resource.PBS.ProtectedWorkloadTypes)
		resource.PBS.ProtectedWorkloadNames = protectedNames
		resource.PBS.ProtectedWorkloadSummary = summarizePBSProtectedWorkloads(resource.PBS.ProtectedWorkloadCount, resource.PBS.ProtectedWorkloadNames)
		resource.PBS.PostureSummary = summarizePBSPosture(resource.PBS)
	}
}

func buildPBSInstanceIndex(resources map[string]*Resource) map[string][]*Resource {
	index := make(map[string][]*Resource)
	for _, resource := range resources {
		if resource == nil || resource.Type != ResourceTypePBS || resource.PBS == nil {
			continue
		}
		aliases := uniqueStrings([]string{
			resource.Name,
			resource.PBS.InstanceID,
			resource.PBS.Hostname,
		})
		for _, alias := range aliases {
			key := normalizePBSLookupName(alias)
			if key == "" {
				continue
			}
			index[key] = append(index[key], resource)
		}
	}
	return index
}

func selectPBSInstanceResource(index map[string][]*Resource, instance string) *Resource {
	key := normalizePBSLookupName(instance)
	if key == "" {
		return nil
	}
	candidates := index[key]
	if len(candidates) != 1 {
		return nil
	}
	return candidates[0]
}

func pbsDatastoreAffectsPosture(resource *Resource) bool {
	if resource == nil {
		return false
	}
	if resource.Storage != nil && resource.Storage.Risk != nil && len(resource.Storage.Risk.Reasons) > 0 {
		return true
	}
	return len(resource.Incidents) > 0
}

func summarizePBSAffectedDatastores(names []string) string {
	if len(names) == 0 {
		return ""
	}
	label := "backup datastore"
	if len(names) != 1 {
		label = "backup datastores"
	}
	if len(names) == 1 {
		return "Affects 1 " + label + ": " + names[0]
	}
	return "Affects " + strconv.Itoa(len(names)) + " " + label + ": " + strings.Join(names, ", ")
}

func summarizePBSProtectedWorkloads(count int, names []string) string {
	if count <= 0 {
		return ""
	}
	label := "protected workload"
	if count != 1 {
		label = "protected workloads"
	}
	if len(names) == 0 {
		return "Puts backups for " + strconv.Itoa(count) + " " + label + " at risk"
	}
	limited := append([]string(nil), names...)
	if len(limited) > maxPBSTopProtectedWorkloads {
		limited = limited[:maxPBSTopProtectedWorkloads]
	}
	if remaining := count - len(limited); remaining > 0 {
		return "Puts backups for " + strconv.Itoa(count) + " " + label + " at risk: " + strings.Join(limited, ", ") + ", and " + strconv.Itoa(remaining) + " more"
	}
	return "Puts backups for " + strconv.Itoa(count) + " " + label + " at risk: " + strings.Join(limited, ", ")
}

func summarizePBSPosture(pbs *PBSData) string {
	if pbs == nil {
		return ""
	}
	switch {
	case pbs.AffectedDatastoreCount > 0 && pbs.ProtectedWorkloadCount > 0:
		return summarizePBSAffectedDatastores(pbs.AffectedDatastores) + ". " + summarizePBSProtectedWorkloads(pbs.ProtectedWorkloadCount, pbs.ProtectedWorkloadNames)
	case pbs.AffectedDatastoreCount > 0:
		return summarizePBSAffectedDatastores(pbs.AffectedDatastores)
	case pbs.ProtectedWorkloadCount > 0:
		return summarizePBSProtectedWorkloads(pbs.ProtectedWorkloadCount, pbs.ProtectedWorkloadNames)
	default:
		return ""
	}
}
