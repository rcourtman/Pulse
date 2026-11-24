package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
)

// Each body places the chosen time behind a pointer, in a slice element and
// in a nested struct, which is where the reports carry their times.
const (
	dockerReportWithTimes = `{
		"agent": {"id": "agent-times", "version": "6.1.1", "intervalSeconds": 30},
		"host": {"hostname": "docker-times", "machineId": "machine-times"},
		"timestamp": "2026-01-02T03:04:05Z",
		"containers": [{"id": "container-1", "name": "job", "image": "busybox", "state": "exited", "status": "Exited (0)", "finishedAt": %[1]q}],
		"services": [{"id": "service-1", "name": "web", "desiredTasks": 3, "runningTasks": 0, "updateStatus": {"state": "updating", "completedAt": %[1]q}}]
	}`
	kubernetesReportWithTimes = `{
		"agent": {"id": "agent-times", "version": "6.1.1", "intervalSeconds": 30},
		"cluster": {"id": "cluster-times", "name": "cluster-times"},
		"timestamp": "2026-01-02T03:04:05Z",
		"pods": [{"uid": "pod-1", "name": "pod-1", "namespace": "default", "phase": "Running", "createdAt": %[1]q}],
		"recovery": {"volumeSnapshots": [{"uid": "snapshot-1", "name": "snapshot-1", "namespace": "default", "readyToUse": true, "creationTime": %[1]q}]}
	}`
	hostReportWithTimes = `{
		"agent": {"id": "agent-times", "version": "6.1.1", "update": {"state": "idle", "lastCheckedAt": %[1]q}},
		"host": {"id": "machine-times", "hostname": "host-times.local", "platform": "linux", "packageUpdates": {"checkedAt": %[1]q}},
		"timestamp": "2026-01-02T03:04:05Z"
	}`
)

func TestAgentReportHandlersRefuseTimesTheServerCannotEncode(t *testing.T) {
	handlers := []struct {
		name   string
		path   string
		body   string
		handle func(t *testing.T) (http.HandlerFunc, *monitoring.Monitor)
	}{
		{
			name: "docker agent", path: "/api/agents/docker/report", body: dockerReportWithTimes,
			handle: func(t *testing.T) (http.HandlerFunc, *monitoring.Monitor) {
				handler, monitor := newDockerAgentHandlers(t, nil)
				return handler.HandleReport, monitor
			},
		},
		{
			name: "kubernetes agent", path: "/api/agents/kubernetes/report", body: kubernetesReportWithTimes,
			handle: func(t *testing.T) (http.HandlerFunc, *monitoring.Monitor) {
				handler, monitor := newKubernetesAgentHandlers(t, nil)
				return handler.HandleReport, monitor
			},
		},
		{
			name: "unified agent", path: "/api/agents/agent/report", body: hostReportWithTimes,
			handle: func(t *testing.T) (http.HandlerFunc, *monitoring.Monitor) {
				handler, monitor := newUnifiedAgentHandlers(t, nil)
				return handler.HandleReport, monitor
			},
		},
	}
	times := []struct {
		name    string
		value   string
		refused bool
	}{
		{name: "ordinary time with a zone offset", value: "2026-01-02T03:04:05+05:30"},
		{name: "zero time", value: "0001-01-01T00:00:00Z"},
		{name: "last second of year 9999 in UTC", value: "9999-12-31T23:59:59Z"},
		{name: "year 10000 in UTC", value: "9999-12-31T23:59:59-23:59", refused: true},
		{name: "year -1 in UTC", value: "0000-01-01T00:00:00+23:59", refused: true},
		{name: "zone offset of +24:00", value: "2026-01-02T03:04:05+24:00", refused: true},
		{name: "zone offset of -24:00", value: "2026-01-02T03:04:05-24:00", refused: true},
	}

	for _, handler := range handlers {
		for _, tc := range times {
			t.Run(handler.name+" "+tc.name, func(t *testing.T) {
				handle, monitor := handler.handle(t)
				rec := invokeAgentReportHandler(handle, handler.path, []byte(fmt.Sprintf(handler.body, tc.value)), "")

				if tc.refused {
					assertAgentReportError(t, rec, http.StatusBadRequest, "invalid_report", "", 0)
				} else if rec.Code != http.StatusOK {
					t.Fatalf("status = %d, want 200: %s", rec.Code, rec.Body.String())
				}
				if _, err := json.Marshal(monitor.BuildFrontendState()); err != nil {
					t.Fatalf("state no longer encodes after the report: %v", err)
				}
			})
		}
	}
}

