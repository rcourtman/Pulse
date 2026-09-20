package vmware

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The vSphere VI JSON API documents a `_typeName` discriminator on request data
// objects. A bare ManagedObjectReference is accepted without one, but the
// composite argument objects (PerfQuerySpec, PerfMetricId, EventFilterSpec and
// its nested EventFilterSpecByEntity) are rejected with HTTP 500 when the
// discriminator is absent. These tests pin the exact request bodies that
// vCenter 8.0.3 rejected for issue #2070.

func newVIJSONRequestClient(t *testing.T, handler http.Handler) (*Client, func()) {
	t.Helper()
	server := httptest.NewTLSServer(handler)
	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		server.Close()
		t.Fatalf("NewClient: %v", err)
	}
	return client, server.Close
}

func TestQueryPerfRequestCarriesTypeNameDiscriminators(t *testing.T) {
	var raw []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/9.0.0.0/PerformanceManager/PerformanceManager/QueryPerf", func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		writeJSON(w, []map[string]any{})
	})
	client, closeServer := newVIJSONRequestClient(t, mux)
	defer closeServer()

	_, err := client.queryPerfMetrics(
		context.Background(),
		"9.0.0.0",
		"vi-session",
		"PerformanceManager",
		"HostSystem",
		"host-101",
		20,
		[]viJSONPerfMetricID{{CounterID: 1, Instance: ""}},
	)
	if err != nil {
		t.Fatalf("queryPerfMetrics: %v", err)
	}

	body := string(raw)
	for _, want := range []string{
		`"_typeName":"PerfQuerySpec"`,
		`"_typeName":"ManagedObjectReference"`,
		`"_typeName":"PerfMetricId"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("QueryPerf request %s missing %s", body, want)
		}
	}
}

func TestQueryEventsRequestCarriesTypeNameDiscriminators(t *testing.T) {
	var raw []byte
	mux := http.NewServeMux()
	mux.HandleFunc("/sdk/vim25/9.0.0.0/EventManager/EventManager/QueryEvents", func(w http.ResponseWriter, r *http.Request) {
		raw, _ = io.ReadAll(r.Body)
		writeJSON(w, []map[string]any{})
	})
	client, closeServer := newVIJSONRequestClient(t, mux)
	defer closeServer()

	_, err := client.collectRecentEvents(context.Background(), "9.0.0.0", "vi-session", "EventManager", "Network", "network-14")
	if err != nil {
		t.Fatalf("collectRecentEvents: %v", err)
	}

	body := string(raw)
	for _, want := range []string{
		`"_typeName":"EventFilterSpec"`,
		`"_typeName":"EventFilterSpecByEntity"`,
		`"_typeName":"ManagedObjectReference"`,
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("QueryEvents request %s missing %s", body, want)
		}
	}
}
