package ai

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

const (
	quickstartBootstrapUseCase        = "patrol"
	quickstartBootstrapRefreshWindow  = 5 * time.Minute
	quickstartProviderUnavailableText = "Quickstart credits require internet access. Connect your API key for offline AI Patrol."
)

// QuickstartCreditManager owns the persisted quickstart bootstrap state used by
// Patrol when no BYOK provider is configured. Cached local state is never
// authoritative for entitlement; it only memoizes the last server snapshot.
type QuickstartCreditManager interface {
	EnsureBootstrap(ctx context.Context) error
	HasCredits() bool
	CreditsRemaining() int
	CreditsTotal() int
	HasBYOK() bool
	GetProvider() providers.Provider
}

type quickstartBootstrapClient interface {
	BootstrapQuickstart(ctx context.Context, bearerToken string, req pkglicensing.QuickstartBootstrapRequest) (*pkglicensing.QuickstartBootstrapResponse, error)
}

// PersistentQuickstartCreditManager persists the server-issued quickstart token
// and the latest server-reported inventory in quickstart.enc.
type PersistentQuickstartCreditManager struct {
	mu          sync.RWMutex
	orgID       string
	persistence *config.ConfigPersistence
	aiConfig    func() *config.AIConfig
	client      quickstartBootstrapClient
	now         func() time.Time
	hostname    func() (string, error)
	newID       func() string
	state       *config.QuickstartState
}

// NewPersistentQuickstartCreditManager creates a server-authoritative
// quickstart manager backed by quickstart.enc.
func NewPersistentQuickstartCreditManager(
	persistence *config.ConfigPersistence,
	orgID string,
	aiConfigGetter func() *config.AIConfig,
) *PersistentQuickstartCreditManager {
	return NewPersistentQuickstartCreditManagerWithClient(
		persistence,
		orgID,
		aiConfigGetter,
		pkglicensing.NewLicenseServerClient(""),
	)
}

// NewPersistentQuickstartCreditManagerWithClient exists for unit tests.
func NewPersistentQuickstartCreditManagerWithClient(
	persistence *config.ConfigPersistence,
	orgID string,
	aiConfigGetter func() *config.AIConfig,
	client quickstartBootstrapClient,
) *PersistentQuickstartCreditManager {
	if aiConfigGetter == nil {
		aiConfigGetter = func() *config.AIConfig { return nil }
	}
	if client == nil {
		client = pkglicensing.NewLicenseServerClient("")
	}
	return &PersistentQuickstartCreditManager{
		orgID:       strings.TrimSpace(orgID),
		persistence: persistence,
		aiConfig:    aiConfigGetter,
		client:      client,
		now:         func() time.Time { return time.Now().UTC() },
		hostname:    os.Hostname,
		newID:       uuid.NewString,
	}
}

func (m *PersistentQuickstartCreditManager) EnsureBootstrap(ctx context.Context) error {
	if m == nil {
		return fmt.Errorf("quickstart: manager unavailable")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		return err
	}
	if !m.bootstrapNeededLocked(state, m.now()) {
		return nil
	}

	bearerToken, req, err := m.bootstrapRequestLocked(state)
	if err != nil {
		return err
	}

	resp, err := m.client.BootstrapQuickstart(ctx, bearerToken, req)
	if err != nil {
		if quickstartBootstrapCreditsExhausted(err) {
			state.QuickstartCreditsRemaining = 0
			nowUnix := m.now().Unix()
			state.LastSyncedAt = &nowUnix
			if persistErr := m.persistence.SaveQuickstartState(*config.NormalizeQuickstartState(state)); persistErr != nil {
				log.Warn().Err(persistErr).Str("orgID", m.orgID).Msg("Quickstart: failed to persist exhausted bootstrap state")
			}
		}
		return err
	}

	m.applyBootstrapLocked(state, resp)
	return m.persistence.SaveQuickstartState(*config.NormalizeQuickstartState(state))
}

func (m *PersistentQuickstartCreditManager) HasCredits() bool {
	state := m.stateSnapshot()
	return state != nil && state.QuickstartCreditsRemaining > 0
}

func (m *PersistentQuickstartCreditManager) CreditsRemaining() int {
	state := m.stateSnapshot()
	if state == nil {
		return 0
	}
	return state.QuickstartCreditsRemaining
}

