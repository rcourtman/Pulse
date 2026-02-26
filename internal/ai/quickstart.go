package ai

import (
	"fmt"
	"sync"

	"github.com/rcourtman/pulse-go-rewrite/internal/ai/providers"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// QuickstartCreditManager provides credit checking and consumption for quickstart
// patrol runs. It is injected into the PatrolService and AI Service by the router.
type QuickstartCreditManager interface {
	// HasCredits returns true if quickstart credits are available (granted and not exhausted).
	HasCredits() bool
	// CreditsRemaining returns the number of unused credits (0 if not granted).
	CreditsRemaining() int
	// ConsumeCredit decrements one credit after a successful patrol run.
	// Returns false if no credits remain.
	ConsumeCredit() error
	// HasBYOK returns true if the user has at least one provider with their own API key.
	HasBYOK() bool
	// GetProvider returns a quickstart Provider for making LLM calls through the hosted proxy.
	// Returns nil if credits are exhausted.
	GetProvider() providers.Provider
	// GrantCredits ensures credits are granted (idempotent). Called on first AI enable.
	GrantCredits() error
}

// FileQuickstartCreditManager implements QuickstartCreditManager backed by FileBillingStore.
type FileQuickstartCreditManager struct {
	mu           sync.Mutex
	billingStore *config.FileBillingStore
	orgID        string
	aiConfig     func() *config.AIConfig // Getter for current AI config (for BYOK check)
	licenseID    string                  // Workspace identifier for the proxy
}

// NewFileQuickstartCreditManager creates a credit manager backed by the billing store.
func NewFileQuickstartCreditManager(
	billingStore *config.FileBillingStore,
	orgID string,
	aiConfigGetter func() *config.AIConfig,
	licenseID string,
) *FileQuickstartCreditManager {
	return &FileQuickstartCreditManager{
		billingStore: billingStore,
		orgID:        orgID,
		aiConfig:     aiConfigGetter,
		licenseID:    licenseID,
	}
}

func (m *FileQuickstartCreditManager) getBillingState() *pkglicensing.BillingState {
	if m.billingStore == nil {
		return nil
	}
	state, err := m.billingStore.GetBillingState(m.orgID)
	if err != nil {
		log.Warn().Err(err).Msg("Quickstart: failed to read billing state")
		return nil
	}
	return state
}

func (m *FileQuickstartCreditManager) HasCredits() bool {
	state := m.getBillingState()
	if state == nil {
		return false
	}
	return state.HasQuickstartCredits()
}

func (m *FileQuickstartCreditManager) CreditsRemaining() int {
	state := m.getBillingState()
	if state == nil {
		return 0
	}
	return state.QuickstartCreditsRemaining()
}

func (m *FileQuickstartCreditManager) ConsumeCredit() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.billingStore.GetBillingState(m.orgID)
	if err != nil {
		return fmt.Errorf("quickstart: read billing state: %w", err)
	}
	if state == nil {
		return fmt.Errorf("quickstart: no billing state")
	}

	if !state.ConsumeQuickstartCredit() {
		return fmt.Errorf("quickstart: no credits remaining")
	}

	if err := m.billingStore.SaveBillingState(m.orgID, state); err != nil {
		return fmt.Errorf("quickstart: save billing state: %w", err)
	}

	remaining := state.QuickstartCreditsRemaining()
	log.Info().
		Int("used", state.QuickstartCreditsUsed).
		Int("remaining", remaining).
		Msg("Quickstart: consumed one patrol credit")

	return nil
}

func (m *FileQuickstartCreditManager) HasBYOK() bool {
	cfg := m.aiConfig()
	if cfg == nil {
		return false
	}
	return len(cfg.GetConfiguredProviders()) > 0
}

func (m *FileQuickstartCreditManager) GetProvider() providers.Provider {
	if !m.HasCredits() {
		return nil
	}
	return providers.NewQuickstartClient(m.licenseID)
}

func (m *FileQuickstartCreditManager) GrantCredits() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, err := m.billingStore.GetBillingState(m.orgID)
	if err != nil {
		return fmt.Errorf("quickstart: read billing state: %w", err)
	}
	if state == nil {
		state = pkglicensing.DefaultBillingState()
	}

	if !state.GrantQuickstartCredits() {
		// Already granted — idempotent.
		return nil
	}

	if err := m.billingStore.SaveBillingState(m.orgID, state); err != nil {
		return fmt.Errorf("quickstart: save billing state: %w", err)
	}

	log.Info().Msg("Quickstart: granted 25 free patrol credits to workspace")
	return nil
}
