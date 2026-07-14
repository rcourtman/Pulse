package agentexec

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"regexp"
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
	mu                               sync.RWMutex
	agents                           map[string]*agentConn                           // agentID -> connection
	pendingReqs                      map[string]chan CommandResultPayload            // scoped request key -> response channel
	pendingHostStorageCleanups       map[string]chan HostStorageCleanupResultPayload // scoped request key -> typed storage-cleanup response
	pendingHostUpdates               map[string]chan HostUpdateResultPayload         // scoped request key -> typed host-update response
	pendingDockerContainerLifecycles map[string]chan DockerContainerLifecycleResultPayload
	pendingDockerContainerUpdates    map[string]chan DockerContainerUpdateResultPayload
	pendingHostOperations            map[string]pendingHostOperation // scoped request key -> exact typed APT operation/query identity
	pendingOperationQueries          map[string]pendingOperationQuery
	deploySubs                       map[string]chan DeployProgressPayload // deploySubKey(agentID, jobID) -> progress subscriber
	validateToken                    func(token string, agentID string, hostname string) bool
	commandPolicy                    *CommandPolicy
	ipConnCounts                     map[string]int
	maxConnsPerIP                    int
	shutdown                         chan struct{}
	shutdownOnce                     sync.Once
	pingInterval                     time.Duration
	commandAuthorizationVerifier     func(CommandAuthorizationRequest) error
	newCommandApprovalGrant          func([]byte, string, ExecuteCommandPayload, time.Time, time.Duration) (*CommandApprovalGrant, error)
	now                              func() time.Time
	agentRegisteredNotifier          func(agentID string)
}

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

	return &Server{
		agents:                           make(map[string]*agentConn),
		pendingReqs:                      make(map[string]chan CommandResultPayload),
		pendingHostStorageCleanups:       make(map[string]chan HostStorageCleanupResultPayload),
		pendingHostUpdates:               make(map[string]chan HostUpdateResultPayload),
		pendingDockerContainerLifecycles: make(map[string]chan DockerContainerLifecycleResultPayload),
		pendingDockerContainerUpdates:    make(map[string]chan DockerContainerUpdateResultPayload),
		pendingHostOperations:            make(map[string]pendingHostOperation),
		pendingOperationQueries:          make(map[string]pendingOperationQuery),
		deploySubs:                       make(map[string]chan DeployProgressPayload),
		validateToken:                    validateToken,
		commandPolicy:                    DefaultPolicy(),
		ipConnCounts:                     make(map[string]int),
		maxConnsPerIP:                    defaultMaxWebSocketConnectionsPerIP,
		shutdown:                         make(chan struct{}),
		pingInterval:                     defaultPingInterval,
		newCommandApprovalGrant:          NewCommandApprovalGrant,
		now:                              time.Now,
	}
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
	identity, err := operationreceipt.NormalizeIdentity(identity)
	if err != nil {
		return "", err
	}
	key := pendingRequestKey(identity.AgentID, identity.AttemptID)
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.pendingHostOperations[key]; exists {
		return "", fmt.Errorf("typed operation request %q is already pending", identity.AttemptID)
	}
	s.pendingHostOperations[key] = pendingHostOperation{actionID: identity.ActionID, operation: identity.OperationKind, identity: identity, subjectID: strings.ToLower(strings.TrimSpace(containerID))}
	return key, nil
}

