package truenas

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/gorilla/websocket"
)

type protocolFixtureReply struct {
	result        any
	err           *trueNASRPCError
	notifications []protocolFixtureNotification
	close         bool
}

type protocolFixtureNotification struct {
	method string
	params any
}

type protocolFixture struct {
	server       *httptest.Server
	sessions     atomic.Int32
	restRequests atomic.Int32
}

func newProtocolFixture(
	t *testing.T,
	handle func(session int, request trueNASRPCRequest) protocolFixtureReply,
	handleREST http.HandlerFunc,
) *protocolFixture {
	t.Helper()

	fixture := &protocolFixture{}
	upgrader := websocket.Upgrader{}
	fixture.server = httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/current" || !websocket.IsWebSocketUpgrade(request) {
			fixture.restRequests.Add(1)
			if handleREST == nil {
				http.NotFound(writer, request)
				return
			}
			handleREST(writer, request)
			return
		}

		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Errorf("upgrade protocol fixture websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		session := int(fixture.sessions.Add(1))

		for {
			var rpcRequest trueNASRPCRequest
			if err := conn.ReadJSON(&rpcRequest); err != nil {
				return
			}
			reply := handle(session, rpcRequest)
			if reply.close {
				return
			}
			response := trueNASRPCResponse{
				JSONRPC: "2.0",
				ID:      rpcRequest.ID,
				Error:   reply.err,
			}
			if reply.err == nil {
				raw, err := json.Marshal(reply.result)
				if err != nil {
					t.Errorf("marshal protocol fixture result: %v", err)
					return
				}
				response.Result = raw
			}
			if err := conn.WriteJSON(response); err != nil {
				return
			}
			for _, notification := range reply.notifications {
				params, err := json.Marshal(notification.params)
				if err != nil {
					t.Errorf("marshal protocol fixture notification: %v", err)
					return
				}
				if err := conn.WriteJSON(trueNASRPCResponse{
					JSONRPC: "2.0",
					Method:  notification.method,
					Params:  params,
				}); err != nil {
					return
				}
			}
		}
	}))
	t.Cleanup(fixture.server.Close)
	return fixture
}

func protocolFixtureClient(t *testing.T, serverURL string, config ClientConfig) *Client {
	t.Helper()
	config.InsecureSkipVerify = true
	client := mustClientForServer(t, serverURL, config)
	t.Cleanup(client.Close)
	return client
}

func TestJSONRPCTransportUsesLoginExForReadonlyAPIKey(t *testing.T) {
	var sawLogin atomic.Bool
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_ex":
			params, ok := request.Params.([]any)
			if !ok || len(params) != 1 {
				t.Errorf("auth.login_ex params = %#v", request.Params)
				return protocolFixtureReply{close: true}
			}
			login, ok := params[0].(map[string]any)
			if !ok {
				t.Errorf("auth.login_ex login payload = %#v", params[0])
				return protocolFixtureReply{close: true}
			}
			if login["mechanism"] != "API_KEY_PLAIN" || login["username"] != "pulse-readonly" || login["api_key"] != "readonly-key" {
				t.Errorf("auth.login_ex login payload = %#v", login)
			}
			sawLogin.Store(true)
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		case "system.info":
			return protocolFixtureReply{result: map[string]any{
				"hostname":       "nas-readonly",
				"version":        "TrueNAS-SCALE-25.10.4",
				"system_serial":  "READONLY-1",
				"uptime_seconds": 42,
				"physical_cores": 8,
				"physmem":        16 * 1024 * 1024 * 1024,
			}}
		default:
			t.Errorf("unexpected rpc method %q", request.Method)
			return protocolFixtureReply{close: true}
		}
	}, nil)

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{
		APIKey:   "readonly-key",
		Username: "pulse-readonly",
	})
	system, err := client.GetSystemInfo(context.Background())
	if err != nil {
		t.Fatalf("GetSystemInfo() error = %v", err)
	}
	if !sawLogin.Load() || system.Hostname != "nas-readonly" || system.Version != "TrueNAS-SCALE-25.10.4" {
		t.Fatalf("unexpected readonly system result: %+v", system)
	}
	if fixture.restRequests.Load() != 0 {
		t.Fatalf("modern JSON-RPC session made %d REST requests", fixture.restRequests.Load())
	}
	status := client.TransportStatus()
	if status.Mode != TransportJSONRPC || !status.Connected || status.AuthMechanism != "api-key-plain" || !status.TLS {
		t.Fatalf("unexpected transport status: %+v", status)
	}
}

