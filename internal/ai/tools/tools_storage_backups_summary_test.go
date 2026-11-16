package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	recoverymanager "github.com/rcourtman/pulse-go-rewrite/internal/recovery/manager"
	proxmoxmapper "github.com/rcourtman/pulse-go-rewrite/internal/recovery/mapper/proxmox"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// A model sees only the first 16,000 characters of a tool result. A month of
// one guest's backups on a full storage ran past that, so Patrol saw the
// newest nine, summed them and concluded the backups could not explain the
// fill. The summary of the backups read comes before the points, so the
// cut keeps it.
func TestPulseStorageBackupsSummaryOutlivesTheModelCut(t *testing.T) {
	ctx := context.Background()
	now := time.Now().UTC()
	snapshot := models.StateSnapshot{Storage: []models.Storage{{ID: "pve1-node1-local", Name: "local", Node: "node1", Instance: "pve1",
		Type: "dir", Status: "available", Content: "backup", Enabled: true, Active: true, Total: 32e9, Used: 29e9, LastSeen: now}}}
	registry := unifiedresources.NewRegistry(nil)
	registry.IngestSnapshot(snapshot)
	var storageID string
	for _, r := range registry.List() {
		if r.Type == unifiedresources.ResourceTypeStorage {
			storageID = r.ID
		}
	}
	require.NotEmpty(t, storageID)

	nextcloud := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "pve1", 111)
	gitea := unifiedresources.ProxmoxGuestCanonicalID(unifiedresources.ResourceTypeSystemContainer, "pve1", 112)
	var backups []models.StorageBackup
	const days = 30
	for day := 1; day <= days; day++ {
		backups = append(backups, models.StorageBackup{ID: fmt.Sprintf("nc-%02d", day), Storage: "local", Node: "node1", StorageNode: "node1",
			Instance: "pve1", Type: "lxc", VMID: 111, Time: now.AddDate(0, 0, -day), Size: 800e6})
	}
	backups = append(backups, models.StorageBackup{ID: "gitea-1", Storage: "local", Node: "node1", StorageNode: "node1",
		Instance: "pve1", Type: "lxc", VMID: 112, Time: now.Add(-time.Hour), Size: 4e6})
	points := proxmoxmapper.FromPVEStorageBackups(backups, map[string]proxmoxmapper.GuestInfo{
		"pve1|node1|111": {ResourceID: nextcloud, ResourceType: unifiedresources.ResourceTypeSystemContainer, Name: "nextcloud"},
		"pve1|node1|112": {ResourceID: gitea, ResourceType: unifiedresources.ResourceTypeSystemContainer, Name: "gitea"},
	})

	manager := recoverymanager.New(config.NewMultiTenantPersistence(t.TempDir()))
	store, err := manager.StoreForOrg("default")
	require.NoError(t, err)
	t.Cleanup(func() { _ = store.Close() })
	require.NoError(t, store.UpsertPoints(ctx, points))
	executor := NewPulseToolExecutor(ExecutorConfig{
		RecoveryPointsProvider:  NewRecoveryPointsToolAdapter(manager, "default"),
		UnifiedResourceProvider: unifiedresources.NewUnifiedAIAdapter(registry),
	})

	summaryOf := func(args map[string]interface{}, cut bool) BackupsSummary {
		t.Helper()
		result, err := executor.executeListBackups(ctx, args)
		require.NoError(t, err)
		require.False(t, result.IsError, result.Content)
		text := result.Content[0].Text
		if cut {
			// The cut the agentic loop applies before a model reads a result
			// (chat.MaxToolResultCharsLimit).
			modelText, truncation := agentcapabilities.ProviderToolResultModelContent(text, agentcapabilities.ProviderToolResultContextOptions{MaxModelContentChars: 16000})
			require.True(t, truncation.Applied, "the list should be long enough to be cut (%d chars)", len(text))
			text = modelText
		}
		start := strings.Index(text, `"summary":`)
		require.GreaterOrEqual(t, start, 0, "no summary in the result the model reads")
		var summary BackupsSummary
		require.NoError(t, json.NewDecoder(strings.NewReader(text[start+len(`"summary":`):])).Decode(&summary))
		return summary
	}

	summary := summaryOf(map[string]interface{}{"storage_id": storageID}, true)
	require.Equal(t, days+1, summary.Backups)
	require.Zero(t, summary.Summarized)
	require.Equal(t, int64(days*800e6+4e6), summary.SizeBytes)
	require.Equal(t, 2, summary.Subjects)
	top := summary.BySubject[0]
	require.Equal(t, "nextcloud", top.Subject)
	require.Equal(t, nextcloud, top.ResourceID)
	require.Equal(t, days, top.Backups)
	require.Equal(t, int64(days*800e6), top.SizeBytes)
	require.NotNil(t, top.Oldest)
	require.NotNil(t, top.Newest)
	require.WithinDuration(t, now.AddDate(0, 0, -days), *top.Oldest, time.Second)
	require.WithinDuration(t, now.AddDate(0, 0, -1), *top.Newest, time.Second)
	require.Equal(t, "gitea", summary.BySubject[1].Subject)

	// A shorter first page says how many it covers; a later page has none.
	first := summaryOf(map[string]interface{}{"storage_id": storageID, "limit": 5}, false)
	require.Equal(t, days+1, first.Backups)
	require.Equal(t, 5, first.Summarized)
	later, err := executor.executeListBackups(ctx, map[string]interface{}{"storage_id": storageID, "limit": 5, "offset": 10})
	require.NoError(t, err)
	require.NotContains(t, later.Content[0].Text, `"summary"`)
}

// A PBS guest's label comes from its backup comment, which can change between
// backups; the recovery subject key keeps it one subject. A backup with no
// recorded size is counted, not read as zero bytes, and a long label is cut.
func TestBackupsSummaryGroupsBySubjectKeyAndCountsUnknownSizes(t *testing.T) {
	at := time.Unix(1800000000, 0).UTC()
	points := proxmoxmapper.FromPBSBackups([]models.PBSBackup{
		{ID: "a", VMID: "120", BackupType: "vm", Instance: "pbs1", Datastore: "store1", BackupTime: at, Size: 100, Comment: "web"},
		{ID: "b", VMID: "120", BackupType: "vm", Instance: "pbs1", Datastore: "store1", BackupTime: at.Add(time.Hour), Size: 200, Comment: "web-renamed"},
		{ID: "c", VMID: "121", BackupType: "vm", Instance: "pbs1", Datastore: "store1", BackupTime: at, Size: 300, Comment: strings.Repeat("x", 500)},
	}, nil)
	require.Len(t, points, 3)
	for i := range points {
		require.Empty(t, points[i].SubjectResourceID)
		if strings.HasSuffix(points[i].ID, ":b") {
			points[i].SizeBytes = nil
		}
	}

	summary := summarizeBackups(points, 3)
	require.Equal(t, 2, summary.Subjects, "%+v", summary.BySubject)
	require.Equal(t, 1, summary.UnknownSize)
	require.Equal(t, int64(400), summary.SizeBytes)
	for _, s := range summary.BySubject {
		require.LessOrEqual(t, len([]rune(s.Subject)), backupSummaryMaxLabel)
		if s.Backups == 2 {
			require.Equal(t, 1, s.UnknownSize)
			require.Equal(t, int64(100), s.SizeBytes)
		}
	}
}
