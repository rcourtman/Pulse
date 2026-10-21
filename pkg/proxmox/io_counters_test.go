package proxmox

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestGetNodeStatusPermissionRefusalDoesNotInventMemory(t *testing.T) {
	var requests atomic.Int32
	var allowed atomic.Bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests.Add(1)
		if r.Method != http.MethodGet || r.URL.Path != "/api2/json/nodes/fixture/status" || r.Header.Get("Authorization") != "PVEAPIToken=fixture@pve!reader=fixture-token" {
			t.Error("ordinary read changed target, credential or method")
		}
		if !allowed.Load() {
			w.WriteHeader(http.StatusForbidden)
			fmt.Fprint(w, "private-fixture-response")
			return
		}
		fmt.Fprint(w, `{"data":{"cpu":0.25,"memory":{"total":256,"used":64,"free":192}}}`)
	}))
	defer server.Close()
	client, err := NewClient(ClientConfig{Host: server.URL, TokenName: "fixture@pve!reader", TokenValue: "fixture-token"})
	if err != nil {
		t.Fatal(err)
	}
	reading, err := client.GetNodeStatus(context.Background(), "fixture")
	if status, ok := APIErrorStatus(err); reading != nil || !ok || status != http.StatusForbidden || GuestAgentErrorReason(err) != "permission-denied" || requests.Load() != 1 {
		t.Fatalf("403 became a reading/retry or lost permission classification: reading=%+v err=%v requests=%d", reading, err, requests.Load())
	}
	allowed.Store(true) // Only this synthetic peer's ACL outcome changes.
	reading, err = client.GetNodeStatus(context.Background(), "fixture")
	if err != nil || reading == nil || reading.Memory == nil || reading.Memory.Total != 256 || reading.Memory.Used != 64 || reading.CPU != 0.25 || requests.Load() != 2 {
		t.Fatalf("same-credential ordinary recovery lost memory/CPU: reading=%+v err=%v requests=%d", reading, err, requests.Load())
	}
}

func TestClusterResourceCounterPresenceDistinguishesZeroNullAndMissing(t *testing.T) {
	var resource ClusterResource
	if err := json.Unmarshal([]byte(`{
		"type":"qemu",
		"diskread":0,
		"diskwrite":null,
		"netin":42
	}`), &resource); err != nil {
		t.Fatal(err)
	}

	presence := resource.IOCounters.Effective()
	if !presence.DiskRead || !presence.NetworkIn {
		t.Fatalf("explicit zero/value fields were not present: %+v", presence)
	}
	if presence.DiskWrite || presence.NetworkOut {
		t.Fatalf("null/missing fields were incorrectly present: %+v", presence)
	}
}

