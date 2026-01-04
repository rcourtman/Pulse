package monitoring

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	"github.com/rcourtman/pulse-go-rewrite/pkg/pbs"
)

func TestMonitor_PollPBSInstance_AuthFailure(t *testing.T) {
	// Setup mock server that returns 401
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer server.Close()

	// Setup client
	client, err := pbs.NewClient(pbs.ClientConfig{
		Host:       server.URL,
		TokenName:  "root@pam!token",
		TokenValue: "secret",
	})
	if err != nil {
		t.Fatal(err)
	}

	// Setup monitor
	m := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-auth-fail", Host: server.URL, MonitorDatastores: true},
			},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
		// We need connectionHealth map initialized if SetConnectionHealth uses it?
		// models.NewState() handles it.
	}

	// Execute
	ctx := context.Background()
	m.pollPBSInstance(ctx, "pbs-auth-fail", client)

	// Verify
	// status should be offline
	// recordAuthFailure should have been called?
	// Monitor stores auth failures in memory map `authFailures`.
	// We can check `m.state.ConnectionHealth` for "pbs-pbs-auth-fail".

	// Verify manually using snapshot
	snapshot := m.state.GetSnapshot()
	if snapshot.ConnectionHealth["pbs-pbs-auth-fail"] {
		t.Error("Expected connection health to be false")
	}

	// We can't easily check authFailures map as it is private and no getter (except checking if it backs off?)
}

func TestMonitor_PollPBSInstance_DatastoreDetails(t *testing.T) {
	// Setup mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "/version") {
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": map[string]interface{}{"version": "2.0"},
			})
			return
		}
		if strings.Contains(r.URL.Path, "/nodes/localhost/status") {
			// Fail node status
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if strings.Contains(r.URL.Path, "/admin/datastore") && strings.HasSuffix(r.URL.Path, "/admin/datastore") {
			// GetDatastores list
			json.NewEncoder(w).Encode(map[string]interface{}{
				"data": []map[string]interface{}{
					{"store": "ds1", "comment": "comment1"}, // GetDatastores list returns small subset of fields
					{"store": "ds2", "comment": "comment2"},
				},
			})
			return
		}

		if strings.Contains(r.URL.Path, "/status") {
			// Datastore Status
			var data map[string]interface{}
			if strings.Contains(r.URL.Path, "ds1") {
				data = map[string]interface{}{"total": 100.0, "used": 50.0, "avail": 50.0}
			} else if strings.Contains(r.URL.Path, "ds2") {
				data = map[string]interface{}{"total-space": 200.0, "used-space": 100.0, "avail-space": 100.0, "deduplication-factor": 1.5}
			}
			json.NewEncoder(w).Encode(map[string]interface{}{"data": data})
			return
		}

		if strings.Contains(r.URL.Path, "/rrd") {
			// RRD
			json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
			return
		}

		if strings.Contains(r.URL.Path, "/namespace") {
			// ListNamespaces
			if strings.Contains(r.URL.Path, "ds1") {
				// DS 1: Fail namespaces
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			if strings.Contains(r.URL.Path, "ds2") {
				// DS 2: Varied namespaces
				json.NewEncoder(w).Encode(map[string]interface{}{
					"data": []map[string]interface{}{
						{"ns": "ns1"},
						{"path": "ns2"}, // alternate field
						{"name": "ns3"}, // alternate field
					},
				})
				return
			}
		}

		// Catch-all success for rrd/status calls from client.GetDatastores (it calls internal methods)
		// Wait, client.GetDatastores calls /api2/json/admin/datastore
		// client.ListNamespaces calls /api2/json/admin/datastore/{store}/namespace?
		// No, client.ListNamespaces: req to /admin/datastore/%s/namespace

		w.WriteHeader(http.StatusOK)
		json.NewEncoder(w).Encode(map[string]interface{}{"data": []interface{}{}})
	}))
	defer server.Close()

	client, err := pbs.NewClient(pbs.ClientConfig{Host: server.URL, TokenName: "root@pam!token", TokenValue: "val"})
	if err != nil {
		t.Fatalf("Failed to create client: %v", err)
	}

	m := &Monitor{
		config: &config.Config{
			PBSInstances: []config.PBSInstance{
				{Name: "pbs-details", Host: server.URL, MonitorDatastores: true},
			},
		},
		state:           models.NewState(),
		authFailures:    make(map[string]int),
		lastAuthAttempt: make(map[string]time.Time),
		pollStatusMap:   make(map[string]*pollStatus),
		circuitBreakers: make(map[string]*circuitBreaker),
	}

	m.pollPBSInstance(context.Background(), "pbs-details", client)

	// Verify State
	snapshot := m.state.GetSnapshot()
	var inst *models.PBSInstance
	for _, i := range snapshot.PBSInstances {
		if i.Name == "pbs-details" {
			copy := i
			inst = &copy
			break
		}
	}

	if inst == nil {
		t.Fatal("Instance not found")
	}

	if len(inst.Datastores) != 2 {
		t.Errorf("Expected 2 datastores, got %d", len(inst.Datastores))
	}

	// Check DS2 size calculation
	var ds2 *models.PBSDatastore
	for _, ds := range inst.Datastores {
		if ds.Name == "ds2" {
			copy := ds
			ds2 = &copy
			break
		}
	}
	if ds2 != nil {
		if ds2.Total != 200 {
			t.Errorf("Expected DS2 total 200, got %d", ds2.Total)
		}
		if len(ds2.Namespaces) != 4 {
			t.Errorf("Expected 4 namespaces for DS2, got %d", len(ds2.Namespaces))
		}
	} else {
		t.Error("DS2 not found")
	}
}
