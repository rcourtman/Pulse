package monitoring

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestPollPBSBackups_DropsStaleCacheOnTerminalDatastoreError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/admin/datastore/archive/groups") {
			http.Error(w, `{"errors":"datastore does not exist"}`, http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	m := &Monitor{state: models.NewState()}
	m.state.UpdatePBSBackups("pbs1", []models.PBSBackup{
		{
			ID:         "pbs-pbs1-archive--vm-100-1700000000",
			Instance:   "pbs1",
			Datastore:  "archive",
			Namespace:  "",
			BackupType: "vm",
			VMID:       "100",
			BackupTime: time.Unix(1700000000, 0),
		},
	})

	m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{
		{Name: "archive"},
	})

	snapshot := m.state.GetSnapshot()
	for _, backup := range snapshot.PBSBackups {
		if backup.Instance == "pbs1" {
			t.Fatalf("expected stale backups to be removed after terminal error, found: %+v", backup)
		}
	}
}

func TestPollPBSBackups_PreservesCacheOnTransientDatastoreError(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/admin/datastore/archive/groups") {
			http.Error(w, `{"errors":"temporary server issue"}`, http.StatusInternalServerError)
			return
		}
		http.NotFound(w, r)
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatalf("failed to create PBS client: %v", err)
	}

	m := &Monitor{state: models.NewState()}
	original := models.PBSBackup{
		ID:         "pbs-pbs1-archive--vm-100-1700000000",
		Instance:   "pbs1",
		Datastore:  "archive",
		Namespace:  "",
		BackupType: "vm",
		VMID:       "100",
		BackupTime: time.Unix(1700000000, 0),
	}
	m.state.UpdatePBSBackups("pbs1", []models.PBSBackup{original})

	m.pollPBSBackups(context.Background(), "pbs1", client, []models.PBSDatastore{
		{Name: "archive"},
	})

	snapshot := m.state.GetSnapshot()
	var found bool
	for _, backup := range snapshot.PBSBackups {
		if backup.Instance == "pbs1" && backup.ID == original.ID {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected cached backup to be preserved on transient error")
	}
}
