package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// isInherentlySharedStorageType returns true for storage types that are inherently
// cluster-wide/shared, even if the Proxmox API doesn't set the 'shared' flag.
// These storage backends are designed to be accessed from multiple nodes simultaneously.
func isInherentlySharedStorageType(storageType string) bool {
	switch strings.ToLower(strings.TrimSpace(storageType)) {
	case "rbd":
		// Ceph RBD - always cluster-wide in Ceph cluster
		return true
	case "cephfs":
		// CephFS - always cluster-wide
		return true
	case "pbs":
		// Proxmox Backup Server - always shared/remote
		return true
	case "glusterfs":
		// GlusterFS - always cluster-wide
		return true
	case "nfs":
		// NFS - network shared storage, typically accessible from all nodes
		return true
	case "cifs", "smb":
		// CIFS/SMB - network shared storage
		return true
	case "iscsi", "iscsidirect":
		// iSCSI - can be shared across nodes
		return true
	default:
		return false
	}
}

// storageContentQueryable reports whether it's safe to inspect the contents of a storage target.
// Proxmox returns storages that exist in the datacenter config even when the current node
// cannot access them. When Active==0 or the entry is disabled, querying /storage/<name>/content
// yields 500 errors that make Pulse think the node is unreachable. Guard calls so we only
// touch storages that are enabled, active on this node, and have a usable name.
func storageContentQueryable(storage proxmox.Storage) bool {
	if strings.TrimSpace(storage.Storage) == "" {
		return false
	}
	if storage.Enabled == 0 {
		return false
	}

	if storage.Active == 1 {
		return true
	}

	// Storage may report Active=0 for various reasons:
	// - PBS storages are accessed remotely via the backup proxy
	// - Shared storage (NFS/CIFS) may not be mounted on all nodes
	// - Storage may be configured for specific nodes only
	//
	// For any storage that can contain backups, we attempt to query it even when Active=0.
	// If the storage is truly unavailable, GetStorageContent will return an error which
	// is handled gracefully in pollStorageBackupsWithNodes (errors are logged and skipped).
	// This ensures datacenter backup tasks stored on shared/remote storage are visible.
	if strings.Contains(storage.Content, "backup") {
		return true
	}

	return false
}
