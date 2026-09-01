package hostagent

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/hex"
	"encoding/json"
	"encoding/pem"
	"errors"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rs/zerolog"
)

func TestNewActionRunnerClientIsTypedOnlyAndEmitsExplicitRole(t *testing.T) {
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://pulse.example", APIToken: "separate-secret",
		StateDir: t.TempDir(), HealthPath: filepath.Join(t.TempDir(), "health.json"),
		ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
	}, "agent-1", "host-1", "v1")
	t.Cleanup(func() { _ = client.Close() })
	if !client.actionRunnerOnly || client.runtimeRole != agentexec.RuntimeRoleActionRunner || client.actionCapability != agentexec.ActionCapabilityTypedV1 {
		t.Fatalf("runner protocol ceiling not configured: %+v", client)
	}
	for _, message := range []messageType{msgTypeExecuteCmd, msgTypeReadFile, msgTypeDeployPreflight, msgTypeDeployInstall, msgTypeDeployCancel} {
		if allowedActionRunnerMessage(message) {
			t.Errorf("forbidden message %q was admitted", message)
		}
	}
	for _, message := range []messageType{msgTypeHostUpdate, msgTypeHostStorageCleanup, msgTypeDockerContainerLifecycle, msgTypeDockerContainerUpdate, msgTypeOperationQuery, msgTypeCancelCmd} {
		if !allowedActionRunnerMessage(message) {
			t.Errorf("typed message %q was rejected", message)
		}
	}
}

func TestActionRunnerWithoutConnectedContainerOperatorHasNoCLIFallback(t *testing.T) {
	client := NewActionRunnerClient(ActionRunnerClientConfig{StateDir: t.TempDir()}, "host-1", "node-1", "test")
	t.Cleanup(func() { _ = client.Close() })
	if client.dockerLifecycle != nil {
		t.Fatal("action runner exposed a Docker/Podman CLI fallback without a connected daemon operator")
	}
}

func TestActionRunnerTransportRequiresTLSExceptLoopback(t *testing.T) {
	logger := zerolog.Nop()
	for _, test := range []struct {
		name    string
		url     string
		wantURL string
		wantErr bool
	}{
		{name: "public plaintext rejected", url: "http://pulse.example.com:7655", wantErr: true},
		{name: "private LAN plaintext rejected", url: "http://192.168.1.20:7655", wantErr: true},
		{name: "localhost subdomain plaintext rejected", url: "http://agent.localhost:7655", wantErr: true},
		{name: "loopback plaintext accepted", url: "http://127.0.0.1:7655", wantURL: "ws://127.0.0.1:7655/api/agent/ws"},
		{name: "HTTPS accepted", url: "https://pulse.example.com:7655", wantURL: "wss://pulse.example.com:7655/api/agent/ws"},
	} {
		t.Run(test.name, func(t *testing.T) {
			client := NewActionRunnerClient(ActionRunnerClientConfig{
				PulseURL: test.url, APIToken: "runner-token", StateDir: t.TempDir(),
				HealthPath: filepath.Join(t.TempDir(), "health.json"), ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
			}, "agent-1", "host-1", "v1")
			t.Cleanup(func() { _ = client.Close() })
			got, err := client.buildWebSocketURL()
			if test.wantErr {
				if err == nil || !strings.Contains(err.Error(), "loopback") {
					t.Fatalf("buildWebSocketURL() = %q, %v; want plaintext rejection", got, err)
				}
				if err := client.activateActionRunnerCredential(context.Background()); err == nil || !strings.Contains(err.Error(), "loopback") {
					t.Fatalf("activation URL error = %v, want plaintext rejection", err)
				}
				return
			}
			if err != nil || got != test.wantURL {
				t.Fatalf("buildWebSocketURL() = %q, %v; want %q", got, err, test.wantURL)
			}
		})
	}
	if err := CancelPendingActionRunnerCredential(context.Background(), ActionRunnerCredentialLifecycleConfig{
		PulseURL: "http://agent.localhost:7655", APIToken: "runner-token",
	}); err == nil || !strings.Contains(err.Error(), "literal loopback") {
		t.Fatalf("localhost subdomain lifecycle error = %v", err)
	}
}