func TestGuestStatusTypesRetainCounterPresence(t *testing.T) {
	tests := []struct {
		name string
		read func() IOCounterPresence
	}{
		{
			name: "vm listing",
			read: func() IOCounterPresence {
				var value VM
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
		{
			name: "lxc status",
			read: func() IOCounterPresence {
				var value Container
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
		{
			name: "qemu status",
			read: func() IOCounterPresence {
				var value VMStatus
				_ = json.Unmarshal([]byte(`{"diskread":0}`), &value)
				return value.IOCounters
			},
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			presence := test.read().Effective()
			if !presence.DiskRead || presence.DiskWrite || presence.NetworkIn || presence.NetworkOut {
				t.Fatalf("presence = %+v, want only diskread", presence)
			}
		})
	}
}

func TestObservationStampUsesOneReceiptTimeForResponse(t *testing.T) {
	observedAt := time.Date(2026, time.July, 24, 8, 30, 0, 0, time.UTC)

	vms := []VM{{VMID: 100}, {VMID: 101}}
	stampVMObservation(vms, observedAt)
	for _, vm := range vms {
		if !vm.ObservedAt.Equal(observedAt) {
			t.Fatalf("VM %d observedAt = %v", vm.VMID, vm.ObservedAt)
		}
	}

	containers := []Container{{VMID: 200}, {VMID: 201}}
	stampContainerObservation(containers, observedAt)
	for _, container := range containers {
		if !container.ObservedAt.Equal(observedAt) {
			t.Fatalf("container %d observedAt = %v", container.VMID, container.ObservedAt)
		}
	}

	resources := []ClusterResource{{VMID: 300}, {VMID: 301}}
	stampClusterResourceObservation(resources, observedAt)
	for _, resource := range resources {
		if !resource.ObservedAt.Equal(observedAt) {
			t.Fatalf("resource %d observedAt = %v", resource.VMID, resource.ObservedAt)
		}
	}
}

func TestInternalCounterMetadataNeverChangesProxmoxWireShape(t *testing.T) {
	payload, err := json.Marshal(ClusterResource{
		VMID:       100,
		DiskRead:   0,
		IOCounters: IOCounterPresence{Explicit: true, DiskRead: true},
		ObservedAt: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(payload, &raw); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"ioCounters", "IOCounters", "observedAt", "ObservedAt"} {
		if _, ok := raw[key]; ok {
			t.Fatalf("internal counter metadata %q leaked into JSON", key)
		}
	}
}

func TestBackupLocksRetainCumulativeCounterPresence(t *testing.T) {
	payload := []byte(`{"lock":"backup","diskread":0,"diskwrite":null,"netin":42}`)
	var listing VM
	var resource ClusterResource
	var status VMStatus
	for _, dst := range []any{&listing, &resource, &status} {
		if err := json.Unmarshal(payload, dst); err != nil {
			t.Fatal(err)
		}
	}
	for name, value := range map[string]struct {
		lock     string
		presence IOCounterPresence
	}{"listing": {listing.Lock, listing.IOCounters}, "resource": {resource.Lock, resource.IOCounters}, "status": {status.Lock, status.IOCounters}} {
		t.Run(name, func(t *testing.T) {
			p := value.presence.Effective()
			if value.lock != "backup" || !p.DiskRead || !p.NetworkIn || p.DiskWrite || p.NetworkOut {
				t.Fatalf("lock decoding corrupted presence: %#v", value)
			}
		})
	}
}

func TestUnverifiedVMConfigPreservesStatusCounterObservations(t *testing.T) {
	var commands atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.HasSuffix(r.URL.Path, "/config"):
			w.Header().Set("Content-Length", "1000")
			fmt.Fprint(w, `{"data":{}}`)
		case strings.HasSuffix(r.URL.Path, "/status/current"):
			fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":0,"diskwrite":null,"netin":42}}`)
		default:
			commands.Add(1)
			backupAgentPayload(w, r)
		}
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if _, err := c.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "lock-unverified" {
		t.Errorf("incomplete config did not defer the guest command: %v", err)
	}
	status, err := c.GetVMStatus(context.Background(), "node", 105)
	if err != nil {
		t.Fatal(err)
	}
	p := status.IOCounters.Effective()
	if status.CPU != 0.25 || status.NetIn != 42 || status.DiskRead != 0 || status.ObservedAt.IsZero() || !p.DiskRead || !p.NetworkIn || p.DiskWrite || p.NetworkOut {
		t.Fatalf("unverified guest lock suppressed/changed current PVE counters: %+v", status)
	}
	if got := commands.Load(); got != 0 {
		t.Errorf("guest commands with incomplete config = %d, want zero", got)
	}
}

func TestGuestAgentTransportDeferralPreservesLiveCounterReceipts(t *testing.T) {
	for _, kind := range []string{"lost reply", "redirect", "server error", "gateway error"} {
		t.Run(kind, func(t *testing.T) {
			var commands, statusCalls atomic.Int32
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch {
				case strings.HasSuffix(r.URL.Path, "/config"):
					fmt.Fprint(w, `{"data":{}}`)
				case strings.HasSuffix(r.URL.Path, "/status/current"):
					statusCalls.Add(1)
					fmt.Fprint(w, `{"data":{"status":"running","cpu":0.25,"diskread":0,"diskwrite":null,"netin":42}}`)
				case strings.Contains(r.URL.Path, "/agent/"):
					commands.Add(1)
					if kind == "server error" || kind == "gateway error" {
						status := http.StatusInternalServerError
						if kind == "gateway error" {
							status = http.StatusBadGateway
						}
						w.WriteHeader(status)
						fmt.Fprint(w, "upstream unavailable")
						return
					}
					if kind == "redirect" {
						http.Redirect(w, r, "/unverified/agent", http.StatusTemporaryRedirect)
						return
					}
					conn, _, err := w.(http.Hijacker).Hijack()
					if err != nil {
						t.Error(err)
						return
					}
					conn.Close()
				default:
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			client := backupTestClient(t, server.URL)
			wantReason := "agent-timeout"
			if kind == "redirect" {
				wantReason = "agent-redirect"
			}
			if kind == "server error" || kind == "gateway error" {
				wantReason = "agent-completion-unverified"
			}
			if _, err := client.GetVMFSInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != wantReason {
				t.Fatalf("uncertain command not deferred: %v", err)
			}
			// The QGA uncertainty is not missing or zero CPU/I/O evidence. Ordinary
			// status still has its own completed response and observation receipt.
			before := time.Now()
			status, err := client.GetVMStatus(context.Background(), "node", 105)
			after := time.Now()
			if err != nil {
				t.Fatal(err)
			}
			presence := status.IOCounters.Effective()
			if status.CPU != 0.25 || status.DiskRead != 0 || status.NetIn != 42 || !presence.DiskRead || !presence.NetworkIn || presence.DiskWrite || presence.NetworkOut || status.ObservedAt.Before(before) || status.ObservedAt.After(after) {
				t.Fatalf("guest transport deferral changed current counter presence/receipt: %+v", status)
			}
			if _, err := client.GetVMAgentInfo(context.Background(), "node", 105); GuestAgentDeferredReason(err) != "agent-cooldown" {
				t.Fatalf("status response erased command uncertainty: %v", err)
			}
			if commands.Load() != 1 || statusCalls.Load() != 1 {
				t.Fatalf("guest/status calls = %d/%d, want 1/1", commands.Load(), statusCalls.Load())
			}
		})
	}
}

// The same ordinary status decoder also preserves memory availability presence.
func TestVMStatusMemoryAvailabilityPresence(t *testing.T) {
	t.Run("round-trip", testVMMemInfoAvailabilityRoundTrip)
	t.Run("invalid-numbers", testVMMemInfoAvailabilityRejectsInvalidNumbers)
	t.Run("reused-decode", testVMMemInfoAvailabilityPresenceDoesNotLeakAcrossDecode)
}

// Complete enumeration still owns the original counter-presence and receipt-
// time semantics; scoped-empty and older LXC encodings remain compatible.
func TestGuestInventoryAcceptedScopedAndExtendedResponses(t *testing.T) {
	for _, kind := range []string{"cluster", "qemu", "lxc"} {
		for _, body := range []string{
			`{"data":[]}`, `{"data":[],"errors":null,"total":0}`,
			`{"data":[{"vmid":105,"type":"qemu","node":"member","name":"fixture","status":"stopped","pool":"department","cpu":0,"maxcpu":4,"cpus":4,"mem":0,"maxmem":1024,"disk":0,"maxdisk":4096,"diskread":0,"netin":null,"uptime":0,"template":1,"tags":"ops","lock":"backup","agent":1,"vendor-metadata":{"version":1}}],"digest":"fixture-metadata"}`,
		} {
			t.Run(kind+"/"+body, func(t *testing.T) {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { fmt.Fprint(w, body) }))
				defer server.Close()
				c := backupTestClient(t, server.URL)
				defer c.httpClient.CloseIdleConnections()
				switch kind {
				case "cluster":
					got, err := c.GetClusterResources(context.Background(), "vm")
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(body, `"vmid"`) && (len(got) != 1 || got[0].VMID != 105 || got[0].Node != "member" || got[0].MaxCPU != 4 || got[0].Pool != "department" || got[0].Lock != "backup" || got[0].Template != 1 || got[0].IOCounters.DiskRead != true || got[0].IOCounters.NetworkIn || got[0].ObservedAt.IsZero()) {
						t.Fatalf("supported cluster fields changed: %+v", got)
					}
				case "qemu":
					got, err := c.GetVMs(context.Background(), "member")
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(body, `"vmid"`) && (len(got) != 1 || got[0].VMID != 105 || got[0].CPUs != 4 || got[0].Agent != 1 || got[0].Lock != "backup" || got[0].Pool != "department" || got[0].IOCounters.NetworkIn || got[0].ObservedAt.IsZero()) {
						t.Fatalf("supported VM fields changed: %+v", got)
					}
				case "lxc":
					got, err := c.GetContainers(context.Background(), "member")
					if err != nil {
						t.Fatal(err)
					}
					if strings.Contains(body, `"vmid"`) && (len(got) != 1 || int(got[0].VMID) != 105 || int(got[0].CPUs) != 4 || got[0].Pool != "department" || got[0].IOCounters.NetworkIn || got[0].ObservedAt.IsZero()) {
						t.Fatalf("supported container fields changed: %+v", got)
					}
				}
			})
		}
	}
	t.Run("lxc-string-identifiers", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"data":[{"vmid":"205","cpus":"2","name":"container"}]}`)
		}))
		defer server.Close()
		got, err := backupTestClient(t, server.URL).GetContainers(context.Background(), "member")
		if err != nil || len(got) != 1 || int(got[0].VMID) != 205 || int(got[0].CPUs) != 2 {
			t.Fatalf("string LXC identifiers lost: %+v %v", got, err)
		}
	})
	t.Run("all-resource-types", func(t *testing.T) {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			fmt.Fprint(w, `{"data":[{"id":"node/member","type":"node","node":"member"},{"id":"storage/member/local","type":"storage","node":"member"},{"id":"pool/department","type":"pool"},{"id":"future/provider","type":"future-type"}]}`)
		}))
		defer server.Close()
		got, err := backupTestClient(t, server.URL).GetClusterResources(context.Background(), "")
		if err != nil || len(got) != 4 {
			t.Fatalf("non-guest resource compatibility lost: %+v %v", got, err)
		}
	})
}

// Task discovery is independent of current guest telemetry. Refusing a partial
// task set must not withdraw a valid zero counter or change its observation.
func TestIncompleteBackupTasksLeaveIndependentGuestCountersUsable(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api2/json/nodes":
			fmt.Fprint(w, `{"data":[{"node":"node","status":"online"}]}`)
		case "/api2/json/nodes/node/tasks":
			fmt.Fprint(w, `{"data":[]}trailing`)
		case "/api2/json/nodes/node/qemu/100/status/current":
			fmt.Fprint(w, `{"data":{"status":"running","diskread":0,"netin":42,"lock":"backup"}}`)
		default:
			t.Errorf("unexpected operation %s", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer server.Close()
	c := backupTestClient(t, server.URL)
	if tasks, err := c.GetBackupTasks(context.Background()); err == nil || tasks != nil {
		t.Fatalf("partial tasks accepted: %+v %v", tasks, err)
	}
	status, err := c.GetVMStatus(context.Background(), "node", 100)
	if err != nil {
		t.Fatal(err)
	}
	p := status.IOCounters.Effective()
	if status.Lock != "backup" || status.DiskRead != 0 || status.NetIn != 42 || !p.DiskRead || !p.NetworkIn || p.DiskWrite || status.ObservedAt.IsZero() {
		t.Fatalf("task failure contaminated independent status: %+v", status)
	}
}

func TestLXCDiskPresenceDistinguishesZeroNullAndMissing(t *testing.T) {
	for _, tc := range []struct {
		name, wire string
		known      bool
	}{
		{"zero", `{"maxdisk":1000,"disk":0}`, true},
		{"used", `{"maxdisk":1000,"disk":980}`, true},
		{"missing-used", `{"maxdisk":1000}`, false},
		{"null-used", `{"maxdisk":1000,"disk":null}`, false},
		{"missing-total", `{"disk":0}`, false},
		{"null-total", `{"maxdisk":null,"disk":0}`, false},
		{"missing-both", `{}`, false},
		{"untrusted-presence", `{"maxdisk":1000,"DiskUsage":{"Explicit":false,"Used":true,"Total":true}}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var ct Container
			var res ClusterResource
			if err := json.Unmarshal([]byte(tc.wire), &ct); err != nil {
				t.Fatal(err)
			}
			if err := json.Unmarshal([]byte(tc.wire), &res); err != nil {
				t.Fatal(err)
			}
			if !ct.DiskUsage.Explicit || ct.DiskUsage.Known() != tc.known || ct.DiskUsage != res.DiskUsage {
				t.Fatalf("lost HTTP field presence: container=%+v cluster=%+v", ct.DiskUsage, res.DiskUsage)
			}
			encoded, err := json.Marshal(ct)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "DiskUsage") {
				t.Fatal("internal presence escaped into API JSON")
			}
		})
	}
	if !(DiskUsagePresence{}).Known() {
		t.Fatal("legacy in-process producer lost compatibility")
	}
}
