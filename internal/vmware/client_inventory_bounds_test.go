package vmware

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func emptyObjectList(entries int) string {
	return "[" + strings.TrimSuffix(strings.Repeat("{},", entries), ",") + "]"
}

func TestJSONListExceeds(t *testing.T) {
	for _, tc := range []struct {
		name  string
		body  string
		limit int
		want  bool
	}{
		{name: "empty body", body: "", limit: 2, want: false},
		{name: "null", body: "null", limit: 2, want: false},
		{name: "empty list", body: "[]", limit: 2, want: false},
		{name: "below the limit", body: `[{"vm":"vm-1"}]`, limit: 2, want: false},
		{name: "at the limit", body: `[{"vm":"vm-1"}, {"vm":"vm-2"}]`, limit: 2, want: false},
		{name: "above the limit", body: `[{"vm":"vm-1"},{"vm":"vm-2"},{}]`, limit: 2, want: true},
		{name: "nested lists count once", body: `[{"nics":[1,2,3]},[4,5,6]]`, limit: 2, want: false},
		{name: "not a list", body: `{"value":[{},{},{}]}`, limit: 2, want: false},
		{name: "malformed", body: `[{},{`, limit: 2, want: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := jsonListExceeds([]byte(tc.body), tc.limit); got != tc.want {
				t.Fatalf("jsonListExceeds(%q, %d) = %v, want %v", tc.body, tc.limit, got, tc.want)
			}
		})
	}
}

func TestDecodeVIJSONBodyRefusesOversizedNestedArrays(t *testing.T) {
	nestedAtLimit := `{"triggeredAlarmState":` + emptyObjectList(inventoryListEntryLimit) + `}`
	var okPayload struct {
		TriggeredAlarmState []struct{} `json:"triggeredAlarmState"`
	}
	if err := decodeVIJSONBody([]byte(nestedAtLimit), "vm alarms", &okPayload); err != nil {
		t.Fatalf("nested array at limit should decode: %v", err)
	}
	if len(okPayload.TriggeredAlarmState) != inventoryListEntryLimit {
		t.Fatalf("decoded len = %d, want %d", len(okPayload.TriggeredAlarmState), inventoryListEntryLimit)
	}

	nestedOverLimit := `{"triggeredAlarmState":` + emptyObjectList(inventoryListEntryLimit+1) + `}`
	var badPayload struct {
		TriggeredAlarmState []struct{} `json:"triggeredAlarmState"`
	}
	err := decodeVIJSONBody([]byte(nestedOverLimit), "vm alarms", &badPayload)
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T (%v)", err, err)
	}
	want := fmt.Sprintf("vm alarms response lists more than %d entries", inventoryListEntryLimit)
	if connectionErr.Category != "endpoint" || !strings.Contains(connectionErr.Message, want) {
		t.Fatalf("unexpected error: %+v", connectionErr)
	}
}

func TestClientRefusesInventoryListBeyondEntryLimit(t *testing.T) {
	var vmEntries atomic.Int32
	vmEntries.Store(inventoryListEntryLimit)
	mux := http.NewServeMux()
	mux.HandleFunc("/api/session", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`"automation-session"`))
	})
	mux.HandleFunc("/api/vcenter/vm", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(emptyObjectList(int(vmEntries.Load()))))
	})
	for _, path := range []string{"/api/vcenter/host", "/api/vcenter/datastore", "/api/vcenter/network"} {
		mux.HandleFunc(path, func(w http.ResponseWriter, _ *http.Request) {
			_, _ = w.Write([]byte("[]"))
		})
	}
	server := httptest.NewTLSServer(mux)
	defer server.Close()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	inventory, _, err := client.collectInventoryBaseWithSession(context.Background())
	if err != nil {
		t.Fatalf("a list at the entry limit should be collected: %v", err)
	}
	if len(inventory.VMs) != inventoryListEntryLimit {
		t.Fatalf("vms = %d, want %d", len(inventory.VMs), inventoryListEntryLimit)
	}

	// A few bytes per entry stay far below the response size cap, so only the
	// entry limit stands between this list and its decoded form.
	vmEntries.Store(inventoryListEntryLimit + 1)
	_, _, err = client.collectInventoryBaseWithSession(context.Background())
	connectionErr, ok := err.(*ConnectionError)
	if !ok {
		t.Fatalf("expected ConnectionError, got %T (%v)", err, err)
	}
	want := fmt.Sprintf("vm inventory response lists more than %d entries", inventoryListEntryLimit)
	if connectionErr.Category != "endpoint" || !strings.Contains(connectionErr.Message, want) {
		t.Fatalf("unexpected error: %+v", connectionErr)
	}

	// A skipped surface is only a name directory and still must not fail the poll.
	client.scope.skipVMs = true
	inventory, _, err = client.collectInventoryBaseWithSession(context.Background())
	if err != nil {
		t.Fatalf("an oversized list of a skipped surface should not fail: %v", err)
	}
	if len(inventory.VMs) != 0 {
		t.Fatalf("vms = %d, want none from a refused list", len(inventory.VMs))
	}
}

func TestClientEnrichInventorySnapshotBoundsGoroutinesByConcurrency(t *testing.T) {
	const networks = 1000

	var inFlight atomic.Int32
	release := make(chan struct{})
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		inFlight.Add(1)
		<-release
		http.NotFound(w, r)
	}))
	defer server.Close()
	var releaseOnce sync.Once
	releaseReads := func() { releaseOnce.Do(func() { close(release) }) }
	defer releaseReads()

	client, err := NewClient(ClientConfig{
		Host:               server.URL,
		Username:           "admin",
		Password:           "secret",
		InsecureSkipVerify: true,
		Timeout:            5 * time.Second,
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	snapshot := &InventorySnapshot{Networks: make([]InventoryNetwork, networks)}
	for i := range snapshot.Networks {
		snapshot.Networks[i].Network = fmt.Sprintf("network-%d", i)
	}

	before := runtime.NumGoroutine()
	done := make(chan error, 1)
	go func() {
		_, err := client.enrichInventorySnapshot(context.Background(), "automation-session", "9.0.0.0", "vi-session", "PerfMgr", "EventManager", perfCounterCatalog{}, snapshot)
		done <- err
	}()

	// Hold every admitted read open, then give an unbounded fan-out time to
	// start the rest of its goroutines.
	deadline := time.Now().Add(5 * time.Second)
	for inFlight.Load() < vmwareSignalEnrichmentConcurrency {
		if time.Now().After(deadline) {
			t.Fatalf("only %d reads in flight, want %d", inFlight.Load(), vmwareSignalEnrichmentConcurrency)
		}
		time.Sleep(5 * time.Millisecond)
	}
	time.Sleep(100 * time.Millisecond)
	if got := int(inFlight.Load()); got != vmwareSignalEnrichmentConcurrency {
		t.Fatalf("%d reads in flight, want %d", got, vmwareSignalEnrichmentConcurrency)
	}
	if grown := runtime.NumGoroutine() - before; grown > networks/4 {
		t.Fatalf("%d goroutines started for %d networks with %d reads admitted", grown, networks, vmwareSignalEnrichmentConcurrency)
	}

	releaseReads()
	if err := <-done; err != nil {
		t.Fatalf("enrichInventorySnapshot: %v", err)
	}
}
