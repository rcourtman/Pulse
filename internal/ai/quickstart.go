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
	bearerToken, fingerprint, err := m.secureBootstrapIdentityLocked()
	if err != nil {
		return err
	}
	if !m.bootstrapNeededLocked(state, m.now()) {
		return nil
	}

	req, err := m.bootstrapRequestLocked(fingerprint)
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
	if m == nil {
		return false
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to read cached state")
		return false
	}
	if state == nil || state.QuickstartCreditsRemaining <= 0 {
		return false
	}
	_, _, err = m.secureBootstrapIdentityLocked()
	return err == nil
}

func (m *PersistentQuickstartCreditManager) CreditsRemaining() int {
	if m == nil {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to read cached state")
		return 0
	}
	if state == nil {
		return 0
	}
	if _, _, err := m.secureBootstrapIdentityLocked(); err != nil {
		return 0
	}
	return state.QuickstartCreditsRemaining
}

func (m *PersistentQuickstartCreditManager) CreditsTotal() int {
	if m == nil {
		return 0
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.loadStateLocked()
	if err != nil {
		log.Warn().Err(err).Str("orgID", m.orgID).Msg("Quickstart: failed to read cached state")
		return 0
	}
	if state == nil {
		return 0
	}
	if _, _, err := m.secureBootstrapIdentityLocked(); err != nil {
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
	if _, _, err := m.secureBootstrapIdentityLocked(); err != nil {
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

func (m *PersistentQuickstartCreditManager) bootstrapRequestLocked(fingerprint string) (pkglicensing.QuickstartBootstrapRequest, error) {
	instanceName := ""
	if hostname, err := m.hostname(); err == nil {
		instanceName = strings.TrimSpace(hostname)
	}

	return pkglicensing.QuickstartBootstrapRequest{
		InstanceFingerprint: strings.TrimSpace(fingerprint),
		InstanceName:        instanceName,
		UseCase:             quickstartBootstrapUseCase,
	}, nil
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

	updated := false
	if serverState.CreditsRemaining != nil {
		state.QuickstartCreditsRemaining = max(0, *serverState.CreditsRemaining)
		updated = true
	}
	if serverState.CreditsTotal != nil {
		state.QuickstartCreditsTotal = max(0, *serverState.CreditsTotal)
		updated = true
	}
	if serverState.TokenExpiresAt != nil {
		expiryUnix := serverState.TokenExpiresAt.UTC().Unix()
		state.QuickstartTokenExpiresAt = &expiryUnix
		updated = true
	}
	if !updated {
		return
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

func (m *PersistentQuickstartCreditManager) secureBootstrapIdentityLocked() (string, string, error) {
	if m.persistence == nil {
		return "", "", fmt.Errorf("quickstart: persistence unavailable")
	}

	licensePersistence, err := pkglicensing.NewPersistence(m.persistence.SharedInstallationDataDir())
	if err != nil {
		return "", "", fmt.Errorf("quickstart: load license persistence: %w", err)
	}
	activationState, err := licensePersistence.LoadActivationState()
	if err != nil {
		return "", "", fmt.Errorf("quickstart: load activation state: %w", err)
	}
	if activationState == nil {
		return "", "", quickstartActivationRequiredError()
	}

	installationToken := strings.TrimSpace(activationState.InstallationToken)
	instanceFingerprint := strings.TrimSpace(activationState.InstanceFingerprint)
	if installationToken == "" || instanceFingerprint == "" {
		return "", "", quickstartActivationRequiredError()
	}

	return installationToken, instanceFingerprint, nil
}

func quickstartBlockedReasonFromError(err error) string {
	switch {
	case err == nil:
		return ""
	case providers.IsQuickstartCreditsExhausted(err), quickstartBootstrapCreditsExhausted(err):
		return patrolQuickstartCreditsExhaustedReason
	case quickstartBootstrapActivationRequired(err):
		return patrolQuickstartActivationRequiredReason
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

// QuickstartCreditsExhaustedReason returns the canonical Patrol quickstart
// availability message shown when the install has consumed its quickstart runs.
func QuickstartCreditsExhaustedReason() string {
	return patrolQuickstartCreditsExhaustedReason
}

// QuickstartUnavailableReason returns the canonical Patrol quickstart
// availability message shown when the server-authoritative bootstrap path
// cannot be reached.
func QuickstartUnavailableReason() string {
	return patrolQuickstartUnavailableReason
}

// QuickstartActivationRequiredReason returns the canonical Patrol quickstart
// availability message shown when the install is not activated or trial-backed.
func QuickstartActivationRequiredReason() string {
	return patrolQuickstartActivationRequiredReason
}

func quickstartActivationRequiredError() error {
	return &pkglicensing.LicenseServerError{
		StatusCode: http.StatusUnauthorized,
		Code:       "activation_required",
		Message:    "Quickstart bootstrap requires an activated or trial-backed installation",
		Retryable:  false,
	}
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

func quickstartBootstrapActivationRequired(err error) bool {
	var serverErr *pkglicensing.LicenseServerError
	if !errors.As(err, &serverErr) {
		return false
	}
	switch serverErr.StatusCode {
	case http.StatusUnauthorized, http.StatusForbidden:
		return true
	}

	switch strings.ToLower(strings.TrimSpace(serverErr.Code)) {
	case "activation_required", "installation_required", "invalid_installation_token", "invalid_token":
		return true
	default:
		return false
	}
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