func (s *Server) matchesPendingDockerOperation(agentID string, result DockerContainerLifecycleResultPayload) bool {
	key := pendingRequestKey(agentID, result.RequestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	actual := operationreceipt.Identity{AttemptID: result.RequestID, ActionID: result.ActionID, OperationKind: result.Operation, OperationVersion: result.OperationVersion, RequestDigest: result.RequestDigest, AgentID: strings.TrimSpace(agentID)}
	return ok && expected.identity == actual && expected.subjectID == strings.ToLower(strings.TrimSpace(result.ContainerID))
}

func (s *Server) matchesPendingDockerUpdateOperation(agentID string, result DockerContainerUpdateResultPayload) bool {
	key := pendingRequestKey(agentID, result.RequestID)
	s.mu.RLock()
	expected, ok := s.pendingHostOperations[key]
	s.mu.RUnlock()
	actual := operationreceipt.Identity{AttemptID: result.RequestID, ActionID: result.ActionID, OperationKind: result.Operation, OperationVersion: result.OperationVersion, RequestDigest: result.RequestDigest, AgentID: strings.TrimSpace(agentID)}
	return ok && expected.identity == actual && expected.subjectID == strings.ToLower(strings.TrimSpace(result.ContainerID))
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
		if result.Before.ReclaimableBytes <= 0 || result.ReclaimedBytes <= 0 || result.After.ReclaimableBytes >= result.Before.ReclaimableBytes || result.ReclaimedBytes != result.Before.ReclaimableBytes-result.After.ReclaimableBytes {
			return fmt.Errorf("verified storage cleanup did not reclaim reported bytes")
		}
	case HostStorageCleanupVerificationFailed, HostStorageCleanupVerificationInconclusive:
	default:
		return fmt.Errorf("unsupported host storage cleanup verification %q", result.Verification)
	}
	if len(result.Error) > 1024 {
		return fmt.Errorf("host storage cleanup error exceeds bounded limit")
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

	// Validate token
	if !s.validateToken(reg.Token, reg.AgentID, reg.Hostname) {
		log.Warn().Str("agent_id", reg.AgentID).Msg("Agent registration rejected: invalid token")
		// Actionable message instead of a bare "Invalid token": the agent logs
		// this verbatim, and the dominant causes (token not recognised, or not
		// bound to this agent) are both fixed by re-enrolling, while a token
		// that exists but lacks the scope is named explicitly. Avoids the silent
		// retry loop that previously gave operators nothing to act on.
		rejectedMsg, err := NewMessage(MsgTypeRegistered, "", RegisteredPayload{Success: false, Message: "agent token not authorized for command execution — re-run the agent installer to enroll an agent:exec-scoped token"})
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
			AgentID:                 reg.AgentID,
			Hostname:                reg.Hostname,
			Version:                 reg.Version,
			Platform:                reg.Platform,
			Tags:                    reg.Tags,
			ConnectedAt:             time.Now(),
			OperationReceiptVersion: reg.OperationReceiptVersion,
		},
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
	// Close existing connection if any
	if existing, ok := s.agents[reg.AgentID]; ok {
		log.Info().
			Str("agent_id", reg.AgentID).
			Str("hostname", reg.Hostname).
			Msg("Replacing existing agent connection")
		close(existing.done)
		if err := existing.conn.Close(); err != nil {
			log.Debug().Err(err).Str("agent_id", reg.AgentID).Msg("Failed to close existing connection during reconnect")
		}
	}
	s.agents[reg.AgentID] = ac
	s.mu.Unlock()

	log.Info().
		Str("agent_id", reg.AgentID).
		Str("hostname", reg.Hostname).
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
	}
	ac.writeMu.Unlock()

	// Start server-side ping loop to keep connection alive
	pingDone := make(chan struct{})
	go s.pingLoop(ac, pingDone)
	defer close(pingDone)

	if notify := s.agentRegisteredNotifier; notify != nil {
		go notify(reg.AgentID)
	}

	// Run read loop (blocking) - don't use goroutine, or HTTP handler will close connection
	s.readLoop(ac)
}

