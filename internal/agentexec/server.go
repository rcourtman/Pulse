package agentexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"github.com/rcourtman/pulse-go-rewrite/internal/operationreceipt"
	"github.com/rcourtman/pulse-go-rewrite/internal/securityutil"
	"github.com/rcourtman/pulse-go-rewrite/internal/unifiedresources"
	"github.com/rs/zerolog/log"
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return isAllowedWebSocketOrigin(r)
	},
}

var (
	jsonMarshal      = json.Marshal
	writeTextMessage = func(conn *websocket.Conn, data []byte) error {
		return conn.WriteMessage(websocket.TextMessage, data)
	}
	defaultPingInterval   = 5 * time.Second
	pingWriteWait         = 5 * time.Second
	readFileTimeout       = 30 * time.Second
	operationQueryTimeout = 10 * time.Second

	errServerShuttingDown = errors.New("agent execution server is shutting down")
)

const maxWebSocketMessageBytes int64 = 1 << 20 // 1 MiB

const (
	maxAgentIDLength                          = 128
	maxRequestIDLength                        = 128
	maxExecuteCommandLength                   = 32 * 1024
	maxTargetIDLength                         = 256
	maxExecuteCommandTimeoutSeconds           = 3600
	defaultMaxWebSocketConnectionsPerIP       = 128
	defaultReadFileMaxBytes             int64 = 1 << 20  // 1 MiB
	maxReadFileMaxBytes                 int64 = 10 << 20 // 10 MiB
	maxReadFilePathLength                     = 4096
)

var safeTargetIDPattern = regexp.MustCompile(`^[a-zA-Z0-9._-]+$`)
var hostUpdateInventoryHashPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)
var hostStorageCleanupFingerprintPattern = regexp.MustCompile(`^sha256:[a-f0-9]{64}$`)

// Server manages WebSocket connections from agents
type Server struct {
	mu                                 sync.RWMutex
	agents                             map[string]*agentConn                           // organizationID + agentID -> connection
	pendingActionRunners               map[string]*agentConn                           // organizationID + agentID -> exact prepared runner transport
	pendingReqs                        map[string]chan CommandResultPayload            // scoped request key -> response channel
	pendingHostStorageCleanups         map[string]chan HostStorageCleanupResultPayload // scoped request key -> typed storage-cleanup response
	pendingHostUpdates                 map[string]chan HostUpdateResultPayload         // scoped request key -> typed host-update response
	pendingProxmoxGuestLifecycles      map[string]chan ProxmoxGuestLifecycleResultPayload
	pendingDockerContainerLifecycles   map[string]chan DockerContainerLifecycleResultPayload
	pendingDockerContainerUpdates      map[string]chan DockerContainerUpdateResultPayload
	pendingDockerContainerObservations map[string]chan DockerContainerObservationResultPayload
	pendingActionPreflights            map[string]chan ActionPreflightResultPayload
	pendingHostOperations              map[string]pendingHostOperation // scoped request key -> exact typed APT operation/query identity
	pendingOperationQueries            map[string]pendingOperationQuery
	deploySubs                         map[string]chan DeployProgressPayload // deploySubKey(agentID, jobID) -> progress subscriber
	admitToken                         AgentRegistrationValidator
	validateSession                    AgentSessionValidator
	commandPolicy                      *CommandPolicy
	ipConnCounts                       map[string]int
	maxConnsPerIP                      int
	shutdown                           chan struct{}
	shutdownOnce                       sync.Once
	pingInterval                       time.Duration
	commandAuthorizationVerifier       func(CommandAuthorizationRequest) error
	newCommandApprovalGrant            func([]byte, string, ExecuteCommandPayload, time.Time, time.Duration) (*CommandApprovalGrant, error)
	now                                func() time.Time
	agentRegisteredNotifier            func(AgentAdmission)
	actionRunnerAdmissionTombstones    map[string]time.Time
}

const defaultOrganizationID = "default"

type organizationContextKey struct{}

// AgentAdmission is the immutable server-owned identity of an admitted command
// session. The raw bearer token is deliberately not retained after
// registration.
type AgentAdmission struct {
	OrganizationID    string
	TokenID           string
	AgentID           string
	Hostname          string
	RuntimeRole       string
	ActionCapability  string
	ActivationPending bool
}

// AgentRegistrationValidator authenticates and binds a registration to one
// organization, token, agent identity, and hostname.
type AgentRegistrationValidator func(token string, agentID string, hostname string) (AgentAdmission, bool)

// AgentSessionValidator revalidates the non-secret admission immediately
// before the server treats a socket as connected or dispatches work to it.
type AgentSessionValidator func(AgentAdmission) bool

// CommandAuthorizationRequest is the complete server-side approval scope
// verified and consumed immediately before an approval grant is signed.
type CommandAuthorizationRequest struct {
	ApprovalID string
	OrgID      string
	ActionID   string
	AgentID    string
	Command    string
	TargetType string
	TargetID   string
}

type agentConn struct {
	conn             *websocket.Conn
	agent            ConnectedAgent
	admission        AgentAdmission
	sessionKey       string
	approvalGrantKey []byte
	writeMu          sync.Mutex
	done             chan struct{}
	doneOnce         sync.Once
}

type pendingHostOperation struct {
	actionID  string
	operation string
	identity  operationreceipt.Identity
	subjectID string
}

type pendingOperationQuery struct {
	identity operationreceipt.Identity
	ch       chan operationreceipt.QueryResult
}

func (ac *agentConn) signalDone() {
	ac.doneOnce.Do(func() {
		defer func() {
			// Some call sites/tests may have already closed done directly.
			_ = recover()
		}()
		close(ac.done)
	})
}

// NewServer creates a new agent execution server.
//
// validateToken is invoked during WebSocket agent registration with the token,
// the agent-claimed agentID, and the hostname from the register payload. The
// hostname is provided because enrollment-minted tokens bind to bound_hostname
// rather than to a predictable agent ID: agents derive their runtime agentID
// from /etc/machine-id (or an override), which the server cannot know when it
// mints the token. Matching on hostname preserves the trust boundary ("the
// bearer is running on the bound host") without requiring the agent to know a
// server-canonical ID format. See internal/api/router.go for the production
// validator.
func NewServer(validateToken func(token string, agentID string, hostname string) bool) *Server {
	if validateToken == nil {
		panic("agentexec: validateToken is required")
	}

	return NewServerWithAdmissionValidator(func(token string, agentID string, hostname string) (AgentAdmission, bool) {
		if !validateToken(token, agentID, hostname) {
			return AgentAdmission{}, false
		}
		return AgentAdmission{
			OrganizationID: defaultOrganizationID,
			AgentID:        strings.TrimSpace(agentID),
			Hostname:       strings.TrimSpace(hostname),
		}, true
	}, nil)
}

// NewServerWithAdmissionValidator creates a command server whose sessions are
// tenant-scoped and can be invalidated after registration without retaining
// bearer tokens in memory.
func NewServerWithAdmissionValidator(admit AgentRegistrationValidator, validateSession AgentSessionValidator) *Server {
	if admit == nil {
		panic("agentexec: admission validator is required")
	}

	return &Server{
		agents:                             make(map[string]*agentConn),
		pendingActionRunners:               make(map[string]*agentConn),
		actionRunnerAdmissionTombstones:    make(map[string]time.Time),
		pendingReqs:                        make(map[string]chan CommandResultPayload),
		pendingHostStorageCleanups:         make(map[string]chan HostStorageCleanupResultPayload),
		pendingHostUpdates:                 make(map[string]chan HostUpdateResultPayload),
		pendingProxmoxGuestLifecycles:      make(map[string]chan ProxmoxGuestLifecycleResultPayload),
		pendingDockerContainerLifecycles:   make(map[string]chan DockerContainerLifecycleResultPayload),
		pendingDockerContainerUpdates:      make(map[string]chan DockerContainerUpdateResultPayload),
		pendingDockerContainerObservations: make(map[string]chan DockerContainerObservationResultPayload),
		pendingActionPreflights:            make(map[string]chan ActionPreflightResultPayload),
		pendingHostOperations:              make(map[string]pendingHostOperation),
		pendingOperationQueries:            make(map[string]pendingOperationQuery),
		deploySubs:                         make(map[string]chan DeployProgressPayload),
		admitToken:                         admit,
		validateSession:                    validateSession,
		commandPolicy:                      DefaultPolicy(),
		ipConnCounts:                       make(map[string]int),
		maxConnsPerIP:                      defaultMaxWebSocketConnectionsPerIP,
		shutdown:                           make(chan struct{}),
		pingInterval:                       defaultPingInterval,
		newCommandApprovalGrant:            NewCommandApprovalGrant,
		now:                                time.Now,
	}
}

func actionRunnerAdmissionTombstoneKey(admission AgentAdmission) string {
	return strings.Join([]string{
		normalizeOrganizationID(admission.OrganizationID),
		strings.TrimSpace(admission.TokenID),
		strings.TrimSpace(admission.AgentID),
		unifiedresources.NormalizeFullHostname(admission.Hostname),
		strings.TrimSpace(admission.RuntimeRole),
		strings.TrimSpace(admission.ActionCapability),
	}, "\x00")
}

// TombstoneActionRunnerAdmission prevents an already-admitted prepared socket
// from registering after its credential has been durably cancelled. The
// tombstone is exact and bounded to the preparation window.
func (s *Server) TombstoneActionRunnerAdmission(admission AgentAdmission, until time.Time) bool {
	if s == nil || strings.TrimSpace(admission.TokenID) == "" || strings.TrimSpace(admission.AgentID) == "" ||
		strings.TrimSpace(admission.Hostname) == "" ||
		strings.TrimSpace(admission.RuntimeRole) != RuntimeRoleActionRunner ||
		strings.TrimSpace(admission.ActionCapability) != ActionCapabilityTypedV1 {
		return false
	}
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	maximum := now.Add(10 * time.Minute)
	if !until.After(now) || until.After(maximum) {
		until = maximum
	}
	key := actionRunnerAdmissionTombstoneKey(admission)
	s.mu.Lock()
	if s.actionRunnerAdmissionTombstones == nil {
		s.actionRunnerAdmissionTombstones = make(map[string]time.Time)
	}
	for existingKey, expiry := range s.actionRunnerAdmissionTombstones {
		if !expiry.After(now) {
			delete(s.actionRunnerAdmissionTombstones, existingKey)
		}
	}
	s.actionRunnerAdmissionTombstones[key] = until
	sessionKey := agentSessionKey(admission.OrganizationID, admission.AgentID)
	var invalidated *agentConn
	if existing, ok := s.pendingActionRunners[sessionKey]; ok && actionRunnerAdmissionTombstoneKey(existing.admission) == key {
		delete(s.pendingActionRunners, sessionKey)
		invalidated = existing
	}
	s.mu.Unlock()
	if invalidated != nil {
		invalidated.signalDone()
		_ = invalidated.conn.Close()
	}
	return true
}

// WithOrganizationID scopes command-session lookup and dispatch to a tenant.
// Empty values normalize to the single-tenant default for compatibility.
func WithOrganizationID(ctx context.Context, organizationID string) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithValue(ctx, organizationContextKey{}, normalizeOrganizationID(organizationID))
}

func normalizeOrganizationID(organizationID string) string {
	if organizationID = strings.TrimSpace(organizationID); organizationID != "" {
		return organizationID
	}
	return defaultOrganizationID
}

func organizationIDFromContext(ctx context.Context) string {
	if ctx != nil {
		if organizationID, ok := ctx.Value(organizationContextKey{}).(string); ok {
			return normalizeOrganizationID(organizationID)
		}
	}
	return defaultOrganizationID
}

// OrganizationServer is a tenant-pinned view of a command server. It exists
// for consumers whose interface cannot carry the request context through
// discovery and dispatch as separate calls (for example, long-lived per-tenant
// Assistant services).
type OrganizationServer struct {
	server         *Server
	organizationID string
}

// ForOrganization returns a command-server view that can only discover and
// dispatch sessions admitted to organizationID.
func (s *Server) ForOrganization(organizationID string) *OrganizationServer {
	return &OrganizationServer{
		server:         s,
		organizationID: normalizeOrganizationID(organizationID),
	}
}

