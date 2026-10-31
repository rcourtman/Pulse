package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/agentcapabilities"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/internal/monitoring"
	internalauth "github.com/rcourtman/pulse-go-rewrite/pkg/auth"
)

const agentEventLifecycleToken = "synthetic-event-lifecycle-token.12345678"
const agentEventControlToken = "synthetic-event-control-token.12345678"

type agentLifecycleStream struct {
	reader *bufio.Reader
}

func openAgentLifecycleStream(t *testing.T, server *httptest.Server, raw, transport string) *agentLifecycleStream {
	t.Helper()
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, server.URL+"/api/agent/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	if transport == "session" {
		req.AddCookie(&http.Cookie{Name: cookieNameSession, Value: raw})
	} else if transport == "bearer" {
		req.Header.Set("Authorization", "Bearer "+raw)
	} else {
		req.Header.Set("X-API-Token", raw)
	}
	resp, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { resp.Body.Close() })
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("stream admission = %d: %s", resp.StatusCode, body)
	}
	stream := &agentLifecycleStream{reader: bufio.NewReader(resp.Body)}
	if first := stream.next(t); first.Kind != AgentEventStreamConnected {
		t.Fatalf("first frame = %q, want connected", first.Kind)
	}
	return stream
}

func (s *agentLifecycleStream) read() (AgentEvent, error) {
	for {
		line, err := s.reader.ReadString('\n')
		if err != nil {
			return AgentEvent{}, err
		}
		if strings.HasPrefix(line, "data: ") {
			var event AgentEvent
			err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event)
			return event, err
		}
	}
}

func (s *agentLifecycleStream) next(t *testing.T) AgentEvent {
	t.Helper()
	for {
		event, err := s.read()
		if err != nil {
			t.Fatalf("read stream frame: %v", err)
		}
		if event.Kind != AgentEventHeartbeat {
			return event
		}
	}
}

func (s *agentLifecycleStream) wantClosed(t *testing.T) {
	t.Helper()
	for {
		event, err := s.read()
		if errors.Is(err, io.EOF) {
			return
		}
		if err != nil {
			t.Fatalf("stream did not close cleanly after authority withdrawal: %v", err)
		}
		// A heartbeat admitted before the fixture mutation may already be in
		// flight. It must not hide a product frame or substitute for EOF.
		if event.Kind != AgentEventHeartbeat {
			t.Fatalf("withdrawn credential still received %q", event.Kind)
		}
	}
}

func newAgentLifecycleRouter(t *testing.T, heartbeat time.Duration) (*Router, *config.Config, *httptest.Server) {
	t.Helper()
	primary := newTokenRecord(t, agentEventLifecycleToken, []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil)
	control := newTokenRecord(t, agentEventControlToken, []string{config.ScopeWildcard}, nil)
	cfg := newTestConfigWithTokens(t, primary, control)
	router := NewRouter(cfg, nil, nil, nil, nil, "test")
	cleanupTestRouter(t, router)
	router.agentEventBroadcaster.heartbeatInterval = heartbeat
	server := httptest.NewServer(router.Handler())
	t.Cleanup(server.Close)
	return router, cfg, server
}

