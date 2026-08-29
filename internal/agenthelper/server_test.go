package agenthelper

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"net"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeSMARTProvider func(context.Context) (json.RawMessage, error)

func (f fakeSMARTProvider) Snapshot(ctx context.Context) (json.RawMessage, error) {
	return f(ctx)
}

type fakeProxmoxProvider func(context.Context) (json.RawMessage, error)

func (f fakeProxmoxProvider) LXCFilesystems(ctx context.Context) (json.RawMessage, error) {
	return f(ctx)
}

func authorizedResolver(uid uint32) PeerResolver {
	return PeerResolverFunc(func(net.Conn) (Peer, error) {
		return Peer{UID: uid, GID: 2000, PID: 3000}, nil
	})
}

func newTestServer(t *testing.T, registry *Registry, resolver PeerResolver, audit AuditHook) *Server {
	t.Helper()
	server, err := NewServer(ServerConfig{
		AllowedUID:          1000,
		PeerResolver:        resolver,
		Registry:            registry,
		MaxOperationTimeout: 100 * time.Millisecond,
		Audit:               audit,
	})
	if err != nil {
		t.Fatalf("NewServer: %v", err)
	}
	return server
}

func validRequest(operation string) Request {
	return Request{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        "request-1",
		Operation:        operation,
		OperationVersion: OperationVersion1,
		DeadlineMillis:   50,
		Payload:          json.RawMessage(`{}`),
	}
}

func exchangeRequest(t *testing.T, server *Server, framed []byte) Response {
	t.Helper()
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		server.HandleConnection(context.Background(), serverConn)
		close(done)
	}()
	if _, err := writeAll(clientConn, framed); err != nil {
		t.Fatalf("write request: %v", err)
	}
	payload, err := readFrame(clientConn, MaxResponseBytes)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	response, err := DecodeResponse(payload)
	if err != nil {
		t.Fatalf("DecodeResponse: %v", err)
	}
	_ = clientConn.Close()
	<-done
	return response
}

func exchange(t *testing.T, server *Server, request Request) Response {
	t.Helper()
	framed, err := EncodeRequestFrame(request)
	if err != nil {
		t.Fatalf("EncodeRequestFrame: %v", err)
	}
	return exchangeRequest(t, server, framed)
}

func requireErrorCode(t *testing.T, response Response, code string) {
	t.Helper()
	if response.Success || response.Error == nil || response.Error.Code != code {
		t.Fatalf("response = %#v, want error %q", response, code)
	}
}

func TestServerHealthAndCapabilities(t *testing.T) {
	smart := fakeSMARTProvider(func(context.Context) (json.RawMessage, error) {
		return json.RawMessage(`{"disks":[]}`), nil
	})
	server := newTestServer(t, NewRegistry(smart, nil), authorizedResolver(1000), nil)

	health := exchange(t, server, validRequest(OperationHealth))
	if !health.Success {
		t.Fatalf("health response = %#v", health)
	}
	var healthResult HealthResult
	if err := decodeStrict(health.Result, &healthResult); err != nil || healthResult.Status != "ok" {
		t.Fatalf("health result = %#v, err=%v", healthResult, err)
	}

	capabilities := exchange(t, server, validRequest(OperationCapabilities))
	if !capabilities.Success {
		t.Fatalf("capabilities response = %#v", capabilities)
	}
	var result CapabilitiesResult
	if err := decodeStrict(capabilities.Result, &result); err != nil {
		t.Fatalf("decode capabilities: %v", err)
	}
	availability := make(map[string]bool)
	for _, capability := range result.Operations {
		availability[capability.Operation] = capability.Available
	}
	if !availability[OperationSMARTSnapshot] || availability[OperationProxmoxLXCFilesystems] {
		t.Fatalf("provider availability = %#v", availability)
	}
}