func (s *OrganizationServer) GetConnectedAgents() []ConnectedAgent {
	if s == nil || s.server == nil {
		return nil
	}
	return s.server.GetConnectedAgentsForOrganization(s.organizationID)
}

func (s *OrganizationServer) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	if s == nil || s.server == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	return s.server.ExecuteCommand(WithOrganizationID(ctx, s.organizationID), agentID, cmd)
}

func agentSessionKey(organizationID, agentID string) string {
	organizationID = normalizeOrganizationID(organizationID)
	agentID = strings.TrimSpace(agentID)
	if organizationID == defaultOrganizationID {
		// Preserve the historical key for direct single-tenant users and tests.
		return agentID
	}
	return organizationID + "\x00" + agentID
}

func connectionSessionKey(ac *agentConn) string {
	if ac == nil {
		return ""
	}
	if strings.TrimSpace(ac.sessionKey) != "" {
		return ac.sessionKey
	}
	return agentSessionKey(ac.admission.OrganizationID, ac.agent.AgentID)
}

// SetCommandAuthorizationVerifier installs the server-owned authorization
// consumer used for approval-gated arbitrary commands.
func (s *Server) SetCommandAuthorizationVerifier(verifier func(CommandAuthorizationRequest) error) {
	if s == nil {
		return
	}
	s.commandAuthorizationVerifier = verifier
}

// SetAgentRegisteredNotifier installs a callback fired after an agent
// completes registration (including a reconnect that replaces an existing
// connection). Durable-dispatch recovery hangs off this: receipt-pending
// reconciliation can only query an agent while it is connected, so the
// registration itself is the recovery trigger. The callback runs on its own
// goroutine because the query response can only be read once this server
// enters the connection's read loop.
func (s *Server) SetAgentRegisteredNotifier(notify func(agentID string)) {
	if s == nil {
		return
	}
	if notify == nil {
		s.agentRegisteredNotifier = nil
		return
	}
	s.agentRegisteredNotifier = func(admission AgentAdmission) {
		notify(admission.AgentID)
	}
}

// SetAgentAdmissionNotifier installs the tenant-aware registration callback.
func (s *Server) SetAgentAdmissionNotifier(notify func(AgentAdmission)) {
	if s == nil {
		return
	}
	s.agentRegisteredNotifier = notify
}

func (s *Server) isShuttingDown() bool {
	select {
	case <-s.shutdown:
		return true
	default:
		return false
	}
}

func pendingRequestKey(agentID, requestID string) string {
	return agentID + "\x00" + requestID
}

func (s *Server) connectionForOrganization(organizationID, agentID string) (*agentConn, bool) {
	if s == nil {
		return nil, false
	}
	key := agentSessionKey(organizationID, agentID)
	s.mu.RLock()
	ac, ok := s.agents[key]
	s.mu.RUnlock()
	if !ok {
		return nil, false
	}
	// Prepared action runners live only in pendingActionRunners. Membership in
	// the active map is the immutable dispatch-authority decision; do not read
	// or mutate admission state outside the server lock during promotion.
	if s.validateSession == nil || s.validateSession(ac.admission) {
		return ac, true
	}

	// A revoked, expired, re-bound, or otherwise stale token must stop being a
	// command authority immediately. Pointer equality prevents an old
	// validation result from evicting a replacement session.
	s.mu.Lock()
	if current, exists := s.agents[key]; exists && current == ac {
		delete(s.agents, key)
	}
	s.mu.Unlock()
	ac.signalDone()
	if ac.conn != nil {
		_ = ac.conn.Close()
	}
	return nil, false
}

// HasActionRunnerSession reports whether the exact runner transport is
// currently registered. It intentionally bypasses dispatch readiness: the
// activation endpoint uses this proof to promote a pending session.
func (s *Server) HasActionRunnerSession(admission AgentAdmission) bool {
	if s == nil {
		return false
	}
	key := agentSessionKey(admission.OrganizationID, admission.AgentID)
	s.mu.RLock()
	current, ok := s.pendingActionRunners[key]
	s.mu.RUnlock()
	return ok && current != nil && admission.ActivationPending && sameActionRunnerAdmission(current.admission, admission)
}

// PromoteActionRunnerSessionForCommit performs only the bounded map mutation
// needed by the credential transaction. Callers may invoke it while holding
// config.Mu; they must run the returned cleanup only after releasing that lock.
// This preserves the sole nested order config.Mu -> Server.mu and keeps socket
// close/logging I/O outside both locks.
func (s *Server) PromoteActionRunnerSessionForCommit(admission AgentAdmission) (func(), bool) {
	if s == nil {
		return nil, false
	}
	key := agentSessionKey(admission.OrganizationID, admission.AgentID)
	s.mu.Lock()
	pending, ok := s.pendingActionRunners[key]
	if !ok || pending == nil || !admission.ActivationPending || !sameActionRunnerAdmission(pending.admission, admission) {
		s.mu.Unlock()
		return nil, false
	}
	delete(s.pendingActionRunners, key)
	displaced := s.agents[key]
	s.agents[key] = pending
	s.mu.Unlock()
	var cleanup func()
	if displaced != nil && displaced != pending {
		cleanup = func() {
			displaced.signalDone()
			if displaced.conn != nil {
				_ = displaced.conn.Close()
			}
		}
	}
	return cleanup, true
}

// PromoteActionRunnerSession is the non-transactional compatibility wrapper.
// Production activation uses PromoteActionRunnerSessionForCommit and defers
// cleanup until after config.Mu has been released.
func (s *Server) PromoteActionRunnerSession(admission AgentAdmission) bool {
	cleanup, promoted := s.PromoteActionRunnerSessionForCommit(admission)
	if cleanup != nil {
		cleanup()
	}
	return promoted
}

// InvalidateActionRunnerSession closes exactly the currently admitted typed
// action-runner session identified by admission. A stale rotation result must
// never evict a replacement session that has already registered for the same
// tenant and host identity.
func (s *Server) InvalidateActionRunnerSession(admission AgentAdmission) bool {
	if s == nil {
		return false
	}
	expected := AgentAdmission{
		OrganizationID:   normalizeOrganizationID(admission.OrganizationID),
		TokenID:          strings.TrimSpace(admission.TokenID),
		AgentID:          strings.TrimSpace(admission.AgentID),
		Hostname:         strings.TrimSpace(admission.Hostname),
		RuntimeRole:      strings.TrimSpace(admission.RuntimeRole),
		ActionCapability: strings.TrimSpace(admission.ActionCapability),
	}
	if expected.TokenID == "" || expected.AgentID == "" || expected.Hostname == "" ||
		expected.RuntimeRole != RuntimeRoleActionRunner || expected.ActionCapability != ActionCapabilityTypedV1 {
		return false
	}
	key := agentSessionKey(expected.OrganizationID, expected.AgentID)
	s.mu.Lock()
	var current *agentConn
	if active, ok := s.agents[key]; ok && active != nil && sameActionRunnerAdmission(active.admission, expected) {
		delete(s.agents, key)
		current = active
	} else if pending, ok := s.pendingActionRunners[key]; ok && pending != nil && sameActionRunnerAdmission(pending.admission, expected) {
		delete(s.pendingActionRunners, key)
		current = pending
	}
	s.mu.Unlock()
	if current == nil {
		return false
	}

	current.signalDone()
	if current.conn != nil {
		_ = current.conn.Close()
	}
	return true
}

// InvalidateAgentSession closes exactly the currently admitted session for a
// credential whose authority has changed. Unlike action-runner rotation this
// accepts either runtime role, but every admission field must still match so a
// stale or cross-tenant credential transition cannot evict another session.
func (s *Server) InvalidateAgentSession(admission AgentAdmission) bool {
	if s == nil {
		return false
	}
	expected := AgentAdmission{
		OrganizationID:   normalizeOrganizationID(admission.OrganizationID),
		TokenID:          strings.TrimSpace(admission.TokenID),
		AgentID:          strings.TrimSpace(admission.AgentID),
		Hostname:         strings.TrimSpace(admission.Hostname),
		RuntimeRole:      strings.TrimSpace(admission.RuntimeRole),
		ActionCapability: strings.TrimSpace(admission.ActionCapability),
	}
	if expected.TokenID == "" || expected.AgentID == "" || expected.Hostname == "" || expected.RuntimeRole == "" {
		return false
	}
	key := agentSessionKey(expected.OrganizationID, expected.AgentID)
	s.mu.Lock()
	current, ok := s.agents[key]
	if !ok || current == nil ||
		normalizeOrganizationID(current.admission.OrganizationID) != expected.OrganizationID ||
		strings.TrimSpace(current.admission.TokenID) != expected.TokenID ||
		strings.TrimSpace(current.admission.AgentID) != expected.AgentID ||
		!unifiedresources.HostnamesEquivalent(current.admission.Hostname, expected.Hostname) ||
		strings.TrimSpace(current.admission.RuntimeRole) != expected.RuntimeRole ||
		strings.TrimSpace(current.admission.ActionCapability) != expected.ActionCapability {
		s.mu.Unlock()
		return false
	}
	delete(s.agents, key)
	s.mu.Unlock()
	current.signalDone()
	if current.conn != nil {
		_ = current.conn.Close()
	}
	return true
}

func sameActionRunnerAdmission(current, expected AgentAdmission) bool {
	return normalizeOrganizationID(current.OrganizationID) == expected.OrganizationID &&
		strings.TrimSpace(current.TokenID) == expected.TokenID &&
		strings.TrimSpace(current.AgentID) == expected.AgentID &&
		(strings.EqualFold(strings.TrimSpace(current.Hostname), expected.Hostname) ||
			unifiedresources.HostnamesEquivalent(current.Hostname, expected.Hostname)) &&
		strings.TrimSpace(current.RuntimeRole) == RuntimeRoleActionRunner &&
		strings.TrimSpace(current.ActionCapability) == ActionCapabilityTypedV1
}

func requireLegacyFullTrustConnection(ac *agentConn, operation string) error {
	if ac == nil {
		return fmt.Errorf("agent connection is unavailable")
	}
	if strings.TrimSpace(ac.admission.RuntimeRole) == RuntimeRoleActionRunner {
		return fmt.Errorf("%s is not available on typed action-runner sessions", operation)
	}
	return nil
}

func (s *Server) connectionForContext(ctx context.Context, agentID string) (*agentConn, bool) {
	return s.connectionForOrganization(organizationIDFromContext(ctx), agentID)
}

