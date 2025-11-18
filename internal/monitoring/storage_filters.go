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

	// PBS storages report Active=0 on every node because they are accessed remotely via the
	// backup proxy. We still need to inspect them so the UI can surface PBS-backed Proxmox
	// backups even when no dedicated PBS instance is configured inside Pulse.
	if strings.Contains(storage.Content, "backup") && storage.Type == "pbs" {
		return true
	}

	return false
}
