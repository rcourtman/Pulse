package memory

import (
	"fmt"
	"strings"

	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
)

const (
	changeHistoryFileName      = "ai_changes.json"
	incidentHistoryFileName    = "ai_incidents.json"
	remediationHistoryFileName = "ai_remediations.json"
	incidentFileName           = incidentHistoryFileName
)

func normalizeOptionalMemoryDataDir(dir string) string {
	trimmed := strings.TrimSpace(dir)
	if trimmed == "" {
		return ""
	}

	normalized, err := securityutil.NormalizeStorageDir(trimmed)
	if err != nil {
		return ""
	}
	return normalized
}

func memoryPersistencePath(dataDir string, leaf string) (string, error) {
	normalizedDir := normalizeOptionalMemoryDataDir(dataDir)
	if normalizedDir == "" {
		return "", fmt.Errorf("memory data directory is required")
	}
	return securityutil.JoinStorageLeaf(normalizedDir, leaf)
}
