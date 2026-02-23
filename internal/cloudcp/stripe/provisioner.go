package stripe

import (
	"context"
	"crypto/rand"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	cpauth "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/auth"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/cpmetrics"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/docker"
	cpemail "github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/email"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	"github.com/rcourtman/pulse-go-rewrite/internal/config"
	"github.com/rcourtman/pulse-go-rewrite/pkg/cloudauth"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

// Provisioner orchestrates tenant creation, billing state updates, and
// container lifecycle in response to Stripe events.
type Provisioner struct {
	registry        *registry.TenantRegistry
	tenantsDir      string
	docker          *docker.Manager // nil if Docker is unavailable
	magicLinks      *cpauth.Service // nil if magic links disabled
	baseURL         string          // e.g. "https://cloud.pulserelay.pro"
	allowDockerless bool
	emailSender     cpemail.Sender
	emailFrom       string
}

type provisioningCleanupState struct {
	tenantID      string
	tenantDataDir string
	containerID   string
	tenantCreated bool
}

// NewProvisioner creates a Provisioner.
func NewProvisioner(reg *registry.TenantRegistry, tenantsDir string, dockerMgr *docker.Manager, magicLinks *cpauth.Service, baseURL string, emailSender cpemail.Sender, emailFrom string, allowDockerless bool) *Provisioner {
	return &Provisioner{
		registry:        reg,
		tenantsDir:      tenantsDir,
		docker:          dockerMgr,
		magicLinks:      magicLinks,
		baseURL:         baseURL,
		allowDockerless: allowDockerless,
		emailSender:     emailSender,
		emailFrom:       strings.TrimSpace(emailFrom),
	}
}

func (p *Provisioner) tenantDataDir(tenantID string) string {
	return filepath.Join(p.tenantsDir, tenantID)
}

func (p *Provisioner) ensureTenantDirs(tenantID string) (tenantDataDir, secretsDir string, err error) {
	tenantDataDir = p.tenantDataDir(tenantID)
	if err := os.MkdirAll(tenantDataDir, 0o755); err != nil {
		return "", "", fmt.Errorf("create tenant data dir: %w", err)
	}
	secretsDir = filepath.Join(tenantDataDir, "secrets")
	if err := os.MkdirAll(secretsDir, 0o700); err != nil {
		return "", "", fmt.Errorf("create tenant secrets dir: %w", err)
	}
	return tenantDataDir, secretsDir, nil
}

func (p *Provisioner) writeHandoffKey(secretsDir string) error {
	key := make([]byte, 32)
	if _, err := rand.Read(key); err != nil {
		return fmt.Errorf("generate handoff key: %w", err)
	}
	path := filepath.Join(secretsDir, "handoff.key")
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return fmt.Errorf("write handoff key: %w", err)
	}
	return nil
}

func (p *Provisioner) writeCloudHandoffKey(tenantDataDir string) error {
	key, err := cloudauth.GenerateHandoffKey()
	if err != nil {
		return fmt.Errorf("generate cloud handoff key: %w", err)
	}
	path := filepath.Join(tenantDataDir, cloudauth.HandoffKeyFile)
	if err := os.WriteFile(path, key, 0o600); err != nil {
		return fmt.Errorf("write cloud handoff key: %w", err)
	}
	return nil
}

func (p *Provisioner) pollHealth(ctx context.Context, containerID string) bool {
	if p.docker == nil || containerID == "" {
		return false
	}
	const (
		interval = 2 * time.Second
		timeout  = 60 * time.Second
	)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	timeoutTimer := time.NewTimer(timeout)
	defer timeoutTimer.Stop()

	for {
		ok, err := p.docker.HealthCheck(ctx, containerID)
		if err == nil && ok {
			return true
		}

		select {
		case <-ctx.Done():
			return false
		case <-timeoutTimer.C:
			return false
		case <-ticker.C:
		}
	}
}