func TestActionRunnerCredentialHTTPSHonorsCAAndRejectsFingerprintMismatch(t *testing.T) {
	server := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodPatch {
			http.Error(w, "wrong method", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: server.Certificate().Raw})
	if err := os.WriteFile(caFile, certificate, 0o600); err != nil {
		t.Fatal(err)
	}
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: t.TempDir(), CACertPath: caFile,
		HealthPath: filepath.Join(t.TempDir(), "health.json"), ActivationNonce: strings.Repeat("b", 32), Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer client.Close()
	if err := client.activateActionRunnerCredential(context.Background()); err != nil {
		t.Fatalf("CA-authenticated HTTPS activation: %v", err)
	}

	mismatch := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: t.TempDir(), InsecureSkipVerify: true,
		ServerFingerprint: strings.Repeat("00", 32), HealthPath: filepath.Join(t.TempDir(), "health.json"),
		ActivationNonce: strings.Repeat("c", 32), Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer mismatch.Close()
	if err := mismatch.activateActionRunnerCredential(context.Background()); err == nil || !strings.Contains(strings.ToLower(err.Error()), "fingerprint") {
		t.Fatalf("fingerprint mismatch error = %v", err)
	}
}

func TestActionRunnerLifecycleTransportEnforcesTrustLoopbackAndRedirectBoundary(t *testing.T) {
	const token = "runner-lifecycle-token"
	authorizedRequests := 0
	tlsServer := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("authorization = %q", request.Header.Get("Authorization"))
		}
		authorizedRequests++
		w.WriteHeader(http.StatusNoContent)
	}))
	defer tlsServer.Close()
	caFile := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: tlsServer.Certificate().Raw}), 0o600); err != nil {
		t.Fatal(err)
	}
	base := ActionRunnerCredentialLifecycleConfig{PulseURL: tlsServer.URL, APIToken: token, CACertPath: caFile}
	if err := CancelPendingActionRunnerCredential(context.Background(), base); err != nil {
		t.Fatalf("custom CA cancel: %v", err)
	}
	fingerprintBytes := sha256.Sum256(tlsServer.Certificate().Raw)
	pinned := ActionRunnerCredentialLifecycleConfig{PulseURL: tlsServer.URL, APIToken: token, ServerFingerprint: hex.EncodeToString(fingerprintBytes[:])}
	if err := RevokeActionRunnerCredential(context.Background(), pinned, "agent-1", "host.local"); err != nil {
		t.Fatalf("exact fingerprint revoke: %v", err)
	}
	beforeMismatch := authorizedRequests
	mismatch := ActionRunnerCredentialLifecycleConfig{PulseURL: tlsServer.URL, APIToken: token, ServerFingerprint: strings.Repeat("00", 32)}
	if err := CancelPendingActionRunnerCredential(context.Background(), mismatch); err == nil || !strings.Contains(strings.ToLower(err.Error()), "fingerprint") {
		t.Fatalf("mismatched fingerprint error = %v", err)
	}
	if authorizedRequests != beforeMismatch {
		t.Fatal("mismatched pin transmitted Authorization to the handler")
	}
	if err := CancelPendingActionRunnerCredential(context.Background(), ActionRunnerCredentialLifecycleConfig{PulseURL: "http://192.0.2.10:7655", APIToken: token}); err == nil || !strings.Contains(err.Error(), "loopback") {
		t.Fatalf("non-loopback HTTP error = %v", err)
	}

	loopback := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("loopback authorization = %q", request.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer loopback.Close()
	if err := CancelPendingActionRunnerCredential(context.Background(), ActionRunnerCredentialLifecycleConfig{PulseURL: loopback.URL, APIToken: token}); err != nil {
		t.Fatalf("loopback HTTP cancel: %v", err)
	}

	redirectTargetAuthorization := ""
	redirectTarget := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		redirectTargetAuthorization = request.Header.Get("Authorization")
		w.WriteHeader(http.StatusNoContent)
	}))
	defer redirectTarget.Close()
	redirector := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		http.Redirect(w, request, redirectTarget.URL, http.StatusTemporaryRedirect)
	}))
	defer redirector.Close()
	err := CancelPendingActionRunnerCredential(context.Background(), ActionRunnerCredentialLifecycleConfig{PulseURL: redirector.URL, APIToken: token})
	if err == nil || !strings.Contains(err.Error(), "redirect") {
		t.Fatalf("redirect error = %v", err)
	}
	if redirectTargetAuthorization != "" {
		t.Fatalf("redirect target received bearer %q", redirectTargetAuthorization)
	}
}