func (s *Server) claimPendingHostOperation(agentID, requestID, actionID, operation string) (string, error) {
	key := pendingRequestKey(agentID, requestID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pendingHostOperations[key]; exists {
		return "", fmt.Errorf("typed host operation request %q is already pending", requestID)
	}
	s.pendingHostOperations[key] = pendingHostOperation{
		actionID:  strings.TrimSpace(actionID),
		operation: strings.TrimSpace(operation),
	}
	return key, nil
}

func (s *Server) matchesPendingHostOperation(agentID, requestID, actionID, operation string) bool {
	key := pendingRequestKey(agentID, requestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	return ok && expected.actionID == strings.TrimSpace(actionID) && expected.operation == strings.TrimSpace(operation)
}

func (s *Server) claimPendingDockerOperation(identity operationreceipt.Identity, containerID string) (string, error) {
	return s.claimPendingDockerOperationForSession(identity.AgentID, identity, containerID)
}

func (s *Server) claimPendingDockerOperationForSession(sessionKey string, identity operationreceipt.Identity, containerID string) (string, error) {
	identity, err := operationreceipt.NormalizeIdentity(identity)
	if err != nil {
		return "", err
	}
	key := pendingRequestKey(sessionKey, identity.AttemptID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pendingHostOperations[key]; exists {
		return "", fmt.Errorf("typed operation request %q is already pending", identity.AttemptID)
	}
	s.pendingHostOperations[key] = pendingHostOperation{actionID: identity.ActionID, operation: identity.OperationKind, identity: identity, subjectID: strings.ToLower(strings.TrimSpace(containerID))}
	return key, nil
}

func (s *Server) matchesPendingDockerOperation(agentID string, result DockerContainerLifecycleResultPayload) bool {
	return s.matchesPendingDockerOperationForSession(agentID, agentID, result)
}

func (s *Server) matchesPendingDockerOperationForSession(sessionKey, agentID string, result DockerContainerLifecycleResultPayload) bool {
	key := pendingRequestKey(sessionKey, result.RequestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	actual := operationreceipt.Identity{AttemptID: result.RequestID, ActionID: result.ActionID, OperationKind: result.Operation, OperationVersion: result.OperationVersion, RequestDigest: result.RequestDigest, AgentID: strings.TrimSpace(agentID)}
	return ok && expected.identity == actual && expected.subjectID == strings.ToLower(strings.TrimSpace(result.ContainerID))
}

func (s *Server) matchesPendingDockerUpdateOperation(agentID string, result DockerContainerUpdateResultPayload) bool {
	return s.matchesPendingDockerUpdateOperationForSession(agentID, agentID, result)
}

func (s *Server) matchesPendingDockerUpdateOperationForSession(sessionKey, agentID string, result DockerContainerUpdateResultPayload) bool {
	key := pendingRequestKey(sessionKey, result.RequestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	actual := operationreceipt.Identity{AttemptID: result.RequestID, ActionID: result.ActionID, OperationKind: result.Operation, OperationVersion: result.OperationVersion, RequestDigest: result.RequestDigest, AgentID: strings.TrimSpace(agentID)}
	return ok && expected.identity == actual && expected.subjectID == strings.ToLower(strings.TrimSpace(result.ContainerID))
}

func (s *Server) matchesPendingProxmoxGuestOperationForSession(sessionKey, agentID string, result ProxmoxGuestLifecycleResultPayload) bool {
	key := pendingRequestKey(sessionKey, result.RequestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	actual := operationreceipt.Identity{AttemptID: result.RequestID, ActionID: result.ActionID, OperationKind: result.Operation, OperationVersion: result.OperationVersion, RequestDigest: result.RequestDigest, AgentID: strings.TrimSpace(agentID)}
	subject := result.GuestKind + ":" + strconv.Itoa(result.VMID)
	return ok && expected.identity == actual && expected.subjectID == subject
}

func (s *Server) releasePendingHostOperation(key string) {
	s.mu.Lock()
	delete(s.pendingHostOperations, key)
	s.mu.Unlock()
}

func deploySubKey(agentID, jobID string) string {
	return agentID + "\x00" + jobID
}

func normalizeTarget(targetType, targetID string) (string, string, error) {
	normalizedType := strings.ToLower(strings.TrimSpace(targetType))
	if normalizedType == "" {
		normalizedType = "agent"
	}

	normalizedTargetID := strings.TrimSpace(targetID)
	switch normalizedType {
	case "agent":
		// Agent-level execution ignores target ID.
		return "agent", "", nil
	case "container", "vm":
		if normalizedTargetID == "" {
			return "", "", fmt.Errorf("target id is required for target type %q", normalizedType)
		}
		if len(normalizedTargetID) > maxTargetIDLength {
			return "", "", fmt.Errorf("target id exceeds %d characters", maxTargetIDLength)
		}
		if !safeTargetIDPattern.MatchString(normalizedTargetID) {
			return "", "", fmt.Errorf("target id contains invalid characters")
		}
		return normalizedType, normalizedTargetID, nil
	default:
		return "", "", fmt.Errorf("invalid target type %q", targetType)
	}
}

func validateExecuteCommandPayload(cmd *ExecuteCommandPayload) error {
	if cmd == nil {
		return fmt.Errorf("command payload is required")
	}

	if strings.TrimSpace(cmd.Command) == "" {
		return fmt.Errorf("command is required")
	}
	cmd.ApprovalID = strings.TrimSpace(cmd.ApprovalID)
	if len(cmd.Command) > maxExecuteCommandLength {
		return fmt.Errorf("command exceeds %d characters", maxExecuteCommandLength)
	}

	targetType, targetID, err := normalizeTarget(cmd.TargetType, cmd.TargetID)
	if err != nil {
		return err
	}
	cmd.TargetType = targetType
	cmd.TargetID = targetID

	if cmd.Timeout < 0 {
		return fmt.Errorf("timeout cannot be negative")
	}
	if cmd.Timeout > maxExecuteCommandTimeoutSeconds {
		return fmt.Errorf("timeout cannot exceed %d seconds", maxExecuteCommandTimeoutSeconds)
	}

	return nil
}

func (s *Server) authorizeCommandPayload(cmd ExecuteCommandPayload) error {
	if s == nil || s.commandPolicy == nil {
		return nil
	}

	switch s.commandPolicy.Evaluate(cmd.Command) {
	case PolicyBlock:
		return fmt.Errorf("command blocked by policy")
	case PolicyRequireApproval:
		// Trusted internal subsystems (e.g. servicediscovery deep scans) carry
		// a hardcoded command catalog and never accept user-supplied commands,
		// so the user-driven approval gate does not apply to them.
		if cmd.Trusted {
			return nil
		}
		if cmd.ApprovalID == "" {
			return fmt.Errorf("command requires approval")
		}
		if cmd.authorization == nil || strings.TrimSpace(cmd.authorization.ActionID) == "" {
			return fmt.Errorf("command requires server-owned approval authorization")
		}
		if s.commandAuthorizationVerifier == nil {
			return fmt.Errorf("command approval authorization verifier is unavailable")
		}
	}

	return nil
}

func validateReadFilePayload(req *ReadFilePayload) error {
	if req == nil {
		return fmt.Errorf("read file payload is required")
	}

	req.Path = strings.TrimSpace(req.Path)
	if req.Path == "" {
		return fmt.Errorf("path is required")
	}
	if len(req.Path) > maxReadFilePathLength {
		return fmt.Errorf("path exceeds %d characters", maxReadFilePathLength)
	}
	if strings.ContainsAny(req.Path, "\x00\r\n") {
		return fmt.Errorf("path contains invalid control characters")
	}

	targetType, targetID, err := normalizeTarget(req.TargetType, req.TargetID)
	if err != nil {
		return err
	}
	req.TargetType = targetType
	req.TargetID = targetID

	if req.MaxBytes < 0 {
		return fmt.Errorf("max bytes cannot be negative")
	}
	if req.MaxBytes == 0 {
		req.MaxBytes = defaultReadFileMaxBytes
	}
	if req.MaxBytes > maxReadFileMaxBytes {
		return fmt.Errorf("max bytes cannot exceed %d", maxReadFileMaxBytes)
	}

	return nil
}

func validateHostUpdatePayload(req *HostUpdatePayload) error {
	if req == nil {
		return fmt.Errorf("host update payload is required")
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ActionID = strings.TrimSpace(req.ActionID)
	req.Operation = strings.TrimSpace(req.Operation)
	req.ExpectedInventoryHash = strings.TrimSpace(req.ExpectedInventoryHash)
	if req.RequestID == "" {
		return fmt.Errorf("request id is required")
	}
	if len(req.RequestID) > maxRequestIDLength {
		return fmt.Errorf("request id exceeds %d characters", maxRequestIDLength)
	}
	if req.ActionID == "" {
		return fmt.Errorf("action id is required")
	}
	if len(req.ActionID) > maxRequestIDLength {
		return fmt.Errorf("action id exceeds %d characters", maxRequestIDLength)
	}
	if req.Operation != HostUpdateOperationInstall {
		return fmt.Errorf("unsupported host update operation %q", req.Operation)
	}
	if req.OperationVersion != HostAPTOperationVersion {
		return fmt.Errorf("unsupported host update operation version %d", req.OperationVersion)
	}
	expectedDigest, err := hostUpdateRequestDigest(*req)
	if err != nil {
		return err
	}
	if req.RequestDigest != expectedDigest {
		return fmt.Errorf("host update request digest mismatch")
	}
	if !hostUpdateInventoryHashPattern.MatchString(req.ExpectedInventoryHash) {
		return fmt.Errorf("expected inventory hash is required and must be sha256")
	}
	if req.Timeout < 0 || req.Timeout > 1800 {
		return fmt.Errorf("host update timeout must be between 0 and 1800 seconds")
	}
	if req.Timeout == 0 {
		req.Timeout = 900
	}
	return nil
}

func validateHostUpdateResultPayload(result *HostUpdateResultPayload) error {
	if result == nil {
		return fmt.Errorf("host update result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.Verification = strings.TrimSpace(result.Verification)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength {
		return fmt.Errorf("invalid request id")
	}
	if result.Before.PendingCount < 0 || result.After.PendingCount < 0 {
		return fmt.Errorf("pending package counts cannot be negative")
	}
	if len(result.Before.Packages) > 200 || len(result.After.Packages) > 200 {
		return fmt.Errorf("package evidence exceeds bounded limit")
	}
	for _, hash := range []string{result.Before.InventoryHash, result.After.InventoryHash} {
		if hash != "" && !hostUpdateInventoryHashPattern.MatchString(hash) {
			return fmt.Errorf("invalid package inventory hash")
		}
	}
	switch result.Verification {
	case HostUpdateVerificationVerified:
		if !result.Success || !result.After.Supported || result.After.Manager != "apt" || result.After.Error != "" || result.After.PendingCount != 0 || result.After.InventoryHash == "" {
			return fmt.Errorf("verified host update lacks a valid zero-pending postcondition")
		}
	case HostUpdateVerificationFailed, HostUpdateVerificationInconclusive:
	default:
		return fmt.Errorf("unsupported host update verification %q", result.Verification)
	}
	if len(result.Error) > 1024 {
		return fmt.Errorf("host update error exceeds bounded limit")
	}
	if result.ReasonCode != "" && !IsActionRefusalReasonCode(result.ReasonCode) {
		return fmt.Errorf("invalid host update refusal reason code")
	}
	if result.ReasonCode != "" && (result.MutationStarted || result.Success) {
		return fmt.Errorf("host update refusal reason conflicts with execution state")
	}
	return nil
}

func validateHostStorageCleanupPayload(req *HostStorageCleanupPayload) error {
	if req == nil {
		return fmt.Errorf("host storage cleanup payload is required")
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	req.ActionID = strings.TrimSpace(req.ActionID)
	req.Operation = strings.TrimSpace(req.Operation)
	req.ExpectedFingerprint = strings.TrimSpace(req.ExpectedFingerprint)
	if req.RequestID == "" || len(req.RequestID) > maxRequestIDLength {
		return fmt.Errorf("invalid request id")
	}
	if req.ActionID == "" || len(req.ActionID) > maxRequestIDLength {
		return fmt.Errorf("invalid action id")
	}
	if req.Operation != HostStorageCleanupOperationPackageCache {
		return fmt.Errorf("unsupported host storage cleanup operation %q", req.Operation)
	}
	if req.OperationVersion != HostAPTOperationVersion {
		return fmt.Errorf("unsupported host storage cleanup operation version %d", req.OperationVersion)
	}
	expectedDigest, err := hostStorageCleanupRequestDigest(*req)
	if err != nil {
		return err
	}
	if req.RequestDigest != expectedDigest {
		return fmt.Errorf("host storage cleanup request digest mismatch")
	}
	if !hostStorageCleanupFingerprintPattern.MatchString(req.ExpectedFingerprint) {
		return fmt.Errorf("expected cleanup fingerprint is required and must be sha256")
	}
	if req.Timeout < 0 || req.Timeout > 900 {
		return fmt.Errorf("host storage cleanup timeout must be between 0 and 900 seconds")
	}
	if req.Timeout == 0 {
		req.Timeout = 300
	}
	return nil
}

func validateHostStorageCleanupResultPayload(result *HostStorageCleanupResultPayload) error {
	if result == nil {
		return fmt.Errorf("host storage cleanup result is required")
	}
	result.RequestID = strings.TrimSpace(result.RequestID)
	result.Verification = strings.TrimSpace(result.Verification)
	result.ReasonCode = strings.TrimSpace(result.ReasonCode)
	if result.RequestID == "" || len(result.RequestID) > maxRequestIDLength {
		return fmt.Errorf("invalid request id")
	}
	if result.Before.ReclaimableBytes < 0 || result.After.ReclaimableBytes < 0 || result.ReclaimedBytes < 0 {
		return fmt.Errorf("storage cleanup byte counts cannot be negative")
	}
	if result.Before.ReclaimableBytes > HostStorageCleanupMaxReportedBytes || result.After.ReclaimableBytes > HostStorageCleanupMaxReportedBytes || result.ReclaimedBytes > HostStorageCleanupMaxReportedBytes {
		return fmt.Errorf("storage cleanup byte counts exceed bounded limit")
	}
	for _, fingerprint := range []string{result.Before.Fingerprint, result.After.Fingerprint} {
		if fingerprint != "" && !hostStorageCleanupFingerprintPattern.MatchString(fingerprint) {
			return fmt.Errorf("invalid storage cleanup fingerprint")
		}
	}
	switch result.Verification {
	case HostStorageCleanupVerificationVerified:
		if !result.Success || !result.After.Supported || result.After.Provider != "apt-package-cache" || result.After.Error != "" || result.After.Fingerprint == "" {
			return fmt.Errorf("verified storage cleanup lacks a valid postcondition")
		}
		if result.Before.ReclaimableBytes == 0 {
			if result.After.ReclaimableBytes != 0 || result.ReclaimedBytes != 0 {
				return fmt.Errorf("already-satisfied storage cleanup has inconsistent byte counts")
			}
		} else if result.ReclaimedBytes <= 0 || result.After.ReclaimableBytes >= result.Before.ReclaimableBytes || result.ReclaimedBytes != result.Before.ReclaimableBytes-result.After.ReclaimableBytes {
			return fmt.Errorf("verified storage cleanup did not reclaim reported bytes")
		}
	case HostStorageCleanupVerificationFailed, HostStorageCleanupVerificationInconclusive:
	default:
		return fmt.Errorf("unsupported host storage cleanup verification %q", result.Verification)
	}
	if len(result.Error) > 1024 {
		return fmt.Errorf("host storage cleanup error exceeds bounded limit")
	}
	if result.ReasonCode != "" && !IsActionRefusalReasonCode(result.ReasonCode) {
		return fmt.Errorf("invalid host storage cleanup refusal reason code")
	}
	if result.ReasonCode != "" && (result.MutationStarted || result.Success) {
		return fmt.Errorf("host storage cleanup refusal reason conflicts with execution state")
	}
	return nil
}

func isAllowedWebSocketOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return false
	}

	return securityutil.SameHostWebSocketOrigin(origin, r.Host)
}

func normalizeWebSocketRemoteIP(remoteAddr string) string {
	remoteAddr = strings.TrimSpace(remoteAddr)
	if remoteAddr == "" {
		return ""
	}

	host, _, err := net.SplitHostPort(remoteAddr)
	if err == nil {
		return strings.Trim(host, "[]")
	}

	return strings.Trim(remoteAddr, "[]")
}

func (s *Server) acquireWebSocketIPSlot(remoteIP string) bool {
	if s == nil || s.maxConnsPerIP <= 0 || remoteIP == "" {
		return true
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if s.ipConnCounts[remoteIP] >= s.maxConnsPerIP {
		return false
	}

	s.ipConnCounts[remoteIP]++
	return true
}

func (s *Server) releaseWebSocketIPSlot(remoteIP string) {
	if s == nil || s.maxConnsPerIP <= 0 || remoteIP == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	count := s.ipConnCounts[remoteIP]
	if count <= 1 {
		delete(s.ipConnCounts, remoteIP)
		return
	}

	s.ipConnCounts[remoteIP] = count - 1
}

// HandleWebSocket handles incoming WebSocket connections from agents
func (s *Server) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	remoteAddr := r.RemoteAddr
	remoteIP := normalizeWebSocketRemoteIP(remoteAddr)

	if s.isShuttingDown() {
		http.Error(w, "agent execution server is shutting down", http.StatusServiceUnavailable)
		return
	}
	if !s.acquireWebSocketIPSlot(remoteIP) {
		log.Warn().
			Str("remote_ip", remoteIP).
			Int("max_connections_per_ip", s.maxConnsPerIP).
			Msg("Rejected agent websocket upgrade due to per-IP connection cap")
		http.Error(w, "Too many agent websocket connections from this IP", http.StatusTooManyRequests)
		return
	}
	defer s.releaseWebSocketIPSlot(remoteIP)

	// CRITICAL: Clear http.Server deadlines BEFORE WebSocket upgrade.
	// The http.Server.ReadTimeout sets a deadline on the underlying connection when
	// the request starts. We must clear it before the upgrade or the connection will
	// be closed when that deadline fires (typically ~15 seconds after connection).
	// Use http.ResponseController (Go 1.20+) to clear the deadline.
	rc := http.NewResponseController(w)
	if err := rc.SetReadDeadline(time.Time{}); err != nil {
		log.Debug().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("Failed to clear read deadline via ResponseController")
	}
	if err := rc.SetWriteDeadline(time.Time{}); err != nil {
		log.Debug().
			Err(err).
			Str("remote_addr", remoteAddr).
			Msg("Failed to clear write deadline via ResponseController")
	}

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to upgrade WebSocket connection")
		return
	}
	conn.SetReadLimit(maxWebSocketMessageBytes)
	closeConn := func(context string) {
		if closeErr := conn.Close(); closeErr != nil {
			log.Debug().Err(closeErr).Msg(context)
		}
	}

	if s.isShuttingDown() {
		conn.Close()
		return
	}

	// Also clear on the WebSocket's underlying connection as a safety net
	if netConn := conn.NetConn(); netConn != nil {
		if err := netConn.SetReadDeadline(time.Time{}); err != nil {
			log.Debug().Err(err).Msg("Failed to clear net.Conn read deadline")
		}
		if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
			log.Debug().Err(err).Msg("Failed to clear net.Conn write deadline")
		}
	}

	// Read first message (must be agent_register)
	if err := conn.SetReadDeadline(time.Now().Add(30 * time.Second)); err != nil {
		log.Warn().Err(err).Msg("Failed to set initial registration read deadline")
	}
	_, msgBytes, err := conn.ReadMessage()
	if err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to read registration message")
		closeConn("Failed to close connection after registration read error")
		return
	}

	var msg Message
	if err := json.Unmarshal(msgBytes, &msg); err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to parse registration message")
		closeConn("Failed to close connection after registration parse error")
		return
	}

	if msg.Type != MsgTypeAgentRegister {
		log.Error().Str("type", string(msg.Type)).Str("remote_addr", remoteAddr).Msg("First message must be agent_register")
		closeConn("Failed to close connection after invalid first message type")
		return
	}

	// Parse registration payload
	var reg AgentRegisterPayload
	if err := msg.DecodePayload(&reg); err != nil {
		log.Error().Err(err).Str("remote_addr", remoteAddr).Msg("Failed to parse registration payload")
		closeConn("Failed to close connection after registration payload parse error")
		return
	}

	reg.AgentID = strings.TrimSpace(reg.AgentID)
	if reg.AgentID == "" {
		log.Warn().Msg("Agent registration rejected: missing agent_id")
		rejMsg, rejErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "Invalid agent_id"})
		if rejErr != nil {
			log.Warn().Err(rejErr).Msg("Failed to encode rejection message")
		} else if sendErr := s.sendMessage(conn, rejMsg); sendErr != nil {
			log.Warn().Err(sendErr).Msg("Failed to send rejection to agent with missing agent_id")
		}
		conn.Close()
		return
	}
	if len(reg.AgentID) > maxAgentIDLength {
		log.Warn().
			Int("agent_id_length", len(reg.AgentID)).
			Msg("Agent registration rejected: agent_id exceeds maximum length")
		rejMsg, rejErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "Invalid agent_id"})
		if rejErr != nil {
			log.Warn().Err(rejErr).Msg("Failed to encode rejection for oversized agent_id")
		} else if sendErr := s.sendMessage(conn, rejMsg); sendErr != nil {
			log.Warn().Err(sendErr).Msg("Failed to send rejection to agent with oversized agent_id")
		}
		conn.Close()
		return
	}

	// Validate and canonicalize the command-session admission. Reporting and
	// command admission are intentionally separate trust decisions.
	admission, admitted := s.admitToken(reg.Token, reg.AgentID, reg.Hostname)
	admission.OrganizationID = normalizeOrganizationID(admission.OrganizationID)
	admission.TokenID = strings.TrimSpace(admission.TokenID)
	admission.AgentID = strings.TrimSpace(admission.AgentID)
	admission.Hostname = strings.TrimSpace(admission.Hostname)
	admission.RuntimeRole = strings.TrimSpace(admission.RuntimeRole)
	admission.ActionCapability = strings.TrimSpace(admission.ActionCapability)
	if admission.AgentID == "" {
		admission.AgentID = reg.AgentID
	}
	if admission.Hostname == "" {
		admission.Hostname = strings.TrimSpace(reg.Hostname)
	}
	if admission.AgentID != reg.AgentID ||
		!unifiedresources.HostnamesEquivalent(admission.Hostname, reg.Hostname) {
		admitted = false
	}
	registrationRole := strings.TrimSpace(reg.RuntimeRole)
	registrationCapability := strings.TrimSpace(reg.ActionCapability)
	if admission.RuntimeRole == RuntimeRoleActionRunner {
		if registrationRole != RuntimeRoleActionRunner ||
			admission.ActionCapability != ActionCapabilityTypedV1 ||
			registrationCapability != admission.ActionCapability {
			admitted = false
		}
	} else if registrationRole == RuntimeRoleActionRunner || registrationCapability != "" {
		// A legacy collector token cannot opt itself into the runner protocol by
		// asserting registration fields that were not bound into its credential.
		admitted = false
	}
	if !admitted {
		log.Warn().Str("agent_id", reg.AgentID).Msg("Agent registration rejected: invalid token")
		// Actionable message instead of a bare "Invalid token": the agent logs
		// this verbatim, and the dominant causes (token not recognised, or not
		// bound to this agent) are both fixed by re-enrolling, while a token
		// that exists but lacks the scope is named explicitly. Avoids the silent
		// retry loop that previously gave operators nothing to act on.
		rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "agent token not authorized for command execution — generate a fresh install command with commands enabled (Settings > Infrastructure > Add Pulse Agent) and re-run it on this host; the existing token cannot be upgraded in place"})
		if err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to encode rejection message")
			conn.Close()
			return
		}
		if err := s.sendMessage(conn, rejectedMsg); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to send rejection to agent")
		}
		closeConn("Failed to close connection after registration rejection")
		return
	}

	// Create agent connection
	ac := &agentConn{
		conn: conn,
		agent: ConnectedAgent{
			OrganizationID:           admission.OrganizationID,
			TokenID:                  admission.TokenID,
			AgentID:                  admission.AgentID,
			Hostname:                 admission.Hostname,
			Version:                  reg.Version,
			Platform:                 reg.Platform,
			Tags:                     reg.Tags,
			RuntimeRole:              admission.RuntimeRole,
			ActionCapability:         admission.ActionCapability,
			ConnectedAt:              time.Now(),
			OperationReceiptVersion:  reg.OperationReceiptVersion,
			ActionPreflightVersion:   reg.ActionPreflightVersion,
			DockerObservationVersion: reg.DockerObservationVersion,
		},
		admission:        admission,
		sessionKey:       agentSessionKey(admission.OrganizationID, admission.AgentID),
		approvalGrantKey: DeriveApprovalGrantKey(reg.Token),
		done:             make(chan struct{}),
	}

	// Clear deadline for normal operation - both on the WebSocket and underlying connection
	// This MUST happen BEFORE registering the agent in the map to avoid race conditions
	// where other goroutines could call ExecuteCommand while we're still configuring the connection.
	if err := conn.SetReadDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear read deadline after registration")
	}
	if err := conn.SetWriteDeadline(time.Time{}); err != nil {
		log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear write deadline after registration")
	}
	if netConn := conn.NetConn(); netConn != nil {
		if err := netConn.SetReadDeadline(time.Time{}); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear net.Conn read deadline after registration")
		}
		if err := netConn.SetWriteDeadline(time.Time{}); err != nil {
			log.Warn().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to clear net.Conn write deadline after registration")
		}
	}

	// Set up ping/pong handlers to keep connection alive
	conn.SetPongHandler(func(appData string) error {
		// Reset read deadline on pong received
		if err := conn.SetReadDeadline(time.Time{}); err != nil {
			return fmt.Errorf("set read deadline on pong: %w", err)
		}
		return nil
	})

	// Register agent - after this point, other goroutines can access the connection
	s.mu.Lock()
	now := time.Now()
	if s.now != nil {
		now = s.now()
	}
	for key, expiry := range s.actionRunnerAdmissionTombstones {
		if !expiry.After(now) {
			delete(s.actionRunnerAdmissionTombstones, key)
		}
	}
	if expiry, cancelled := s.actionRunnerAdmissionTombstones[actionRunnerAdmissionTombstoneKey(admission)]; cancelled && expiry.After(now) {
		s.mu.Unlock()
		log.Warn().Str("agent_id", reg.AgentID).Msg("Action runner registration rejected: prepared credential was cancelled")
		rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "action runner credential preparation was cancelled"})
		if err == nil {
			_ = s.sendMessage(conn, rejectedMsg)
		}
		closeConn("Failed to close cancelled action runner registration")
		return
	}
	for key, existing := range s.agents {
		if key != ac.sessionKey &&
			normalizeOrganizationID(existing.admission.OrganizationID) == admission.OrganizationID &&
			unifiedresources.HostnamesEquivalent(existing.agent.Hostname, ac.agent.Hostname) {
			s.mu.Unlock()
			log.Warn().
				Str("organization_id", admission.OrganizationID).
				Str("connected_agent_id", existing.agent.AgentID).
				Str("requested_agent_id", admission.AgentID).
				Str("hostname", admission.Hostname).
				Msg("Agent registration rejected: hostname is already owned by another command identity")
			rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "agent hostname is already connected under another identity"})
			if err == nil {
				_ = s.sendMessage(conn, rejectedMsg)
			}
			closeConn("Failed to close duplicate agent hostname connection")
			return
		}
	}
	var replaced *agentConn
	if admission.ActivationPending && admission.RuntimeRole == RuntimeRoleActionRunner {
		// A prepared transport is staged separately. Reconnect/flood traffic can
		// replace only the one bounded pending slot and cannot evict or interrupt
		// the active dispatch session before durable activation.
		replaced = s.pendingActionRunners[ac.sessionKey]
		s.pendingActionRunners[ac.sessionKey] = ac
	} else {
		if existing, ok := s.agents[ac.sessionKey]; ok {
			if !unifiedresources.HostnamesEquivalent(existing.agent.Hostname, ac.agent.Hostname) {
				s.mu.Unlock()
				log.Warn().
					Str("organization_id", admission.OrganizationID).
					Str("agent_id", admission.AgentID).
					Str("connected_hostname", existing.agent.Hostname).
					Str("requested_hostname", admission.Hostname).
					Msg("Agent registration rejected: duplicate identity is already connected from another host")
				rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "agent identity is already connected from another host"})
				if err == nil {
					_ = s.sendMessage(conn, rejectedMsg)
				}
				closeConn("Failed to close duplicate agent identity connection")
				return
			}
			replaced = existing
		}
		s.agents[ac.sessionKey] = ac
	}
	s.mu.Unlock()
	if replaced != nil && replaced != ac {
		log.Info().
			Str("organization_id", admission.OrganizationID).
			Str("agent_id", admission.AgentID).
			Str("hostname", admission.Hostname).
			Bool("activation_pending", admission.ActivationPending).
			Msg("Replacing existing agent connection")
		replaced.signalDone()
		if replaced.conn != nil {
			if err := replaced.conn.Close(); err != nil {
				log.Debug().Err(err).Str("agent_id", admission.AgentID).Msg("Failed to close existing connection during reconnect")
			}
		}
	}

	log.Info().
		Str("organization_id", admission.OrganizationID).
		Str("agent_id", admission.AgentID).
		Str("hostname", admission.Hostname).
		Str("version", reg.Version).
		Str("platform", reg.Platform).
		Msg("Agent connected")

	// Send registration success
	ackMsg, ackErr := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: true, Message: "Registered"})
	if ackErr != nil {
		log.Warn().Err(ackErr).Str("agent_id", reg.AgentID).Msg("Failed to encode registration ack")
		conn.Close()
		return
	}
	ac.writeMu.Lock()
	if sendErr := s.sendMessage(conn, ackMsg); sendErr != nil {
		log.Warn().
			Err(sendErr).
			Str("agent_id", reg.AgentID).
			Str("hostname", reg.Hostname).
			Msg("Failed to send registration ack")
		ac.writeMu.Unlock()
		s.mu.Lock()
		if existing, ok := s.pendingActionRunners[ac.sessionKey]; ok && existing == ac {
			delete(s.pendingActionRunners, ac.sessionKey)
		}
		if existing, ok := s.agents[ac.sessionKey]; ok && existing == ac {
			delete(s.agents, ac.sessionKey)
		}
		s.mu.Unlock()
		ac.signalDone()
		_ = conn.Close()
		return
	}
	ac.writeMu.Unlock()

	// Start server-side ping loop to keep connection alive
	pingDone := make(chan struct{})
	go s.pingLoop(ac, pingDone)
	defer close(pingDone)

	if notify := s.agentRegisteredNotifier; notify != nil {
		go notify(admission)
	}

	// Run read loop (blocking) - don't use goroutine, or HTTP handler will close connection
	s.readLoop(ac)
}

