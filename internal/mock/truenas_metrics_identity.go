package mock

import (
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/truenas"
)

func trueNASDiskMetricsResourceID(disk truenas.Disk) string {
	resourceID := strings.TrimSpace(disk.Serial)
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.ID)
	}
	if resourceID == "" {
		resourceID = strings.TrimSpace(disk.Name)
	}
	return resourceID
}