func (p *Provisioner) generateAndLogMagicLink(email, tenantID string) {
	if p.magicLinks == nil || email == "" {
		return
	}
	token, err := p.magicLinks.GenerateToken(email, tenantID)
	if err != nil {
		log.Error().Err(err).Str("tenant_id", tenantID).Msg("Failed to generate magic link token")
		return
	}
	magicURL := cpauth.BuildVerifyURL(p.baseURL, token)
	if magicURL == "" {
		log.Error().
			Str("tenant_id", tenantID).
			Str("base_url", strings.TrimSpace(p.baseURL)).
			Msg("Failed to build magic link URL")
		return
	}

	// Try to send email
	if p.emailSender != nil && p.emailFrom != "" {
		htmlBody, textBody, err := cpemail.RenderMagicLinkEmail(cpemail.MagicLinkData{
			MagicLinkURL: magicURL,
		})
		if err != nil {
			log.Error().Err(err).Str("tenant_id", tenantID).Msg("Failed to render magic link email")
		} else {
			if sendErr := p.emailSender.Send(context.Background(), cpemail.Message{
				From:    p.emailFrom,
				To:      email,
				Subject: "Sign in to Pulse",
				HTML:    htmlBody,
				Text:    textBody,
			}); sendErr != nil {
				log.Error().Err(sendErr).
					Str("tenant_id", tenantID).
					Str("email", email).
					Msg("Failed to send magic link email — falling back to log")
			} else {
				log.Info().
					Str("tenant_id", tenantID).
					Str("email", email).
					Msg("Magic link email sent")
				return // Email sent successfully, don't log the URL
			}
		}
	}

	// Fallback: log the magic link URL
	log.Info().
		Str("tenant_id", tenantID).
		Str("email", email).
		Str("magic_link_url_redacted", redactMagicLinkURL(magicURL)).
		Msg("Magic link generated for new tenant")
}

func redactMagicLinkURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil {
		return ""
	}
	if u.Scheme == "" || u.Host == "" {
		return ""
	}
	u.RawQuery = ""
	u.Fragment = ""
	return u.String()
}

func (p *Provisioner) writeBillingState(tenantDataDir string, state *pkglicensing.BillingState) error {
	billingStore := config.NewFileBillingStore(tenantDataDir)
	if err := billingStore.SaveBillingState("default", state); err != nil {
		return fmt.Errorf("write billing state: %w", err)
	}
	return nil
}

func (p *Provisioner) maybeStartContainer(ctx context.Context, tenantID, tenantDataDir string) (containerID string, err error) {
	if p.docker == nil {
		if p.allowDockerless {
			log.Warn().
				Str("tenant_id", tenantID).
				Msg("Docker unavailable; CP_ALLOW_DOCKERLESS_PROVISIONING enabled")
			return "", nil
		}
		return "", fmt.Errorf("docker manager unavailable")
	}
	id, err := p.docker.CreateAndStart(ctx, tenantID, tenantDataDir)
	if err != nil {
		if p.allowDockerless && dockerUnavailableError(err) {
			log.Warn().
				Err(err).
				Str("tenant_id", tenantID).
				Msg("Docker start failed; continuing because CP_ALLOW_DOCKERLESS_PROVISIONING is enabled")
			return "", nil
		}
		return "", err
	}
	return id, nil
}

func dockerUnavailableError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	switch {
	case strings.Contains(msg, "cannot connect to the docker daemon"):
		return true
	case strings.Contains(msg, "dial unix") && strings.Contains(msg, "docker.sock"):
		return true
	case strings.Contains(msg, "connection refused"):
		return true
	case strings.Contains(msg, "no such file or directory") && strings.Contains(msg, "docker.sock"):
		return true
	default:
		return false
	}
}

func (p *Provisioner) ensureAccountOwnerMembership(accountID, email string) error {
	accountID = strings.TrimSpace(accountID)
	email = strings.ToLower(strings.TrimSpace(email))
	if accountID == "" || email == "" {
		return nil
	}

	user, err := p.registry.GetUserByEmail(email)
	if err != nil {
		return fmt.Errorf("lookup user by email: %w", err)
	}
	if user == nil {
		userID, genErr := registry.GenerateUserID()
		if genErr != nil {
			return fmt.Errorf("generate user id: %w", genErr)
		}
		candidate := &registry.User{
			ID:    userID,
			Email: email,
		}
		if createErr := p.registry.CreateUser(candidate); createErr != nil {
			reloaded, reloadErr := p.registry.GetUserByEmail(email)
			if reloadErr != nil || reloaded == nil {
				return fmt.Errorf("create user: %w", createErr)
			}
			user = reloaded
		} else {
			user = candidate
		}
	}

	m, err := p.registry.GetMembership(accountID, user.ID)
	if err != nil {
		return fmt.Errorf("lookup membership: %w", err)
	}
	if m == nil {
		m = &registry.AccountMembership{
			AccountID: accountID,
			UserID:    user.ID,
			Role:      registry.MemberRoleOwner,
		}
		if createErr := p.registry.CreateMembership(m); createErr != nil {
			reloaded, reloadErr := p.registry.GetMembership(accountID, user.ID)
			if reloadErr != nil || reloaded == nil {
				return fmt.Errorf("create membership: %w", createErr)
			}
		}
	}

	_ = p.registry.UpdateUserLastLogin(user.ID)
	return nil
}

