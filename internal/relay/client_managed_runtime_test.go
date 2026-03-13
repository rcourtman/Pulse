package relay

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rs/zerolog"
)

type managedRelayServer struct {
	cmd     *exec.Cmd
	logBuf  *bytes.Buffer
	addr    string
	dataDir string
}

func TestManagedRuntimeRelayRegistrationReconnectDrain(t *testing.T) {
	t.Parallel()

	pulseRoot, pulseProRelayDir := managedRelayWorkspaceRoots(t)
	relayBinary := buildManagedRelayBinary(t, pulseProRelayDir)

	publicKey, privateKey, err := ed25519.GenerateKey(nil)
	if err != nil {
		t.Fatalf("generate relay signing keypair: %v", err)
	}
	publicKeyB64 := base64.StdEncoding.EncodeToString(publicKey)
	licenseToken := signManagedRelayJWT(t, privateKey, map[string]any{
		"lid":   "lic_managed_runtime_relay",
		"email": "managed-runtime@example.test",
		"tier":  "pro",
		"iat":   time.Now().Unix(),
		"exp":   time.Now().Add(30 * 24 * time.Hour).Unix(),
	})

	apiSlowStarted := make(chan struct{}, 1)
	apiSlowCancelled := make(chan struct{}, 1)
	apiServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/status":
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{
				"endpoint": "status",
				"ok":       true,
			})
		case "/api/slow-resource":
			select {
			case apiSlowStarted <- struct{}{}:
			default:
			}
			select {
			case <-r.Context().Done():
				select {
				case apiSlowCancelled <- struct{}{}:
				default:
				}
				return
			case <-time.After(30 * time.Second):
				t.Error("slow relay runtime request completed instead of being cancelled during drain")
				return
			}
		default:
			http.NotFound(w, r)
		}
	}))
	defer apiServer.Close()

	relayAddr := allocateLoopbackAddress(t)
	server1 := startManagedRelayServer(t, relayBinary, relayAddr, t.TempDir(), publicKeyB64)
	defer server1.stopKill()

	logBuffer := &bytes.Buffer{}
	logger := zerolog.New(logBuffer).With().Timestamp().Logger()

	client := NewClient(
		Config{
			Enabled:        true,
			ServerURL:      "ws://" + relayAddr + "/ws/instance",
			InstanceSecret: "managed-runtime-instance-secret",
		},
		ClientDeps{
			LicenseTokenFunc: func() string { return licenseToken },
			TokenValidator:   func(token string) bool { return token == "managed-runtime-app-token" },
			LocalAddr:        strings.TrimPrefix(apiServer.URL, "http://"),
			ServerVersion:    "6.0.0-rc.2",
			IdentityPubKey:   "managed-runtime-identity-pub",
		},
		logger,
	)

	runCtx, cancelRun := context.WithCancel(context.Background())
	defer cancelRun()
	runErrCh := make(chan error, 1)
	go func() {
		runErrCh <- client.Run(runCtx)
	}()
	defer func() {
		cancelRun()
		select {
		case err := <-runErrCh:
			if err != nil && !errors.Is(err, context.Canceled) {
				t.Fatalf("relay client run exited unexpectedly: %v\nclient logs:\n%s", err, logBuffer.String())
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("timed out waiting for relay client shutdown\nclient logs:\n%s", logBuffer.String())
		}
	}()

	instanceID := waitForManagedRelayConnected(t, client, 20*time.Second)
	if !strings.HasPrefix(instanceID, "relay_") {
		t.Fatalf("managed runtime instance ID %q does not look canonical", instanceID)
	}
	verifyManagedRelayProxyRoundTrip(t, relayAddr, instanceID, "/api/status")

	// Phase 1: normal reconnect after abrupt relay restart with persistent data.
	server1.stopKill()
	server2 := startManagedRelayServer(t, relayBinary, relayAddr, server1.dataDir, publicKeyB64)
	defer server2.stopKill()
	waitForManagedRelayConnected(t, client, 20*time.Second)
	verifyManagedRelayProxyRoundTrip(t, relayAddr, instanceID, "/api/status")

	// Phase 2: stale-session recovery after relay restart with a fresh data dir.
	server2.stopKill()
	server3 := startManagedRelayServer(t, relayBinary, relayAddr, t.TempDir(), publicKeyB64)
	defer server3.stopKill()
	waitForManagedRelayLog(t, logBuffer, "relay session resume rejected, retrying fresh registration", 20*time.Second)
	waitForManagedRelayConnected(t, client, 20*time.Second)
	verifyManagedRelayProxyRoundTrip(t, relayAddr, instanceID, "/api/status")

	// Phase 3: drain during inflight work, then reconnect to a replacement server.
	appConn, channelID := openManagedRelayAppConnection(t, relayAddr, instanceID, "managed-runtime-app-token")
	sendManagedRelayProxyRequest(t, appConn, channelID, ProxyRequest{
		ID:     "req_drain_inflight",
		Method: "GET",
		Path:   "/api/slow-resource",
	})
	select {
	case <-apiSlowStarted:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for slow managed-runtime relay request to start\nclient logs:\n%s", logBuffer.String())
	}
	server3.stopTerm()
	server4 := startManagedRelayServer(t, relayBinary, relayAddr, server3.dataDir, publicKeyB64)
	defer server4.stopKill()
	waitForManagedRelayLog(t, logBuffer, "Relay server draining, will reconnect", 20*time.Second)
	select {
	case <-apiSlowCancelled:
	case <-time.After(5 * time.Second):
		t.Fatalf("timed out waiting for slow managed-runtime relay request cancellation\nclient logs:\n%s", logBuffer.String())
	}
	_ = appConn.Close()
	waitForManagedRelayConnected(t, client, 20*time.Second)
	verifyManagedRelayProxyRoundTrip(t, relayAddr, instanceID, "/api/status")
	waitForManagedRelayIdle(t, client, 5*time.Second)

	if !strings.Contains(logBuffer.String(), "relay session resume rejected, retrying fresh registration") {
		t.Fatalf("managed runtime exercise never observed stale-session recovery\nclient logs:\n%s", logBuffer.String())
	}
	if !strings.Contains(logBuffer.String(), "Relay server draining, will reconnect") {
		t.Fatalf("managed runtime exercise never observed drain handling\nclient logs:\n%s", logBuffer.String())
	}

	_ = pulseRoot // document intent: this test is intentionally workspace-scoped.
}