func TestActionRunnerBearerHTTPBypassesAmbientProxy(t *testing.T) {
	const token = "runner-proxy-sensitive-token"
	proxyRequests := 0
	proxyAuthorization := ""
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		proxyRequests++
		proxyAuthorization = request.Header.Get("Authorization")
		http.Error(w, "proxy must not be used", http.StatusBadGateway)
	}))
	defer proxy.Close()
	for _, key := range []string{"HTTP_PROXY", "HTTPS_PROXY", "ALL_PROXY", "http_proxy", "https_proxy", "all_proxy"} {
		t.Setenv(key, proxy.URL)
	}
	t.Setenv("NO_PROXY", "")
	t.Setenv("no_proxy", "")

	targetRequests := 0
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		targetRequests++
		if request.Header.Get("Authorization") != "Bearer "+token {
			t.Errorf("target authorization = %q", request.Header.Get("Authorization"))
		}
		w.WriteHeader(http.StatusNoContent)
	}))
	defer target.Close()
	pulseURL := strings.Replace(target.URL, "127.0.0.1", "localhost", 1)
	if pulseURL == target.URL {
		t.Fatalf("unexpected httptest URL %q", target.URL)
	}

	directClient, err := newActionRunnerHTTPClient("", false, "")
	if err != nil {
		t.Fatal(err)
	}
	transport, ok := directClient.Transport.(*http.Transport)
	if !ok || transport.Proxy != nil {
		t.Fatalf("action-runner transport proxy configured = %t, want false", transport.Proxy != nil)
	}
	config := ActionRunnerCredentialLifecycleConfig{PulseURL: pulseURL, APIToken: token}
	if err := CancelPendingActionRunnerCredential(context.Background(), config); err != nil {
		t.Fatalf("direct cancellation: %v", err)
	}
	if err := RevokeActionRunnerCredential(context.Background(), config, "agent-1", "runner.localhost"); err != nil {
		t.Fatalf("direct self-revoke: %v", err)
	}
	logger := zerolog.Nop()
	runner := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: pulseURL, APIToken: token, StateDir: t.TempDir(),
		HealthPath: filepath.Join(t.TempDir(), "health.json"), ActivationNonce: strings.Repeat("a", 32), Logger: &logger,
	}, "agent-1", "runner.localhost", "v1")
	defer runner.Close()
	if err := runner.activateActionRunnerCredential(context.Background()); err != nil {
		t.Fatalf("direct activation: %v", err)
	}
	if proxyRequests != 0 || proxyAuthorization != "" {
		t.Fatalf("ambient proxy saw %d requests and authorization %q", proxyRequests, proxyAuthorization)
	}
	if targetRequests != 3 {
		t.Fatalf("direct target requests = %d, want cancel, revoke, and activate", targetRequests)
	}
}

