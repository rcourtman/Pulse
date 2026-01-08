package monitoring

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

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
