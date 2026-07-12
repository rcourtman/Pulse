package hostagent

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rs/zerolog"
)

func TestRealServerAndUnifiedAgentWebSocketQueriesFakeTypedOperationWithoutPackageManager(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "token" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: httpServer.URL, APIToken: "token", StateDir: t.TempDir(), Logger: &logger}, "agent-real", "host", "linux", "6")
	if client.packageUpdates != nil || client.storageCleanup != nil {
		t.Fatal("test must not install package-manager adapters")
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected("agent-real") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected("agent-real") {
		t.Fatal("real command client did not connect")
	}
	digest, _ := operationreceipt.DigestCanonicalJSON(map[string]string{"fake": "typed-read-only-proof"})
	identity := operationreceipt.Identity{AttemptID: "fake.dispatch.1", ActionID: "fake", OperationKind: "fake.typed", OperationVersion: 1, RequestDigest: digest, AgentID: "agent-real"}
	result, err := server.QueryAgentOperation(context.Background(), "agent-real", identity)
	if err != nil || result.Status != operationreceipt.QueryNotFound {
		t.Fatalf("query=%+v err=%v", result, err)
	}
	if _, fresh, err := client.operationReceipts.Admit(identity); err != nil || !fresh {
		t.Fatalf("admit fresh=%v err=%v", fresh, err)
	}
	if _, err := client.operationReceipts.MarkStarted(identity); err != nil {
		t.Fatal(err)
	}
	result, err = server.QueryAgentOperation(context.Background(), "agent-real", identity)
	if err != nil || result.Status != operationreceipt.QueryFoundInterrupted {
		t.Fatalf("interrupted query=%+v err=%v", result, err)
	}
	cancel()
	_ = client.Close()
	select {
	case err := <-done:
		if err != nil && !strings.Contains(err.Error(), "context canceled") {
			t.Fatal(err)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("client did not stop")
	}
}
