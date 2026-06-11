package monitoring

import "github.com/rcourtman/pulse-go-rewrite/internal/models"

// splitReclaimableMemory splits the reclaimable page cache out of a memory
// snapshot whose Free currently holds "available" (Total - Used, with Used
// already excluding cache). trulyFree is the node/guest-reported free page
// count; when it is known and smaller than Free, the gap is reclaimable
// buff/cache and the snapshot becomes used | cache | free, which is what the
// frontend memory bar needs to reconcile Pulse's percentage with the
// cache-inclusive number the Proxmox UI shows.
func splitReclaimableMemory(memory *models.Memory, trulyFree uint64) {
	if memory == nil || trulyFree == 0 {
		return
	}
	if memory.Free <= 0 || int64(trulyFree) >= memory.Free {
		return
	}
	memory.Cache = memory.Free - int64(trulyFree)
	memory.Free = int64(trulyFree)
}