func managedRelayWorkspaceRoots(t *testing.T) (string, string) {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve runtime caller for managed relay test")
	}
	pulseRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	pulseProRoot := os.Getenv("PULSE_REPO_ROOT_PULSE_PRO")
	if pulseProRoot == "" {
		pulseProRoot = filepath.Join(filepath.Dir(pulseRoot), "pulse-pro")
	}
	pulseProRelayDir := filepath.Join(pulseProRoot, "relay-server")
	if _, err := os.Stat(filepath.Join(pulseProRelayDir, "main.go")); err != nil {
		if os.Getenv("GITHUB_ACTIONS") == "true" && os.Getenv("PULSE_REPO_ROOT_PULSE_PRO") == "" {
			t.Skipf("managed relay runtime proof requires sibling pulse-pro relay-server; skipping in GitHub Actions without PULSE_REPO_ROOT_PULSE_PRO override: %v", err)
		}
		t.Fatalf("managed relay runtime proof requires sibling pulse-pro relay-server at %s: %v", pulseProRelayDir, err)
	}
	return pulseRoot, pulseProRelayDir
}

func buildManagedRelayBinary(t *testing.T, relayServerDir string) string {
	t.Helper()
	binaryPath := filepath.Join(t.TempDir(), "pulse-relay-managed-runtime")
	cmd := exec.Command("go", "build", "-o", binaryPath, ".")
	cmd.Dir = relayServerDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("build managed relay server binary: %v\n%s", err, string(output))
	}
	return binaryPath
}

func signManagedRelayJWT(t *testing.T, privateKey ed25519.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(claims)
	if err != nil {
		t.Fatalf("marshal managed relay claims: %v", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)
	signedData := []byte(header + "." + payload)
	signature := ed25519.Sign(privateKey, signedData)
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func allocateLoopbackAddress(t *testing.T) string {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("allocate loopback port: %v", err)
	}
	addr := listener.Addr().String()
	if err := listener.Close(); err != nil {
		t.Fatalf("close loopback allocator: %v", err)
	}
	return addr
}

func startManagedRelayServer(
	t *testing.T,
	binaryPath string,
	addr string,
	dataDir string,
	publicKeyB64 string,
) *managedRelayServer {
	t.Helper()
	logBuf := &bytes.Buffer{}
	cmd := exec.Command(binaryPath)
	cmd.Env = append(os.Environ(),
		"PULSE_RELAY_ADDR="+addr,
		"PULSE_RELAY_PUBLIC_KEY="+publicKeyB64,
		"PULSE_RELAY_DATA_DIR="+dataDir,
		"PULSE_RELAY_DRAIN_TIMEOUT=2s",
	)
	cmd.Stdout = logBuf
	cmd.Stderr = logBuf
	if err := cmd.Start(); err != nil {
		t.Fatalf("start managed relay server: %v", err)
	}

	server := &managedRelayServer{
		cmd:     cmd,
		logBuf:  logBuf,
		addr:    addr,
		dataDir: dataDir,
	}
	t.Cleanup(func() {
		server.stopKill()
	})
	waitForManagedRelayHealth(t, addr, logBuf, 10*time.Second)
	return server
}

func (s *managedRelayServer) stopKill() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
		return
	}
	_ = s.cmd.Process.Kill()
	_, _ = s.cmd.Process.Wait()
}

func (s *managedRelayServer) stopTerm() {
	if s == nil || s.cmd == nil || s.cmd.Process == nil {
		return
	}
	if s.cmd.ProcessState != nil && s.cmd.ProcessState.Exited() {
		return
	}
	_ = s.cmd.Process.Signal(os.Interrupt)
	done := make(chan struct{})
	go func() {
		_, _ = s.cmd.Process.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		_ = s.cmd.Process.Kill()
		<-done
	}
}