func (p *Provisioner) maybeStopAndRemoveContainer(ctx context.Context, containerID string) error {
	if p.docker == nil || strings.TrimSpace(containerID) == "" {
		return nil
	}
	return p.docker.StopAndRemove(ctx, containerID)
}

func (p *Provisioner) rollbackProvisioning(state provisioningCleanupState) {
	if p == nil {
		return
	}

	// Use a fresh context so cleanup still runs if the request context was canceled.
	cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := p.maybeStopAndRemoveContainer(cleanupCtx, state.containerID); err != nil {
		log.Warn().
			Err(err).
			Str("tenant_id", state.tenantID).
			Str("container_id", state.containerID).
			Msg("Provisioning rollback: failed to remove container")
	}

	if state.tenantCreated && strings.TrimSpace(state.tenantID) != "" {
		if err := p.registry.Delete(state.tenantID); err != nil {
			log.Warn().
				Err(err).
				Str("tenant_id", state.tenantID).
				Msg("Provisioning rollback: failed to delete tenant registry record")
		}
	}

	if strings.TrimSpace(state.tenantDataDir) == "" {
		return
	}
	if err := os.RemoveAll(state.tenantDataDir); err != nil {
		log.Warn().
			Err(err).
			Str("tenant_id", state.tenantID).
			Str("tenant_data_dir", state.tenantDataDir).
			Msg("Provisioning rollback: failed to remove tenant data directory")
	}
}

