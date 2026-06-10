package memory

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
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

// loadMemoryHistory loads a JSON history slice from dataDir/fileName sorted
// by less, enforcing the shared 10 MiB on-disk safety cap. The boolean result
// is false when persistence is disabled (empty dataDir) or the file does not
// exist; label feeds the too-large error message.
func loadMemoryHistory[T any](dataDir, fileName, label string, less func(a, b T) bool) ([]T, bool, error) {
	if dataDir == "" {
		return nil, false, nil
	}

	path, err := memoryPersistencePath(dataDir, fileName)
	if err != nil {
		return nil, false, err
	}
	if st, err := os.Stat(path); err == nil {
		const maxOnDiskBytes = 10 << 20 // 10 MiB safety cap
		if st.Size() > maxOnDiskBytes {
			return nil, false, fmt.Errorf("%s file too large (%d bytes)", label, st.Size())
		}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, false, nil
		}
		return nil, false, err
	}

	var items []T
	if err := json.Unmarshal(data, &items); err != nil {
		return nil, false, err
	}

	sort.Slice(items, func(i, j int) bool {
		return less(items[i], items[j])
	})
	return items, true, nil
}