func waitForManagedRelayHealth(t *testing.T, addr string, logBuf *bytes.Buffer, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	url := "http://" + addr + "/healthz"
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			body, _ := io.ReadAll(resp.Body)
			_ = resp.Body.Close()
			if resp.StatusCode == http.StatusOK && strings.Contains(string(body), `"status":"ok"`) {
				return
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("managed relay server at %s never became healthy\nserver logs:\n%s", addr, logBuf.String())
}

func waitForManagedRelayConnected(t *testing.T, client *Client, timeout time.Duration) string {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := client.Status()
		if status.Connected && status.InstanceID != "" {
			return status.InstanceID
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for managed relay client connection; final status=%+v", client.Status())
	return ""
}

func waitForManagedRelayIdle(t *testing.T, client *Client, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		status := client.Status()
		if status.Connected && status.ActiveChannels == 0 {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for managed relay client idle status; final status=%+v", client.Status())
}

func waitForManagedRelayLog(t *testing.T, logBuf *bytes.Buffer, needle string, timeout time.Duration) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if strings.Contains(logBuf.String(), needle) {
			return
		}
		time.Sleep(100 * time.Millisecond)
	}
	t.Fatalf("timed out waiting for managed relay log %q\nlogs:\n%s", needle, logBuf.String())
}

func openManagedRelayAppConnection(t *testing.T, relayAddr string, instanceID string, authToken string) (*websocket.Conn, uint32) {
	t.Helper()
	conn, _, err := websocket.DefaultDialer.Dial("ws://"+relayAddr+"/ws/app", nil)
	if err != nil {
		t.Fatalf("dial managed relay app websocket: %v", err)
	}

	connectBytes, err := MarshalControlPayload(ConnectPayload{
		InstanceID: instanceID,
		AuthToken:  authToken,
	})
	if err != nil {
		t.Fatalf("marshal managed relay connect payload: %v", err)
	}
	frame := NewFrame(FrameConnect, 0, connectBytes)
	encoded, err := EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode managed relay connect frame: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		t.Fatalf("write managed relay connect frame: %v", err)
	}

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set managed relay app read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read managed relay connect ack: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	ackFrame, err := DecodeFrame(msg)
	if err != nil {
		t.Fatalf("decode managed relay connect ack: %v", err)
	}
	if ackFrame.Type != FrameConnectAck {
		t.Fatalf("expected CONNECT_ACK from managed relay app websocket, got %s", FrameTypeName(ackFrame.Type))
	}
	var ack ConnectAckPayload
	if err := UnmarshalControlPayload(ackFrame.Payload, &ack); err != nil {
		t.Fatalf("unmarshal managed relay connect ack: %v", err)
	}
	return conn, ack.ChannelID
}

func sendManagedRelayProxyRequest(t *testing.T, conn *websocket.Conn, channelID uint32, request ProxyRequest) {
	t.Helper()
	payload, err := json.Marshal(request)
	if err != nil {
		t.Fatalf("marshal managed relay proxy request: %v", err)
	}
	frame := NewFrame(FrameData, channelID, payload)
	encoded, err := EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode managed relay proxy request: %v", err)
	}
	if err := conn.WriteMessage(websocket.BinaryMessage, encoded); err != nil {
		t.Fatalf("write managed relay proxy request: %v", err)
	}
}

func verifyManagedRelayProxyRoundTrip(t *testing.T, relayAddr string, instanceID string, path string) {
	t.Helper()
	conn, channelID := openManagedRelayAppConnection(t, relayAddr, instanceID, "managed-runtime-app-token")
	defer conn.Close()

	sendManagedRelayProxyRequest(t, conn, channelID, ProxyRequest{
		ID:     "req_roundtrip_" + strings.Trim(strings.ReplaceAll(path, "/", "_"), "_"),
		Method: "GET",
		Path:   path,
	})

	if err := conn.SetReadDeadline(time.Now().Add(5 * time.Second)); err != nil {
		t.Fatalf("set managed relay roundtrip read deadline: %v", err)
	}
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read managed relay roundtrip response: %v", err)
	}
	_ = conn.SetReadDeadline(time.Time{})
	frame, err := DecodeFrame(msg)
	if err != nil {
		t.Fatalf("decode managed relay roundtrip response: %v", err)
	}
	if frame.Type != FrameData {
		t.Fatalf("expected DATA from managed relay roundtrip, got %s", FrameTypeName(frame.Type))
	}
	var response ProxyResponse
	if err := json.Unmarshal(frame.Payload, &response); err != nil {
		t.Fatalf("unmarshal managed relay proxy response: %v", err)
	}
	if response.Status != http.StatusOK {
		t.Fatalf("managed relay proxy response status=%d body=%q", response.Status, response.Body)
	}
	bodyBytes, err := base64.StdEncoding.DecodeString(response.Body)
	if err != nil {
		t.Fatalf("decode managed relay proxy response body: %v", err)
	}
	if !strings.Contains(string(bodyBytes), `"endpoint":"status"`) {
		t.Fatalf("managed relay proxy response body=%s, want endpoint=status", string(bodyBytes))
	}
}