func TestJSONRPCLegacyAPIKeyLoginRemovalRequestsOwnerUsername(t *testing.T) {
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		if request.Method != "auth.login_with_api_key" {
			t.Errorf("unexpected rpc method %q", request.Method)
			return protocolFixtureReply{close: true}
		}
		return protocolFixtureReply{err: &trueNASRPCError{
			Code:    -32601,
			Message: "Method not found",
		}}
	}, nil)

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "upgrade-key"})
	_, err := client.GetSystemInfo(context.Background())
	if err == nil || !strings.Contains(err.Error(), "requires the API key owner username") {
		t.Fatalf("GetSystemInfo() error = %v, want upgrade guidance", err)
	}
	if fixture.restRequests.Load() != 0 {
		t.Fatalf("removed login method made %d REST downgrade requests", fixture.restRequests.Load())
	}
}

func TestJSONRPCPermissionFailureDoesNotDowngradeToREST(t *testing.T) {
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_ex":
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		case "system.info":
			data, _ := json.Marshal(map[string]any{
				"reason":  "Not authorized",
				"errname": "EACCES",
			})
			return protocolFixtureReply{err: &trueNASRPCError{
				Code:    -32001,
				Message: "Method call error",
				Data:    data,
			}}
		default:
			return protocolFixtureReply{close: true}
		}
	}, func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("REST must not be called after an authoritative JSON-RPC permission error")
		http.Error(writer, "unexpected", http.StatusInternalServerError)
	})

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{
		APIKey:   "readonly-key",
		Username: "pulse-readonly",
	})
	_, err := client.GetSystemInfo(context.Background())
	if err == nil || !strings.Contains(err.Error(), "EACCES") {
		t.Fatalf("GetSystemInfo() error = %v, want structured permission failure", err)
	}
	if fixture.restRequests.Load() != 0 {
		t.Fatalf("permission failure made %d REST requests", fixture.restRequests.Load())
	}
	status := client.TransportStatus()
	if status.Mode != TransportJSONRPC || !status.Connected {
		t.Fatalf("permission failure changed transport authority: %+v", status)
	}
}

func TestJSONRPCReadonlyActionDenialIsAuthoritativeAndNotRetried(t *testing.T) {
	var actionCalls atomic.Int32
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_ex":
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		case "app.start":
			actionCalls.Add(1)
			data, _ := json.Marshal(map[string]any{
				"reason":  "Not authorized",
				"errname": "EACCES",
			})
			return protocolFixtureReply{err: &trueNASRPCError{
				Code:    -32001,
				Message: "Method call error",
				Data:    data,
			}}
		default:
			return protocolFixtureReply{close: true}
		}
	}, func(writer http.ResponseWriter, _ *http.Request) {
		t.Error("REST must not be called after an authoritative action denial")
		http.Error(writer, "unexpected", http.StatusInternalServerError)
	})

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{
		APIKey:   "readonly-key",
		Username: "pulse-readonly",
	})
	err := client.StartApp(context.Background(), "nextcloud")
	var rpcErr *RPCError
	if !errors.As(err, &rpcErr) || rpcErr.Errname != "EACCES" || rpcErr.Method != "app.start" {
		t.Fatalf("StartApp() error = %v, want structured app.start permission denial", err)
	}
	if actionCalls.Load() != 1 || fixture.sessions.Load() != 1 || fixture.restRequests.Load() != 0 {
		t.Fatalf("actionCalls=%d sessions=%d restRequests=%d, denial was retried or downgraded",
			actionCalls.Load(), fixture.sessions.Load(), fixture.restRequests.Load())
	}
}

