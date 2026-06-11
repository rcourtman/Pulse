package api

import (
	"crypto/ed25519"
	"net/http"
	"time"

	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
)

// Sentinel errors for user-friendly activation error mapping.
var (
	errMalformedLicenseSentinel = pkglicensing.ErrMalformedLicense
	errInvalidLicenseSentinel   = pkglicensing.ErrInvalidLicense
	errSignatureInvalidSentinel = pkglicensing.ErrSignatureInvalid
	errExpiredLicenseSentinel   = pkglicensing.ErrExpiredLicense
	errNoPublicKeySentinel      = pkglicensing.ErrNoPublicKey
)

type licenseServerErrorModel = pkglicensing.LicenseServerError

type licenseService = pkglicensing.Service
type licenseModel = pkglicensing.License
type licenseStatus = pkglicensing.LicenseStatus
type licensePersistence = pkglicensing.Persistence
type licenseEvaluator = pkglicensing.Evaluator
type licenseFeatureServiceResolver = pkglicensing.FeatureServiceResolver
type licenseFeatureChecker = pkglicensing.FeatureChecker
type licenseFeaturesResponse = pkglicensing.LicenseFeaturesResponse
type activateLicenseRequestModel = pkglicensing.ActivateLicenseRequest
type activateLicenseResponseModel = pkglicensing.ActivateLicenseResponse
type activationStateModel = pkglicensing.ActivationState
type billingStoreModel = pkglicensing.BillingStore
type billingState = pkglicensing.BillingState
type subscriptionState = pkglicensing.SubscriptionState
type entitlementPayloadModel = pkglicensing.EntitlementPayload
type commercialPosturePayloadModel = pkglicensing.CommercialPosturePayload
type runtimeCapabilitiesPayloadModel = pkglicensing.RuntimeCapabilitiesPayload
type runtimeIdentityModel = pkglicensing.RuntimeIdentity
type limitStatusModel = pkglicensing.LimitStatus
type upgradeReasonModel = pkglicensing.UpgradeReason
type entitlementUsageSnapshotModel = pkglicensing.EntitlementUsageSnapshot
type legacyConnectionCountsModel = pkglicensing.LegacyConnectionCounts
type commercialMigrationStatusModel = pkglicensing.CommercialMigrationStatus
type purchaseReturnClaimsModel = pkglicensing.PurchaseReturnClaims
type entitlementLeaseClaimsModel = pkglicensing.EntitlementLeaseClaims
type licenseTier = pkglicensing.Tier
type checkoutPortalHandoffRequestModel = pkglicensing.CheckoutPortalHandoffRequest
type checkoutPortalHandoffResponseModel = pkglicensing.CheckoutPortalHandoffResponse

const (
	featureMultiTenantKey          = pkglicensing.FeatureMultiTenant
	featureAgentProfilesValue      = pkglicensing.FeatureAgentProfiles
	featureAIPatrolValue           = pkglicensing.FeatureAIPatrol
	featureAIAutoFixValue          = pkglicensing.FeatureAIAutoFix
	featureAuditLoggingValue       = pkglicensing.FeatureAuditLogging
	featureRBACValue               = pkglicensing.FeatureRBAC
	featureAdvancedReportingValue  = pkglicensing.FeatureAdvancedReporting
	featureWhiteLabelValue         = pkglicensing.FeatureWhiteLabel
	featureLongTermMetricsValue    = pkglicensing.FeatureLongTermMetrics
	featureDemoFixturesValue       = pkglicensing.FeatureDemoFixtures
	maxUsersLicenseGateKey         = pkglicensing.MaxUsersLicenseGateKey
	subscriptionStateActiveValue   = pkglicensing.SubStateActive
	subscriptionStateExpiredValue  = pkglicensing.SubStateExpired
	subscriptionStateGraceValue    = pkglicensing.SubStateGrace
	subscriptionStateCanceledValue = pkglicensing.SubStateCanceled
	subscriptionStateTrialValue    = pkglicensing.SubStateTrial
	activationKeyPrefixValue       = pkglicensing.ActivationKeyPrefix
)

func newLicenseService() *licenseService {
	return pkglicensing.NewService()
}

func hasMultiTenantLicense(service *licenseService) bool {
	return service != nil && service.HasFeature(featureMultiTenantKey)
}

func upgradeURLForFeatureFromLicensing(feature string) string {
	return pkglicensing.UpgradeURLForFeature(feature)
}

func proTrialSignupURLFromLicensing(override string) string {
	return pkglicensing.ResolveProTrialSignupURL(override)
}

