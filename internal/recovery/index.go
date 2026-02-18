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

func isWorkloadSubjectType(subjectType string) bool {
	t := strings.ToLower(strings.TrimSpace(subjectType))
	if t == "" {
		return false
	}
	// Workload subjects (restore points for things you actually run).
	if strings.Contains(t, "proxmox-vm") || strings.Contains(t, "proxmox-lxc") {
		return true
	}
	if strings.Contains(t, "docker-container") || strings.Contains(t, "container") {
		return true
	}
	if strings.Contains(t, "k8s-pvc") || strings.Contains(t, "k8s-pod") {
		return true
	}
	return false
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

		if name != "" {
			return name
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
		return v
	}
	if p.Details != nil {
		if raw, ok := p.Details["vmid"]; ok {
			if v := anyToString(raw); v != "" {
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

	isWorkload := isWorkloadSubjectType(subjectType)
	// If the point is linked to a unified resource, treat it as a protected subject (workload)
	// even if the provider didn't include a subject type ref.
	if !isWorkload && strings.TrimSpace(p.SubjectResourceID) != "" {
		isWorkload = true
	}

	return PointIndex{
		SubjectLabel:    subjectLabel(p),
		SubjectType:     subjectType,
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
		IsWorkload:      idx.IsWorkload,
		ClusterLabel:    idx.ClusterLabel,
		NodeHostLabel:   idx.NodeHostLabel,
		NamespaceLabel:  idx.NamespaceLabel,
		EntityIDLabel:   idx.EntityIDLabel,
		RepositoryLabel: idx.RepositoryLabel,
		DetailsSummary:  idx.DetailsSummary,
	}
}