// HandleCheckout provisions a new tenant from a checkout.session.completed event.
func (p *Provisioner) HandleCheckout(ctx context.Context, session CheckoutSession) (err error) {
	cpmetrics.ProvisioningTotal.WithLabelValues("attempt").Inc()
	cleanup := provisioningCleanupState{}
	skippedExisting := false
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
			p.rollbackProvisioning(cleanup)
		} else if skippedExisting {
			outcome = "skipped_existing"
		}
		cpmetrics.ProvisioningTotal.WithLabelValues(outcome).Inc()
	}()

	customerID := strings.TrimSpace(session.Customer)
	if customerID == "" {
		return fmt.Errorf("checkout session missing customer")
	}
	if !IsSafeStripeID(customerID) {
		return fmt.Errorf("invalid stripe customer id: %s", customerID)
	}

	email := strings.ToLower(strings.TrimSpace(session.CustomerEmail))
	if email == "" {
		email = strings.ToLower(strings.TrimSpace(session.CustomerDetails.Email))
	}

	// Consolidated billing: one Stripe customer per account.
	// For individual Cloud signups, we create an "individual" account on first checkout.
	accountID := ""
	sa, err := p.registry.GetStripeAccountByCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup stripe account by customer: %w", err)
	}
	if sa != nil {
		accountID = strings.TrimSpace(sa.AccountID)
	}

	// Check if a tenant already exists for this Stripe customer
	existing, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup existing tenant: %w", err)
	}
	if existing != nil {
		log.Info().
			Str("tenant_id", existing.ID).
			Str("customer_id", customerID).
			Msg("Tenant already exists for Stripe customer, skipping provisioning")
		skippedExisting = true
		return nil
	}

	// Generate tenant ID
	tenantID, err := registry.GenerateTenantID()
	if err != nil {
		return fmt.Errorf("generate tenant id: %w", err)
	}

	planVersion := DerivePlanVersion(session.Metadata, "")

	// Ensure the account exists for this Stripe customer (individual Cloud signup path).
	if accountID == "" {
		kind := registry.AccountKindIndividual
		if session.Metadata != nil {
			switch strings.ToLower(strings.TrimSpace(session.Metadata["account_kind"])) {
			case "msp":
				kind = registry.AccountKindMSP
			case "individual", "":
				kind = registry.AccountKindIndividual
			}
		}

		displayName := ""
		if session.Metadata != nil {
			displayName = strings.TrimSpace(session.Metadata["account_display_name"])
			if displayName == "" {
				displayName = strings.TrimSpace(session.Metadata["display_name"])
			}
		}
		if displayName == "" {
			displayName = email
		}

		newAccountID, err := registry.GenerateAccountID()
		if err != nil {
			return fmt.Errorf("generate account id: %w", err)
		}
		a := &registry.Account{
			ID:          newAccountID,
			Kind:        kind,
			DisplayName: displayName,
		}
		if err := p.registry.CreateAccount(a); err != nil {
			return fmt.Errorf("create account: %w", err)
		}

		newSA := &registry.StripeAccount{
			AccountID:                 a.ID,
			StripeCustomerID:          customerID,
			StripeSubscriptionID:      strings.TrimSpace(session.Subscription),
			PlanVersion:               planVersion,
			SubscriptionState:         "trial",
			StripeSubItemWorkspacesID: "",
		}
		if err := p.registry.CreateStripeAccount(newSA); err != nil {
			// Best-effort fallback: if a competing worker created the row, reuse it.
			existingSA, getErr := p.registry.GetStripeAccountByCustomerID(customerID)
			if getErr != nil || existingSA == nil {
				return fmt.Errorf("create stripe account mapping: %w", err)
			}
			accountID = strings.TrimSpace(existingSA.AccountID)
		} else {
			accountID = a.ID
		}
	} else if sa != nil {
		// Backfill subscription ID/plan version if the mapping exists but hasn't been updated yet.
		changed := false
		if strings.TrimSpace(sa.StripeSubscriptionID) == "" && strings.TrimSpace(session.Subscription) != "" {
			sa.StripeSubscriptionID = strings.TrimSpace(session.Subscription)
			changed = true
		}
		if strings.TrimSpace(sa.PlanVersion) == "" && strings.TrimSpace(planVersion) != "" {
			sa.PlanVersion = strings.TrimSpace(planVersion)
			changed = true
		}
		if changed {
			if updateErr := p.registry.UpdateStripeAccount(sa); updateErr != nil {
				log.Warn().
					Err(updateErr).
					Str("customer_id", customerID).
					Str("account_id", sa.AccountID).
					Msg("Failed to backfill Stripe account metadata")
			}
		}
	}

	if err := p.ensureAccountOwnerMembership(accountID, email); err != nil {
		return fmt.Errorf("ensure account owner membership: %w", err)
	}

	tenantDataDir, secretsDir, err := p.ensureTenantDirs(tenantID)
	if err != nil {
		return fmt.Errorf("prepare tenant directories for %s: %w", tenantID, err)
	}
	cleanup.tenantDataDir = tenantDataDir
	if err := p.writeHandoffKey(secretsDir); err != nil {
		return fmt.Errorf("write handoff key for tenant %s: %w", tenantID, err)
	}
	if err := p.writeCloudHandoffKey(tenantDataDir); err != nil {
		return fmt.Errorf("write cloud handoff key for tenant %s: %w", tenantID, err)
	}

	state := &pkglicensing.BillingState{
		Capabilities:         pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil),
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          planVersion,
		SubscriptionState:    pkglicensing.SubStateActive,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(session.Subscription),
	}
	if err := p.writeBillingState(tenantDataDir, state); err != nil {
		return fmt.Errorf("write initial billing state for tenant %s: %w", tenantID, err)
	}

	// Insert registry record
	tenant := &registry.Tenant{
		ID:                   tenantID,
		AccountID:            strings.TrimSpace(accountID),
		Email:                email,
		State:                registry.TenantStateProvisioning,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(session.Subscription),
		PlanVersion:          planVersion,
	}
	if err := p.registry.Create(tenant); err != nil {
		return fmt.Errorf("create tenant record: %w", err)
	}
	cleanup.tenantID = tenantID
	cleanup.tenantCreated = true

	// Start container if Docker is available.
	containerID, err := p.maybeStartContainer(ctx, tenantID, tenantDataDir)
	if err != nil {
		return fmt.Errorf("start container: %w", err)
	}
	tenant.ContainerID = containerID
	cleanup.containerID = containerID

	// Poll health check before declaring the tenant active.
	if containerID == "" {
		if p.allowDockerless {
			tenant.State = registry.TenantStateActive
			if err := p.registry.Update(tenant); err != nil {
				return fmt.Errorf("update tenant record: %w", err)
			}
			p.generateAndLogMagicLink(email, tenantID)
			log.Warn().
				Str("tenant_id", tenantID).
				Msg("Provisioned without container because CP_ALLOW_DOCKERLESS_PROVISIONING is enabled")
			return nil
		}
		return fmt.Errorf("container did not start for tenant %s", tenantID)
	}
	if p.pollHealth(ctx, containerID) {
		tenant.State = registry.TenantStateActive
		if err := p.registry.Update(tenant); err != nil {
			return fmt.Errorf("update tenant record: %w", err)
		}
		p.generateAndLogMagicLink(email, tenantID)
	} else {
		log.Warn().
			Str("tenant_id", tenantID).
			Str("container_id", containerID[:min(12, len(containerID))]).
			Msg("Container health check timed out; aborting provisioning")
		return fmt.Errorf("tenant %s container failed health check", tenantID)
	}

	log.Info().
		Str("tenant_id", tenantID).
		Str("customer_id", customerID).
		Str("email", email).
		Str("plan_version", planVersion).
		Msg("Tenant provisioned from checkout")

	return nil
}