func (m *PersistentQuickstartCreditManager) CreditsTotal() int {
	state := m.stateSnapshot()
	if state == nil {
		return 0
	}
	return state.QuickstartCreditsTotal
}

func (m *PersistentQuickstartCreditManager) HasBYOK() bool {
	if m == nil || m.aiConfig == nil {
		return false
	}
	cfg := m.aiConfig()
	if cfg == nil {
		return false
	}
	return len(cfg.GetConfiguredProviders()) > 0
}

func (m *PersistentQuickstartCreditManager) GetProvider() providers.Provider {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to load cached quickstart state")
		return nil
	}
	if state == nil || state.QuickstartCreditsRemaining <= 0 {
		return nil
	}
	if strings.TrimSpace(state.QuickstartToken) == "" || state.TokenExpired(m.now()) {
		return nil
	}

	token := state.QuickstartToken
	return providers.NewQuickstartClientWithToken(
		token,
		m.syncServerState,
		m.invalidateToken,
	)
}

func (m *PersistentQuickstartCreditManager) stateSnapshot() *config.QuickstartState {
	if m == nil {
		return nil
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to read cached state")
		return &config.QuickstartState{}
	}
	return config.NormalizeQuickstartState(state)
}

func (m *PersistentQuickstartCreditManager) loadStateLocked() (*config.QuickstartState, error) {
	if m.state != nil {
		return m.state, nil
	}
	if m.persistence == nil {
		m.state = &config.QuickstartState{}
		return m.state, fmt.Errorf("quickstart: persistence unavailable")
	}
	state, err := m.persistence.LoadQuickstartState()
	if err != nil {
		return nil, fmt.Errorf("quickstart: load quickstart state: %w", err)
	}
	m.state = config.NormalizeQuickstartState(state)
	return m.state, nil
}

func (m *PersistentQuickstartCreditManager) bootstrapNeededLocked(state *config.QuickstartState, now time.Time) bool {
	if state == nil {
		return true
	}
	if strings.TrimSpace(state.QuickstartToken) == "" {
		return true
	}
	if state.TokenExpired(now) {
		return true
	}
	if state.LastSyncedAt == nil || *state.LastSyncedAt <= 0 {
		return true
	}
	return now.Unix()-*state.LastSyncedAt >= int64(quickstartBootstrapRefreshWindow/time.Second)
}

func (m *PersistentQuickstartCreditManager) bootstrapRequestLocked(state *config.QuickstartState) (string, pkglicensing.QuickstartBootstrapRequest, error) {
	if state == nil {
		return "", pkglicensing.QuickstartBootstrapRequest{}, fmt.Errorf("quickstart: missing state")
	}

	instanceName := ""
	if hostname, err := m.hostname(); err == nil {
		instanceName = strings.TrimSpace(hostname)
	}

	req := pkglicensing.QuickstartBootstrapRequest{
		InstanceName: instanceName,
		UseCase:      quickstartBootstrapUseCase,
	}

	if m.persistence == nil {
		return "", req, fmt.Errorf("quickstart: persistence unavailable")
	}

	licensePersistence, err := pkglicensing.NewPersistence(m.persistence.GetConfigDir())
	if err != nil {
		return "", req, fmt.Errorf("quickstart: load license persistence: %w", err)
	}
	activationState, err := licensePersistence.LoadActivationState()
	if err != nil {
		return "", req, fmt.Errorf("quickstart: load activation state: %w", err)
	}

	if activationState != nil && strings.TrimSpace(activationState.InstallationToken) != "" {
		req.InstanceFingerprint = strings.TrimSpace(activationState.InstanceFingerprint)
		return strings.TrimSpace(activationState.InstallationToken), req, nil
	}

	if strings.TrimSpace(state.ClientInstallationID) == "" {
		state.ClientInstallationID = m.newID()
		if err := m.persistence.SaveQuickstartState(*config.NormalizeQuickstartState(state)); err != nil {
			return "", req, fmt.Errorf("quickstart: persist client installation id: %w", err)
		}
	}

	req.ClientInstallationID = state.ClientInstallationID
	req.InstanceFingerprint = state.ClientInstallationID
	return "", req, nil
}