func TestTransportNegotiationAllowsOnlyRecognizedLegacyREST(t *testing.T) {
	t.Run("legacy scale", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/api/current":
				http.NotFound(writer, request)
			case "/api/v2.0/system/info":
				_, _ = writer.Write([]byte(`{"hostname":"legacy","version":"TrueNAS-SCALE-24.10.2","uptime_seconds":1}`))
			case "/api/v2.0/pool":
				_, _ = writer.Write([]byte(`[{"id":1,"name":"tank","status":"ONLINE"}]`))
			default:
				http.NotFound(writer, request)
			}
		}))
		t.Cleanup(server.Close)

		client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "legacy-key"})
		pools, err := client.GetPools(context.Background())
		if err != nil {
			t.Fatalf("GetPools() legacy error = %v", err)
		}
		if len(pools) != 1 || pools[0].Name != "tank" {
			t.Fatalf("unexpected legacy pools: %+v", pools)
		}
		if status := client.TransportStatus(); status.Mode != TransportLegacyREST || status.ApplianceVersion != "TrueNAS-SCALE-24.10.2" {
			t.Fatalf("unexpected legacy status: %+v", status)
		}
	})

	t.Run("legacy core", func(t *testing.T) {
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/api/current":
				http.NotFound(writer, request)
			case "/api/v2.0/system/info":
				_, _ = writer.Write([]byte(`{"hostname":"core","version":"TrueNAS-13.0-U6.4","uptime_seconds":1}`))
			case "/api/v2.0/pool":
				_, _ = writer.Write([]byte(`[{"id":1,"name":"core-tank","status":"ONLINE"}]`))
			default:
				http.NotFound(writer, request)
			}
		}))
		t.Cleanup(server.Close)

		client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "legacy-core-key"})
		pools, err := client.GetPools(context.Background())
		if err != nil {
			t.Fatalf("GetPools() CORE error = %v", err)
		}
		if len(pools) != 1 || pools[0].Name != "core-tank" {
			t.Fatalf("unexpected CORE pools: %+v", pools)
		}
		if status := client.TransportStatus(); status.Mode != TransportLegacyREST ||
			status.ApplianceVersion != "TrueNAS-13.0-U6.4" {
			t.Fatalf("unexpected CORE status: %+v", status)
		}
	})

	t.Run("current scale fail closed", func(t *testing.T) {
		var poolRESTCalls atomic.Int32
		server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
			switch request.URL.Path {
			case "/api/current":
				http.NotFound(writer, request)
			case "/api/v2.0/system/info":
				_, _ = writer.Write([]byte(`{"hostname":"current","version":"TrueNAS-SCALE-25.10.4","uptime_seconds":1}`))
			case "/api/v2.0/pool":
				poolRESTCalls.Add(1)
				_, _ = writer.Write([]byte(`[]`))
			default:
				http.NotFound(writer, request)
			}
		}))
		t.Cleanup(server.Close)

		client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "current-key"})
		_, err := client.GetPools(context.Background())
		if err == nil || !strings.Contains(err.Error(), "refusing REST downgrade") {
			t.Fatalf("GetPools() error = %v, want fail-closed downgrade error", err)
		}
		if poolRESTCalls.Load() != 0 {
			t.Fatalf("current SCALE used pool REST %d times", poolRESTCalls.Load())
		}
	})
}

func TestJSONRPCCredentialsRequireTLSWithoutSendingAuthentication(t *testing.T) {
	var rpcRequests atomic.Int32
	var restRequests atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/current" || !websocket.IsWebSocketUpgrade(request) {
			restRequests.Add(1)
			http.NotFound(writer, request)
			return
		}
		conn, err := upgrader.Upgrade(writer, request, nil)
		if err != nil {
			t.Errorf("upgrade plaintext websocket: %v", err)
			return
		}
		defer func() { _ = conn.Close() }()
		var rpcRequest trueNASRPCRequest
		if conn.ReadJSON(&rpcRequest) == nil {
			rpcRequests.Add(1)
		}
	}))
	t.Cleanup(server.Close)

	client := mustClientForServer(t, server.URL, ClientConfig{
		APIKey:   "must-not-cross-plaintext",
		Username: "pulse-readonly",
	})
	client.allowInsecureRPC = false
	t.Cleanup(client.Close)
	_, err := client.GetPools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "credentials require TLS") {
		t.Fatalf("GetPools() error = %v, want TLS requirement", err)
	}
	if rpcRequests.Load() != 0 || restRequests.Load() != 0 {
		t.Fatalf("plaintext negotiation sent rpc=%d REST=%d requests after websocket upgrade",
			rpcRequests.Load(), restRequests.Load())
	}
}

