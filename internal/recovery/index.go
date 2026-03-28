package recovery

import (
	"fmt"
	"strings"
)

// PointIndex is the normalized, queryable index derived from a RecoveryPoint.
// It is persisted in the recovery_points sqlite table to enable efficient cross-platform UIs.
type PointIndex struct {
	SubjectLabel    string
	SubjectType     string
	ItemType        string
	IsWorkload      bool
	ClusterLabel    string
	NodeHostLabel   string
	NamespaceLabel  string
	EntityIDLabel   string
	RepositoryLabel string
	DetailsSummary  string
}

func trimString(v any) string {
	s, ok := v.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(s)
}

func anyToString(v any) string {
	switch t := v.(type) {
	case string:
		return strings.TrimSpace(t)
	case fmt.Stringer:
		return strings.TrimSpace(t.String())
	case int:
		return fmt.Sprintf("%d", t)
	case int64:
		return fmt.Sprintf("%d", t)
	case float64:
		// JSON numbers often come through as float64.
		if t == float64(int64(t)) {
			return fmt.Sprintf("%d", int64(t))
		}
		return fmt.Sprintf("%v", t)
	default:
		return ""
	}
}

func detailsString(p RecoveryPoint, key string) string {
	if p.Details == nil {
		return ""
	}
	return trimString(p.Details[key])
}