func (m *PersistentQuickstartCreditManager) applyBootstrapLocked(state *config.QuickstartState, resp *pkglicensing.QuickstartBootstrapResponse) {
	if state == nil || resp == nil {
		return
	}

	state.QuickstartToken = strings.TrimSpace(resp.QuickstartToken)
	state.QuickstartCreditsRemaining = max(0, resp.CreditsRemaining)
	state.QuickstartCreditsTotal = max(0, resp.CreditsTotal)
	if expiry := parseQuickstartRFC3339(resp.QuickstartTokenExpiresAt); expiry != nil {
		state.QuickstartTokenExpiresAt = expiry
	} else {
		state.QuickstartTokenExpiresAt = nil
	}
	nowUnix := m.now().Unix()
	state.LastSyncedAt = &nowUnix
}

func (m *PersistentQuickstartCreditManager) syncServerState(serverState providers.QuickstartServerState) {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to load cached state for sync")
		return
	}

	state.QuickstartCreditsRemaining = max(0, serverState.CreditsRemaining)
	state.QuickstartCreditsTotal = max(0, serverState.CreditsTotal)
	if serverState.TokenExpiresAt != nil {
		expiryUnix := serverState.TokenExpiresAt.UTC().Unix()
		state.QuickstartTokenExpiresAt = &expiryUnix
	}
	nowUnix := m.now().Unix()
	state.LastSyncedAt = &nowUnix

	if m.persistence == nil {
		return
	}
	if err := m.persistence.SaveQuickstartState(*config.NormalizeQuickstartState(state)); err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to persist synced server state")
	}
}

func (m *PersistentQuickstartCreditManager) invalidateToken() {
	if m == nil {
		return
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to load cached state for token invalidation")
		return
	}

	state.QuickstartToken = ""
	state.QuickstartTokenExpiresAt = nil
	nowUnix := m.now().Unix()
	state.LastSyncedAt = &nowUnix

	if m.persistence == nil {
		return
	}
	if err := m.persistence.SaveQuickstartState(*config.NormalizeQuickstartState(state)); err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to persist invalidated token state")
	}
}

func quickstartBlockedReasonFromError(err error) string {
	switch {
	case err == nil:
		return ""
	case providers.IsQuickstartCreditsExhausted(err), quickstartBootstrapCreditsExhausted(err):
		return patrolQuickstartCreditsExhaustedReason
	case providers.IsQuickstartUnavailable(err), quickstartBootstrapUnavailable(err):
		return patrolQuickstartUnavailableReason
	default:
		return quickstartProviderUnavailableText
	}
}

// QuickstartBlockedReasonForError exposes quickstart block classification to
// adjacent runtime/API layers without duplicating the mapping logic.
func QuickstartBlockedReasonForError(err error) string {
	return quickstartBlockedReasonFromError(err)
}

// QuickstartUnavailableReason returns the canonical Patrol quickstart
// availability message shown when the server-authoritative bootstrap path
// cannot be reached.
func QuickstartUnavailableReason() string {
	return patrolQuickstartUnavailableReason
}

func quickstartBootstrapCreditsExhausted(err error) bool {
	var serverErr *pkglicensing.LicenseServerError
	if !errors.As(err, &serverErr) {
		return false
	}
	if serverErr.StatusCode == http.StatusPaymentRequired {
		return true
	}
	return strings.EqualFold(strings.TrimSpace(serverErr.Code), "quickstart_credits_exhausted")
}

func quickstartBootstrapUnavailable(err error) bool {
	var serverErr *pkglicensing.LicenseServerError
	if errors.As(err, &serverErr) {
		switch serverErr.StatusCode {
		case http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			return true
		}
		return serverErr.Retryable
	}

	lower := strings.ToLower(strings.TrimSpace(err.Error()))
	switch {
	case strings.Contains(lower, "connection refused"),
		strings.Contains(lower, "no such host"),
		strings.Contains(lower, "dial tcp"),
		strings.Contains(lower, "i/o timeout"),
		strings.Contains(lower, "context deadline exceeded"),
		strings.Contains(lower, "network is unreachable"),
		strings.Contains(lower, "network unreachable"):
		return true
	default:
		return false
	}
}

func parseQuickstartRFC3339(raw string) *int64 {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return nil
	}
	unix := parsed.UTC().Unix()
	return &unix
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