func TestTransportNegotiationIsIsolatedAcrossAppliances(t *testing.T) {
	modern := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_ex":
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		case "pool.query":
			return protocolFixtureReply{result: []map[string]any{{"id": 1, "name": "modern", "status": "ONLINE"}}}
		default:
			return protocolFixtureReply{close: true}
		}
	}, nil)
	legacyServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/current":
			http.NotFound(writer, request)
		case "/api/v2.0/system/info":
			_, _ = writer.Write([]byte(`{"hostname":"legacy","version":"TrueNAS-SCALE-24.10.2"}`))
		case "/api/v2.0/pool":
			_, _ = writer.Write([]byte(`[{"id":2,"name":"legacy","status":"ONLINE"}]`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(legacyServer.Close)

	modernClient := protocolFixtureClient(t, modern.server.URL, ClientConfig{
		APIKey:   "modern-key",
		Username: "pulse-readonly",
	})
	legacyClient := protocolFixtureClient(t, legacyServer.URL, ClientConfig{APIKey: "legacy-key"})

	modernPools, modernErr := modernClient.GetPools(context.Background())
	legacyPools, legacyErr := legacyClient.GetPools(context.Background())
	if modernErr != nil || legacyErr != nil {
		t.Fatalf("multi-appliance polls failed: modern=%v legacy=%v", modernErr, legacyErr)
	}
	if len(modernPools) != 1 || modernPools[0].Name != "modern" ||
		len(legacyPools) != 1 || legacyPools[0].Name != "legacy" {
		t.Fatalf("unexpected isolated inventories: modern=%+v legacy=%+v", modernPools, legacyPools)
	}
	modernStatus := modernClient.TransportStatus()
	legacyStatus := legacyClient.TransportStatus()
	if modernStatus.Mode != TransportJSONRPC || modernStatus.Endpoint == legacyStatus.Endpoint ||
		legacyStatus.Mode != TransportLegacyREST {
		t.Fatalf("transport decisions leaked across clients: modern=%+v legacy=%+v", modernStatus, legacyStatus)
	}
}

func TestNegotiatedJSONRPCTransportNeverRenegotiatesToREST(t *testing.T) {
	var websocketAttempts atomic.Int32
	var restRequests atomic.Int32
	upgrader := websocket.Upgrader{}
	server := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch request.URL.Path {
		case "/api/current":
			if websocketAttempts.Add(1) > 1 {
				http.NotFound(writer, request)
				return
			}
			conn, err := upgrader.Upgrade(writer, request, nil)
			if err != nil {
				t.Errorf("upgrade initial websocket: %v", err)
				return
			}
			defer func() { _ = conn.Close() }()
			for {
				var rpcRequest trueNASRPCRequest
				if err := conn.ReadJSON(&rpcRequest); err != nil {
					return
				}
				var result any
				switch rpcRequest.Method {
				case "auth.login_with_api_key":
					result = true
				case "pool.query":
					result = []map[string]any{{"id": 1, "name": "modern", "status": "ONLINE"}}
				default:
					t.Errorf("unexpected rpc method %q", rpcRequest.Method)
					return
				}
				raw, err := json.Marshal(result)
				if err != nil {
					t.Errorf("marshal rpc result: %v", err)
					return
				}
				if err := conn.WriteJSON(trueNASRPCResponse{
					JSONRPC: "2.0",
					ID:      rpcRequest.ID,
					Result:  raw,
				}); err != nil {
					return
				}
			}
		case "/api/v2.0/system/info", "/api/v2.0/pool":
			restRequests.Add(1)
			_, _ = writer.Write([]byte(`{"version":"TrueNAS-SCALE-24.10.2"}`))
		default:
			http.NotFound(writer, request)
		}
	}))
	t.Cleanup(server.Close)

	client := protocolFixtureClient(t, server.URL, ClientConfig{APIKey: "modern-key"})
	if _, err := client.GetPools(context.Background()); err != nil {
		t.Fatalf("initial GetPools() error = %v", err)
	}

	client.rpcMu.Lock()
	client.closeRPCLocked()
	client.rpcMu.Unlock()

	_, err := client.GetPools(context.Background())
	if err == nil || !strings.Contains(err.Error(), "transport is immutable") {
		t.Fatalf("GetPools() reconnect error = %v, want immutable transport failure", err)
	}
	if restRequests.Load() != 0 {
		t.Fatalf("negotiated JSON-RPC transport made %d REST downgrade requests", restRequests.Load())
	}
	status := client.TransportStatus()
	if status.Mode != TransportJSONRPC || status.Connected {
		t.Fatalf("immutable reconnect changed transport authority: %+v", status)
	}
}

