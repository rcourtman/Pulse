package hostagent

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rs/zerolog"
)

func boundProxmoxPayload(t *testing.T) agentexec.ProxmoxGuestLifecyclePayload {
	t.Helper()
	payload := agentexec.ProxmoxGuestLifecyclePayload{RequestID: "attempt-pve-1", ActionID: "action-pve-1", Operation: "shutdown", GuestKind: "ct", VMID: 141, ExpectedStatus: "running", Timeout: 30}
	if err := agentexec.BindProxmoxGuestLifecyclePayload(&payload); err != nil {
		t.Fatal(err)
	}
	return payload
}

func TestProxmoxGuestLifecycleExecutesOnlyFixedCatalogAndNumericVMID(t *testing.T) {
	payload := boundProxmoxPayload(t)
	manager := newProxmoxGuestLifecycleManager()
	manager.now = func() time.Time { return time.Unix(100, 0).UTC() }
	var calls [][]string
	manager.run = func(_ context.Context, tool string, args ...string) ([]byte, error) {
		calls = append(calls, append([]string{tool}, args...))
		if len(calls) == 1 {
			return []byte("status: running\n"), nil
		}
		if len(calls) == 2 {
			return nil, nil
		}
		return []byte("status: stopped\n"), nil
	}
	result := manager.Apply(context.Background(), payload)
	want := [][]string{{"pct", "status", "141"}, {"pct", "shutdown", "141"}, {"pct", "status", "141"}}
	if !reflect.DeepEqual(calls, want) || result.ExecutionPhase != agentexec.ProxmoxGuestPhaseComplete || !result.MutationCompleted || !result.ReadbackRan {
		t.Fatalf("calls=%v result=%+v", calls, result)
	}
}

func TestProxmoxGuestLifecycleCancellationStopsMutationAndProducesBoundFailure(t *testing.T) {
	payload := boundProxmoxPayload(t)
	manager := newProxmoxGuestLifecycleManager()
	started := make(chan struct{})
	manager.run = func(ctx context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "status" {
			return []byte("status: running"), nil
		}
		close(started)
		<-ctx.Done()
		return nil, ctx.Err()
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan agentexec.ProxmoxGuestLifecycleResultPayload, 1)
	go func() { done <- manager.Apply(ctx, payload) }()
	<-started
	cancel()
	result := <-done
	if !result.MutationStarted || result.MutationCompleted || result.Error == "" {
		t.Fatalf("canceled result = %+v", result)
	}
}

func TestProxmoxGuestLifecycleTerminalReceiptReplaysWithoutSecondMutation(t *testing.T) {
	payload := boundProxmoxPayload(t)
	client := &CommandClient{agentID: "agent-pve", logger: zerolog.Nop(), activeCommands: make(map[string]context.CancelFunc)}
	receipts, err := operationreceipt.Open(filepath.Join(t.TempDir(), "receipts.db"), hostOperationReceiptConfig())
	if err != nil {
		t.Fatal(err)
	}
	defer receipts.Close()
	client.operationReceipts = receipts
	client.proxmoxGuestLifecycle = newProxmoxGuestLifecycleManager()
	mutations := 0
	client.proxmoxGuestLifecycle.run = func(_ context.Context, _ string, args ...string) ([]byte, error) {
		if args[0] == "status" {
			if mutations == 0 {
				return []byte("status: running"), nil
			}
			return []byte("status: stopped"), nil
		}
		if args[0] != "shutdown" || args[1] != strconv.Itoa(payload.VMID) {
			return nil, errors.New("unexpected arguments")
		}
		mutations++
		return nil, nil
	}
	upgrader := websocket.Upgrader{CheckOrigin: func(*http.Request) bool { return true }}
	serverConnections := make(chan *websocket.Conn)
	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, upgradeErr := upgrader.Upgrade(w, r, nil)
		if upgradeErr != nil {
			return
		}
		serverConnections <- conn
		<-release
		_ = conn.Close()
	}))
	defer server.Close()
	for attempt := 0; attempt < 2; attempt++ {
		remote, _, dialErr := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
		if dialErr != nil {
			t.Fatal(dialErr)
		}
		serverConn := <-serverConnections
		client.handleProxmoxGuestLifecycle(context.Background(), serverConn, payload)
		var message wsMessage
		if err := remote.ReadJSON(&message); err != nil {
			t.Fatal(err)
		}
		var result agentexec.ProxmoxGuestLifecycleResultPayload
		if message.Type != msgTypeProxmoxGuestLifecycleResult || json.Unmarshal(message.Payload, &result) != nil || !result.MutationCompleted {
			t.Fatalf("attempt %d message=%+v result=%+v", attempt, message, result)
		}
		_ = remote.Close()
		release <- struct{}{}
	}
	if mutations != 1 {
		t.Fatalf("mutations=%d, want 1", mutations)
	}
}