func TestServerDispatchesTypedProvidersWithoutCallerArguments(t *testing.T) {
	smartCalled := false
	proxmoxCalled := false
	registry := NewRegistry(
		fakeSMARTProvider(func(context.Context) (json.RawMessage, error) {
			smartCalled = true
			return json.RawMessage(`{"disks":[{"device":"sda"}]}`), nil
		}),
		fakeProxmoxProvider(func(context.Context) (json.RawMessage, error) {
			proxmoxCalled = true
			return json.RawMessage(`{"containers":[]}`), nil
		}),
	)
	server := newTestServer(t, registry, authorizedResolver(1000), nil)
	if response := exchange(t, server, validRequest(OperationSMARTSnapshot)); !response.Success || !smartCalled {
		t.Fatalf("SMART response = %#v called=%t", response, smartCalled)
	}
	if response := exchange(t, server, validRequest(OperationProxmoxLXCFilesystems)); !response.Success || !proxmoxCalled {
		t.Fatalf("Proxmox response = %#v called=%t", response, proxmoxCalled)
	}

	request := validRequest(OperationSMARTSnapshot)
	request.Payload = json.RawMessage(`{"path":"/dev/sda"}`)
	requireErrorCode(t, exchange(t, server, request), ErrorInvalidRequest)
	request = validRequest(OperationProxmoxLXCFilesystems)
	request.Payload = json.RawMessage(`{"args":["exec","100"]}`)
	requireErrorCode(t, exchange(t, server, request), ErrorInvalidRequest)
}

func TestServerRejectsEnvelopeAndRegistryViolations(t *testing.T) {
	server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
	tests := []struct {
		name   string
		mutate func(*Request)
		code   string
	}{
		{name: "protocol", mutate: func(r *Request) { r.ProtocolVersion = 2 }, code: ErrorUnsupportedProtocol},
		{name: "unknown operation", mutate: func(r *Request) { r.Operation = "host.exec" }, code: ErrorUnknownOperation},
		{name: "operation version", mutate: func(r *Request) { r.OperationVersion = 2 }, code: ErrorUnsupportedOperation},
		{name: "missing request id", mutate: func(r *Request) { r.RequestID = "" }, code: ErrorInvalidRequest},
		{name: "unsafe request id", mutate: func(r *Request) { r.RequestID = "request\nforged" }, code: ErrorInvalidRequest},
		{name: "zero deadline", mutate: func(r *Request) { r.DeadlineMillis = 0 }, code: ErrorInvalidRequest},
		{name: "excessive deadline", mutate: func(r *Request) { r.DeadlineMillis = 101 }, code: ErrorInvalidRequest},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			request := validRequest(OperationHealth)
			test.mutate(&request)
			requireErrorCode(t, exchange(t, server, request), test.code)
		})
	}
}

func TestServerRejectsStrictJSONViolations(t *testing.T) {
	server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
	for _, raw := range []string{
		`{"protocolVersion":1,"requestId":"r","operation":"helper.health","operationVersion":1,"deadlineMillis":50,"unexpected":true}`,
		`{"protocolVersion":1,"requestId":"r","operation":"helper.health","operationVersion":1,"deadlineMillis":50}{}`,
	} {
		framed := make([]byte, 4+len(raw))
		binary.BigEndian.PutUint32(framed[:4], uint32(len(raw)))
		copy(framed[4:], raw)
		requireErrorCode(t, exchangeRequest(t, server, framed), ErrorInvalidRequest)
	}
}

func TestServerRejectsOversizedFrameBeforeReadingPayload(t *testing.T) {
	server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1000), nil)
	framed := make([]byte, 4)
	binary.BigEndian.PutUint32(framed, MaxRequestBytes+1)
	requireErrorCode(t, exchangeRequest(t, server, framed), ErrorInvalidFrame)
}