func TestJSONRPCReadReconnectsButActionIsNeverReplayed(t *testing.T) {
	t.Run("read reconnect", func(t *testing.T) {
		var poolCalls atomic.Int32
		fixture := newProtocolFixture(t, func(session int, request trueNASRPCRequest) protocolFixtureReply {
			if request.Method == "auth.login_with_api_key" {
				return protocolFixtureReply{result: true}
			}
			if request.Method != "pool.query" {
				return protocolFixtureReply{close: true}
			}
			call := poolCalls.Add(1)
			if session == 1 && call == 2 {
				return protocolFixtureReply{close: true}
			}
			return protocolFixtureReply{result: []map[string]any{{"id": 1, "name": "tank", "status": "ONLINE"}}}
		}, nil)

		client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "legacy-auth-key"})
		if _, err := client.GetPools(context.Background()); err != nil {
			t.Fatalf("first GetPools() error = %v", err)
		}
		if _, err := client.GetPools(context.Background()); err != nil {
			t.Fatalf("reconnected GetPools() error = %v", err)
		}
		if fixture.sessions.Load() != 2 || poolCalls.Load() != 3 {
			t.Fatalf("sessions=%d poolCalls=%d, want one replay on a new session", fixture.sessions.Load(), poolCalls.Load())
		}
		if status := client.TransportStatus(); status.Reconnects != 1 || status.Mode != TransportJSONRPC {
			t.Fatalf("unexpected reconnect status: %+v", status)
		}
	})

	t.Run("action outcome unknown", func(t *testing.T) {
		var actionCalls atomic.Int32
		fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
			if request.Method == "auth.login_with_api_key" {
				return protocolFixtureReply{result: true}
			}
			if request.Method == "app.start" {
				actionCalls.Add(1)
				return protocolFixtureReply{close: true}
			}
			return protocolFixtureReply{close: true}
		}, nil)

		client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "app-write-key"})
		err := client.StartApp(context.Background(), "nextcloud")
		if err == nil || !strings.Contains(err.Error(), "outcome is unknown") {
			t.Fatalf("StartApp() error = %v, want non-replay outcome", err)
		}
		if actionCalls.Load() != 1 || fixture.sessions.Load() != 1 {
			t.Fatalf("actionCalls=%d sessions=%d, action was replayed", actionCalls.Load(), fixture.sessions.Load())
		}
	})
}