func TestAgentReportTimeRefusalNamesTheTime(t *testing.T) {
	handler, _ := newDockerAgentHandlers(t, nil)
	rec := postDockerReport(handler, []byte(fmt.Sprintf(dockerReportWithTimes, "9999-12-31T23:59:59-23:59")), "")
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400: %s", rec.Code, rec.Body.String())
	}
	var response struct {
		Error string `json:"error"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&response); err != nil {
		t.Fatalf("decode error response: %v", err)
	}
	if !strings.Contains(response.Error, "9999-12-31T23:59:59-23:59") {
		t.Fatalf("error = %q, want it to name the refused time", response.Error)
	}
}

func TestValidateAgentReportTimesFindsEveryPlacement(t *testing.T) {
	var refused time.Time
	if err := json.Unmarshal([]byte(`"9999-12-31T23:59:59-23:59"`), &refused); err != nil {
		t.Fatalf("decode fixture time: %v", err)
	}
	accepted := time.Date(2026, time.January, 2, 3, 4, 5, 0, time.UTC)

	type nested struct {
		unexported time.Time
		ByName     map[string]agentsdocker.Task
		Fixed      [1]*time.Time
		Labels     map[string]string
	}
	tests := []struct {
		name   string
		report func(at time.Time) any
	}{
		{name: "value", report: func(at time.Time) any { return &agentsdocker.Report{Timestamp: at} }},
		{name: "pointer", report: func(at time.Time) any {
			return &agentshost.Report{Agent: agentshost.AgentInfo{Update: &agentshost.UpdateStatus{LastSuccessAt: &at}}}
		}},
		{name: "slice element", report: func(at time.Time) any {
			return &agentsk8s.Report{Events: []agentsk8s.Event{{}, {LastSeen: &at}}}
		}},
		{name: "nested pointer in slice element", report: func(at time.Time) any {
			return &agentsdocker.Report{Services: []agentsdocker.Service{{UpdateStatus: &agentsdocker.ServiceUpdate{CompletedAt: &at}}}}
		}},
		{name: "map value", report: func(at time.Time) any {
			return &nested{ByName: map[string]agentsdocker.Task{"task": {CreatedAt: at}}}
		}},
		{name: "array element", report: func(at time.Time) any { return &nested{Fixed: [1]*time.Time{&at}} }},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if err := validateAgentReportTimes(tc.report(accepted)); err != nil {
				t.Fatalf("ordinary time refused: %v", err)
			}
			if err := validateAgentReportTimes(tc.report(refused)); err == nil {
				t.Fatal("time that is year 10000 in UTC was accepted")
			}
		})
	}

	t.Run("unset and unexported", func(t *testing.T) {
		if err := validateAgentReportTimes(&nested{unexported: refused, Labels: map[string]string{"a": "b"}}); err != nil {
			t.Fatalf("report without decoded times refused: %v", err)
		}
		if err := validateAgentReportTimes(&agentsk8s.Report{}); err != nil {
			t.Fatalf("empty report refused: %v", err)
		}
	})
}

// The limits are time.Time.MarshalJSON's own, so the two must keep agreeing.
func TestValidateAgentReportTimeAgreesWithTheEncoder(t *testing.T) {
	times := []time.Time{
		{},
		time.Date(2026, time.January, 2, 3, 4, 5, 6, time.Local),
		time.Date(10000, time.January, 1, 0, 0, 0, 0, time.FixedZone("", 60*60)),
		time.Date(-1, time.December, 31, 23, 0, 0, 0, time.FixedZone("", -60*60)),
	}
	for _, value := range []string{
		"2026-01-02T03:04:05.123456789+05:30",
		"0000-01-01T00:00:00Z",
		"0000-01-01T00:00:00-00:01",
		"0000-01-01T00:00:00+00:01",
		"9999-12-31T23:59:59.999999999Z",
		"9999-12-31T23:59:59+00:01",
		"9999-12-31T23:59:59-00:01",
		"2026-01-02T03:04:05+23:59",
		"2026-01-02T03:04:05-23:59",
		"2026-01-02T03:04:05+24:00",
		"2026-01-02T03:04:05-24:00",
		"0001-01-02T00:00:00+24:00",
	} {
		var decoded time.Time
		if err := json.Unmarshal([]byte(`"`+value+`"`), &decoded); err != nil {
			t.Fatalf("decode %s: %v", value, err)
		}
		times = append(times, decoded)
	}

	for _, at := range times {
		_, asReceived := at.MarshalJSON()
		_, inUTC := at.UTC().MarshalJSON()
		encodes := asReceived == nil && inUTC == nil
		if err := validateAgentReportTime(at); (err == nil) != encodes {
			t.Errorf("%s: validation error = %v, encoder as received = %v, in UTC = %v",
				at.Format(time.RFC3339Nano), err, asReceived, inUTC)
		}
	}
}
