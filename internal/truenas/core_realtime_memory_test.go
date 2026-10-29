package truenas

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

// Exact memory fields from the historical CORE 13.0-U6.1 native event in
// https://github.com/rcourtman/Pulse/issues/2077#issuecomment-5784440810.
// Unrelated CPU/network/disk fields are omitted. The DDP server exchange below
// is a protocol fixture, not a capture or current appliance acceptance.
func coreRealtimeMemoryFixture(t *testing.T) []byte {
	t.Helper()
	raw, err := os.ReadFile("testdata/core13_realtime_memory.json")
	if err != nil {
		t.Fatal(err)
	}
	return raw
}

type coreMemoryScenario struct {
	plaintext, flood, noEndpoint, emptyReporting    bool
	entered                                         chan struct{}
	done                                            chan struct{}
	version                                         string
	frame                                           []byte
	rejectAuth, rejectSub, silent, malformed        bool
	restStatus                                      int
	restARC                                         int64
	sockets, authentications, subscriptions, closed atomic.Int32
}

func coreMemoryClient(t *testing.T, scenario *coreMemoryScenario, config ClientConfig) *Client {
	t.Helper()
	if scenario.version == "" {
		scenario.version = "TrueNAS-13.0-U6.1"
	}
	if scenario.frame == nil {
		scenario.frame = coreRealtimeMemoryFixture(t)
	}
	routes := defaultAPIResponses()
	upgrader := websocket.Upgrader{}
	scenario.done = make(chan struct{}, 16)
	scenario.entered = make(chan struct{}, 16)
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/current" {
			http.NotFound(w, r)
			return
		}
		if r.URL.Path == "/websocket" {
			scenario.sockets.Add(1)
			if scenario.noEndpoint {
				http.NotFound(w, r)
				return
			}
			conn, err := upgrader.Upgrade(w, r, nil)
			if err != nil {
				t.Errorf("upgrade: %v", err)
				return
			}
			defer func() { conn.Close(); scenario.closed.Add(1); scenario.done <- struct{}{} }()
			conn.SetReadDeadline(time.Now().Add(3 * time.Second))
			var connect struct {
				Msg, Version string
				Support      []string
			}
			if err := conn.ReadJSON(&connect); err != nil {
				return
			}
			if connect.Msg != "connect" || connect.Version != "1" || len(connect.Support) != 1 || connect.Support[0] != "1" {
				t.Error("incorrect CORE handshake")
				return
			}
			scenario.entered <- struct{}{}
			if scenario.flood {
				for i := 0; i < 70; i++ {
					if conn.WriteJSON(map[string]any{"msg": "unrelated"}) != nil {
						return
					}
				}
				return
			}
			if scenario.silent {
				var ignored any
				_ = conn.ReadJSON(&ignored)
				return
			}
			if scenario.malformed {
				_ = conn.WriteMessage(websocket.TextMessage, []byte(`{`))
				return
			}
			_ = conn.WriteJSON(map[string]any{"msg": "connected", "session": "synthetic"})
			var auth struct {
				Msg, ID, Method string
				Params          []string
			}
			if err := conn.ReadJSON(&auth); err != nil {
				return
			}
			scenario.authentications.Add(1)
			expectedMethod := "auth.login_with_api_key"
			expectedParams := []string{config.APIKey}
			if config.APIKey == "" {
				expectedMethod = "auth.login"
				expectedParams = []string{config.Username, config.Password}
			}
			if auth.Msg != "method" || auth.ID == "" || auth.Method != expectedMethod || fmt.Sprint(auth.Params) != fmt.Sprint(expectedParams) {
				t.Error("incorrect CORE authentication")
				return
			}
			// A native-looking frame before authentication must not be consumed.
			_ = conn.WriteMessage(websocket.TextMessage, scenario.frame)
			_ = conn.WriteJSON(map[string]any{"msg": "result", "id": "unrelated", "result": true})
			_ = conn.WriteJSON(map[string]any{"msg": "result", "id": auth.ID, "result": !scenario.rejectAuth})
			if scenario.rejectAuth {
				var ignored any
				_ = conn.ReadJSON(&ignored)
				return
			}
			var sub struct{ Msg, ID, Name string }
			if err := conn.ReadJSON(&sub); err != nil {
				return
			}
			scenario.subscriptions.Add(1)
			if sub.Msg != "sub" || sub.ID == "" || sub.Name != "reporting.realtime" {
				t.Error("incorrect CORE subscription")
				return
			}
			if scenario.rejectSub {
				_ = conn.WriteJSON(map[string]any{"msg": "nosub", "id": sub.ID, "error": map[string]any{"reason": "synthetic private refusal"}})
				return
			}
			_ = conn.WriteJSON(map[string]any{"msg": "ping", "id": "keepalive"})
			var pong struct{ Msg, ID string }
			if err := conn.ReadJSON(&pong); err != nil || pong.Msg != "pong" || pong.ID != "keepalive" {
				t.Error("missing DDP pong")
				return
			}
			_ = conn.WriteJSON(map[string]any{"msg": "ready", "subs": []string{sub.ID}})
			_ = conn.WriteJSON(map[string]any{"msg": "added", "collection": "reporting.realtime.other", "fields": map[string]any{"memory": map[string]any{"classes": map[string]any{"free": 1}}}})
			_ = conn.WriteMessage(websocket.TextMessage, scenario.frame)
			var ignored any
			_ = conn.ReadJSON(&ignored) // client must close its bounded stream
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/v2.0/") {
			if config.APIKey != "" && r.Header.Get("Authorization") != "Bearer "+config.APIKey {
				t.Error("REST API key changed")
			}
			if config.APIKey == "" {
				u, p, ok := r.BasicAuth()
				if !ok || u != config.Username || p != config.Password {
					t.Error("REST credentials changed")
				}
			}
		}
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v2.0/system/info":
			fmt.Fprintf(w, `{"hostname":"core-memory-fixture","version":%q,"physmem":34277445632,"cores":8}`, scenario.version)
		case "/api/v2.0/reporting/graphs":
			fmt.Fprint(w, `[]`)
		case "/api/v2.0/reporting/get_data":
			if scenario.emptyReporting {
				fmt.Fprint(w, `[]`)
				return
			}
			if scenario.restStatus != 0 {
				w.WriteHeader(scenario.restStatus)
				fmt.Fprint(w, `{"error":"synthetic"}`)
				return
			}
			now := time.Now().Unix()
			fmt.Fprintf(w, `[{"name":"cpu","legend":["usage"],"start":%d,"end":%d,"step":10,"data":[[12]]},{"name":"memory","legend":["free"],"start":%d,"end":%d,"step":10,"data":[[2178945024]]},{"name":"arcsize","legend":["arc_size"],"start":%d,"end":%d,"step":10,"data":[[%d]]}]`, now, now, now, now, now, now, scenario.restARC)
		default:
			if response, ok := routes[r.URL.Path]; ok {
				if response.status != 0 {
					w.WriteHeader(response.status)
				}
				fmt.Fprint(w, response.body)
			} else {
				http.NotFound(w, r)
			}
		}
	}))
	if scenario.plaintext {
		server.Start()
	} else {
		server.StartTLS()
	}
	t.Cleanup(server.Close)
	config.Host = server.URL
	if !scenario.plaintext {
		sum := sha256.Sum256(server.TLS.Certificates[0].Certificate[0])
		config.Fingerprint = hex.EncodeToString(sum[:])
		config.InsecureSkipVerify = true // fixture CA is private; exact fingerprint remains mandatory
	}
	client, err := NewClient(config)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(client.Close)
	return client
}

