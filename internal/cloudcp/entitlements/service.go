package entitlements

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/rcourtman/pulse-go-rewrite/internal/cloudcp/registry"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rs/zerolog/log"
)

const defaultCloudPlanVersion = "cloud_v1"

var (
	ErrHostedEntitlementNotFound       = errors.New("hosted entitlement not found")
	ErrHostedEntitlementTargetMismatch = errors.New("hosted entitlement target mismatch")
	ErrHostedEntitlementInactive       = errors.New("hosted entitlement inactive")
)

type Service struct {
	registry                  *registry.TenantRegistry
	baseURL                   string
	trialActivationPrivateKey string
	now                       func() time.Time
}

type RefreshResult struct {
	EntitlementJWT string
}

func NewService(reg *registry.TenantRegistry, baseURL, trialActivationPrivateKey string) *Service {
	return &Service{
		registry:                  reg,
		baseURL:                   strings.TrimSpace(baseURL),
		trialActivationPrivateKey: strings.TrimSpace(trialActivationPrivateKey),
		now:                       func() time.Time { return time.Now().UTC() },
	}
}

func (s *Service) SetNow(now func() time.Time) {
	if s == nil || now == nil {
		return
	}
	s.now = func() time.Time { return now().UTC() }
}

func (s *Service) IssueTenantBillingState(tenant *registry.Tenant, requestedSubState pkglicensing.SubscriptionState, requestedPlanVersion, stripeCustomerID, stripeSubscriptionID, stripePriceID string) (*pkglicensing.BillingState, error) {
	if s == nil || s.registry == nil {
		return nil, fmt.Errorf("hosted entitlement service is unavailable")
	}
	if tenant == nil {
		return nil, fmt.Errorf("tenant is nil")
	}

	ctx, err := s.resolveTenantLeaseContext(tenant, requestedSubState, requestedPlanVersion)
	if err != nil {
		return nil, err
	}
	if !pkglicensing.ShouldGrantPaidCapabilities(ctx.subscriptionState) {
		return &pkglicensing.BillingState{
			Capabilities:         []string{},
			Limits:               map[string]int64{},
			MetersEnabled:        []string{},
			PlanVersion:          strings.TrimSpace(ctx.planVersion),
			SubscriptionState:    ctx.subscriptionState,
			StripeCustomerID:     strings.TrimSpace(stripeCustomerID),
			StripeSubscriptionID: strings.TrimSpace(stripeSubscriptionID),
			StripePriceID:        strings.TrimSpace(stripePriceID),
		}, nil
	}

	refreshToken, err := s.ensureRefreshToken(tenant.ID)
	if err != nil {
		return nil, fmt.Errorf("issue hosted entitlement refresh token: %w", err)
	}
	entitlementJWT, err := s.signTenantLease(ctx, tenantInstanceHost(s.baseURL, tenant.ID), s.now().UTC())
	if err != nil {
		return nil, err
	}

	return &pkglicensing.BillingState{
		Capabilities:            []string{},
		Limits:                  map[string]int64{},
		MetersEnabled:           []string{},
		EntitlementJWT:          entitlementJWT,
		EntitlementRefreshToken: refreshToken,
		StripeCustomerID:        strings.TrimSpace(stripeCustomerID),
		StripeSubscriptionID:    strings.TrimSpace(stripeSubscriptionID),
		StripePriceID:           strings.TrimSpace(stripePriceID),
	}, nil
}