func (s *Server) readLoop(ac *agentConn) {
	defer func() {
		agentID := ac.agent.AgentID
		sessionKey := connectionSessionKey(ac)
		s.mu.Lock()
		wasActive := false
		ownsSession := false
		if existing, exists := s.agents[sessionKey]; exists && existing == ac {
			delete(s.agents, sessionKey)
			wasActive = true
			ownsSession = true
		}
		if existing, exists := s.pendingActionRunners[sessionKey]; exists && existing == ac {
			delete(s.pendingActionRunners, sessionKey)
			ownsSession = true
		}
		// Close all deploy progress subscriptions for this agent so
		// processPreflightProgress goroutines unblock and detect disconnect.
		var closeChs []chan DeployProgressPayload
		if ownsSession && wasActive {
			prefix := sessionKey + "\x00"
			for key, ch := range s.deploySubs {
				if strings.HasPrefix(key, prefix) {
					closeChs = append(closeChs, ch)
					delete(s.deploySubs, key)
				}
			}
		}
		s.mu.Unlock()
		for _, ch := range closeChs {
			close(ch)
		}
		if err := ac.conn.Close(); err != nil {
			log.Debug().Err(err).Str("agent_id", agentID).Msg("Failed to close connection during read-loop cleanup")
		}
		log.Info().Str("agent_id", agentID).Msg("Agent disconnected")
	}()

	log.Debug().Str("agent_id", ac.agent.AgentID).Msg("Starting read loop for agent")

	for {
		select {
		case <-ac.done:
			log.Debug().Str("agent_id", ac.agent.AgentID).Msg("Read loop exiting: done channel closed")
			return
		default:
		}

		_, msgBytes, err := ac.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Unexpected WebSocket close error")
			} else {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Read loop exiting: read error")
			}
			return
		}

		var msg Message
		if err := json.Unmarshal(msgBytes, &msg); err != nil {
			log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse message")
			continue
		}

		switch msg.Type {
		case MsgTypeAgentPing:
			pongMsg, err := NewMessage(MsgTypePong, "", nil)
			if err != nil {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to encode pong message")
				continue
			}
			ac.writeMu.Lock()
			if err := s.sendMessage(ac.conn, pongMsg); err != nil {
				log.Debug().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to send pong")
			}
			ac.writeMu.Unlock()

		case MsgTypeCommandResult:
			var result CommandResultPayload
			if err := msg.DecodePayload(&result); err != nil {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse command result")
				continue
			}
			result.RequestID = strings.TrimSpace(result.RequestID)
			if result.RequestID == "" {
				log.Warn().Str("agent_id", ac.agent.AgentID).Msg("Dropping command result with empty request_id")
				continue
			}
			if len(result.RequestID) > maxRequestIDLength {
				log.Warn().
					Str("agent_id", ac.agent.AgentID).
					Int("request_id_length", len(result.RequestID)).
					Msg("Dropping command result with oversized request_id")
				continue
			}

			s.mu.RLock()
			ch, ok := s.pendingReqs[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()

			if ok {
				select {
				case ch <- result:
					log.Debug().
						Str("agent_id", ac.agent.AgentID).
						Str("request_id", result.RequestID).
						Bool("success", result.Success).
						Int("exit_code", result.ExitCode).
						Int64("duration_ms", result.Duration).
						Msg("Received command result from agent")
				default:
					log.Warn().
						Str("agent_id", ac.agent.AgentID).
						Str("request_id", result.RequestID).
						Msg("Result channel full, dropping")
				}
			} else {
				log.Warn().
					Str("agent_id", ac.agent.AgentID).
					Str("request_id", result.RequestID).
					Msg("No pending request for result")
			}

		case MsgTypeHostUpdateResult:
			result, decodeErr := DecodeHostUpdateResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid host update result")
				continue
			}
			if !s.matchesPendingHostOperation(connectionSessionKey(ac), result.RequestID, result.ActionID, HostUpdateOperationInstall) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated host update result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingHostUpdates[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Host update result channel full, dropping")
				}
			}

		case MsgTypeActionPreflightResult:
			result, decodeErr := DecodeActionPreflightResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid action preflight result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingActionPreflights[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Action preflight result channel full, dropping")
				}
			}

		case MsgTypeDockerContainerObserveResult:
			result, decodeErr := DecodeDockerContainerObservationResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid docker container observation result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingDockerContainerObservations[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Docker observation result channel full, dropping")
				}
			}

		case MsgTypeHostStorageCleanupResult:
			result, decodeErr := DecodeHostStorageCleanupResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid host storage cleanup result")
				continue
			}
			if !s.matchesPendingHostOperation(connectionSessionKey(ac), result.RequestID, result.ActionID, HostStorageCleanupOperationPackageCache) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated host storage cleanup result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingHostStorageCleanups[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Host storage cleanup result channel full, dropping")
				}
			}

		case MsgTypeProxmoxGuestLifecycleResult:
			result, decodeErr := DecodeProxmoxGuestLifecycleResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid Proxmox guest lifecycle result")
				continue
			}
			if !s.matchesPendingProxmoxGuestOperationForSession(connectionSessionKey(ac), ac.agent.AgentID, result) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated Proxmox guest lifecycle result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingProxmoxGuestLifecycles[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Proxmox guest lifecycle result channel full, dropping")
				}
			}

		case MsgTypeDockerContainerLifecycleResult:
			result, decodeErr := DecodeDockerContainerLifecycleResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid docker container lifecycle result")
				continue
			}
			if !s.matchesPendingDockerOperationForSession(connectionSessionKey(ac), ac.agent.AgentID, result) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated docker lifecycle result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingDockerContainerLifecycles[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Docker lifecycle result channel full, dropping")
				}
			}

		case MsgTypeDockerContainerUpdateResult:
			result, decodeErr := DecodeDockerContainerUpdateResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid docker container update result")
				continue
			}
			if !s.matchesPendingDockerUpdateOperationForSession(connectionSessionKey(ac), ac.agent.AgentID, result) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated docker update result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingDockerContainerUpdates[pendingRequestKey(connectionSessionKey(ac), result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Docker update result channel full, dropping")
				}
			}

		case MsgTypeOperationQueryResult:
			result, decodeErr := operationreceipt.DecodeQueryResult(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid operation query result")
				continue
			}
			key := pendingRequestKey(connectionSessionKey(ac), strings.TrimSpace(msg.ID))
			s.mu.RLock()
			pending, ok := s.pendingOperationQueries[key]
			s.mu.RUnlock()
			if !ok {
				continue
			}
			if result.Record != nil && result.Record.Identity != pending.identity {
				log.Warn().Str("agent_id", ac.agent.AgentID).Msg("Dropping mismatched operation query result")
				continue
			}
			if err := ValidateOperationQueryResultForIdentity(result, pending.identity, s.currentTime()); err != nil {
				log.Warn().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid correlated operation query result")
				continue
			}
			select {
			case pending.ch <- result:
			default:
			}
		case MsgTypeDeployProgress:
			var progress DeployProgressPayload
			if err := msg.DecodePayload(&progress); err != nil {
				log.Error().Err(err).Str("agent_id", ac.agent.AgentID).Msg("Failed to parse deploy progress")
				continue
			}
			if progress.JobID == "" {
				log.Warn().Str("agent_id", ac.agent.AgentID).Msg("Dropping deploy progress with empty job_id")
				continue
			}

			subKey := deploySubKey(connectionSessionKey(ac), progress.JobID)

			// Hold the read lock across map lookup AND the non-blocking send to
			// prevent UnsubscribeDeployProgress from closing the channel between
			// lookup and send (it needs the write lock to delete + close).
			sent := false
			s.mu.RLock()
			ch, ok := s.deploySubs[subKey]
			if ok {
				select {
				case ch <- progress:
					sent = true
				default:
				}
			}
			s.mu.RUnlock()

			// Final messages must be delivered — retry with backoff if the
			// initial non-blocking send failed (channel was full).
			if ok && !sent && progress.Final {
				deadline := time.After(5 * time.Second)
				ticker := time.NewTicker(50 * time.Millisecond)
			retryLoop:
				for {
					select {
					case <-deadline:
						log.Error().
							Str("agent_id", ac.agent.AgentID).
							Str("job_id", progress.JobID).
							Msg("Deploy final progress send timed out — force-closing subscription")
						// Force-close the subscription so the consumer goroutine
						// unblocks on channel close and can finalize the job.
						s.mu.Lock()
						if closeCh, exists := s.deploySubs[subKey]; exists {
							delete(s.deploySubs, subKey)
							close(closeCh)
						}
						s.mu.Unlock()
						break retryLoop
					case <-ticker.C:
						s.mu.RLock()
						ch, ok = s.deploySubs[subKey]
						if !ok {
							s.mu.RUnlock()
							break retryLoop // channel was closed/unsubscribed
						}
						select {
						case ch <- progress:
							sent = true
							s.mu.RUnlock()
							break retryLoop
						default:
							s.mu.RUnlock()
						}
					}
				}
				ticker.Stop()
			} else if ok && !sent {
				log.Warn().
					Str("agent_id", ac.agent.AgentID).
					Str("job_id", progress.JobID).
					Msg("Deploy progress channel full, dropping")
			}

			if ok {
				if sent {
					log.Debug().
						Str("agent_id", ac.agent.AgentID).
						Str("job_id", progress.JobID).
						Str("target_id", progress.TargetID).
						Str("phase", string(progress.Phase)).
						Str("status", string(progress.Status)).
						Bool("final", progress.Final).
						Msg("Received deploy progress from agent")
				}
			} else {
				log.Debug().
					Str("agent_id", ac.agent.AgentID).
					Str("job_id", progress.JobID).
					Msg("No subscriber for deploy progress")
			}
		}
	}
}

