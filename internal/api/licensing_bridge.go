package api

import (
	"crypto/ed25519"
	"net/http"
	"time"

	"github.com/rcourtman/pulse-go-rewrite/internal/models"
	agentsdocker "github.com/rcourtman/pulse-go-rewrite/pkg/agents/docker"
	agentshost "github.com/rcourtman/pulse-go-rewrite/pkg/agents/host"
	agentsk8s "github.com/rcourtman/pulse-go-rewrite/pkg/agents/kubernetes"
	pkglicensing "github.com/rcourtman/pulse-go-rewrite/pkg/licensing"
	"github.com/rcourtman/pulse-go-rewrite/pkg/licensing/metering"
)

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
type trialStartDecisionModel = pkglicensing.TrialStartDecision
type trialStartDenialReasonModel = pkglicensing.TrialStartDenialReason
type billingStoreModel = pkglicensing.BillingStore
type billingState = pkglicensing.BillingState
type subscriptionState = pkglicensing.SubscriptionState
type entitlementPayloadModel = pkglicensing.EntitlementPayload
type limitStatusModel = pkglicensing.LimitStatus
type upgradeReasonModel = pkglicensing.UpgradeReason
type entitlementUsageSnapshotModel = pkglicensing.EntitlementUsageSnapshot
type conversionRecorder = pkglicensing.Recorder
type conversionPipelineHealth = pkglicensing.PipelineHealth
type conversionCollectionConfig = pkglicensing.CollectionConfig
type conversionStore = pkglicensing.ConversionStore
type conversionEvent = pkglicensing.ConversionEvent
type conversionHealthStatus = pkglicensing.HealthStatus
type conversionCollectionConfigSnapshot = pkglicensing.CollectionConfigSnapshot
type trialActivationClaimsModel = pkglicensing.TrialActivationClaims

