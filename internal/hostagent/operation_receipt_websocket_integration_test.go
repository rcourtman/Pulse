package hostagent

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
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

func TestRealServerAndUnifiedAgentWebSocketExecutesAPTThroughFakeTypedManagersAndReplaysWithoutMutation(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "token" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	logger := zerolog.Nop()
	client := NewCommandClient(Config{PulseURL: httpServer.URL, APIToken: "token", StateDir: t.TempDir(), Logger: &logger}, "agent-apt", "host", "linux", "6")
	lease := newPackageManagerLease()
	updates := newPackageUpdateManager("linux", lease)
	cleanup := newStorageCleanupManager("linux", lease)

	const pendingSimulation = "Inst pulse-safe [1.0] (1.1 stable [amd64])\n"
	var mu sync.Mutex
	upgraded := false
	cleaned := false
	refreshCalls := 0
	upgradeCalls := 0
	cleanCalls := 0
	fakeRun := func(_ context.Context, _ []string, command string, args ...string) packageUpdateCommandResult {
		if command == "dpkg" && strings.Join(args, " ") == "--audit" {
			return packageUpdateCommandResult{}
		}
		if command != "apt-get" {
			return packageUpdateCommandResult{err: errors.New("unexpected fake command")}
		}
		mu.Lock()
		defer mu.Unlock()
		joined := strings.Join(args, " ")
		switch {
		case joined == "update":
			refreshCalls++
		case strings.Contains(joined, "-y --no-remove"):
			upgradeCalls++
			upgraded = true
		case joined == "clean":
			cleanCalls++
			cleaned = true
		case strings.Contains(joined, "-s -o Debug::NoLocking=1 upgrade"):
			if !upgraded {
				return packageUpdateCommandResult{stdout: pendingSimulation}
			}
		default:
			return packageUpdateCommandResult{err: errors.New("unexpected fake apt catalog call")}
		}
		return packageUpdateCommandResult{}
	}
	updates.run = fakeRun
	updates.lookPath = func(string) (string, error) { return "/fake/apt-get", nil }
	updates.stat = func(string) (os.FileInfo, error) { return nil, os.ErrNotExist }
	updates.cacheTTL = 0
	cleanup.run = fakeRun
	cleanup.lookPath = func(string) (string, error) { return "/fake/apt-get", nil }
	cleanup.cacheTTL = 0
	cleanup.scan = func() (agentexec.HostStorageCleanupSnapshot, error) {
		mu.Lock()
		defer mu.Unlock()
		if cleaned {
			return agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("d", 64), ReclaimableBytes: 8 * 1024 * 1024}, nil
		}
		return agentexec.HostStorageCleanupSnapshot{Fingerprint: "sha256:" + strings.Repeat("c", 64), ReclaimableBytes: 512 * 1024 * 1024}, nil
	}
	client.packageUpdates = updates
	client.storageCleanup = cleanup

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- client.Run(ctx) }()
	defer func() {
		cancel()
		_ = client.Close()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("client did not stop")
		}
	}()
	deadline := time.Now().Add(3 * time.Second)
	for !server.IsAgentConnected("agent-apt") && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if !server.IsAgentConnected("agent-apt") {
		t.Fatal("real command client did not connect")
	}

	updateReq := agentexec.HostUpdatePayload{RequestID: "update.dispatch.1", ActionID: "update", Operation: agentexec.HostUpdateOperationInstall, ExpectedInventoryHash: aptUpgradeInventoryHash(pendingSimulation), Timeout: 5}
	if err := agentexec.BindHostUpdatePayload(&updateReq); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.ExecuteHostUpdate(context.Background(), "agent-apt", updateReq)
		if err != nil || result == nil || !result.Success || result.Verification != agentexec.HostUpdateVerificationVerified {
			t.Fatalf("update replay %d result=%+v err=%v", i, result, err)
		}
	}

	cleanupReq := agentexec.HostStorageCleanupPayload{RequestID: "cleanup.dispatch.1", ActionID: "cleanup", Operation: agentexec.HostStorageCleanupOperationPackageCache, ExpectedFingerprint: "sha256:" + strings.Repeat("c", 64), Timeout: 5}
	if err := agentexec.BindHostStorageCleanupPayload(&cleanupReq); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		result, err := server.ExecuteHostStorageCleanup(context.Background(), "agent-apt", cleanupReq)
		if err != nil || result == nil || !result.Success || result.Verification != agentexec.HostStorageCleanupVerificationVerified {
			t.Fatalf("cleanup replay %d result=%+v err=%v", i, result, err)
		}
	}
	mu.Lock()
	defer mu.Unlock()
	if refreshCalls != 1 || upgradeCalls != 1 || cleanCalls != 1 {
		t.Fatalf("fake mutation catalog calls: refresh=%d upgrade=%d clean=%d", refreshCalls, upgradeCalls, cleanCalls)
	}
}