func TestJSONRPCQueryReturnsCompleteUnpaginatedInventory(t *testing.T) {
	poolFixture := make([]map[string]any, 0, 225)
	for i := 0; i < 225; i++ {
		poolFixture = append(poolFixture, map[string]any{"id": i + 1, "name": "pool-" + strings.Repeat("x", i%4), "status": "ONLINE"})
	}
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		if request.Method == "auth.login_with_api_key" {
			return protocolFixtureReply{result: true}
		}
		if request.Method != "pool.query" {
			return protocolFixtureReply{close: true}
		}
		params, ok := request.Params.([]any)
		if !ok || len(params) != 2 {
			t.Errorf("pool.query params = %#v", request.Params)
		} else if options, ok := params[1].(map[string]any); !ok || len(options) != 0 {
			t.Errorf("pool.query options must request the authoritative complete set, got %#v", params[1])
		}
		return protocolFixtureReply{result: poolFixture}
	}, nil)

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "readonly-key"})
	pools, err := client.GetPools(context.Background())
	if err != nil {
		t.Fatalf("GetPools() error = %v", err)
	}
	if len(pools) != len(poolFixture) {
		t.Fatalf("GetPools() returned %d pools, want %d", len(pools), len(poolFixture))
	}
}

func TestJSONRPCFetchSnapshotKeepsAllModernInventoryOnOneTransport(t *testing.T) {
	methodCalls := make(map[string]int)
	var methodCallsMu sync.Mutex
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		methodCallsMu.Lock()
		methodCalls[request.Method]++
		methodCallsMu.Unlock()

		switch request.Method {
		case "auth.login_ex":
			return protocolFixtureReply{result: map[string]any{"response_type": "SUCCESS", "user_info": nil}}
		case "system.info":
			return protocolFixtureReply{result: map[string]any{
				"hostname":       "inventory-nas",
				"version":        "TrueNAS-SCALE-25.10.4",
				"system_serial":  "INVENTORY-1",
				"uptime_seconds": 60,
				"physical_cores": 4,
				"physmem":        8 * 1024 * 1024 * 1024,
			}}
		case "reporting.get_data":
			return protocolFixtureReply{result: []any{}}
		case "core.subscribe":
			params, _ := request.Params.([]any)
			event, _ := params[0].(string)
			if strings.HasPrefix(event, "reporting.realtime:") {
				return protocolFixtureReply{
					result: "sub-realtime",
					notifications: []protocolFixtureNotification{{
						method: "collection_update",
						params: map[string]any{
							"collection": event,
							"fields":     map[string]any{"cpu": map[string]any{"usage": 12}},
						},
					}},
				}
			}
			if strings.HasPrefix(event, "app.stats:") {
				return protocolFixtureReply{
					result: "sub-app-stats",
					notifications: []protocolFixtureNotification{{
						method: "collection_update",
						params: map[string]any{
							"collection": event,
							"fields":     []any{},
						},
					}},
				}
			}
			t.Errorf("unexpected subscription event %q", event)
			return protocolFixtureReply{close: true}
		case "core.unsubscribe":
			return protocolFixtureReply{result: nil}
		case "pool.query", "pool.dataset.query", "disk.query", "alert.list",
			"service.query", "app.query", "vm.query", "sharing.smb.query",
			"sharing.nfs.query", "zfs.resource.snapshot.query", "replication.query":
			return protocolFixtureReply{result: []any{}}
		case "boot.get_state":
			return protocolFixtureReply{result: map[string]any{
				"id": "boot-pool", "name": "boot-pool", "status": "ONLINE",
			}}
		default:
			t.Errorf("unexpected rpc method %q", request.Method)
			return protocolFixtureReply{close: true}
		}
	}, nil)

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{
		APIKey:   "readonly-key",
		Username: "pulse-readonly",
	})
	snapshot, err := client.FetchSnapshot(context.Background())
	if err != nil {
		t.Fatalf("FetchSnapshot() error = %v", err)
	}
	if snapshot.System.Hostname != "inventory-nas" || snapshot.System.CPUPercent != 12 {
		t.Fatalf("unexpected snapshot system: %+v", snapshot.System)
	}

	for _, method := range []string{
		"system.info",
		"pool.query",
		"boot.get_state",
		"pool.dataset.query",
		"disk.query",
		"alert.list",
		"service.query",
		"app.query",
		"vm.query",
		"sharing.smb.query",
		"sharing.nfs.query",
		"zfs.resource.snapshot.query",
		"replication.query",
	} {
		methodCallsMu.Lock()
		count := methodCalls[method]
		methodCallsMu.Unlock()
		if count != 1 {
			t.Errorf("%s calls = %d, want exactly one", method, count)
		}
	}
	methodCallsMu.Lock()
	subscribeCalls := methodCalls["core.subscribe"]
	unsubscribeCalls := methodCalls["core.unsubscribe"]
	methodCallsMu.Unlock()
	if subscribeCalls != 2 || unsubscribeCalls != 2 {
		t.Fatalf("subscription lifecycle calls: subscribe=%d unsubscribe=%d, want 2/2", subscribeCalls, unsubscribeCalls)
	}
	if fixture.sessions.Load() != 1 || fixture.restRequests.Load() != 0 {
		t.Fatalf("snapshot sessions=%d REST=%d, want one modern session and no REST",
			fixture.sessions.Load(), fixture.restRequests.Load())
	}
}

