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
	return storage.Active == 1
}