const (
	featureMultiTenantKey          = pkglicensing.FeatureMultiTenant
	featureAgentProfilesValue      = pkglicensing.FeatureAgentProfiles
	featureAIPatrolValue           = pkglicensing.FeatureAIPatrol
	featureAIAutoFixValue          = pkglicensing.FeatureAIAutoFix
	featureAuditLoggingValue       = pkglicensing.FeatureAuditLogging
	featureRBACValue               = pkglicensing.FeatureRBAC
	featureAdvancedReportingValue  = pkglicensing.FeatureAdvancedReporting
	featureLongTermMetricsValue    = pkglicensing.FeatureLongTermMetrics
	maxNodesLicenseGateKey         = pkglicensing.MaxNodesLicenseGateKey
	maxUsersLicenseGateKey         = pkglicensing.MaxUsersLicenseGateKey
	subscriptionStateActiveValue   = pkglicensing.SubStateActive
	subscriptionStateGraceValue    = pkglicensing.SubStateGrace
	subscriptionStateCanceledValue = pkglicensing.SubStateCanceled
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

func defaultBillingStateFromLicensing() *billingState {
	return pkglicensing.DefaultBillingState()
}

func normalizeBillingStateFromLicensing(state *billingState) *billingState {
	return pkglicensing.NormalizeBillingState(state)
}

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

func buildProTrialBillingStateFromLicensing(now time.Time) *billingState {
	return pkglicensing.BuildTrialBillingState(now, pkglicensing.TierFeatures[pkglicensing.TierPro])
}

func evaluateTrialStartEligibilityFromLicensing(hasActiveLicense bool, existing *billingState) trialStartDecisionModel {
	return pkglicensing.EvaluateTrialStartEligibility(hasActiveLicense, existing)
}

func trialStartErrorFromLicensing(reason trialStartDenialReasonModel) (code, message string, includeOrgID bool) {
	return pkglicensing.TrialStartError(reason)
}

func newLicensePersistenceFromLicensing(configDir string) (*licensePersistence, error) {
	return pkglicensing.NewPersistence(configDir)
}

func newLicenseEvaluatorForBillingStoreFromLicensing(store billingStoreModel, orgID string, cacheTTL time.Duration) *licenseEvaluator {
	return pkglicensing.NewEvaluator(pkglicensing.NewDatabaseSource(store, orgID, cacheTTL))
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

func configuredNodeCountFromLicensing(pveInstances, pbsInstances, pmgInstances int) int {
	return pkglicensing.ConfiguredNodeCount(pveInstances, pbsInstances, pmgInstances)
}

func registeredNodeSlotCountFromLicensing(configuredCount int, state models.StateSnapshot) int {
	return pkglicensing.RegisteredNodeSlotCount(configuredCount, state)
}

func exceedsNodeLimitFromLicensing(current, additions, limit int) bool {
	return pkglicensing.ExceedsNodeLimit(current, additions, limit)
}

func nodeLimitExceededMessageFromLicensing(current, limit int) string {
	return pkglicensing.NodeLimitExceededMessage(current, limit)
}

func hostReportTargetsExistingHostFromLicensing(snapshot models.StateSnapshot, report agentshost.Report, tokenID string) bool {
	return pkglicensing.HostReportTargetsExistingHost(snapshot, report, tokenID)
}

func dockerReportTargetsExistingHostFromLicensing(snapshot models.StateSnapshot, report agentsdocker.Report, tokenID string) bool {
	return pkglicensing.DockerReportTargetsExistingHost(snapshot, report, tokenID)
}

func kubernetesReportTargetsExistingClusterFromLicensing(snapshot models.StateSnapshot, report agentsk8s.Report, tokenID string) bool {
	return pkglicensing.KubernetesReportTargetsExistingCluster(snapshot, report, tokenID)
}

func kubernetesReportIdentifierFromLicensing(report agentsk8s.Report) string {
	return pkglicensing.KubernetesReportIdentifier(report)
}

func collectNonEmptyStringsFromLicensing(values ...string) []string {
	return pkglicensing.CollectNonEmptyStrings(values...)
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

func limitStateFromLicensing(current, limit int64) string {
	return pkglicensing.LimitState(current, limit)
}

func newCollectionConfigFromLicensing() *conversionCollectionConfig {
	return pkglicensing.NewCollectionConfig()
}

func newConversionRecorderFromLicensing(store *conversionStore) *conversionRecorder {
	return pkglicensing.NewRecorderFromWindowedAggregator(metering.NewWindowedAggregator(), store)
}

func newConversionPipelineHealthFromLicensing() *conversionPipelineHealth {
	return pkglicensing.NewPipelineHealth()
}

func conversionValidationReasonFromLicensing(err error) string {
	return pkglicensing.ConversionValidationReason(err)
}

func parseOptionalTimeParamFromLicensing(raw string, defaultValue time.Time) (time.Time, error) {
	return pkglicensing.ParseOptionalTimeParam(raw, defaultValue)
}

func trialActivationPublicKeyFromLicensing() (ed25519.PublicKey, error) {
	return pkglicensing.TrialActivationPublicKey()
}

func verifyTrialActivationTokenFromLicensing(token string, key ed25519.PublicKey, expectedInstanceHost string, now time.Time) (*trialActivationClaimsModel, error) {
	return pkglicensing.VerifyTrialActivationToken(token, key, expectedInstanceHost, now)
}

func writePaymentRequiredFromLicensing(w http.ResponseWriter, payload map[string]interface{}) {
	pkglicensing.WritePaymentRequired(w, payload)
}

func writeLicenseRequiredFromLicensing(w http.ResponseWriter, feature, message string) {
	pkglicensing.WriteLicenseRequired(w, feature, message, pkglicensing.UpgradeURLForFeature)
}

func recordConversionInvalidMetric(reason string) {
	pkglicensing.GetConversionMetrics().RecordInvalid(reason)
}

func recordConversionSkippedMetric(reason string) {
	pkglicensing.GetConversionMetrics().RecordSkipped(reason)
}

func recordConversionEventMetric(eventType, surface string) {
	pkglicensing.GetConversionMetrics().RecordEvent(eventType, surface)
}