func normalizeStripeAccountSubscriptionState(status string) string {
	switch strings.ToLower(strings.TrimSpace(status)) {
	case "active":
		return "active"
	case "trialing":
		return "trial"
	case "canceled":
		return "canceled"
	case "past_due", "unpaid", "paused", "incomplete", "incomplete_expired":
		return "past_due"
	default:
		return "past_due"
	}
}

func applyStripeAccountGraceWindow(sa *registry.StripeAccount, subState pkglicensing.SubscriptionState, now time.Time) {
	if sa == nil {
		return
	}
	if subState == pkglicensing.SubStateGrace {
		if sa.GraceStartedAt == nil || *sa.GraceStartedAt <= 0 {
			ts := now.UTC().Unix()
			sa.GraceStartedAt = &ts
		}
		return
	}
	sa.GraceStartedAt = nil
}

func planVersionFromMetadata(metadata map[string]string, fallback string) string {
	if metadata != nil {
		if v := strings.TrimSpace(metadata["plan_version"]); v != "" {
			return v
		}
		if v := strings.TrimSpace(metadata["plan"]); v != "" {
			return v
		}
	}
	return strings.TrimSpace(fallback)
}

// ProvisionWorkspace provisions a new workspace (tenant) under an account, without Stripe checkout.
func (p *Provisioner) ProvisionWorkspace(ctx context.Context, accountID, displayName string) (tenant *registry.Tenant, err error) {
	cpmetrics.ProvisioningTotal.WithLabelValues("attempt").Inc()
	cleanup := provisioningCleanupState{}
	defer func() {
		outcome := "success"
		if err != nil {
			outcome = "error"
			p.rollbackProvisioning(cleanup)
		}
		cpmetrics.ProvisioningTotal.WithLabelValues(outcome).Inc()
	}()

	accountID = strings.TrimSpace(accountID)
	displayName = strings.TrimSpace(displayName)
	if accountID == "" {
		return nil, fmt.Errorf("missing account id")
	}
	if displayName == "" {
		return nil, fmt.Errorf("missing display name")
	}

	tenantID, err := registry.GenerateTenantID()
	if err != nil {
		return nil, fmt.Errorf("generate tenant id: %w", err)
	}

	tenantDataDir, secretsDir, err := p.ensureTenantDirs(tenantID)
	if err != nil {
		return nil, fmt.Errorf("prepare tenant directories for %s: %w", tenantID, err)
	}
	cleanup.tenantDataDir = tenantDataDir
	if err := p.writeHandoffKey(secretsDir); err != nil {
		return nil, fmt.Errorf("write handoff key for tenant %s: %w", tenantID, err)
	}
	if err := p.writeCloudHandoffKey(tenantDataDir); err != nil {
		return nil, fmt.Errorf("write cloud handoff key for tenant %s: %w", tenantID, err)
	}

	planVersion := "msp_hosted_v1"
	state := &pkglicensing.BillingState{
		Capabilities:      pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil),
		Limits:            map[string]int64{},
		MetersEnabled:     []string{},
		PlanVersion:       planVersion,
		SubscriptionState: pkglicensing.SubStateActive,
	}
	if err := p.writeBillingState(tenantDataDir, state); err != nil {
		return nil, fmt.Errorf("write initial billing state for tenant %s: %w", tenantID, err)
	}

	tenant = &registry.Tenant{
		ID:          tenantID,
		AccountID:   accountID,
		DisplayName: displayName,
		State:       registry.TenantStateProvisioning,
		PlanVersion: planVersion,
	}
	if err := p.registry.Create(tenant); err != nil {
		return nil, fmt.Errorf("create tenant record: %w", err)
	}
	cleanup.tenantID = tenantID
	cleanup.tenantCreated = true

	containerID, err := p.maybeStartContainer(ctx, tenantID, tenantDataDir)
	if err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}
	if containerID == "" {
		if p.allowDockerless {
			tenant.State = registry.TenantStateActive
			if err := p.registry.Update(tenant); err != nil {
				return nil, fmt.Errorf("update tenant record: %w", err)
			}
			log.Warn().
				Str("tenant_id", tenantID).
				Msg("Provisioned workspace without container because CP_ALLOW_DOCKERLESS_PROVISIONING is enabled")
			return tenant, nil
		}
		return nil, fmt.Errorf("container did not start for tenant %s", tenantID)
	}
	tenant.ContainerID = containerID
	cleanup.containerID = containerID
	if !p.pollHealth(ctx, containerID) {
		return nil, fmt.Errorf("tenant %s container failed health check", tenantID)
	}
	tenant.State = registry.TenantStateActive
	if err := p.registry.Update(tenant); err != nil {
		return nil, fmt.Errorf("update tenant record: %w", err)
	}

	return tenant, nil
}