func TestCORE13RealtimeMemoryJourney(t *testing.T) {
	for _, password := range []bool{false, true} {
		name := "api_key"
		cfg := ClientConfig{APIKey: "synthetic-core-key", Timeout: time.Second}
		if password {
			name = "password"
			cfg.APIKey = ""
			cfg.Username = "synthetic-user"
			cfg.Password = "synthetic-password"
		}
		t.Run(name, func(t *testing.T) {
			scenario := &coreMemoryScenario{}
			client := coreMemoryClient(t, scenario, cfg)
			before := time.Now().UTC()
			snapshot, err := client.FetchSnapshot(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			system := snapshot.System
			if system.MemoryTotalBytes != 34277445632 || system.MemoryAvailableBytes != 1079291904 || system.ARCSizeBytes != 25668261968 || system.Telemetry == nil || !system.Telemetry.Memory {
				t.Fatalf("same-frame native memory lost: total=%d free=%d arc=%d availability=%+v", system.MemoryTotalBytes, system.MemoryAvailableBytes, system.ARCSizeBytes, system.Telemetry)
			}
			if system.CollectedAt.Before(before) || time.Since(system.CollectedAt) > time.Second || system.CPUPercent != 12 || len(snapshot.Disks) != 2 || len(snapshot.Pools) != 1 {
				t.Fatal("native memory lost freshness/independent REST inventory/readings")
			}
			records := truenasRecordsFromSnapshot(snapshot, "synthetic-core-connection", nil)
			var host *unifiedresources.Resource
			for _, rec := range records {
				if rec.Resource.Type == unifiedresources.ResourceTypeAgent {
					copy := rec.Resource
					host = &copy
				}
			}
			if host == nil || host.Agent == nil || host.Agent.Memory == nil || host.Metrics == nil || host.Metrics.Memory == nil || host.Metrics.Memory.Used == nil {
				t.Fatal("native memory did not reach canonical host")
			}
			wantUsed := int64(34277445632 - 1079291904 - 25668261968)
			if host.Agent.Memory.Cache != 25668261968 || host.Agent.Memory.Free != 1079291904 || host.Agent.Memory.Used != wantUsed || *host.Metrics.Memory.Used != wantUsed {
				t.Fatalf("canonical memory lost native meaning: %+v %+v", host.Agent.Memory, host.Metrics.Memory)
			}
			raw, err := json.Marshal(host)
			if err != nil {
				t.Fatal(err)
			}
			var roundtrip unifiedresources.Resource
			if err := json.Unmarshal(raw, &roundtrip); err != nil || roundtrip.Agent.Memory.Cache != 25668261968 {
				t.Fatal("serialized canonical ARC lost")
			}
			if !password {
				t.Logf("CORE_REALTIME_RESOURCE=%s", raw)
			}
			if scenario.sockets.Load() != 1 || scenario.authentications.Load() != 1 || scenario.subscriptions.Load() != 1 {
				t.Fatal("native memory required extra negotiation/retry")
			}
			status := client.TransportStatus()
			if status.Mode != TransportLegacyREST || status.ApplianceVersion != "TrueNAS-13.0-U6.1" {
				t.Fatalf("memory supplement changed transport: %+v", status)
			}
		})
	}
}

func TestCORE13RealtimeMemoryFieldBoundaries(t *testing.T) {
	const valid = `{"zfs":{"arc_size":8},"memory":{"classes":{"free":2,"arc":8}},"virtual_memory":{"total":16}}`
	stamp := time.Unix(1793233000, 0).UTC()
	for _, tc := range []struct {
		name, fields string
		valid        bool
		free, arc    int64
	}{
		{"complete", valid, true, 2, 8},
		{"native bytes", `{"zfs":{"arc_size":25668261968},"memory":{"classes":{"free":1079291904,"arc":25668261968}},"virtual_memory":{"total":34277445632}}`, true, 1079291904, 25668261968},
		{"classes ARC", `{"memory":{"classes":{"free":2,"arc":8}},"virtual_memory":{"total":16}}`, true, 2, 8},
		{"zfs ARC", `{"zfs":{"arc_size":8},"memory":{"classes":{"free":2}},"virtual_memory":{"total":16}}`, true, 2, 8},
		{"zero free", strings.Replace(valid, `"free":2`, `"free":0`, 1), true, 0, 8},
		{"zero ARC", strings.ReplaceAll(valid, `:8`, `:0`), true, 2, 0},
		{"all free", strings.ReplaceAll(strings.ReplaceAll(valid, `:8`, `:0`), `"free":2`, `"free":16`), true, 16, 0},
		{"no fields", `{}`, false, 0, 0},
		{"null fields", `null`, false, 0, 0},
		{"no free", strings.Replace(valid, `"free":2,`, "", 1), false, 0, 0},
		{"null free", strings.Replace(valid, `"free":2`, `"free":null`, 1), false, 0, 0},
		{"missing ARC", `{"memory":{"classes":{"free":2,"wire":12}},"virtual_memory":{"total":16,"wired":12}}`, false, 0, 0},
		{"null ARC", strings.ReplaceAll(valid, `:8`, `:null`), false, 0, 0},
		{"conflicting ARC", strings.Replace(valid, `"arc":8`, `"arc":7`, 1), false, 0, 0},
		{"missing total", strings.Replace(valid, `"total":16`, `"available":16`, 1), false, 0, 0},
		{"null total", strings.Replace(valid, `"total":16`, `"total":null`, 1), false, 0, 0},
		{"zero total", strings.Replace(valid, `"total":16`, `"total":0`, 1), false, 0, 0},
		{"negative total", strings.Replace(valid, `"total":16`, `"total":-1`, 1), false, 0, 0},
		{"negative free", strings.Replace(valid, `"free":2`, `"free":-1`, 1), false, 0, 0},
		{"negative ARC", strings.ReplaceAll(valid, `:8`, `: -8`), false, 0, 0},
		{"free above total", strings.Replace(valid, `"free":2`, `"free":17`, 1), false, 0, 0},
		{"ARC above headroom", strings.ReplaceAll(valid, `:8`, `:15`), false, 0, 0},
		{"overflow", strings.Replace(valid, `"total":16`, `"total":9223372036854775808`, 1), false, 0, 0},
		{"fraction", strings.Replace(valid, `"free":2`, `"free":2.5`, 1), false, 0, 0},
		{"numeric string", strings.Replace(valid, `"free":2`, `"free":"2"`, 1), false, 0, 0},
		{"nonfinite string", strings.Replace(valid, `"free":2`, `"free":"NaN"`, 1), false, 0, 0},
		{"wrong class shape", strings.Replace(valid, `"classes":{"free":2,"arc":8}`, `"classes":[]`, 1), false, 0, 0},
		{"available is not free", `{"zfs":{"arc_size":8},"virtual_memory":{"total":16,"available":2}}`, false, 0, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got := parseCORE13RealtimeMemory(json.RawMessage(tc.fields), stamp)
			if (got != nil) != tc.valid {
				t.Fatalf("validity=%v, want %v", got != nil, tc.valid)
			}
			if got != nil && (got.MemoryAvailableBytes != tc.free || got.ARCSizeBytes != tc.arc || !got.CollectedAt.Equal(stamp) || !got.Telemetry.Memory || got.Telemetry.CPU || got.Telemetry.DiskRead) {
				t.Fatalf("memory tuple lost meaning: %+v", got)
			}
		})
	}
}