func (s *Service) RefreshPaidEntitlement(refreshToken, instanceHost string) (*RefreshResult, error) {
	if s == nil || s.registry == nil {
		return nil, fmt.Errorf("hosted entitlement service is unavailable")
	}
	refreshToken = strings.TrimSpace(refreshToken)
	instanceHost = strings.ToLower(strings.TrimSpace(instanceHost))
	if refreshToken == "" {
		return nil, ErrHostedEntitlementNotFound
	}
	if instanceHost == "" {
		return nil, ErrHostedEntitlementTargetMismatch
	}

	entitlement, err := s.registry.GetHostedEntitlementByRefreshToken(refreshToken)
	if err != nil {
		return nil, err
	}
	if entitlement == nil {
		return nil, ErrHostedEntitlementNotFound
	}
	if entitlement.RevokedAt != nil {
		return nil, ErrHostedEntitlementInactive
	}

	tenant, err := s.registry.Get(entitlement.TenantID)
	if err != nil {
		return nil, err
	}
	if tenant == nil {
		return nil, ErrHostedEntitlementNotFound
	}

	expectedInstanceHost := tenantInstanceHost(s.baseURL, tenant.ID)
	if expectedInstanceHost == "" || instanceHost != expectedInstanceHost {
		return nil, ErrHostedEntitlementTargetMismatch
	}

	ctx, err := s.resolveTenantLeaseContext(tenant, "", "")
	if err != nil {
		return nil, err
	}
	if !pkglicensing.ShouldGrantPaidCapabilities(ctx.subscriptionState) {
		return nil, ErrHostedEntitlementInactive
	}

	now := s.now().UTC()
	entitlementJWT, err := s.signTenantLease(ctx, expectedInstanceHost, now)
	if err != nil {
		return nil, err
	}
	if err := s.registry.MarkHostedEntitlementRefreshed(tenant.ID, now); err != nil {
		return nil, err
	}
	return &RefreshResult{EntitlementJWT: entitlementJWT}, nil
}

func (s *Service) RevokeTenantEntitlement(tenantID string, revokedAt time.Time) error {
	if s == nil || s.registry == nil {
		return fmt.Errorf("hosted entitlement service is unavailable")
	}
	return s.registry.RevokeHostedEntitlement(tenantID, revokedAt)
}

type tenantLeaseContext struct {
	tenant            *registry.Tenant
	stripeAccount     *registry.StripeAccount
	subscriptionState pkglicensing.SubscriptionState
	planVersion       string
}

func (s *Service) resolveTenantLeaseContext(tenant *registry.Tenant, requestedSubState pkglicensing.SubscriptionState, requestedPlanVersion string) (*tenantLeaseContext, error) {
	if tenant == nil {
		return nil, fmt.Errorf("tenant is nil")
	}

	ctx := &tenantLeaseContext{
		tenant:            tenant,
		subscriptionState: requestedSubState,
		planVersion:       strings.TrimSpace(requestedPlanVersion),
	}
	if ctx.planVersion == "" {
		ctx.planVersion = strings.TrimSpace(tenant.PlanVersion)
	}

	if strings.TrimSpace(tenant.AccountID) != "" {
		stripeAccount, err := s.registry.GetStripeAccount(tenant.AccountID)
		if err != nil {
			return nil, fmt.Errorf("load stripe account for tenant %s: %w", tenant.ID, err)
		}
		ctx.stripeAccount = stripeAccount
		if ctx.planVersion == "" && stripeAccount != nil && strings.TrimSpace(stripeAccount.PlanVersion) != "" {
			ctx.planVersion = strings.TrimSpace(stripeAccount.PlanVersion)
		}
		if ctx.subscriptionState == "" && stripeAccount != nil {
			ctx.subscriptionState = pkglicensing.MapStripeSubscriptionStatusToState(stripeAccount.SubscriptionState)
			if ctx.subscriptionState == pkglicensing.SubStateTrial && stripeAccount.TrialEndsAt == nil {
				ctx.subscriptionState = pkglicensing.SubStateActive
			}
		}
		if ctx.planVersion == "" {
			account, err := s.registry.GetAccount(tenant.AccountID)
			if err != nil {
				return nil, fmt.Errorf("load account for tenant %s: %w", tenant.ID, err)
			}
			if account != nil && account.Kind == registry.AccountKindMSP {
				ctx.planVersion = "msp_hosted_v1"
			}
		}
	}

	if ctx.planVersion == "" {
		ctx.planVersion = defaultCloudPlanVersion
	}
	if ctx.subscriptionState == "" {
		ctx.subscriptionState = tenantSubscriptionState(tenant)
	}
	return ctx, nil
}

func (s *Service) ensureRefreshToken(tenantID string) (string, error) {
	rawToken, err := randomRefreshToken()
	if err != nil {
		return "", err
	}
	storedToken, _, err := s.registry.StoreOrIssueHostedEntitlement(tenantID, rawToken, s.now().UTC())
	if err != nil {
		return "", err
	}
	return storedToken, nil
}