// DeprovisionWorkspaceContainer stops/removes a workspace container if Docker is available.
func (p *Provisioner) DeprovisionWorkspaceContainer(ctx context.Context, tenant *registry.Tenant) error {
	if tenant == nil {
		return nil
	}
	if err := p.maybeStopAndRemoveContainer(ctx, tenant.ContainerID); err != nil {
		return fmt.Errorf("stop/remove container: %w", err)
	}
	return nil
}

// HandleSubscriptionUpdated syncs billing state when a subscription changes.
func (p *Provisioner) HandleSubscriptionUpdated(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	tenant, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup tenant by customer: %w", err)
	}
	if tenant == nil {
		log.Warn().Str("customer_id", customerID).Msg("subscription.updated: tenant not found")
		return nil
	}

	subState := MapSubscriptionStatus(sub.Status)
	priceID := sub.FirstPriceID()
	planVersion := DerivePlanVersion(sub.Metadata, priceID)
	if (planVersion == "" || planVersion == "stripe") && strings.TrimSpace(tenant.PlanVersion) != "" {
		planVersion = strings.TrimSpace(tenant.PlanVersion)
	}

	// Update billing.json
	var caps []string
	if ShouldGrantCapabilities(subState) {
		caps = pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil)
	}

	tenantDataDir := p.tenantDataDir(tenant.ID)
	state := &pkglicensing.BillingState{
		Capabilities:         caps,
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          planVersion,
		SubscriptionState:    subState,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(sub.ID),
		StripePriceID:        priceID,
	}
	if err := p.writeBillingState(tenantDataDir, state); err != nil {
		return fmt.Errorf("write billing state for tenant %s: %w", tenant.ID, err)
	}

	// Update registry
	tenant.StripeSubscriptionID = strings.TrimSpace(sub.ID)
	tenant.StripePriceID = priceID
	tenant.PlanVersion = planVersion
	if subState == pkglicensing.SubStateSuspended {
		tenant.State = registry.TenantStateSuspended
	} else if subState == pkglicensing.SubStateActive || subState == pkglicensing.SubStateTrial || subState == pkglicensing.SubStateGrace {
		tenant.State = registry.TenantStateActive
	} else if subState == pkglicensing.SubStateCanceled || subState == pkglicensing.SubStateExpired {
		tenant.State = registry.TenantStateCanceled
	}
	if err := p.registry.Update(tenant); err != nil {
		return fmt.Errorf("update tenant record: %w", err)
	}

	if sa, saErr := p.registry.GetStripeAccountByCustomerID(customerID); saErr == nil && sa != nil {
		sa.StripeSubscriptionID = strings.TrimSpace(sub.ID)
		sa.PlanVersion = planVersion
		sa.SubscriptionState = normalizeStripeAccountSubscriptionState(sub.Status)
		applyStripeAccountGraceWindow(sa, subState, time.Now().UTC())
		if updateErr := p.registry.UpdateStripeAccount(sa); updateErr != nil {
			log.Warn().
				Err(updateErr).
				Str("tenant_id", tenant.ID).
				Str("customer_id", customerID).
				Msg("Failed to update stripe account mapping after subscription update")
		}
	}

	log.Info().
		Str("tenant_id", tenant.ID).
		Str("customer_id", customerID).
		Str("subscription_state", string(subState)).
		Msg("Subscription updated")

	return nil
}