func (s *Server) pingLoop(ac *agentConn, done chan struct{}) {
	ticker := time.NewTicker(s.pingInterval)
	defer ticker.Stop()

	// Track consecutive ping failures to detect dead connections faster
	consecutiveFailures := 0
	const maxConsecutiveFailures = 3

	for {
		select {
		case <-done:
			return
		case <-ac.done:
			return
		case <-ticker.C:
			ac.writeMu.Lock()
			err := ac.conn.WriteControl(websocket.PingMessage, []byte{}, time.Now().Add(pingWriteWait))
			ac.writeMu.Unlock()
			if err != nil {
				consecutiveFailures++
				log.Warn().
					Err(err).
					Str("agent_id", ac.agent.AgentID).
					Str("hostname", ac.agent.Hostname).
					Int("consecutive_failures", consecutiveFailures).
					Msg("Failed to send ping to agent")

				if consecutiveFailures >= maxConsecutiveFailures {
					log.Error().
						Err(err).
						Str("agent_id", ac.agent.AgentID).
						Str("hostname", ac.agent.Hostname).
						Int("failures", consecutiveFailures).
						Msg("Agent connection appears dead after multiple ping failures, closing connection")

					// Close the connection - this will cause readLoop to exit and clean up
					if closeErr := ac.conn.Close(); closeErr != nil {
						log.Debug().Err(closeErr).Str("agent_id", ac.agent.AgentID).Msg("Failed to close dead connection after ping failures")
					}
					return
				}
			} else {
				// Reset failure counter on successful ping
				consecutiveFailures = 0
			}
		}
	}
}