func TestCORE13RealtimeMemoryFailureKeepsReporting(t *testing.T) {
	for _, tc := range []struct {
		name      string
		scenario  coreMemoryScenario
		auth, sub int32
	}{
		{"authentication refusal", coreMemoryScenario{rejectAuth: true}, 1, 0},
		{"subscription refusal", coreMemoryScenario{rejectSub: true}, 1, 1},
		{"silent peer", coreMemoryScenario{silent: true}, 0, 0},
		{"malformed handshake", coreMemoryScenario{malformed: true}, 0, 0},
		{"message flood", coreMemoryScenario{flood: true}, 0, 0},
		{"missing endpoint", coreMemoryScenario{noEndpoint: true}, 0, 0},
		{"incomplete native frame", coreMemoryScenario{frame: []byte(`{"msg":"added","collection":"reporting.realtime","fields":{"zfs":{"arc_size":25668261968}}}`)}, 1, 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := tc.scenario
			scenario.restARC = 8 << 30
			client := coreMemoryClient(t, &scenario, ClientConfig{APIKey: "synthetic-key", Username: "unused-user", Password: "unused-password", Timeout: 200 * time.Millisecond})
			start := time.Now()
			system, err := client.GetSystemTelemetry(context.Background())
			if err != nil || system == nil {
				t.Fatalf("optional native failure discarded reporting: %+v %v", system, err)
			}
			if time.Since(start) > time.Second {
				t.Fatal("optional stream exceeded operation budget")
			}
			if system.MemoryAvailableBytes != 2178945024 || system.ARCSizeBytes != 8<<30 || system.CPUPercent != 12 || !system.Telemetry.Memory {
				t.Fatalf("independent reporting readings lost: %+v", system)
			}
			if scenario.sockets.Load() != 1 || scenario.authentications.Load() != tc.auth || scenario.subscriptions.Load() != tc.sub {
				t.Fatal("native failure retried/switched authentication/consumed preauth event")
			}
			if !scenario.noEndpoint {
				select {
				case <-scenario.done:
				case <-time.After(time.Second):
					t.Fatal("failed native stream not closed")
				}
			}
			if status := client.TransportStatus(); status.Mode != TransportLegacyREST {
				t.Fatal("failure changed transport")
			}
		})
	}
}

