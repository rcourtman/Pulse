package monitoring

import (
	"encoding/json"

	"github.com/rcourtman/pulse-go-rewrite/pkg/proxmox"
)

// A storage name is cluster-wide configuration, not proof of shared bytes.
// Only PVE's shared flag permits one node's read to replace another's view.
type pveBackupStorageScope struct {
	storage string
	node    string // empty for shared storage (or an older, unscoped row)
}

func pveStorageBackupScope(storage proxmox.Storage, node string) pveBackupStorageScope {
	if storage.Shared > 0 {
		node = ""
	}
	return pveBackupStorageScope{storage: storage.Storage, node: node}
}

func pveStorageBackupID(instance string, scope pveBackupStorageScope, volid string) string {
	if scope.node == "" {
		// Shared views retain their existing identity regardless of poll order.
		return instance + "-" + volid
	}
	// JSON string tuples keep operator-chosen connection names and provider
	// identifiers unambiguous; delimiter concatenation can merge distinct hosts.
	identity, _ := json.Marshal([3]string{instance, scope.node, volid})
	return "pve-local:" + string(identity)
}