func (s *Server) sendMessage(conn *websocket.Conn, msg Message) error {
	msgBytes, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal websocket message: %w", err)
	}
	if err := writeTextMessage(conn, msgBytes); err != nil {
		return fmt.Errorf("write websocket message: %w", err)
	}
	return nil
}

// Shutdown gracefully stops the server by closing all active agent connections.
// The method is idempotent.
func (s *Server) Shutdown() {
	s.shutdownOnce.Do(func() {
		close(s.shutdown)

		s.mu.Lock()
		agents := make([]*agentConn, 0, len(s.agents)+len(s.pendingActionRunners))
		for _, ac := range s.agents {
			agents = append(agents, ac)
		}
		for _, ac := range s.pendingActionRunners {
			agents = append(agents, ac)
		}
		s.agents = make(map[string]*agentConn)
		s.pendingActionRunners = make(map[string]*agentConn)
		s.mu.Unlock()

		for _, ac := range agents {
			ac.signalDone()
			_ = ac.conn.Close()
		}
	})
}

// ExecuteCommand sends a command to an agent and waits for the result
func (s *Server) ExecuteCommand(ctx context.Context, agentID string, cmd ExecuteCommandPayload) (*CommandResultPayload, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	cmd.RequestID = strings.TrimSpace(cmd.RequestID)
	if cmd.RequestID == "" {
		cmd.RequestID = uuid.New().String()
	}
	if len(cmd.RequestID) > maxRequestIDLength {
		return nil, fmt.Errorf("request id exceeds %d characters", maxRequestIDLength)
	}
	if err := validateExecuteCommandPayload(&cmd); err != nil {
		return nil, err
	}

	// Never dispatch under a context that has already expired: the send
	// would succeed, this call would return the context error a moment
	// later, and the agent would be left executing a command nobody is
	// waiting for. A caller polling on a dead deadline can otherwise
	// re-issue the same command every cycle while every previous copy is
	// still running on the target host (minipc probe-storm incident,
	// 2026-08-20).
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("command %q not dispatched: %w", cmd.RequestID, err)
	}

	startedAt := time.Now()

	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", cmd.RequestID).
			Msg("Execute command requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if err := requireLegacyFullTrustConnection(ac, "execute_command"); err != nil {
		return nil, err
	}
	if err := s.authorizeCommandPayload(cmd); err != nil {
		return nil, err
	}
	requiresApproval := !cmd.Trusted && s.commandPolicy != nil && s.commandPolicy.Evaluate(cmd.Command) == PolicyRequireApproval
	if requiresApproval {
		if len(ac.approvalGrantKey) == 0 {
			return nil, fmt.Errorf("command approval grant signer is unavailable")
		}
		auth := cmd.authorization
		if err := s.commandAuthorizationVerifier(CommandAuthorizationRequest{
			ApprovalID: cmd.ApprovalID,
			OrgID:      auth.OrgID,
			ActionID:   auth.ActionID,
			AgentID:    agentID,
			Command:    cmd.Command,
			TargetType: cmd.TargetType,
			TargetID:   cmd.TargetID,
		}); err != nil {
			return nil, fmt.Errorf("command approval authorization rejected: %w", err)
		}

		// Approval grants are an internal transport credential. Never accept a
		// caller-supplied grant, even if it happens to be structurally valid.
		grant, grantErr := s.newCommandApprovalGrant(ac.approvalGrantKey, agentID, cmd, time.Now(), DefaultApprovalGrantTTL)
		if grantErr != nil {
			return nil, fmt.Errorf("failed to issue approval grant: %w", grantErr)
		}
		cmd.ApprovalGrant = grant
	}

	execLog := log.With().
		Str("agent_id", agentID).
		Str("request_id", cmd.RequestID).
		Str("target_type", cmd.TargetType).
		Str("target_id", cmd.TargetID).
		Logger()

	// Create response channel
	respCh := make(chan CommandResultPayload, 1)
	reqKey := pendingRequestKey(connectionSessionKey(ac), cmd.RequestID)
	s.mu.Lock()
	if _, exists := s.pendingReqs[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("command request %q is already pending", cmd.RequestID)
	}
	s.pendingReqs[reqKey] = respCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingReqs, reqKey)
		s.mu.Unlock()
	}()

	// Send command
	execMsg, execErr := NewMessage(MsgTypeExecuteCmd, cmd.RequestID, cmd)
	if execErr != nil {
		return nil, fmt.Errorf("failed to encode command: %w", execErr)
	}

	ac.writeMu.Lock()
	err := s.sendMessage(ac.conn, execMsg)
	ac.writeMu.Unlock()

	if err != nil {
		execLog.Error().
			Err(err).
			Dur("duration", time.Since(startedAt)).
			Msg("Failed to send command to agent")
		return nil, fmt.Errorf("failed to send command: %w", err)
	}

	// Wait for result
	timeout := time.Duration(cmd.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 60 * time.Second
	}
	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case result := <-respCh:
		execLog.Info().
			Bool("success", result.Success).
			Int("exit_code", result.ExitCode).
			Int64("agent_duration_ms", result.Duration).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command completed")
		return &result, nil
	case <-timer.C:
		s.cancelAgentCommand(ac, cmd.RequestID)
		execLog.Warn().
			Dur("timeout", timeout).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command timed out")
		return nil, fmt.Errorf("command timed out after %v", timeout)
	case <-ctx.Done():
		s.cancelAgentCommand(ac, cmd.RequestID)
		execLog.Warn().
			Err(ctx.Err()).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command canceled")
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before command result", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// cancelAgentCommand tells an agent to abort an execute_command request the
// server has stopped waiting for, so the agent can reap the command's process
// tree instead of running it to its full timeout. Best effort: agents that
// predate the cancel_command message ignore it and fall back to their own
// per-command timeout.
func (s *Server) cancelAgentCommand(ac *agentConn, requestID string) {
	msg, err := NewMessage(MsgTypeCancelCmd, requestID, CancelCommandPayload{RequestID: requestID})
	if err != nil {
		log.Debug().Err(err).Str("request_id", requestID).Msg("Failed to encode command cancellation")
		return
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		log.Debug().Err(err).Str("request_id", requestID).Msg("Failed to send command cancellation to agent")
	}
}

// hostOperationPayload exposes the durable operation identity shared by the
// typed host APT operation request payloads.
type hostOperationPayload interface {
	hostOperationIdentity() (requestID, actionID, operation string, timeoutSeconds int)
}

func (p HostUpdatePayload) hostOperationIdentity() (string, string, string, int) {
	return p.RequestID, p.ActionID, p.Operation, p.Timeout
}

func (p HostStorageCleanupPayload) hostOperationIdentity() (string, string, string, int) {
	return p.RequestID, p.ActionID, p.Operation, p.Timeout
}

// hostOperationDispatch names the per-operation pieces of the shared typed
// host-operation dispatch cycle: claim → send → await validated receipt.
type hostOperationDispatch[Req hostOperationPayload, Res any] struct {
	msgType        MessageType
	label          string
	pending        map[string]chan Res
	validateResult func(Req, Res, time.Time) error
}

