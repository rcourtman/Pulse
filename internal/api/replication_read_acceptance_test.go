package api

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/rcourtman/pulse-go-rewrite/internal/mock"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
)

// Consume the monitor's actual emitted rows, exercising the unchanged handler,
// filters and JSON encoder. The connected driver binds all three source stages.
func TestReplicationReadHTTPFrames(t *testing.T) {
	input := os.Getenv("PULSE_REPLICATION_MONITOR_FRAMES")
	if input == "" {
		t.Skip("run scripts/replication_read_acceptance.py for connected producer/consumer proof")
	}
	data, err := os.ReadFile(input)
	if err != nil {
		t.Fatal(err)
	}
	var frames map[string][]models.ReplicationJob
	if err := json.Unmarshal(data, &frames); err != nil {
		t.Fatal(err)
	}
	previous := mock.IsMockEnabled()
	if err := mock.SetEnabled(false); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := mock.SetEnabled(previous); err != nil {
			t.Error(err)
		}
	})
	output := make(map[string]json.RawMessage)
	for _, phase := range []string{"initial", "short-inventory", "ambiguous-status", "recovered", "empty"} {
		rows, ok := frames[phase]
		if !ok {
			t.Fatalf("missing monitor frame: %s", phase)
		}
		state := models.NewState()
		state.ReplicationJobs = rows
		monitor := &monitoring.Monitor{}
		setUnexportedField(t, monitor, "state", state)
		router := &Router{monitor: monitor}
		for _, query := range []string{"?platform=proxmox-pve", "?instance=site-a", "?instance=site-b", "?guest=101", "?platform=unknown"} {
			request := httptest.NewRequest(http.MethodGet, "/api/replication/jobs"+query, nil)
			recorder := httptest.NewRecorder()
			router.handleListReplicationJobs(recorder, request)
			if recorder.Code != http.StatusOK {
				t.Fatalf("%s %s returned %d", phase, query, recorder.Code)
			}
			var response ReplicationJobsResponse
			if err := json.Unmarshal(recorder.Body.Bytes(), &response); err != nil {
				t.Fatal(err)
			}
			want := filterReplicationJobs(rows, request)
			expectedJSON, err := json.Marshal(want)
			if err != nil {
				t.Fatal(err)
			}
			actualJSON, err := json.Marshal(response.Data)
			if err != nil {
				t.Fatal(err)
			}
			if response.Meta.Total != len(want) || !bytes.Equal(actualJSON, expectedJSON) {
				t.Fatalf("%s %s changed producer fields or scope", phase, query)
			}
			if query == "?platform=proxmox-pve" {
				output[phase] = append(json.RawMessage(nil), recorder.Body.Bytes()...)
			}
		}
	}
	wire, err := json.Marshal(output)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("REPLICATION_HTTP_FRAMES=%s", wire)
}