func TestJSONRPCMalformedSubscriptionEventDiscardsSession(t *testing.T) {
	fixture := newProtocolFixture(t, func(session int, request trueNASRPCRequest) protocolFixtureReply {
		switch request.Method {
		case "auth.login_with_api_key":
			return protocolFixtureReply{result: true}
		case "core.subscribe":
			return protocolFixtureReply{
				result: "sub-malformed",
				notifications: []protocolFixtureNotification{{
					method: "collection_update",
					params: map[string]any{
						"collection": "app.stats:{\"interval\":2}",
						"fields":     "not-an-array",
					},
				}},
			}
		case "pool.query":
			if session != 2 {
				t.Errorf("pool.query reused malformed subscription session %d", session)
			}
			return protocolFixtureReply{result: []map[string]any{{"id": 1, "name": "tank", "status": "ONLINE"}}}
		default:
			t.Errorf("unexpected rpc method %q", request.Method)
			return protocolFixtureReply{close: true}
		}
	}, nil)

	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "readonly-key"})
	if _, err := client.GetAppStats(context.Background()); err == nil ||
		!strings.Contains(err.Error(), "decode truenas app.stats notification") {
		t.Fatalf("GetAppStats() error = %v, want malformed event error", err)
	}
	pools, err := client.GetPools(context.Background())
	if err != nil {
		t.Fatalf("GetPools() after discarded stream error = %v", err)
	}
	if len(pools) != 1 || pools[0].Name != "tank" || fixture.sessions.Load() != 2 {
		t.Fatalf("unexpected post-discard state: pools=%+v sessions=%d", pools, fixture.sessions.Load())
	}
	if status := client.TransportStatus(); status.Reconnects != 1 {
		t.Fatalf("discarded stream reconnect was not observable: %+v", status)
	}
}

func TestTransportDiagnosticsAreConnectionLocalAndSecretFree(t *testing.T) {
	fixture := newProtocolFixture(t, func(_ int, request trueNASRPCRequest) protocolFixtureReply {
		if request.Method == "auth.login_with_api_key" {
			data, _ := json.Marshal(map[string]any{
				"reason":  "invalid credential never-expose-this-key",
				"errname": "EINVAL",
			})
			return protocolFixtureReply{err: &trueNASRPCError{
				Code:    -32001,
				Message: "Method call error",
				Data:    data,
			}}
		}
		return protocolFixtureReply{close: true}
	}, nil)
	client := protocolFixtureClient(t, fixture.server.URL, ClientConfig{APIKey: "never-expose-this-key"})

	if _, err := client.GetPools(context.Background()); err == nil {
		t.Fatal("expected authentication failure")
	}
	status := client.TransportStatus()
	encoded, err := json.Marshal(status)
	if err != nil {
		t.Fatalf("Marshal(status) error = %v", err)
	}
	if strings.Contains(string(encoded), "never-expose-this-key") {
		t.Fatalf("transport status exposed API key: %s", encoded)
	}
	if status.Mode != TransportUnknown {
		t.Fatalf("failed authentication selected a transport: %+v", status)
	}
}