// prepareHostOperationRequest runs the shared request prologue of the typed
// host-operation dispatchers: normalize the agent id, default the request id,
// then bind and validate the payload. It returns the normalized agent id.
func prepareHostOperationRequest(s *Server, agentID string, requestID *string, bind func() error, validate func() error) (string, error) {
	if s == nil {
		return "", fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return "", fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(*requestID) == "" {
		*requestID = uuid.New().String()
	}
	if err := bind(); err != nil {
		return "", err
	}
	if err := validate(); err != nil {
		return "", err
	}
	return agentID, nil
}

func dispatchHostOperation[Req hostOperationPayload, Res any](ctx context.Context, s *Server, agentID string, req Req, op hostOperationDispatch[Req, Res]) (*Res, error) {
	requestID, actionID, operation, timeoutSeconds := req.hostOperationIdentity()

	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return nil, fmt.Errorf("agent does not support durable operation receipts")
	}

	respCh := make(chan Res, 1)
	sessionKey := connectionSessionKey(ac)
	reqKey := pendingRequestKey(sessionKey, requestID)
	hostOperationKey, err := s.claimPendingHostOperation(sessionKey, requestID, actionID, operation)
	if err != nil {
		return nil, err
	}
	defer s.releasePendingHostOperation(hostOperationKey)
	s.mu.Lock()
	if _, exists := op.pending[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("%s request %q is already pending", op.label, requestID)
	}
	op.pending[reqKey] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(op.pending, reqKey)
		s.mu.Unlock()
	}()

	msg, err := NewMessage(op.msgType, requestID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s request: %w", op.label, err)
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send %s request: %w", op.label, err)
	}

	timer := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
	defer timer.Stop()
	select {
	case result := <-respCh:
		if err := op.validateResult(req, result, s.currentTime()); err != nil {
			return nil, fmt.Errorf("%s result validation failed: %w", op.label, err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("%s timed out after %s", op.label, time.Duration(timeoutSeconds)*time.Second)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before %s receipt", agentID, op.label)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ExecuteHostUpdate dispatches the closed typed host-package operation. Unlike
// ExecuteCommand, no command text crosses this boundary; the agent owns the
// package-manager catalog, preflight, mutation, and read-after-write proof.
func (s *Server) ExecuteHostUpdate(ctx context.Context, agentID string, req HostUpdatePayload) (*HostUpdateResultPayload, error) {
	agentID, err := prepareHostOperationRequest(s, agentID, &req.RequestID,
		func() error { return BindHostUpdatePayload(&req) },
		func() error { return ValidateHostUpdatePayload(&req) })
	if err != nil {
		return nil, err
	}
	return dispatchHostOperation(ctx, s, agentID, req, hostOperationDispatch[HostUpdatePayload, HostUpdateResultPayload]{
		msgType: MsgTypeHostUpdate, label: "host update",
		pending: s.pendingHostUpdates, validateResult: ValidateHostUpdateResultForRequestAt,
	})
}

// PreflightAction asks the current Unified Agent to evaluate the exact typed
// operation without admitting a durable operation or starting a mutation.
func (s *Server) PreflightAction(ctx context.Context, agentID string, req ActionPreflightPayload) (*ActionPreflightResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.NewString()
	}
	if err := ValidateActionPreflightPayload(&req); err != nil {
		return nil, err
	}
	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.ActionPreflightVersion != ActionPreflightProtocolVersion {
		return nil, fmt.Errorf("agent does not support action preflight protocol")
	}
	ch := make(chan ActionPreflightResultPayload, 1)
	key := pendingRequestKey(connectionSessionKey(ac), req.RequestID)
	s.mu.Lock()
	if _, exists := s.pendingActionPreflights[key]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("action preflight request %q is already pending", req.RequestID)
	}
	s.pendingActionPreflights[key] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pendingActionPreflights, key)
		s.mu.Unlock()
	}()
	msg, err := NewMessage(MsgTypeActionPreflight, req.RequestID, req)
	if err != nil {
		return nil, err
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send action preflight request: %w", err)
	}
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	select {
	case result := <-ch:
		if err := ValidateActionPreflightResultForRequest(req, result, s.currentTime()); err != nil {
			return nil, fmt.Errorf("action preflight result validation failed: %w", err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("action preflight timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before action preflight result", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ObserveDockerContainer asks the current Unified Agent for a fresh read-only
// Docker/Podman daemon observation. It is a separate request from the mutation
// receipt and carries no dispatch authority.
func (s *Server) ObserveDockerContainer(ctx context.Context, agentID string, req DockerContainerObservationPayload) (*DockerContainerObservationResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.NewString()
	}
	if err := BindDockerContainerObservationPayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateDockerContainerObservationPayload(&req); err != nil {
		return nil, err
	}
	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.DockerObservationVersion != DockerContainerObservationProtocolVersion {
		return nil, fmt.Errorf("agent does not support docker observation protocol")
	}
	ch := make(chan DockerContainerObservationResultPayload, 1)
	key := pendingRequestKey(connectionSessionKey(ac), req.RequestID)
	s.mu.Lock()
	if _, exists := s.pendingDockerContainerObservations[key]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("docker observation request %q is already pending", req.RequestID)
	}
	s.pendingDockerContainerObservations[key] = ch
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pendingDockerContainerObservations, key)
		s.mu.Unlock()
	}()
	msg, err := NewMessage(MsgTypeDockerContainerObserve, req.RequestID, req)
	if err != nil {
		return nil, err
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send docker observation request: %w", err)
	}
	timer := time.NewTimer(20 * time.Second)
	defer timer.Stop()
	select {
	case result := <-ch:
		if err := ValidateDockerContainerObservationResultForRequest(req, result, s.currentTime()); err != nil {
			return nil, fmt.Errorf("docker observation result validation failed: %w", err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("docker observation timed out")
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before docker observation result", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ExecuteHostStorageCleanup dispatches the closed package-cache cleanup
// operation. No command text, path, package selector, or removal policy crosses
// the server/agent boundary.
func (s *Server) ExecuteHostStorageCleanup(ctx context.Context, agentID string, req HostStorageCleanupPayload) (*HostStorageCleanupResultPayload, error) {
	agentID, err := prepareHostOperationRequest(s, agentID, &req.RequestID,
		func() error { return BindHostStorageCleanupPayload(&req) },
		func() error { return ValidateHostStorageCleanupPayload(&req) })
	if err != nil {
		return nil, err
	}
	return dispatchHostOperation(ctx, s, agentID, req, hostOperationDispatch[HostStorageCleanupPayload, HostStorageCleanupResultPayload]{
		msgType: MsgTypeHostStorageCleanup, label: "host storage cleanup",
		pending: s.pendingHostStorageCleanups, validateResult: ValidateHostStorageCleanupResultForRequestAt,
	})
}

// ExecuteDockerContainerLifecycle dispatches one closed typed container
// operation. The Unified Agent owns the fixed runtime command catalog and
// performs preflight plus read-after-write inside this single dispatch.
func (s *Server) ExecuteDockerContainerLifecycle(ctx context.Context, agentID string, req DockerContainerLifecyclePayload) (*DockerContainerLifecycleResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.NewString()
	}
	if err := BindDockerContainerLifecyclePayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateDockerContainerLifecyclePayload(&req); err != nil {
		return nil, err
	}
	identity := DockerContainerLifecycleOperationIdentity(agentID, req)
	return dispatchTypedDockerContainerOperation(ctx, s, agentID, req.RequestID, req.Timeout, identity, req.ContainerID,
		MsgTypeDockerContainerLifecycle, req, s.pendingDockerContainerLifecycles, "docker container lifecycle",
		func(result DockerContainerLifecycleResultPayload) error {
			return ValidateDockerContainerLifecycleResultForRequest(req, result)
		})
}

// ExecuteProxmoxGuestLifecycle dispatches one closed Proxmox guest action to
// an action-runner session. The wire contract contains only guest kind, fixed
// lifecycle verb, numeric VMID, and request-bound before state.
func (s *Server) ExecuteProxmoxGuestLifecycle(ctx context.Context, agentID string, req ProxmoxGuestLifecyclePayload) (*ProxmoxGuestLifecycleResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.NewString()
	}
	if err := BindProxmoxGuestLifecyclePayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateProxmoxGuestLifecyclePayload(&req); err != nil {
		return nil, err
	}
	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.admission.RuntimeRole != RuntimeRoleActionRunner || ac.admission.ActionCapability != ActionCapabilityTypedV1 {
		return nil, fmt.Errorf("Proxmox guest lifecycle requires a typed action-runner session")
	}
	identity := ProxmoxGuestLifecycleOperationIdentity(agentID, req)
	return dispatchTypedDockerContainerOperation(ctx, s, agentID, req.RequestID, req.Timeout, identity, req.GuestKind+":"+strconv.Itoa(req.VMID),
		MsgTypeProxmoxGuestLifecycle, req, s.pendingProxmoxGuestLifecycles, "Proxmox guest lifecycle",
		func(result ProxmoxGuestLifecycleResultPayload) error {
			return ValidateProxmoxGuestLifecycleResultForRequest(req, result)
		})
}

// dispatchTypedDockerContainerOperation owns the shared skeleton for closed
// typed container dispatches: durable-receipt capability check, pending
// operation claim, single-flight request registration, send, and the bounded
// wait for the validated result.
func dispatchTypedDockerContainerOperation[Res any](
	ctx context.Context, s *Server, agentID, requestID string, timeoutSeconds int,
	identity operationreceipt.Identity, containerID string,
	msgType MessageType, payload any,
	pending map[string]chan Res, label string,
	validate func(Res) error,
) (*Res, error) {
	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return nil, fmt.Errorf("agent does not support durable operation receipts")
	}

	respCh := make(chan Res, 1)
	sessionKey := connectionSessionKey(ac)
	reqKey := pendingRequestKey(sessionKey, requestID)
	hostOperationKey, err := s.claimPendingDockerOperationForSession(sessionKey, identity, containerID)
	if err != nil {
		return nil, err
	}
	defer s.releasePendingHostOperation(hostOperationKey)
	s.mu.Lock()
	if _, exists := pending[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("%s request %q is already pending", label, requestID)
	}
	pending[reqKey] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(pending, reqKey)
		s.mu.Unlock()
	}()

	msg, err := NewMessage(msgType, requestID, payload)
	if err != nil {
		return nil, fmt.Errorf("failed to encode %s request: %w", label, err)
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send %s request: %w", label, err)
	}
	timer := time.NewTimer(time.Duration(timeoutSeconds) * time.Second)
	defer timer.Stop()
	select {
	case result := <-respCh:
		if err := validate(result); err != nil {
			return nil, fmt.Errorf("%s result validation failed: %w", label, err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("%s timed out after %s", label, time.Duration(timeoutSeconds)*time.Second)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before %s receipt", agentID, label)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ExecuteDockerContainerUpdate dispatches one closed typed container image
// update. The Unified Agent owns pull, backup, recreate, verification, and
// rollback inside this single dispatch and reports the compensation outcome.
func (s *Server) ExecuteDockerContainerUpdate(ctx context.Context, agentID string, req DockerContainerUpdatePayload) (*DockerContainerUpdateResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.NewString()
	}
	if err := BindDockerContainerUpdatePayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateDockerContainerUpdatePayload(&req); err != nil {
		return nil, err
	}
	identity := DockerContainerUpdateOperationIdentity(agentID, req)
	return dispatchTypedDockerContainerOperation(ctx, s, agentID, req.RequestID, req.Timeout, identity, req.ContainerID,
		MsgTypeDockerContainerUpdate, req, s.pendingDockerContainerUpdates, "docker container update",
		func(result DockerContainerUpdateResultPayload) error {
			return ValidateDockerContainerUpdateResultForRequest(req, result)
		})
}

func (s *Server) currentTime() time.Time {
	if s != nil && s.now != nil {
		return s.now().UTC()
	}
	return time.Now().UTC()
}

func (s *Server) AgentOperationReceiptVersion(agentID string) int {
	return s.AgentOperationReceiptVersionForOrganization(defaultOrganizationID, agentID)
}

// AgentOperationReceiptVersionForOrganization reports the live protocol
// version only for a currently admitted tenant-scoped session.
func (s *Server) AgentOperationReceiptVersionForOrganization(organizationID, agentID string) int {
	if s == nil {
		return 0
	}
	connection, ok := s.connectionForOrganization(organizationID, agentID)
	if !ok {
		return 0
	}
	return connection.agent.OperationReceiptVersion
}

// QueryAgentOperation reconciles a committed attempt without mutation or resend.
func (s *Server) QueryAgentOperation(ctx context.Context, agentID string, identity operationreceipt.Identity) (operationreceipt.QueryResult, error) {
	identity, err := operationreceipt.NormalizeIdentity(identity)
	if err != nil {
		return operationreceipt.QueryResult{}, err
	}
	agentID = strings.TrimSpace(agentID)
	if identity.AgentID != agentID {
		return operationreceipt.QueryResult{}, operationreceipt.ErrBindingConflict
	}
	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return operationreceipt.QueryResult{}, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return operationreceipt.QueryResult{}, fmt.Errorf("agent does not support durable operation receipts")
	}
	queryID := identity.AttemptID + ".query." + uuid.NewString()
	key := pendingRequestKey(connectionSessionKey(ac), queryID)
	ch := make(chan operationreceipt.QueryResult, 1)
	s.mu.Lock()
	if _, exists := s.pendingOperationQueries[key]; exists {
		s.mu.Unlock()
		return operationreceipt.QueryResult{}, fmt.Errorf("operation query %q is already pending", identity.AttemptID)
	}
	s.pendingOperationQueries[key] = pendingOperationQuery{identity: identity, ch: ch}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); delete(s.pendingOperationQueries, key); s.mu.Unlock() }()
	msg, err := NewMessage(MsgTypeOperationQuery, queryID, operationreceipt.Query{Version: operationreceipt.ProtocolVersion, Identity: identity})
	if err != nil {
		return operationreceipt.QueryResult{}, err
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return operationreceipt.QueryResult{}, err
	}
	timer := time.NewTimer(operationQueryTimeout)
	defer timer.Stop()
	select {
	case result := <-ch:
		return result, nil
	case <-ctx.Done():
		return operationreceipt.QueryResult{}, ctx.Err()
	case <-timer.C:
		return operationreceipt.QueryResult{}, fmt.Errorf("operation receipt query timed out")
	case <-ac.done:
		return operationreceipt.QueryResult{}, fmt.Errorf("agent %s disconnected during operation query", agentID)
	case <-s.shutdown:
		return operationreceipt.QueryResult{}, errServerShuttingDown
	}
}