// HandleSubscriptionDeleted revokes capabilities on cancellation.
func (p *Provisioner) HandleSubscriptionDeleted(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	tenant, err := p.registry.GetByStripeCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup tenant by customer: %w", err)
	}
	if tenant == nil {
		log.Warn().Str("customer_id", customerID).Msg("subscription.deleted: tenant not found")
		return nil
	}

	// Revoke capabilities immediately
	tenantDataDir := p.tenantDataDir(tenant.ID)
	state := &pkglicensing.BillingState{
		Capabilities:         []string{},
		Limits:               map[string]int64{},
		MetersEnabled:        []string{},
		PlanVersion:          tenant.PlanVersion,
		SubscriptionState:    pkglicensing.SubStateCanceled,
		StripeCustomerID:     customerID,
		StripeSubscriptionID: strings.TrimSpace(sub.ID),
	}
	if err := p.writeBillingState(tenantDataDir, state); err != nil {
		return fmt.Errorf("write canceled billing state for tenant %s: %w", tenant.ID, err)
	}

	// Update registry
	tenant.State = registry.TenantStateCanceled
	if err := p.registry.Update(tenant); err != nil {
		return fmt.Errorf("update tenant record: %w", err)
	}
	if sa, saErr := p.registry.GetStripeAccountByCustomerID(customerID); saErr == nil && sa != nil {
		sa.StripeSubscriptionID = strings.TrimSpace(sub.ID)
		sa.SubscriptionState = "canceled"
		sa.GraceStartedAt = nil
		if updateErr := p.registry.UpdateStripeAccount(sa); updateErr != nil {
			log.Warn().
				Err(updateErr).
				Str("tenant_id", tenant.ID).
				Str("customer_id", customerID).
				Msg("Failed to update stripe account mapping after subscription delete")
		}
	}

	log.Info().
		Str("tenant_id", tenant.ID).
		Str("customer_id", customerID).
		Msg("Subscription deleted, capabilities revoked")

	return nil
}

// HandleInvoicePaymentFailed transitions subscription state to grace/past_due.
func (p *Provisioner) HandleInvoicePaymentFailed(ctx context.Context, invoice Invoice) error {
	customerID := strings.TrimSpace(invoice.Customer)
	if customerID == "" {
		return fmt.Errorf("invoice missing customer")
	}
	sub := Subscription{
		ID:       strings.TrimSpace(invoice.Subscription),
		Customer: customerID,
		Status:   "past_due",
	}
	return p.HandleSubscriptionUpdated(ctx, sub)
}

// HandleMSPSubscriptionUpdated updates billing state for all tenants under an MSP account.
func (p *Provisioner) HandleMSPSubscriptionUpdated(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	sa, err := p.registry.GetStripeAccountByCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup stripe account by customer: %w", err)
	}
	if sa == nil {
		log.Warn().Str("customer_id", customerID).Msg("msp subscription.updated: stripe account mapping not found")
		return nil
	}

	account, err := p.registry.GetAccount(sa.AccountID)
	if err != nil {
		return fmt.Errorf("lookup account: %w", err)
	}
	if account == nil {
		log.Warn().Str("account_id", sa.AccountID).Msg("msp subscription.updated: account not found")
		return nil
	}
	if account.Kind != registry.AccountKindMSP {
		if err := p.HandleSubscriptionUpdated(ctx, sub); err != nil {
			return fmt.Errorf("handle non-msp subscription update: %w", err)
		}
		return nil
	}

	subState := MapSubscriptionStatus(sub.Status)
	priceID := sub.FirstPriceID()

	planVersion := planVersionFromMetadata(sub.Metadata, sa.PlanVersion)
	if planVersion == "" {
		planVersion = "msp_hosted_v1"
	}

	// Persist account-level Stripe state.
	sa.StripeSubscriptionID = strings.TrimSpace(sub.ID)
	sa.PlanVersion = planVersion
	sa.SubscriptionState = normalizeStripeAccountSubscriptionState(sub.Status)
	applyStripeAccountGraceWindow(sa, subState, time.Now().UTC())
	if err := p.registry.UpdateStripeAccount(sa); err != nil {
		return fmt.Errorf("update stripe account: %w", err)
	}

	tenants, err := p.registry.ListByAccountID(sa.AccountID)
	if err != nil {
		return fmt.Errorf("list tenants by account: %w", err)
	}

	var caps []string
	if ShouldGrantCapabilities(subState) {
		caps = pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil)
	}

	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		tenantDataDir := p.tenantDataDir(tenant.ID)
		state := &pkglicensing.BillingState{
			Capabilities:         caps,
			Limits:               map[string]int64{},
			MetersEnabled:        []string{},
			PlanVersion:          planVersion,
			SubscriptionState:    subState,
			StripeCustomerID:     customerID,
			StripeSubscriptionID: strings.TrimSpace(sub.ID),
			StripePriceID:        priceID,
		}
		if err := p.writeBillingState(tenantDataDir, state); err != nil {
			return fmt.Errorf("write billing state for tenant %s: %w", tenant.ID, err)
		}

		tenant.PlanVersion = planVersion
		switch subState {
		case pkglicensing.SubStateSuspended:
			tenant.State = registry.TenantStateSuspended
		case pkglicensing.SubStateCanceled, pkglicensing.SubStateExpired:
			tenant.State = registry.TenantStateCanceled
		default:
			tenant.State = registry.TenantStateActive
		}
		if err := p.registry.Update(tenant); err != nil {
			return fmt.Errorf("update tenant record: %w", err)
		}
	}

	log.Info().
		Str("account_id", sa.AccountID).
		Str("customer_id", customerID).
		Str("subscription_state", string(subState)).
		Int("tenants", len(tenants)).
		Msg("MSP subscription updated")

	return nil
}