func (s *Service) signTenantLease(ctx *tenantLeaseContext, instanceHost string, now time.Time) (string, error) {
	privateKey, err := s.entitlementPrivateKey()
	if err != nil {
		return "", err
	}
	claims := buildPaidEntitlementLeaseClaims(ctx, instanceHost, now)
	signed, err := pkglicensing.SignEntitlementLeaseToken(privateKey, claims)
	if err != nil {
		return "", fmt.Errorf("sign hosted entitlement lease: %w", err)
	}
	return signed, nil
}

func (s *Service) entitlementPrivateKey() (ed25519.PrivateKey, error) {
	privateKey, err := pkglicensing.DecodeEd25519PrivateKey(strings.TrimSpace(s.trialActivationPrivateKey))
	if err != nil {
		return nil, fmt.Errorf("decode hosted entitlement signing key: %w", err)
	}
	return privateKey, nil
}

func randomRefreshToken() (string, error) {
	raw := make([]byte, 24)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("generate entitlement refresh token: %w", err)
	}
	return "etr_paid_" + hex.EncodeToString(raw), nil
}

func tenantSubscriptionState(tenant *registry.Tenant) pkglicensing.SubscriptionState {
	if tenant == nil {
		return pkglicensing.SubStateExpired
	}
	switch tenant.State {
	case registry.TenantStateSuspended:
		return pkglicensing.SubStateSuspended
	case registry.TenantStateCanceled, registry.TenantStateDeleted, registry.TenantStateDeleting:
		return pkglicensing.SubStateCanceled
	default:
		return pkglicensing.SubStateActive
	}
}

func buildPaidEntitlementLeaseClaims(ctx *tenantLeaseContext, instanceHost string, now time.Time) pkglicensing.EntitlementLeaseClaims {
	limits, known := pkglicensing.LimitsForCloudPlan(ctx.planVersion)
	if !known && ctx.tenant != nil {
		log.Warn().
			Str("tenant_id", ctx.tenant.ID).
			Str("plan_version", ctx.planVersion).
			Int64("default_max_agents", limits["max_agents"]).
			Msg("Unknown plan version during hosted entitlement lease build; applying safe default agent limit")
	}

	var capabilities []string
	if pkglicensing.ShouldGrantPaidCapabilities(ctx.subscriptionState) {
		capabilities = pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil)
	}
	claims := pkglicensing.EntitlementLeaseClaims{
		OrgID:             "default",
		Email:             strings.TrimSpace(ctx.tenant.Email),
		InstanceHost:      instanceHost,
		PlanVersion:       strings.TrimSpace(ctx.planVersion),
		SubscriptionState: ctx.subscriptionState,
		Capabilities:      capabilities,
		Limits:            limits,
		MetersEnabled:     []string{},
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now.UTC()),
			ExpiresAt: jwt.NewNumericDate(now.UTC().Add(24 * time.Hour)),
		},
	}
	if ctx.stripeAccount != nil && ctx.subscriptionState == pkglicensing.SubStateTrial && ctx.stripeAccount.TrialEndsAt != nil && *ctx.stripeAccount.TrialEndsAt > 0 {
		trialEndsAt := *ctx.stripeAccount.TrialEndsAt
		claims.TrialEndsAt = &trialEndsAt
		claims.ExpiresAt = jwt.NewNumericDate(time.Unix(trialEndsAt, 0).UTC())
	}
	return claims
}

func tenantInstanceHost(baseURL, tenantID string) string {
	tenantID = strings.TrimSpace(tenantID)
	if tenantID == "" {
		return ""
	}
	baseDomain := strings.TrimSpace(baseDomainFromURL(baseURL))
	if baseDomain == "" {
		return ""
	}
	return strings.ToLower(fmt.Sprintf("%s.%s", tenantID, baseDomain))
}

func baseDomainFromURL(baseURL string) string {
	baseURL = strings.TrimSpace(baseURL)
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(baseURL, prefix) {
			baseURL = strings.TrimPrefix(baseURL, prefix)
			break
		}
	}
	for i := 0; i < len(baseURL); i++ {
		if baseURL[i] == ':' || baseURL[i] == '/' {
			return baseURL[:i]
		}
	}
	return baseURL
}