func TestCORE13RealtimeMemoryGatesAndTrust(t *testing.T) {
	for _, tc := range []struct {
		name, version string
		plaintext     bool
	}{
		{"CORE12", "TrueNAS-12.0-U8", false},
		{"legacy SCALE", "TrueNAS-SCALE-24.10.2", false},
		{"unqualified CORE", "TrueNAS-13.1-U1", false},
		{"HTTP credentials never sent to DDP", "TrueNAS-13.0-U6.1", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			scenario := &coreMemoryScenario{version: tc.version, plaintext: tc.plaintext}
			client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
			if _, err := client.GetSystemTelemetry(context.Background()); err != nil {
				t.Fatal(err)
			}
			if scenario.sockets.Load() != 0 {
				t.Fatal("unqualified/plaintext native path attempted")
			}
		})
	}
	t.Run("pin rejection", func(t *testing.T) {
		scenario := &coreMemoryScenario{}
		client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
		if _, err := client.GetSystemInfo(context.Background()); err != nil {
			t.Fatal(err)
		}
		// The HTTP transport keeps its original good fixture pin. A different pin
		// on the independent DDP handshake must fail before sending credentials.
		client.config.Fingerprint = strings.Repeat("00", 32)
		system, err := client.GetSystemTelemetry(context.Background())
		if err != nil || system == nil || system.MemoryAvailableBytes != 2178945024 || scenario.sockets.Load() != 0 || scenario.authentications.Load() != 0 {
			t.Fatal("native TLS rejection bypassed trust or discarded REST")
		}
	})
	t.Run("REST authentication refusal", func(t *testing.T) {
		scenario := &coreMemoryScenario{restStatus: http.StatusUnauthorized}
		client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
		if _, err := client.GetSystemTelemetry(context.Background()); err == nil {
			t.Fatal("REST refusal lost")
		}
		if scenario.sockets.Load() != 0 {
			t.Fatal("REST refusal triggered another authentication route")
		}
	})
	t.Run("modern mode cannot enter legacy event path", func(t *testing.T) {
		scenario := &coreMemoryScenario{}
		client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key"})
		client.mode = TransportJSONRPC
		client.updateTransportStatus(func(s *TransportStatus) { s.ApplianceVersion = "TrueNAS-13.0-U6.1" })
		got, err := client.getCORE13RealtimeMemory(context.Background())
		if err != nil || got != nil || scenario.sockets.Load() != 0 {
			t.Fatal("modern transport downgraded to DDP")
		}
	})
}

