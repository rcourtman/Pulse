package agenthelper

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"testing"
	"time"
)

func pipeDial(handler func(net.Conn)) DialContextFunc {
	return func(context.Context, string, string) (net.Conn, error) {
		client, server := net.Pipe()
		go handler(server)
		return client, nil
	}
}

func newPipeClient(t *testing.T, handler func(net.Conn)) *Client {
	t.Helper()
	client, err := NewClient(ClientConfig{
		SocketPath:   "/run/pulse-agent/helper.sock",
		MaxDeadline:  time.Second,
		DialContext:  pipeDial(handler),
		NewRequestID: func() (string, error) { return "client-request", nil },
	})
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return client
}

func serveClientResponse(t *testing.T, conn net.Conn, mutate func(*Response)) {
	t.Helper()
	defer conn.Close()
	frame, err := readFrame(conn, MaxRequestBytes)
	if err != nil {
		t.Errorf("read request: %v", err)
		return
	}
	request, err := decodeRequest(frame)
	if err != nil {
		t.Errorf("decode request: %v", err)
		return
	}
	response := Response{
		ProtocolVersion:  ProtocolVersion,
		RequestID:        request.RequestID,
		Operation:        request.Operation,
		OperationVersion: request.OperationVersion,
		Success:          true,
		Result:           json.RawMessage(`{"status":"ok"}`),
	}
	if mutate != nil {
		mutate(&response)
	}
	if _, err := writeFrame(conn, response, MaxResponseBytes); err != nil {
		t.Errorf("write response: %v", err)
	}
}

func TestClientCallUsesUnixFramingAndStrictCorrelation(t *testing.T) {
	client := newPipeClient(t, func(conn net.Conn) { serveClientResponse(t, conn, nil) })
	var result struct {
		Status string `json:"status"`
	}
	requestID, err := client.Call(context.Background(), OperationHealth, 1, time.Second, struct{}{}, &result)
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	if requestID != "client-request" || result.Status != "ok" {
		t.Fatalf("requestID=%q result=%#v", requestID, result)
	}
}

func TestClientReturnsTypedRemoteError(t *testing.T) {
	client := newPipeClient(t, func(conn net.Conn) {
		serveClientResponse(t, conn, func(response *Response) {
			response.Success = false
			response.Result = nil
			response.Error = &ResponseError{Code: ErrorProviderUnavailable, Message: "not installed", Retryable: false}
		})
	})
	requestID, err := client.Call(context.Background(), OperationSMARTSnapshot, 1, time.Second, nil, nil)
	if requestID != "client-request" {
		t.Fatalf("request ID = %q", requestID)
	}
	var remote *RemoteError
	if !errors.As(err, &remote) || remote.Code != ErrorProviderUnavailable || remote.RequestID != requestID {
		t.Fatalf("error = %#v", err)
	}
}

func TestClientRejectsMismatchedAndMalformedResponses(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*Response)
	}{
		{name: "protocol", mutate: func(r *Response) { r.ProtocolVersion = 2 }},
		{name: "request id", mutate: func(r *Response) { r.RequestID = "other" }},
		{name: "operation", mutate: func(r *Response) { r.Operation = OperationCapabilities }},
		{name: "version", mutate: func(r *Response) { r.OperationVersion = 2 }},
		{name: "success with error", mutate: func(r *Response) { r.Error = &ResponseError{Code: ErrorInternal} }},
		{name: "failure without error", mutate: func(r *Response) { r.Success = false; r.Result = nil }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			client := newPipeClient(t, func(conn net.Conn) { serveClientResponse(t, conn, test.mutate) })
			if _, err := client.Call(context.Background(), OperationHealth, 1, time.Second, nil, nil); err == nil {
				t.Fatal("malformed response accepted")
			}
		})
	}
}

func TestClientRejectsInvalidConfigurationAndCalls(t *testing.T) {
	for _, path := range []string{"", "relative.sock", "/run/../tmp/helper.sock"} {
		if _, err := NewClient(ClientConfig{SocketPath: path}); err == nil {
			t.Fatalf("invalid socket path accepted: %q", path)
		}
	}
	client, err := NewClient(ClientConfig{
		SocketPath:  "/run/pulse-agent/helper.sock",
		MaxDeadline: time.Second,
		DialContext: func(context.Context, string, string) (net.Conn, error) {
			return nil, errors.New("must not dial")
		},
		NewRequestID: func() (string, error) { return "request", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, call := range []struct {
		operation string
		version   int
		deadline  time.Duration
	}{
		{operation: "", version: 1, deadline: time.Second},
		{operation: OperationHealth, version: 0, deadline: time.Second},
		{operation: OperationHealth, version: 1, deadline: 0},
		{operation: OperationHealth, version: 1, deadline: 2 * time.Second},
	} {
		if _, err := client.Call(context.Background(), call.operation, call.version, call.deadline, nil, nil); err == nil {
			t.Fatalf("invalid call accepted: %#v", call)
		}
	}
}

func TestClientRejectsUnsafeGeneratedRequestID(t *testing.T) {
	client, err := NewClient(ClientConfig{
		SocketPath:   "/run/pulse-agent/helper.sock",
		MaxDeadline:  time.Second,
		DialContext:  func(context.Context, string, string) (net.Conn, error) { return nil, errors.New("must not dial") },
		NewRequestID: func() (string, error) { return "forged\nrequest", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Call(context.Background(), OperationHealth, 1, time.Second, nil, nil); err == nil {
		t.Fatal("unsafe request ID accepted")
	}
}

func TestClientHardcodesUnixTransport(t *testing.T) {
	var network, address string
	client, err := NewClient(ClientConfig{
		SocketPath:  "/run/pulse-agent/helper.sock",
		MaxDeadline: time.Second,
		DialContext: func(_ context.Context, gotNetwork, gotAddress string) (net.Conn, error) {
			network, address = gotNetwork, gotAddress
			return nil, errors.New("stop")
		},
		NewRequestID: func() (string, error) { return "request", nil },
	})
	if err != nil {
		t.Fatal(err)
	}
	_, _ = client.Call(context.Background(), OperationHealth, 1, time.Second, nil, nil)
	if network != "unix" || address != "/run/pulse-agent/helper.sock" {
		t.Fatalf("dialed network=%q address=%q", network, address)
	}
}