func (s *Server) readLoop(ac *agentConn) {
	defer func() {
		agentID := ac.agent.AgentID
		s.mu.Lock()
		if existing, ok := s.agents[agentID]; ok && existing == ac {
			delete(s.agents, agentID)
		}
		// Close all deploy progress subscriptions for this agent so
		// processPreflightProgress goroutines unblock and detect disconnect.
		var closeChs []chan DeployProgressPayload
		prefix := agentID + "\x00"
		for key, ch := range s.deploySubs {
			if strings.HasPrefix(key, prefix) {
				closeChs = append(closeChs, ch)
				delete(s.deploySubs, key)
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
			ch, ok := s.pendingReqs[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
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
			if !s.matchesPendingHostOperation(ac.agent.AgentID, result.RequestID, result.ActionID, HostUpdateOperationInstall) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated host update result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingHostUpdates[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Host update result channel full, dropping")
				}
			}

		case MsgTypeHostStorageCleanupResult:
			result, decodeErr := DecodeHostStorageCleanupResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid host storage cleanup result")
				continue
			}
			if !s.matchesPendingHostOperation(ac.agent.AgentID, result.RequestID, result.ActionID, HostStorageCleanupOperationPackageCache) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated host storage cleanup result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingHostStorageCleanups[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
			s.mu.RUnlock()
			if ok {
				select {
				case ch <- result:
				default:
					log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Host storage cleanup result channel full, dropping")
				}
			}

		case MsgTypeDockerContainerLifecycleResult:
			result, decodeErr := DecodeDockerContainerLifecycleResultPayload(msg.Payload)
			if decodeErr != nil {
				log.Warn().Err(decodeErr).Str("agent_id", ac.agent.AgentID).Msg("Dropping invalid docker container lifecycle result")
				continue
			}
			if !s.matchesPendingDockerOperation(ac.agent.AgentID, result) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated docker lifecycle result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingDockerContainerLifecycles[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
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
			if !s.matchesPendingDockerUpdateOperation(ac.agent.AgentID, result) {
				log.Warn().Str("agent_id", ac.agent.AgentID).Str("request_id", result.RequestID).Msg("Dropping uncorrelated docker update result")
				continue
			}
			s.mu.RLock()
			ch, ok := s.pendingDockerContainerUpdates[pendingRequestKey(ac.agent.AgentID, result.RequestID)]
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
			key := pendingRequestKey(ac.agent.AgentID, strings.TrimSpace(msg.ID))
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

			subKey := deploySubKey(ac.agent.AgentID, progress.JobID)

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
		agents := make([]*agentConn, 0, len(s.agents))
		for _, ac := range s.agents {
			agents = append(agents, ac)
		}
		s.agents = make(map[string]*agentConn)
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

	startedAt := time.Now()

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", cmd.RequestID).
			Msg("Execute command requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
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
	reqKey := pendingRequestKey(agentID, cmd.RequestID)
	s.mu.Lock()
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
	case <-time.After(timeout):
		execLog.Warn().
			Dur("timeout", timeout).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command timed out")
		return nil, fmt.Errorf("command timed out after %v", timeout)
	case <-ctx.Done():
		execLog.Warn().
			Err(ctx.Err()).
			Dur("duration", time.Since(startedAt)).
			Msg("Agent command canceled")
		return nil, ctx.Err()
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ExecuteHostUpdate dispatches the closed typed host-package operation. Unlike
// ExecuteCommand, no command text crosses this boundary; the agent owns the
// package-manager catalog, preflight, mutation, and read-after-write proof.
func (s *Server) ExecuteHostUpdate(ctx context.Context, agentID string, req HostUpdatePayload) (*HostUpdateResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.New().String()
	}
	if err := BindHostUpdatePayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateHostUpdatePayload(&req); err != nil {
		return nil, err
	}

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return nil, fmt.Errorf("agent does not support durable operation receipts")
	}

	respCh := make(chan HostUpdateResultPayload, 1)
	reqKey := pendingRequestKey(agentID, req.RequestID)
	hostOperationKey, err := s.claimPendingHostOperation(agentID, req.RequestID, req.ActionID, req.Operation)
	if err != nil {
		return nil, err
	}
	defer s.releasePendingHostOperation(hostOperationKey)
	s.mu.Lock()
	if _, exists := s.pendingHostUpdates[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("host update request %q is already pending", req.RequestID)
	}
	s.pendingHostUpdates[reqKey] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pendingHostUpdates, reqKey)
		s.mu.Unlock()
	}()

	msg, err := NewMessage(MsgTypeHostUpdate, req.RequestID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode host update request: %w", err)
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send host update request: %w", err)
	}

	timer := time.NewTimer(time.Duration(req.Timeout) * time.Second)
	defer timer.Stop()
	select {
	case result := <-respCh:
		if err := ValidateHostUpdateResultForRequestAt(req, result, s.currentTime()); err != nil {
			return nil, fmt.Errorf("host update result validation failed: %w", err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("host update timed out after %s", time.Duration(req.Timeout)*time.Second)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before host update receipt", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// ExecuteHostStorageCleanup dispatches the closed package-cache cleanup
// operation. No command text, path, package selector, or removal policy crosses
// the server/agent boundary.
func (s *Server) ExecuteHostStorageCleanup(ctx context.Context, agentID string, req HostStorageCleanupPayload) (*HostStorageCleanupResultPayload, error) {
	if s == nil {
		return nil, fmt.Errorf("agent execution server is unavailable")
	}
	agentID = strings.TrimSpace(agentID)
	if agentID == "" {
		return nil, fmt.Errorf("agent id is required")
	}
	if strings.TrimSpace(req.RequestID) == "" {
		req.RequestID = uuid.New().String()
	}
	if err := BindHostStorageCleanupPayload(&req); err != nil {
		return nil, err
	}
	if err := ValidateHostStorageCleanupPayload(&req); err != nil {
		return nil, err
	}

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return nil, fmt.Errorf("agent does not support durable operation receipts")
	}

	respCh := make(chan HostStorageCleanupResultPayload, 1)
	reqKey := pendingRequestKey(agentID, req.RequestID)
	hostOperationKey, err := s.claimPendingHostOperation(agentID, req.RequestID, req.ActionID, req.Operation)
	if err != nil {
		return nil, err
	}
	defer s.releasePendingHostOperation(hostOperationKey)
	s.mu.Lock()
	if _, exists := s.pendingHostStorageCleanups[reqKey]; exists {
		s.mu.Unlock()
		return nil, fmt.Errorf("host storage cleanup request %q is already pending", req.RequestID)
	}
	s.pendingHostStorageCleanups[reqKey] = respCh
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		delete(s.pendingHostStorageCleanups, reqKey)
		s.mu.Unlock()
	}()

	msg, err := NewMessage(MsgTypeHostStorageCleanup, req.RequestID, req)
	if err != nil {
		return nil, fmt.Errorf("failed to encode host storage cleanup request: %w", err)
	}
	ac.writeMu.Lock()
	err = s.sendMessage(ac.conn, msg)
	ac.writeMu.Unlock()
	if err != nil {
		return nil, fmt.Errorf("failed to send host storage cleanup request: %w", err)
	}

	timer := time.NewTimer(time.Duration(req.Timeout) * time.Second)
	defer timer.Stop()
	select {
	case result := <-respCh:
		if err := ValidateHostStorageCleanupResultForRequestAt(req, result, s.currentTime()); err != nil {
			return nil, fmt.Errorf("host storage cleanup result validation failed: %w", err)
		}
		return &result, nil
	case <-timer.C:
		return nil, fmt.Errorf("host storage cleanup timed out after %s", time.Duration(req.Timeout)*time.Second)
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-ac.done:
		return nil, fmt.Errorf("agent %s disconnected before host storage cleanup receipt", agentID)
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
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
	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return nil, fmt.Errorf("agent does not support durable operation receipts")
	}

	respCh := make(chan Res, 1)
	reqKey := pendingRequestKey(agentID, requestID)
	hostOperationKey, err := s.claimPendingDockerOperation(identity, containerID)
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
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	connection, ok := s.agents[strings.TrimSpace(agentID)]
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
	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()
	if !ok {
		return operationreceipt.QueryResult{}, fmt.Errorf("agent %s not connected", agentID)
	}
	if ac.agent.OperationReceiptVersion != operationreceipt.ProtocolVersion {
		return operationreceipt.QueryResult{}, fmt.Errorf("agent does not support durable operation receipts")
	}
	queryID := identity.AttemptID + ".query." + uuid.NewString()
	key := pendingRequestKey(agentID, queryID)
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

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		log.Warn().
			Str("agent_id", agentID).
			Str("request_id", req.RequestID).
			Msg("Read file requested for disconnected agent")
		return nil, fmt.Errorf("agent %s not connected", agentID)
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
	reqKey := pendingRequestKey(agentID, req.RequestID)
	s.mu.Lock()
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
		return nil, fmt.Errorf("read_file timed out after %v", timeout)
	case <-ctx.Done():
		return nil, fmt.Errorf("read_file %q on agent %q canceled: %w", req.RequestID, agentID, ctx.Err())
	case <-s.shutdown:
		return nil, errServerShuttingDown
	}
}

// GetConnectedAgents returns a list of currently connected agents
func (s *Server) GetConnectedAgents() []ConnectedAgent {
	s.mu.RLock()
	defer s.mu.RUnlock()

	agents := make([]ConnectedAgent, 0, len(s.agents))
	for _, ac := range s.agents {
		agents = append(agents, ac.agent)
	}
	return agents
}

// IsAgentConnected checks if an agent is currently connected
func (s *Server) IsAgentConnected(agentID string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.agents[agentID]
	return ok
}

// GetAgentForHost finds the agent for a given hostname using the canonical
// hostname-equivalence contract shared with the unified identity layer.
func (s *Server) GetAgentForHost(hostname string) (string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, ac := range s.agents {
		if unifiedresources.HostnamesEquivalent(ac.agent.Hostname, hostname) {
			return ac.agent.AgentID, true
		}
	}
	return "", false
}

// --- Deploy protocol ---

// SubscribeDeployProgress registers a channel to receive deploy progress
// events for the given agent and job ID. Returns a buffered channel. The caller
// must call UnsubscribeDeployProgress when done.
func (s *Server) SubscribeDeployProgress(agentID, jobID string, bufSize int) chan DeployProgressPayload {
	if bufSize <= 0 {
		bufSize = 64
	}
	ch := make(chan DeployProgressPayload, bufSize)
	s.mu.Lock()
	s.deploySubs[deploySubKey(agentID, jobID)] = ch
	s.mu.Unlock()
	return ch
}

// UnsubscribeDeployProgress removes and closes the progress subscriber for an agent's job.
// Safe to call multiple times — a no-op if already unsubscribed (e.g. by readLoop cleanup).
func (s *Server) UnsubscribeDeployProgress(agentID, jobID string) {
	key := deploySubKey(agentID, jobID)
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

	s.mu.RLock()
	ac, ok := s.agents[agentID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("agent %s not connected", agentID)
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