func TestCORE13RealtimeMemoryDoesNotCacheOrMixFrames(t *testing.T) {
	scenario := &coreMemoryScenario{restARC: 8 << 30}
	client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
	for cycle, frame := range [][]byte{
		coreRealtimeMemoryFixture(t),
		[]byte(`{"msg":"added","collection":"reporting.realtime","fields":{"memory":{"classes":{"free":1}},"virtual_memory":{"total":16}}}`),
		[]byte(`{"msg":"added","collection":"reporting.realtime","fields":{"memory":{"classes":{"free":0,"arc":0}},"virtual_memory":{"total":16}}}`),
	} {
		scenario.frame = frame
		system, err := client.GetSystemTelemetry(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		select {
		case <-scenario.done:
		case <-time.After(time.Second):
			t.Fatal("native stream not closed")
		}
		switch cycle {
		case 0:
			if system.MemoryAvailableBytes != 1079291904 || system.ARCSizeBytes != 25668261968 {
				t.Fatal("native frame lost")
			}
		case 1:
			if system.MemoryAvailableBytes != 2178945024 || system.ARCSizeBytes != 8<<30 {
				t.Fatal("partial event mixed native/RRD/previous frame")
			}
		case 2:
			if system.MemoryTotalBytes != 16 || system.MemoryAvailableBytes != 0 || system.ARCSizeBytes != 0 || !system.Telemetry.Memory {
				t.Fatal("measured zeros lost or old ARC retained")
			}
			previous := SystemInfo{MemoryTotalBytes: 16, MemoryAvailableBytes: 2, ARCSizeBytes: 8}
			mergeSystemTelemetry(&previous, system)
			if previous.MemoryAvailableBytes != 0 || previous.ARCSizeBytes != 0 {
				t.Fatal("merge retained old cache over measured zero")
			}
		}
	}
	if scenario.sockets.Load() != 3 {
		t.Fatal("ordinary reads reused/retried native socket")
	}
}

func TestCORE13RealtimeMemoryCancellation(t *testing.T) {
	scenario := &coreMemoryScenario{silent: true}
	client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
	if _, err := client.GetSystemInfo(context.Background()); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.getCORE13RealtimeMemory(ctx); err == nil {
		t.Fatal("pre-cancelled native call succeeded")
	}
	if scenario.sockets.Load() != 0 {
		t.Fatal("pre-cancelled call performed I/O")
	}
	ctx, cancel = context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { _, err := client.getCORE13RealtimeMemory(ctx); done <- err }()
	// Observe entry into the fixture stream via the server's handshake read,
	// not a delay or a polling loop.
	select {
	case <-scenario.entered:
		cancel()
	case <-time.After(time.Second):
		t.Fatal("fixture handshake not reached")
	}
	select {
	case err := <-done:
		if err == nil {
			t.Fatal("cancelled native read succeeded")
		}
	case <-time.After(time.Second):
		t.Fatal("cancelled read did not stop")
	}
	select {
	case <-scenario.done:
	case <-time.After(time.Second):
		t.Fatal("cancelled native socket leaked")
	}
}

func TestCORE13RealtimeMemoryIndependentOfEmptyGraphs(t *testing.T) {
	for _, valid := range []bool{true, false} {
		t.Run(fmt.Sprint(valid), func(t *testing.T) {
			scenario := &coreMemoryScenario{emptyReporting: true}
			if !valid {
				scenario.frame = []byte(`{"msg":"added","collection":"reporting.realtime","fields":{}}`)
			}
			client := coreMemoryClient(t, scenario, ClientConfig{APIKey: "synthetic-key", Timeout: time.Second})
			system, err := client.GetSystemTelemetry(context.Background())
			if !valid {
				if err == nil || system != nil {
					t.Fatal("empty graph/event fabricated memory")
				}
				return
			}
			if err != nil || system == nil || !system.Telemetry.Memory || system.Telemetry.CPU || system.Telemetry.DiskRead || system.ARCSizeBytes != 25668261968 {
				t.Fatalf("native memory depended on unrelated graphs: %+v %v", system, err)
			}
		})
	}
}