func pulseAccountPortalURLFromLicensing(override string) string {
	return pkglicensing.ResolvePulseAccountPortalURL(override)
}

func defaultBillingStateFromLicensing() *billingState {
	return pkglicensing.DefaultBillingState()
}

func normalizeBillingStateFromLicensing(state *billingState) *billingState {
	return pkglicensing.NormalizeBillingState(state)
}

func cloneCommercialMigrationStatusFromLicensing(state *commercialMigrationStatusModel) *commercialMigrationStatusModel {
	return pkglicensing.CloneCommercialMigrationStatus(state)
}

func classifyLegacyExchangeErrorFromLicensing(err error) *commercialMigrationStatusModel {
	return pkglicensing.ClassifyLegacyExchangeError(err)
}

func classifyPersistedLicenseLoadErrorFromLicensing(err error) *commercialMigrationStatusModel {
	return pkglicensing.ClassifyPersistedLicenseLoadError(err)
}

const commercialMigrationStatePendingValue = pkglicensing.CommercialMigrationStatePending

func isValidBillingSubscriptionStateFromLicensing(state subscriptionState) bool {
	return pkglicensing.IsValidBillingSubscriptionState(state)
}

func cloudCapabilitiesFromLicensing() []string {
	return pkglicensing.DeriveCapabilitiesFromTier(pkglicensing.TierCloud, nil)
}

func defaultTrialDurationFromLicensing() time.Duration {
	return pkglicensing.DefaultTrialDuration
}

func buildTrialBillingStateWithPlanFromLicensing(now time.Time, capabilities []string, planVersion string, duration time.Duration) *billingState {
	return pkglicensing.BuildTrialBillingStateWithPlan(now, capabilities, planVersion, duration)
}

func newLicensePersistenceFromLicensing(configDir string) (*licensePersistence, error) {
	return pkglicensing.NewPersistence(configDir)
}

func newLicenseServerClientFromLicensing(baseURL string) *pkglicensing.LicenseServerClient {
	return pkglicensing.NewLicenseServerClient(baseURL)
}

func isLicenseValidationDevModeFromLicensing() bool {
	return pkglicensing.IsLicenseValidationDevMode()
}

func newLicenseEvaluatorForBillingStoreFromLicensing(store billingStoreModel, orgID string, cacheTTL time.Duration, expectedInstanceHost string) *licenseEvaluator {
	return pkglicensing.NewEvaluator(
		pkglicensing.NewDatabaseSource(store, orgID, cacheTTL).WithExpectedInstanceHost(expectedInstanceHost),
	)
}

func maxUsersLimitFromLicensing(lic *licenseModel) int {
	return pkglicensing.MaxUsersLimitFromLicense(lic)
}

func exceedsUserLimitFromLicensing(current, additions, limit int) bool {
	return pkglicensing.ExceedsUserLimit(current, additions, limit)
}

func userLimitExceededMessageFromLicensing(current, limit int) string {
	return pkglicensing.UserLimitExceededMessage(current, limit)
}

func mapStripeSubscriptionStatusToStateFromLicensing(status string) subscriptionState {
	return pkglicensing.MapStripeSubscriptionStatusToState(status)
}

func shouldGrantPaidCapabilitiesFromLicensing(state subscriptionState) bool {
	return pkglicensing.ShouldGrantPaidCapabilities(state)
}

func deriveStripePlanVersionFromLicensing(metadata map[string]string, priceID string) string {
	return pkglicensing.DeriveStripePlanVersion(metadata, priceID)
}

func buildEntitlementPayloadFromLicensing(status *licenseStatus, subscriptionState string) entitlementPayloadModel {
	return pkglicensing.BuildEntitlementPayload(status, subscriptionState)
}

func buildRuntimeCapabilitiesPayloadFromLicensing(
	status *licenseStatus,
	subscriptionState string,
) runtimeCapabilitiesPayloadModel {
	return pkglicensing.BuildRuntimeCapabilitiesPayload(status, subscriptionState)
}

func buildCommercialPosturePayloadFromLicensing(
	status *licenseStatus,
	subscriptionState string,
) commercialPosturePayloadModel {
	return pkglicensing.BuildCommercialPosturePayload(status, subscriptionState)
}

func buildFeatureMapFromLicensing(service *licenseService) map[string]bool {
	return pkglicensing.BuildFeatureMap(service, nil)
}

func buildEntitlementPayloadWithUsageFromLicensing(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshotModel,
	trialEndsAtUnix *int64,
) entitlementPayloadModel {
	return pkglicensing.BuildEntitlementPayloadWithUsage(status, subscriptionState, usage, trialEndsAtUnix)
}