func TestAgentEventTokenLifecycleRejectsWithdrawnAuthority(t *testing.T) {
	for _, transport := range []string{"header", "bearer"} {
		for _, change := range []string{"revoked", "expired", "read_scope_removed", "org_rebound", "identity_replaced", "principal_replaced", "invalid_role"} {
			t.Run(transport+"/"+change, func(t *testing.T) {
				router, cfg, server := newAgentLifecycleRouter(t, time.Hour)
				primary := openAgentLifecycleStream(t, server, agentEventLifecycleToken, transport)
				control := openAgentLifecycleStream(t, server, agentEventControlToken, transport)
				config.Mu.Lock()
				switch change {
				case "revoked":
					cfg.RemoveAPIToken(cfg.APITokens[0].ID)
				case "expired":
					past := time.Now().UTC().Add(-time.Second)
					cfg.APITokens[0].ExpiresAt = &past
				case "read_scope_removed":
					cfg.APITokens[0].Scopes = []string{config.ScopeSettingsRead}
				case "org_rebound":
					cfg.APITokens[0].OrgID = "another-org"
				case "identity_replaced":
					cfg.APITokens[0].ID = "replacement-identity"
				case "principal_replaced":
					cfg.APITokens[0].Metadata = map[string]string{apiTokenMetadataOwnerUserID: "replacement-owner"}
				case "invalid_role":
					cfg.APITokens[0].Metadata = map[string]string{internalauth.RuntimeRoleMetadataKey: internalauth.RuntimeRoleActionRunner}
				}
				config.Mu.Unlock()

				router.agentEventBroadcaster.PublishActionCompletedRecord(agentDisclosureRecord(t, false))
				primary.wantClosed(t)
				event := control.next(t)
				if event.Kind != AgentEventActionCompleted {
					t.Fatalf("independent subscriber lost action: %q", event.Kind)
				}
				body, _ := json.Marshal(event)
				assertAgentDisclosureWire(t, body, false)
				if router.agentEventBroadcaster.SubscriberCount() != 1 {
					t.Fatal("withdrawn stream did not release its subscription")
				}
			})
		}
	}
}

func TestAgentEventTokenLifecycleClosesIdleStream(t *testing.T) {
	for _, change := range []string{"revoke", "natural_expiry"} {
		t.Run(change, func(t *testing.T) {
			_, cfg, server := newAgentLifecycleRouter(t, 10*time.Millisecond)
			primary := openAgentLifecycleStream(t, server, agentEventLifecycleToken, "header")
			config.Mu.Lock()
			if change == "revoke" {
				cfg.RemoveAPIToken(cfg.APITokens[0].ID)
			} else {
				expires := time.Now().UTC().Add(30 * time.Millisecond)
				cfg.APITokens[0].ExpiresAt = &expires
			}
			config.Mu.Unlock()
			primary.wantClosed(t)
		})
	}
}

func TestAgentEventTokenLifecycleResolvesLiveTenantInventory(t *testing.T) {
	const orgID = "tenant-a"
	for _, change := range []string{"replaced", "removed", "same_credential_in_new_owner", "changes_during_validation", "no_global_fallback"} {
		t.Run(change, func(t *testing.T) {
			record := newTokenRecord(t, agentEventLifecycleToken, []string{config.ScopeMonitoringRead, config.ScopeAIExecute}, nil)
			record.OrgID = orgID
			base := &config.Config{}
			mtp := config.NewMultiTenantPersistence(t.TempDir())
			if _, err := mtp.GetPersistence(orgID); err != nil {
				t.Fatal(err)
			}
			makeManager := func(cfg *config.Config) *monitoring.MultiTenantMonitor {
				manager := monitoring.NewMultiTenantMonitor(base, mtp, nil)
				monitor := &monitoring.Monitor{}
				setUnexportedField(t, monitor, "config", cfg)
				setUnexportedField(t, manager, "monitors", map[string]*monitoring.Monitor{orgID: monitor})
				t.Cleanup(manager.Stop)
				return manager
			}
			old := makeManager(&config.Config{APITokens: []config.APITokenRecord{record}})
			nextCfg := &config.Config{}
			if change == "same_credential_in_new_owner" {
				nextCfg.APITokens = []config.APITokenRecord{record.Clone()}
				nextCfg.APITokens[0].Scopes = []string{config.ScopeMonitoringRead}
			}
			next := makeManager(nextCfg)
			req := httptest.NewRequest(http.MethodGet, "/api/agent/events", nil)
			req.Header.Set("X-Pulse-Org-ID", orgID)
			req.Header.Set("X-API-Token", agentEventLifecycleToken)
			req = extractAndStoreAuthContext(base, old, req)
			req = req.WithContext(context.WithValue(req.Context(), OrgIDContextKey, orgID))
			source := req.Context().Value(tenantTokenSourceContextKey).(tenantTokenSource)
			currentManager := old
			resolutions := 0
			source.resolveManager = func() *monitoring.MultiTenantMonitor {
				resolutions++
				if change == "changes_during_validation" && resolutions%2 == 0 {
					return next
				}
				return currentManager
			}
			req = req.WithContext(context.WithValue(req.Context(), tenantTokenSourceContextKey, source))
			validate := agentEventTokenRevalidator(base, req)
			if change != "changes_during_validation" && !validate() {
				t.Fatal("initial tenant credential rejected")
			}
			currentManager = next
			if change == "removed" {
				currentManager = nil
			}
			if change == "changes_during_validation" {
				currentManager = old
			}
			if change == "no_global_fallback" {
				base.APITokens = []config.APITokenRecord{record.Clone()}
			}
			want := change == "same_credential_in_new_owner"
			if got := validate(); got != want {
				t.Fatalf("live tenant validation = %v, want %v", got, want)
			}
			if want && requestCanReadAgentCommandPayloads(req) {
				t.Fatal("live owner scope removal left stale command disclosure")
			}
		})
	}
}