func TestActionRunnerWebSocketCustomCADisablesRawInsecureBypass(t *testing.T) {
	targetRequests := 0
	target := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		targetRequests++
		http.Error(w, "TLS verification should fail before HTTP", http.StatusInternalServerError)
	}))
	defer target.Close()
	caFile := filepath.Join(t.TempDir(), "unrelated-ca.pem")
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	template := &x509.Certificate{
		SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "unrelated-action-runner-test-ca"},
		NotBefore: now.Add(-time.Minute), NotAfter: now.Add(time.Hour), IsCA: true, BasicConstraintsValid: true,
		KeyUsage: x509.KeyUsageCertSign | x509.KeyUsageDigitalSignature,
	}
	certificate, err := x509.CreateCertificate(rand.Reader, template, template, &privateKey.PublicKey, privateKey)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(caFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate}), 0o600); err != nil {
		t.Fatal(err)
	}
	logger := zerolog.Nop()
	runner := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: target.URL, APIToken: "runner-token", StateDir: t.TempDir(),
		HealthPath: filepath.Join(t.TempDir(), "health.json"), ActivationNonce: strings.Repeat("a", 32),
		InsecureSkipVerify: true, CACertPath: caFile, Logger: &logger,
	}, "agent-1", "host.local", "v1")
	defer runner.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	err = runner.connectAndHandle(ctx)
	if err == nil {
		t.Fatal("websocket connection succeeded with an unrelated custom CA and raw insecure flag")
	}
	if targetRequests != 0 {
		t.Fatalf("raw insecure bypass reached target handler %d times", targetRequests)
	}
}

func TestActionRunnerTransportRegistersRoleWritesHealthAndRejectsGenericExec(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	registration := make(chan registerPayload, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		conn, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message wsMessage
		if err := conn.ReadJSON(&message); err != nil {
			return
		}
		var payload registerPayload
		if json.Unmarshal(message.Payload, &payload) != nil {
			return
		}
		registration <- payload
		ack, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: ack})
		forbidden, _ := json.Marshal(executeCommandPayload{RequestID: "r1", Command: "id", TargetType: "agent"})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeExecuteCmd, Timestamp: time.Now(), Payload: forbidden})
	}))
	defer server.Close()
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: filepath.Join(dir, "state"),
		HealthPath: healthPath, ActivationNonce: strings.Repeat("b", 32), InsecureSkipVerify: true, Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer client.Close()
	err := client.connectAndHandle(context.Background())
	if err == nil || !strings.Contains(err.Error(), "forbidden message") {
		t.Fatalf("transport error = %v", err)
	}
	payload := <-registration
	if payload.RuntimeRole != agentexec.RuntimeRoleActionRunner || payload.ActionCapability != agentexec.ActionCapabilityTypedV1 || payload.Token != "runner-token" {
		t.Fatalf("registration = %+v", payload)
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	var health actionRunnerHealth
	if json.Unmarshal(data, &health) != nil || !health.Registered || !health.Activated || health.HostID != "agent-1" || health.ActivationNonce != strings.Repeat("b", 32) {
		t.Fatalf("health = %s", data)
	}
}

func TestActionRunnerHealthIsAtomicBoundedAndSecretFree(t *testing.T) {
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: "https://pulse.example", APIToken: "must-not-appear",
		StateDir: filepath.Join(dir, "state"), HealthPath: healthPath,
		ActivationNonce: strings.Repeat("c", 32), Logger: &logger,
	}, "agent-1", "host-1", "v1")
	t.Cleanup(func() { _ = client.Close() })
	if err := os.WriteFile(healthPath, []byte("stale-health-marker"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := client.writeActionRunnerHealth(true); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	if json.Valid(data) == false || string(data) == "" {
		t.Fatalf("invalid health marker: %q", data)
	}
	if contains := string(data); contains == "must-not-appear" || jsonContains(data, "must-not-appear") {
		t.Fatal("health marker leaked the action credential")
	}
	var health actionRunnerHealth
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if !health.Registered || !health.Activated || health.ActivationNonce != strings.Repeat("c", 32) || health.RuntimeRole != agentexec.RuntimeRoleActionRunner || health.HostID != "agent-1" || health.Server != "https://pulse.example" || health.RegisteredAt.IsZero() {
		t.Fatalf("health = %+v", health)
	}
	info, err := os.Lstat(healthPath)
	if err != nil {
		t.Fatalf("inspect health marker: %v", err)
	}
	if err := securityutil.ValidatePrivatePath(healthPath, info); err != nil {
		t.Fatalf("health marker is not private: %v", err)
	}
	matches, err := filepath.Glob(filepath.Join(dir, ".health-*.tmp"))
	if err != nil || len(matches) != 0 {
		t.Fatalf("temporary health files = %v, %v", matches, err)
	}
}

func TestActionRunnerActivationFailureLeavesOnlyPendingHealthProof(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch {
			http.Error(w, "persistence unavailable", http.StatusServiceUnavailable)
			return
		}
		conn, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message wsMessage
		if conn.ReadJSON(&message) != nil {
			return
		}
		ack, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: ack})
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: filepath.Join(dir, "state"),
		HealthPath: healthPath, ActivationNonce: strings.Repeat("d", 32), InsecureSkipVerify: true, Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer client.Close()
	if err := client.connectAndHandle(context.Background()); err == nil || !strings.Contains(err.Error(), "503 Service Unavailable") {
		t.Fatalf("activation error = %v", err)
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	var health actionRunnerHealth
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if !health.Registered || health.Activated || health.ActivationNonce != strings.Repeat("d", 32) {
		t.Fatalf("pending health = %+v", health)
	}
}