func buildRuntimeCapabilitiesPayloadWithUsageFromLicensing(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshotModel,
) runtimeCapabilitiesPayloadModel {
	return pkglicensing.BuildRuntimeCapabilitiesPayloadWithUsage(status, subscriptionState, usage)
}

func communityRuntimeIdentityFromLicensing() runtimeIdentityModel {
	return pkglicensing.CommunityRuntimeIdentity()
}

func proRuntimeIdentityFromLicensing() runtimeIdentityModel {
	return pkglicensing.ProRuntimeIdentity()
}

func normalizeRuntimeIdentityFromLicensing(identity runtimeIdentityModel) runtimeIdentityModel {
	return pkglicensing.NormalizeRuntimeIdentity(identity)
}

func cloneRuntimeIdentityFromLicensing(identity runtimeIdentityModel) *runtimeIdentityModel {
	return pkglicensing.CloneRuntimeIdentity(identity)
}

func filterCapabilitiesForRuntimeIdentityFromLicensing(
	capabilities []string,
	identity runtimeIdentityModel,
) ([]string, []pkglicensing.RuntimeCapabilityBlock) {
	return pkglicensing.FilterCapabilitiesForRuntimeIdentity(capabilities, identity)
}

func buildCommercialPosturePayloadWithUsageFromLicensing(
	status *licenseStatus,
	subscriptionState string,
	usage entitlementUsageSnapshotModel,
	trialEndsAtUnix *int64,
) commercialPosturePayloadModel {
	return pkglicensing.BuildCommercialPosturePayloadWithUsage(
		status,
		subscriptionState,
		usage,
		trialEndsAtUnix,
	)
}

func commercialPosturePayloadFromEntitlementPayloadFromLicensing(
	payload entitlementPayloadModel,
) commercialPosturePayloadModel {
	return pkglicensing.CommercialPosturePayloadFromEntitlementPayload(payload)
}

func limitStateFromLicensing(current, limit int64) string {
	return pkglicensing.LimitState(current, limit)
}

func trialActivationPublicKeyFromLicensing() (ed25519.PublicKey, error) {
	return pkglicensing.HostedEntitlementPublicKey()
}

func signPurchaseReturnTokenFromLicensing(signingKey []byte, claims purchaseReturnClaimsModel) (string, error) {
	return pkglicensing.SignPurchaseReturnToken(signingKey, claims)
}

func verifyPurchaseReturnTokenFromLicensing(token string, signingKey []byte, expectedInstanceHost string, now time.Time) (*purchaseReturnClaimsModel, error) {
	return pkglicensing.VerifyPurchaseReturnToken(token, signingKey, expectedInstanceHost, now)
}

func verifyEntitlementLeaseTokenFromLicensing(token string, key ed25519.PublicKey, expectedInstanceHost string, now time.Time) (*entitlementLeaseClaimsModel, error) {
	return pkglicensing.VerifyEntitlementLeaseToken(token, key, expectedInstanceHost, now)
}

func parseEntitlementLeaseTokenFromLicensing(token string, key ed25519.PublicKey, expectedInstanceHost string) (*entitlementLeaseClaimsModel, error) {
	return pkglicensing.ParseEntitlementLeaseToken(token, key, expectedInstanceHost)
}

func writePaymentRequiredFromLicensing(w http.ResponseWriter, payload map[string]interface{}) {
	pkglicensing.WritePaymentRequired(w, payload)
}

func writeLicenseRequiredFromLicensing(w http.ResponseWriter, feature, message string) {
	pkglicensing.WriteLicenseRequired(w, feature, message, pkglicensing.UpgradeURLForFeature)
}

// licenseTierFreeValue is the canonical free-tier constant for use outside the bridge.
const licenseTierFreeValue = pkglicensing.TierFree

// overflowDaysRemainingFromLicensing returns the number of days remaining in the overflow window.
func overflowDaysRemainingFromLicensing(tier licenseTier, overflowGrantedAt *int64, now time.Time) int {
	return pkglicensing.OverflowDaysRemaining(tier, overflowGrantedAt, now)
}

// freeHistoryDaysDefault is the fallback history days when no license service is available.
var freeHistoryDaysDefault = pkglicensing.TierHistoryDays[pkglicensing.TierFree]

func tierHistoryDaysFromLicensing(tier pkglicensing.Tier) int {
	days := pkglicensing.TierHistoryDays[tier]
	if days == 0 {
		days = freeHistoryDaysDefault
	}
	return days
}