func TestAgentEventTokenLifecyclePreservesSessionDisclosure(t *testing.T) {
	router, _, server := newAgentLifecycleRouter(t, time.Hour)
	const session = "synthetic-event-browser-session"
	GetSessionStore().CreateSession(session, time.Hour, "fixture", "127.0.0.1", "operator")
	t.Cleanup(func() { GetSessionStore().DeleteSession(session) })
	stream := openAgentLifecycleStream(t, server, session, "session")
	router.agentEventBroadcaster.PublishActionCompletedRecord(agentDisclosureRecord(t, false))
	body, _ := json.Marshal(stream.next(t))
	assertAgentDisclosureWire(t, body, false)
}

type agentLifecycleMCPClient struct {
	client *http.Client
	ready  chan struct{}
}

func (c agentLifecycleMCPClient) Do(req *http.Request) (*http.Response, error) {
	resp, err := c.client.Do(req)
	close(c.ready)
	return resp, err
}

// Use the same SSE-to-MCP consumer the shipped pulse-mcp adapter calls, but
// against the real registered API route, not a fixture-only SSE producer.
func TestAgentEventTokenLifecycleMCPConsumer(t *testing.T) {
	router, cfg, server := newAgentLifecycleRouter(t, time.Hour)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	ready := make(chan struct{})
	notifications := make(chan agentcapabilities.JSONRPCRequest, 4)
	done := make(chan error, 1)
	go func() {
		done <- agentcapabilities.StreamMCPEventNotifications(ctx, agentLifecycleMCPClient{server.Client(), ready}, server.URL, agentcapabilities.AgentEventsPath, agentEventLifecycleToken, func(n agentcapabilities.JSONRPCRequest) error {
			notifications <- n
			return nil
		})
	}()
	select {
	case <-ready:
	case <-ctx.Done():
		t.Fatal("MCP stream never connected")
	}
	router.agentEventBroadcaster.PublishFindingCreated(AgentEventFindingCreatedPayload{FindingID: "before-revocation"})
	select {
	case n := <-notifications:
		if n.Method != "notifications/finding.created" || !strings.Contains(string(n.Params), "before-revocation") {
			t.Fatalf("valid MCP notification changed: %+v", n)
		}
	case <-ctx.Done():
		t.Fatal("valid MCP notification never arrived")
	}
	config.Mu.Lock()
	cfg.RemoveAPIToken(cfg.APITokens[0].ID)
	config.Mu.Unlock()
	router.agentEventBroadcaster.PublishActionCompletedRecord(agentDisclosureRecord(t, false))
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("withdrawn MCP stream did not reach clean EOF: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("withdrawn MCP stream stayed open")
	}
	if len(notifications) != 0 {
		t.Fatal("MCP forwarded an event after revocation")
	}
	err := agentcapabilities.StreamMCPEventNotifications(ctx, server.Client(), server.URL, agentcapabilities.AgentEventsPath, agentEventLifecycleToken, nil)
	if err == nil || !strings.Contains(err.Error(), "status 401") {
		t.Fatalf("MCP reconnect did not retain revocation: %v", err)
	}
}
