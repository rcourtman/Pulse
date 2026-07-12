package actionlifecycle

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/agentexec"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	unified "github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
)

type serverQueryReconciler struct {
	server                       *agentexec.Server
	identity                     operationreceipt.Identity
	executeCalls, reconcileCalls int
}

func (e *serverQueryReconciler) ExecuteAction(context.Context, unified.ActionAuditRecord) (*unified.ExecutionResult, error) {
	e.executeCalls++
	return nil, context.DeadlineExceeded
}
func (e *serverQueryReconciler) BindActionDispatch(_ context.Context, _ unified.ActionAuditRecord, a unified.ActionDispatchAttempt) (unified.ActionDispatchAttempt, error) {
	e.identity.AttemptID = a.ID
	e.identity.ActionID = a.ActionID
	return unified.BindActionDispatchAttempt(a, unified.ActionDispatchBinding{OperationKind: e.identity.OperationKind, OperationVersion: e.identity.OperationVersion, RequestDigest: e.identity.RequestDigest, AgentID: e.identity.AgentID})
}
func (e *serverQueryReconciler) ReconcileActionDispatch(ctx context.Context, _ unified.ActionAuditRecord, _ unified.ActionDispatchAttempt) (*unified.ExecutionResult, unified.ActionDispatchReceipt, bool, error) {
	e.reconcileCalls++
	result, err := e.server.QueryAgentOperation(ctx, e.identity.AgentID, e.identity)
	if err != nil {
		return nil, unified.ActionDispatchReceipt{}, false, err
	}
	return nil, unified.ActionDispatchReceipt{}, result.Status == operationreceipt.QueryFoundTerminal, nil
}

func TestAuthenticatedServerNotFoundRecoveryStaysQueryOnlyAndReceiptPending(t *testing.T) {
	server := agentexec.NewServer(func(token, agent, host string) bool { return token == "ok" })
	httpServer := httptest.NewServer(http.HandlerFunc(server.HandleWebSocket))
	defer httpServer.Close()
	wsURL := "ws" + strings.TrimPrefix(httpServer.URL, "http")
	headers := http.Header{"Origin": []string{httpServer.URL}}
	conn, _, err := websocket.DefaultDialer.Dial(wsURL, headers)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	register, _ := agentexec.NewMessage(agentexec.MsgTypeAgentRegister, "", agentexec.AgentRegisterPayload{AgentID: "agent-1", Hostname: "host", Version: "6", Platform: "linux", Token: "ok", OperationReceiptVersion: operationreceipt.ProtocolVersion})
	if err := conn.WriteJSON(register); err != nil {
		t.Fatal(err)
	}
	var registered agentexec.Message
	if err := conn.ReadJSON(&registered); err != nil {
		t.Fatal(err)
	}
	queryCount := make(chan struct{}, 4)
	go func() {
		for {
			var message agentexec.Message
			if err := conn.ReadJSON(&message); err != nil {
				return
			}
			if message.Type != agentexec.MsgTypeOperationQuery {
				continue
			}
			reply, _ := agentexec.NewMessage(agentexec.MsgTypeOperationQueryResult, message.ID, operationreceipt.QueryResult{Version: operationreceipt.ProtocolVersion, Status: operationreceipt.QueryNotFound})
			if err := conn.WriteJSON(reply); err != nil {
				return
			}
			queryCount <- struct{}{}
		}
	}()
	digest, _ := operationreceipt.DigestCanonicalJSON(map[string]string{"intent": "fake typed mutation"})
	executor := &serverQueryReconciler{server: server, identity: operationreceipt.Identity{OperationKind: "fake.typed", OperationVersion: 1, RequestDigest: digest, AgentID: "agent-1"}}
	store := unified.NewMemoryStore()
	now := time.Now().UTC()
	service := serviceForStore(t, store, testResource(now, unified.ApprovalNone), executor)
	plan, err := service.Plan(context.Background(), "default", restartRequest(), testActionActor("requester", "default"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Execute(context.Background(), "default", plan.ActionID, testActionActor("operator", "default"), "initial"); err == nil {
		t.Fatal("expected unresolved initial transport")
	}
	baseline := executor.executeCalls
	for n := 0; n < 2; n++ {
		records, err := service.RecoverExecutingActions(context.Background(), "default", "system:recovery", 10)
		if err != nil || len(records) != 1 || records[0].State != unified.ActionStateExecuting {
			t.Fatalf("pass=%d records=%#v err=%v", n, records, err)
		}
		select {
		case <-queryCount:
		case <-time.After(2 * time.Second):
			t.Fatal("authenticated query not observed")
		}
	}
	if executor.executeCalls != baseline || executor.reconcileCalls != 2 {
		t.Fatalf("execute=%d baseline=%d reconcile=%d", executor.executeCalls, baseline, executor.reconcileCalls)
	}
	attempt, found, err := store.GetActionDispatchAttempt(plan.ActionID)
	if err != nil || !found || attempt.State != unified.ActionDispatchReceiptPending {
		t.Fatalf("attempt=%#v found=%v err=%v", attempt, found, err)
	}
	if _, found, err := store.GetActionDispatchReceipt(attempt.ID); err != nil || found {
		t.Fatalf("receipt found=%v err=%v", found, err)
	}
	audit, found, err := store.GetActionAudit(plan.ActionID)
	if err != nil || !found || audit.State != unified.ActionStateExecuting || audit.Result != nil {
		t.Fatalf("audit=%#v found=%v err=%v", audit, found, err)
	}
}