// ReadFile reads a file from an agent
func (s *Server) ReadFile(ctx context.Context, agentID string, req ReadFilePayload) (*CommandResultPayload, error) {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	req.RequestID = strings.TrimSpace(req.RequestID)
	if req.RequestID == "" {
		req.RequestID = uuid.New().String()
	}
	if err := validateReadFilePayload(&req); err != nil {
		return nil, err
	}

	// Same rule as ExecuteCommand: never dispatch work the caller has
	// already stopped waiting for.
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("read_file %q not dispatched: %w", req.RequestID, err)
	}

	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", req.RequestID).
			Msg("Read file requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if err := requireLegacyFullTrustConnection(ac, "read_file"); err != nil {
		return nil, err
	}

	readLog := log.With().
		Str("agent_id", agentID).
		Str("request_id", req.RequestID).
		Str("path", req.Path).
		Str("target_type", req.TargetType).
		Str("target_id", req.TargetID).
		Int64("max_bytes", req.MaxBytes).
		Logger()

	startedAt := time.Now()

	// Create response channel
	respCh := make(chan CommandResultPayload, 1)
	reqKey := pendingRequestKey(connectionSessionKey(ac), req.RequestID)
	s.mu.Lock()
	if _, exists := s.pendingReqs[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("read_file request %q is already pending", req.RequestID)
	}
	s.pendingReqs[reqKey] = respCh
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.pendingReqs, reqKey)
		s.mu.Unlock()
	}()

	// Send request
	readPayloadBytes, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode read_file request: %w", err)
	}
	msg := Message{
		Type:      MsgTypeReadFile,
		ID:        req.RequestID,
		Timestamp: time.Now(),
		Payload:   readPayloadBytes,
	}

	ac.writeMu.Lock()
	sendErr := s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()

	if sendErr != nil {
		readLog.Error().
			Err(sendErr).
			Dur("duration", time.Since(startedAt)).
			Msg("Failed to send read_file request to agent")
		return nil, fmt.Errorf("failed to send read_file request: %w", sendErr)
	}

	// Wait for result
	timeout := readFileTimeout
	timer := time.NewTimer(timeout)
	defer func() {
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
	}()

	select {
	case result := <-respCh:
		readLog.Info().
			Bool("success", result.Success).
			Int("exit_code", result.ExitCode).
			Int64("agent_duration_ms", result.Duration).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent read_file completed")
		return &result, nil
	case <-timer.C:
		s.cancelAgentCommand(ac, req.RequestID)
		return nil, fmt.Errorf("read_file timed out after %v", timeout)
	case <-ctx.Done():
		s.cancelAgentCommand(ac, req.RequestID)
		return nil, fmt.Errorf("read_file %q on agent %q canceled: %w", req.RequestID, agentID, ctx.Err())
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before read_file result", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// GetConnectedAgents returns a list of currently connected agents
func (s *Server) GetConnectedAgents() []ConnectedAgent {
	return s.GetConnectedAgentsForOrganization(defaultOrganizationID)
}

// GetConnectedAgentsForOrganization returns only currently admitted sessions
// owned by one tenant.
func (s *Server) GetConnectedAgentsForOrganization(organizationID string) []ConnectedAgent {
	if s == nil {
		return nil
	}
	organizationID = normalizeOrganizationID(organizationID)
	s.mu.RLock()
	ids := make([]string, 0, len(s.agents))
	for _, ac := range s.agents {
		if normalizeOrganizationID(ac.admission.OrganizationID) == organizationID {
			ids = append(ids, ac.agent.AgentID)
		}
	}
	s.mu.RUnlock()
	agents := make([]ConnectedAgent, 0, len(ids))
	for _, agentID := range ids {
		if ac, ok := s.connectionForOrganization(organizationID, agentID); ok {
			agents = append(agents, ac.agent)
		}
	}
	return agents
}

// IsAgentConnected checks if an agent is currently connected
func (s *Server) IsAgentConnected(agentID string) bool {
	return s.IsAgentConnectedForOrganization(defaultOrganizationID, agentID)
}

// IsAgentConnectedForOrganization checks command-channel admission rather than
// telemetry liveness.
func (s *Server) IsAgentConnectedForOrganization(organizationID, agentID string) bool {
	_, ok := s.connectionForOrganization(organizationID, agentID)
	return ok
}

// GetAgentForHost finds the agent for a given hostname using the canonical
// hostname-equivalence contract shared with the unified identity layer.
func (s *Server) GetAgentForHost(hostname string) (string, bool) {
	return s.GetAgentForHostForOrganization(defaultOrganizationID, hostname)
}

// GetAgentForHostForOrganization resolves a hostname only within one tenant.
func (s *Server) GetAgentForHostForOrganization(organizationID, hostname string) (string, bool) {
	if s == nil {
		return "", false
	}
	organizationID = normalizeOrganizationID(organizationID)
	s.mu.RLock()
	ids := make([]string, 0, len(s.agents))
	for _, ac := range s.agents {
		if normalizeOrganizationID(ac.admission.OrganizationID) == organizationID &&
			unifiedresources.HostnamesEquivalent(ac.agent.Hostname, hostname) {
			ids = append(ids, ac.agent.AgentID)
		}
	}
	s.mu.RUnlock()
	if len(ids) != 1 {
		return "", false
	}
	for _, agentID := range ids {
		if _, ok := s.connectionForOrganization(organizationID, agentID); ok {
			return agentID, true
		}
	}
	return "", false
}

// GetAgentForTokenForOrganization resolves the canonical live command session
// for the same enrollment token that owns a telemetry resource.
func (s *Server) GetAgentForTokenForOrganization(organizationID, tokenID string) (string, bool) {
	if s == nil || strings.TrimSpace(tokenID) == "" {
		return "", false
	}
	organizationID = normalizeOrganizationID(organizationID)
	tokenID = strings.TrimSpace(tokenID)
	s.mu.RLock()
	ids := make([]string, 0, 1)
	for _, ac := range s.agents {
		if normalizeOrganizationID(ac.admission.OrganizationID) == organizationID &&
			strings.TrimSpace(ac.admission.TokenID) == tokenID {
			ids = append(ids, ac.agent.AgentID)
		}
	}
	s.mu.RUnlock()
	if len(ids) != 1 {
		return "", false
	}
	for _, agentID := range ids {
		if _, ok := s.connectionForOrganization(organizationID, agentID); ok {
			return agentID, true
		}
	}
	return "", false
}

// GetAgentForIdentityForOrganization resolves a live command session only
// when one tenant-scoped admission matches both the supplied agent ID and the
// canonical hostname. This is the safe recovery path for telemetry whose
// last-seen token ID predates an enrollment-token rotation.
func (s *Server) GetAgentForIdentityForOrganization(organizationID, agentID, hostname string) (string, bool) {
	if s == nil {
		return "", false
	}
	organizationID = normalizeOrganizationID(organizationID)
	agentID = strings.TrimSpace(agentID)
	hostname = strings.TrimSpace(hostname)
	if agentID == "" || hostname == "" {
		return "", false
	}
	ac, ok := s.connectionForOrganization(organizationID, agentID)
	if !ok || normalizeOrganizationID(ac.admission.OrganizationID) != organizationID ||
		strings.TrimSpace(ac.admission.AgentID) != agentID ||
		!unifiedresources.HostnamesEquivalent(ac.admission.Hostname, hostname) {
		return "", false
	}
	return agentID, true
}

// --- Deploy protocol ---

// SubscribeDeployProgress registers a channel to receive deploy progress
// events for the given agent and job ID. Returns a buffered channel. The caller
// must call UnsubscribeDeployProgress when done.
func (s *Server) SubscribeDeployProgress(agentID, jobID string, bufSize int) chan DeployProgressPayload {
	return s.SubscribeDeployProgressForOrganization(defaultOrganizationID, agentID, jobID, bufSize)
}

func (s *Server) SubscribeDeployProgressForOrganization(organizationID, agentID, jobID string, bufSize int) chan DeployProgressPayload {
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan DeployProgressPayload, bufSize)
	s.mu.Lock()
	s.deploySubs[deploySubKey(agentSessionKey(organizationID, agentID), jobID)] = ch
	s.mu.Unlock()
	return ch
}

// UnsubscribeDeployProgress removes and closes the progress subscriber for an agent's job.
// Safe to call multiple times — a no-op if already unsubscribed (e.g. by readLoop cleanup).
func (s *Server) UnsubscribeDeployProgress(agentID, jobID string) {
	s.UnsubscribeDeployProgressForOrganization(defaultOrganizationID, agentID, jobID)
}

func (s *Server) UnsubscribeDeployProgressForOrganization(organizationID, agentID, jobID string) {
	key := deploySubKey(agentSessionKey(organizationID, agentID), jobID)
	s.mu.Lock()
	ch, exists := s.deploySubs[key]
	delete(s.deploySubs, key)
	s.mu.Unlock()
	if exists {
		close(ch)
	}
}

// SendDeployPreflight sends a preflight check command to the source agent.
// The caller should subscribe to deploy progress for the job ID before calling
// this method. Results stream back as DeployProgressPayload messages.
func (s *Server) SendDeployPreflight(ctx context.Context, agentID string, payload DeployPreflightPayload) error {
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	return s.sendDeployCommand(ctx, agentID, MsgTypeDeployPreflight, payload.RequestID, payload)
}

// SendDeployInstall sends an install command to the source agent.
// The caller should subscribe to deploy progress for the job ID before calling
// this method. Results stream back as DeployProgressPayload messages.
func (s *Server) SendDeployInstall(ctx context.Context, agentID string, payload DeployInstallPayload) error {
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	return s.sendDeployCommand(ctx, agentID, MsgTypeDeployInstall, payload.RequestID, payload)
}

// SendDeployCancel sends a cancel command to the source agent.
func (s *Server) SendDeployCancel(ctx context.Context, agentID string, payload DeployCancelPayload) error {
	payload.RequestID = strings.TrimSpace(payload.RequestID)
	return s.sendDeployCommand(ctx, agentID, MsgTypeDeployCancelJob, payload.RequestID, payload)
}

func (s *Server) sendDeployCommand(ctx context.Context, agentID string, msgType MessageType, requestID string, payload any) error {
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return fmt.Errorf("agent id is required")
	}

	ac, ok := s.connectionForContext(ctx, agentID)
	if !ok {
		return fmt.Errorf("agent %s not connected", agentID)
	}
	if err := requireLegacyFullTrustConnection(ac, "deploy command"); err != nil {
		return err
	}

	requestID = strings.TrimSpace(requestID)
	if requestID == "" {
		return fmt.Errorf("request id is required for deploy commands")
	}
	if len(requestID) > maxRequestIDLength {
		return fmt.Errorf("request id exceeds %d characters", maxRequestIDLength)
	}

	msg, err := NewMessage(msgType, requestID, payload)
	if err != nil {
		return fmt.Errorf("failed to encode deploy command: %w", err)
	}

	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()

	if err != nil {
		return fmt.Errorf("failed to send deploy command: %w", err)
	}

	return nil
}