func TestActionRunnerPostCommitHealthFailureKeepsPendingProof(t *testing.T) {
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	activationCommitted := make(chan struct{}, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method == http.MethodPatch {
			activationCommitted <- struct{}{}
			w.WriteHeader(http.StatusNoContent)
			return
		}
		conn, err := upgrader.Upgrade(w, request, nil)
		if err != nil {
			return
		}
		defer conn.Close()
		var message wsMessage
		if conn.ReadJSON(&message) != nil {
			return
		}
		ack, _ := json.Marshal(registeredPayload{Success: true})
		_ = conn.WriteJSON(wsMessage{Type: msgTypeRegistered, Timestamp: time.Now(), Payload: ack})
		_, _, _ = conn.ReadMessage()
	}))
	defer server.Close()
	dir := t.TempDir()
	healthPath := filepath.Join(dir, "health.json")
	logger := zerolog.Nop()
	client := NewActionRunnerClient(ActionRunnerClientConfig{
		PulseURL: server.URL, APIToken: "runner-token", StateDir: filepath.Join(dir, "state"),
		HealthPath: healthPath, ActivationNonce: strings.Repeat("e", 32), InsecureSkipVerify: true, Logger: &logger,
	}, "agent-1", "host-1", "v1")
	defer client.Close()
	client.actionHealthWriter = func(activated bool) error {
		if activated {
			return errors.New("injected activated health replacement failure")
		}
		return client.writeActionRunnerHealth(false)
	}
	if err := client.connectAndHandle(context.Background()); err == nil || !strings.Contains(err.Error(), "injected activated health replacement failure") {
		t.Fatalf("post-commit health error = %v", err)
	}
	select {
	case <-activationCommitted:
	default:
		t.Fatal("credential activation did not commit before the injected health failure")
	}
	data, err := os.ReadFile(healthPath)
	if err != nil {
		t.Fatal(err)
	}
	var health actionRunnerHealth
	if err := json.Unmarshal(data, &health); err != nil {
		t.Fatal(err)
	}
	if !health.Registered || health.Activated || health.ActivationNonce != strings.Repeat("e", 32) {
		t.Fatalf("post-commit failed health marker = %+v", health)
	}
}

func jsonContains(data []byte, value string) bool {
	var decoded any
	if json.Unmarshal(data, &decoded) != nil {
		return false
	}
	return containsJSONValue(decoded, value)
}

func containsJSONValue(value any, secret string) bool {
	switch typed := value.(type) {
	case string:
		return typed == secret
	case []any:
		for _, item := range typed {
			if containsJSONValue(item, secret) {
				return true
			}
		}
	case map[string]any:
		for _, item := range typed {
			if containsJSONValue(item, secret) {
				return true
			}
		}
	}
	return false
}