func isNumericOnlyLabel(value string) bool {
	value = strings.TrimSpace(value)
	if value == "" {
		return false
	}
	for _, r := range value {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func preferredProxmoxBackupCommentLabel(comment, entityID string) string {
	comment = strings.TrimSpace(comment)
	entityID = strings.TrimSpace(entityID)
	if comment == "" {
		return ""
	}
	if entityID != "" {
		if comment == entityID {
			return ""
		}
		parts := strings.Split(comment, ",")
		if len(parts) >= 2 {
			first := strings.TrimSpace(parts[0])
			last := strings.TrimSpace(parts[len(parts)-1])
			if last == entityID && first != "" && first != entityID {
				return first
			}
		}
	}
	return comment
}

func isOpaqueProxmoxTaskLabel(value string) bool {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return false
	}
	return strings.HasPrefix(value, "pve-task:") || strings.Contains(value, "upid:")
}

func preferredProxmoxTaskLabelFromDetails(p RecoveryPoint, currentLabel string) string {
	if strings.TrimSpace(string(p.Provider)) != string(ProviderProxmoxPVE) {
		return ""
	}
	currentLabel = strings.TrimSpace(currentLabel)
	if currentLabel != "" && !isOpaqueProxmoxTaskLabel(currentLabel) {
		return ""
	}

	taskType := strings.ToLower(strings.TrimSpace(detailsString(p, "type")))
	baseLabel := ""
	switch taskType {
	case "vzdump", "backup":
		baseLabel = "backup task"
	case "":
		baseLabel = "task"
	default:
		baseLabel = strings.ReplaceAll(taskType, "-", " ") + " task"
	}

	entityID := entityIDLabel(p)
	node := nodeHostLabel(p)
	if entityID != "" {
		if node != "" {
			return fmt.Sprintf("%s guest %s %s", node, entityID, baseLabel)
		}
		return fmt.Sprintf("guest %s %s", entityID, baseLabel)
	}
	if node != "" {
		return node + " " + baseLabel
	}
	if cluster := clusterLabel(p); cluster != "" {
		return cluster + " " + baseLabel
	}
	return "proxmox " + baseLabel
}

func preferredSubjectLabelFromDetails(p RecoveryPoint, currentLabel string) string {
	currentLabel = strings.TrimSpace(currentLabel)
	if !strings.HasPrefix(string(p.Provider), "proxmox-") {
		return ""
	}

	entityID := entityIDLabel(p)
	if currentLabel != "" && currentLabel != entityID && !isNumericOnlyLabel(currentLabel) {
		if candidate := preferredProxmoxTaskLabelFromDetails(p, currentLabel); candidate != "" {
			return candidate
		}
		return ""
	}

	for _, key := range []string{"comment", "notes"} {
		if candidate := preferredProxmoxBackupCommentLabel(detailsString(p, key), entityID); candidate != "" {
			return candidate
		}
	}

	if candidate := preferredProxmoxTaskLabelFromDetails(p, currentLabel); candidate != "" {
		return candidate
	}

	return ""
}

func NormalizeRecoveryItemType(value string) string {
	t := strings.ToLower(strings.TrimSpace(value))
	switch t {
	case "", "all":
		return ""
	case "proxmox-vm", "proxmox-vm-backup", "vm", "vm-backup":
		return "vm"
	case "proxmox-lxc", "lxc", "ct", "container", "system-container":
		return "system-container"
	case "docker-container", "docker", "app-container":
		return "app-container"
	case "oci-container":
		return "oci-container"
	case "k8s-pod", "pod":
		return "pod"
	case "k8s-pvc", "pvc":
		return "pvc"
	case "truenas-dataset", "dataset":
		return "dataset"
	case "velero-backup":
		return "velero-backup"
	case "proxmox-guest", "guest":
		return "guest"
	default:
		if strings.HasPrefix(t, "proxmox-") {
			return strings.TrimPrefix(t, "proxmox-")
		}
		if strings.HasPrefix(t, "truenas-") {
			return strings.TrimPrefix(t, "truenas-")
		}
		if strings.HasPrefix(t, "k8s-") {
			return strings.TrimPrefix(t, "k8s-")
		}
		return t
	}
}

func isWorkloadSubjectType(subjectType string) bool {
	switch NormalizeRecoveryItemType(subjectType) {
	case "vm", "system-container", "app-container", "oci-container", "pod", "pvc", "agent":
		return true
	default:
		return false
	}
}

func subjectLabel(p RecoveryPoint) string {
	rid := strings.TrimSpace(p.SubjectResourceID)
	ref := p.SubjectRef
	if ref != nil {
		ns := strings.TrimSpace(ref.Namespace)
		name := strings.TrimSpace(ref.Name)
		id := strings.TrimSpace(ref.ID)
		uid := strings.TrimSpace(ref.UID)

		// Kubernetes-style subjects should render as ns/name when possible.
		if strings.TrimSpace(string(p.Provider)) == string(ProviderKubernetes) {
			if ns != "" && name != "" {
				return ns + "/" + name
			}
			if name != "" {
				return name
			}
		}

		if candidate := preferredSubjectLabelFromDetails(p, name); candidate != "" {
			return candidate
		}
		if name != "" {
			return name
		}
		if candidate := preferredSubjectLabelFromDetails(p, id); candidate != "" {
			return candidate
		}
		if ns != "" {
			return ns
		}
		if id != "" {
			return id
		}
		if uid != "" {
			return uid
		}
	}
	if rid != "" {
		return rid
	}
	if candidate := preferredSubjectLabelFromDetails(p, ""); candidate != "" {
		return candidate
	}
	return strings.TrimSpace(p.ID)
}

func clusterLabel(p RecoveryPoint) string {
	if v := detailsString(p, "k8sClusterName"); v != "" {
		return v
	}
	// Proxmox: "instance" is the cluster/site name.
	if v := detailsString(p, "instance"); v != "" {
		return v
	}
	// TrueNAS: hostname.
	if v := detailsString(p, "hostname"); v != "" {
		return v
	}
	// Proxmox also stores instance name on the subject ref namespace.
	if strings.HasPrefix(string(p.Provider), "proxmox-") && p.SubjectRef != nil {
		if v := strings.TrimSpace(p.SubjectRef.Namespace); v != "" {
			return v
		}
	}
	return ""
}

func nodeHostLabel(p RecoveryPoint) string {
	// Proxmox: node is explicit.
	if v := detailsString(p, "node"); v != "" {
		return v
	}
	// Proxmox subject refs also store node in Class.
	if strings.HasPrefix(string(p.Provider), "proxmox-") && p.SubjectRef != nil {
		if v := strings.TrimSpace(p.SubjectRef.Class); v != "" {
			return v
		}
	}
	// TrueNAS: hostname.
	if v := detailsString(p, "hostname"); v != "" {
		return v
	}
	return ""
}

func namespaceLabel(p RecoveryPoint) string {
	// Kubernetes: the namespace is part of the subject identity.
	if strings.TrimSpace(string(p.Provider)) == string(ProviderKubernetes) {
		if p.SubjectRef != nil {
			if v := strings.TrimSpace(p.SubjectRef.Namespace); v != "" {
				return v
			}
		}
		if v := detailsString(p, "namespace"); v != "" {
			return v
		}
		return ""
	}
	// PBS: namespaces are a repository/backup property; default to root when unset.
	if strings.TrimSpace(string(p.Provider)) == string(ProviderProxmoxPBS) {
		if v := detailsString(p, "namespace"); v != "" {
			return v
		}
		if p.RepositoryRef != nil {
			if v := strings.TrimSpace(p.RepositoryRef.Class); v != "" {
				return v
			}
		}
		return "root"
	}
	return ""
}

func entityIDLabel(p RecoveryPoint) string {
	// Proxmox VMID (int or string depending on source).
	if v := detailsString(p, "vmid"); v != "" {
		if strings.TrimSpace(string(p.Provider)) == string(ProviderProxmoxPVE) && v == "0" {
			return ""
		}
		return v
	}
	if p.Details != nil {
		if raw, ok := p.Details["vmid"]; ok {
			if v := anyToString(raw); v != "" {
				if strings.TrimSpace(string(p.Provider)) == string(ProviderProxmoxPVE) && v == "0" {
					return ""
				}
				return v
			}
		}
	}
	// Kubernetes UID.
	if p.SubjectRef != nil {
		if v := strings.TrimSpace(p.SubjectRef.UID); v != "" {
			return v
		}
		if v := strings.TrimSpace(p.SubjectRef.ID); v != "" {
			return v
		}
	}
	return ""
}

func repositoryLabel(p RecoveryPoint) string {
	repo := p.RepositoryRef
	if repo != nil {
		repoName := strings.TrimSpace(repo.Name)
		repoType := strings.TrimSpace(repo.Type)
		repoClass := strings.TrimSpace(repo.Class)

		switch repoType {
		case "proxmox-pbs-datastore":
			if repoName == "" {
				return ""
			}
			if repoClass == "" {
				return repoName
			}
			return repoName + " (" + repoClass + ")"
		case "velero-backup-storage-location":
			if repoName == "" {
				return ""
			}
			return "Velero: " + repoName
		case "k8s-volume-snapshot-class":
			if repoName == "" {
				return ""
			}
			return "SnapshotClass: " + repoName
		default:
			if repoName != "" {
				return repoName
			}
		}
	}

	// Fallbacks for older/partial payloads.
	if v := detailsString(p, "storage"); v != "" {
		return v
	}
	if v := detailsString(p, "datastore"); v != "" {
		return v
	}
	if v := detailsString(p, "storageLocation"); v != "" {
		return "Velero: " + v
	}
	if v := detailsString(p, "targetDataset"); v != "" {
		return v
	}
	return ""
}

func detailsSummary(p RecoveryPoint) string {
	// Kubernetes Velero backups: render ns/name for uniqueness.
	veleroName := detailsString(p, "veleroName")
	if veleroName != "" {
		if ns := detailsString(p, "veleroNs"); ns != "" {
			return ns + "/" + veleroName
		}
		return veleroName
	}

	// Proxmox snapshots and Kubernetes snapshots both set snapshotName.
	if v := detailsString(p, "snapshotName"); v != "" {
		return v
	}
	// TrueNAS ZFS snapshot name.
	if v := detailsString(p, "snapshot"); v != "" {
		return v
	}
	if v := detailsString(p, "fullName"); v != "" {
		return v
	}

	// TrueNAS replication tasks.
	if v := detailsString(p, "taskName"); v != "" {
		if snap := detailsString(p, "lastSnapshot"); snap != "" {
			return v + " (" + snap + ")"
		}
		return v
	}

	// Proxmox backup artifacts.
	if v := detailsString(p, "volid"); v != "" {
		return v
	}
	if v := detailsString(p, "notes"); v != "" {
		return v
	}
	if v := detailsString(p, "comment"); v != "" {
		return v
	}

	// Status/phase information is useful for running/failed artifacts.
	if v := detailsString(p, "phase"); v != "" {
		return v
	}
	if v := detailsString(p, "error"); v != "" {
		return v
	}
	return ""
}

// DeriveIndex computes queryable, provider-neutral fields for a recovery point.
func DeriveIndex(p RecoveryPoint) PointIndex {
	subjectType := ""
	if p.SubjectRef != nil {
		subjectType = strings.TrimSpace(p.SubjectRef.Type)
	}
	subjectLabel := subjectLabel(p)
	itemType := NormalizeRecoveryItemType(subjectType)
	if itemType == "" && preferredProxmoxTaskLabelFromDetails(p, "") != "" {
		itemType = "task"
	}

	isWorkload := isWorkloadSubjectType(subjectType)
	// If the point is linked to a unified resource, treat it as a protected subject (workload)
	// even if the provider didn't include a subject type ref.
	if !isWorkload && strings.TrimSpace(p.SubjectResourceID) != "" {
		isWorkload = true
	}

	return PointIndex{
		SubjectLabel:    subjectLabel,
		SubjectType:     subjectType,
		ItemType:        itemType,
		IsWorkload:      isWorkload,
		ClusterLabel:    clusterLabel(p),
		NodeHostLabel:   nodeHostLabel(p),
		NamespaceLabel:  namespaceLabel(p),
		EntityIDLabel:   entityIDLabel(p),
		RepositoryLabel: repositoryLabel(p),
		DetailsSummary:  detailsSummary(p),
	}
}

// ToDisplay maps the stored PointIndex into the UI-oriented RecoveryPointDisplay payload.
func (idx PointIndex) ToDisplay() *RecoveryPointDisplay {
	if strings.TrimSpace(idx.SubjectLabel) == "" &&
		strings.TrimSpace(idx.SubjectType) == "" &&
		strings.TrimSpace(idx.ItemType) == "" &&
		strings.TrimSpace(idx.ClusterLabel) == "" &&
		strings.TrimSpace(idx.NodeHostLabel) == "" &&
		strings.TrimSpace(idx.NamespaceLabel) == "" &&
		strings.TrimSpace(idx.EntityIDLabel) == "" &&
		strings.TrimSpace(idx.RepositoryLabel) == "" &&
		strings.TrimSpace(idx.DetailsSummary) == "" &&
		!idx.IsWorkload {
		return nil
	}
	return &RecoveryPointDisplay{
		SubjectLabel:    idx.SubjectLabel,
		SubjectType:     idx.SubjectType,
		ItemType:        idx.ItemType,
		IsWorkload:      idx.IsWorkload,
		ClusterLabel:    idx.ClusterLabel,
		NodeHostLabel:   idx.NodeHostLabel,
		NamespaceLabel:  idx.NamespaceLabel,
		EntityIDLabel:   idx.EntityIDLabel,
		RepositoryLabel: idx.RepositoryLabel,
		DetailsSummary:  idx.DetailsSummary,
	}
}