// HandleMSPSubscriptionDeleted revokes capabilities for all tenants under an MSP account.
func (p *Provisioner) HandleMSPSubscriptionDeleted(ctx context.Context, sub Subscription) error {
	customerID := strings.TrimSpace(sub.Customer)
	if customerID == "" {
		return fmt.Errorf("subscription missing customer")
	}

	sa, err := p.registry.GetStripeAccountByCustomerID(customerID)
	if err != nil {
		return fmt.Errorf("lookup stripe account by customer: %w", err)
	}
	if sa == nil {
		log.Warn().Str("customer_id", customerID).Msg("msp subscription.deleted: stripe account mapping not found")
		return nil
	}

	account, err := p.registry.GetAccount(sa.AccountID)
	if err != nil {
		return fmt.Errorf("lookup account: %w", err)
	}
	if account == nil {
		log.Warn().Str("account_id", sa.AccountID).Msg("msp subscription.deleted: account not found")
		return nil
	}
	if account.Kind != registry.AccountKindMSP {
		if err := p.HandleSubscriptionDeleted(ctx, sub); err != nil {
			return fmt.Errorf("handle non-msp subscription deletion: %w", err)
		}
		return nil
	}

	// Persist account-level Stripe state.
	sa.StripeSubscriptionID = strings.TrimSpace(sub.ID)
	sa.SubscriptionState = "canceled"
	sa.GraceStartedAt = nil
	if err := p.registry.UpdateStripeAccount(sa); err != nil {
		return fmt.Errorf("update stripe account: %w", err)
	}

	tenants, err := p.registry.ListByAccountID(sa.AccountID)
	if err != nil {
		return fmt.Errorf("list tenants by account: %w", err)
	}

	for _, tenant := range tenants {
		if tenant == nil {
			continue
		}
		tenantDataDir := p.tenantDataDir(tenant.ID)
		state := &pkglicensing.BillingState{
			Capabilities:         []string{},
			Limits:               map[string]int64{},
			MetersEnabled:        []string{},
			PlanVersion:          tenant.PlanVersion,
			SubscriptionState:    pkglicensing.SubStateCanceled,
			StripeCustomerID:     customerID,
			StripeSubscriptionID: strings.TrimSpace(sub.ID),
		}
		if err := p.writeBillingState(tenantDataDir, state); err != nil {
			return fmt.Errorf("write canceled billing state for tenant %s: %w", tenant.ID, err)
		}

		tenant.State = registry.TenantStateCanceled
		if err := p.registry.Update(tenant); err != nil {
			return fmt.Errorf("update tenant record: %w", err)
		}
	}

	log.Info().
		Str("account_id", sa.AccountID).
		Str("customer_id", customerID).
		Int("tenants", len(tenants)).
		Msg("MSP subscription deleted, capabilities revoked")

	return nil
}