func TestServerRejectsUnauthorizedPeer(t *testing.T) {
	server := newTestServer(t, NewRegistry(nil, nil), authorizedResolver(1001), nil)
	serverConn, clientConn := net.Pipe()
	done := make(chan struct{})
	go func() {
		server.HandleConnection(context.Background(), serverConn)
		close(done)
	}()
	payload, err := readFrame(clientConn, MaxResponseBytes)
	if err != nil {
		t.Fatalf("read unauthorized response: %v", err)
	}
	response, err := DecodeResponse(payload)
	if err != nil {
		t.Fatalf("decode unauthorized response: %v", err)
	}
	requireErrorCode(t, response, ErrorUnauthorizedPeer)
	_ = clientConn.Close()
	<-done
}

func TestServerBoundsOperationDeadline(t *testing.T) {
	provider := fakeSMARTProvider(func(ctx context.Context) (json.RawMessage, error) {
		<-ctx.Done()
		return nil, ctx.Err()
	})
	server := newTestServer(t, NewRegistry(provider, nil), authorizedResolver(1000), nil)
	request := validRequest(OperationSMARTSnapshot)
	request.DeadlineMillis = 5
	response := exchange(t, server, request)
	requireErrorCode(t, response, ErrorDeadlineExceeded)
	if !response.Error.Retryable {
		t.Fatal("deadline error must be retryable")
	}
}

func TestServerReplacesOversizedResponseWithTypedError(t *testing.T) {
	oversized := json.RawMessage(`{"data":"` + strings.Repeat("x", int(MaxResponseBytes)) + `"}`)
	server := newTestServer(t, NewRegistry(fakeSMARTProvider(func(context.Context) (json.RawMessage, error) {
		return oversized, nil
	}), nil), authorizedResolver(1000), nil)
	requireErrorCode(t, exchange(t, server, validRequest(OperationSMARTSnapshot)), ErrorResponseTooLarge)
}

func TestServerMapsProviderErrorsAndInvalidResults(t *testing.T) {
	tests := []struct {
		name     string
		provider fakeSMARTProvider
		code     string
	}{
		{name: "typed", provider: func(context.Context) (json.RawMessage, error) {
			return nil, &ProviderError{Code: ErrorProviderUnavailable, Message: "not configured"}
		}, code: ErrorProviderUnavailable},
		{name: "untyped", provider: func(context.Context) (json.RawMessage, error) {
			return nil, errors.New("secret provider detail")
		}, code: ErrorInternal},
		{name: "invalid JSON", provider: func(context.Context) (json.RawMessage, error) {
			return json.RawMessage(`not-json`), nil
		}, code: ErrorInternal},
		{name: "unstable provider code", provider: func(context.Context) (json.RawMessage, error) {
			return nil, &ProviderError{Code: "invented_provider_code", Message: "bounded"}
		}, code: ErrorInternal},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			server := newTestServer(t, NewRegistry(test.provider, nil), authorizedResolver(1000), nil)
			response := exchange(t, server, validRequest(OperationSMARTSnapshot))
			requireErrorCode(t, response, test.code)
			if strings.Contains(response.Error.Message, "secret provider detail") {
				t.Fatal("untyped provider detail escaped helper boundary")
			}
		})
	}
}

func TestAuditHookContainsMetadataOnly(t *testing.T) {
	var (
		mu     sync.Mutex
		events []AuditEvent
	)
	server := newTestServer(t, NewRegistry(fakeSMARTProvider(func(context.Context) (json.RawMessage, error) {
		return json.RawMessage(`{"secret":"must-not-be-audited"}`), nil
	}), nil), authorizedResolver(1000), func(event AuditEvent) {
		mu.Lock()
		defer mu.Unlock()
		events = append(events, event)
	})
	response := exchange(t, server, validRequest(OperationSMARTSnapshot))
	if !response.Success {
		t.Fatalf("response = %#v", response)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(events) != 1 || events[0].RequestID != "request-1" || !events[0].Success {
		t.Fatalf("audit events = %#v", events)
	}
	encoded, err := json.Marshal(events[0])
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(encoded, []byte("must-not-be-audited")) {
		t.Fatal("result data reached audit metadata")
	}
}
