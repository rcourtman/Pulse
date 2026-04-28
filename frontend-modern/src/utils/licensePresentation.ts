import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import type {
  CommercialMigrationStatus,
  LicenseCommercialEntitlements,
  LicenseStatus,
  MonitoredSystemContinuityStatus,
} from '@/api/license';
import { CLOUD_PLAN_LABELS } from '@/utils/cloudPlans';
import {
  formatMonitoredSystemUsageUnavailableMessage,
  getMonitoredSystemLimitUnavailableReason,
  isMonitoredSystemLimitUsageAvailable,
  resolveMonitoredSystemCapacityStatus,
  type MonitoredSystemCapacityStatus,
  type MonitoredSystemLimitUsageStatus,
} from '@/utils/monitoredSystemPresentation';
import { getSelfHostedPlanDefinitionForBillingTier } from '@/utils/selfHostedPlans';
import {
  getSelfHostedFeatureCatalogEntry,
  isDisplayableSelfHostedFeatureKey,
} from '@/utils/selfHostedFeatureCatalog.generated';
import { titleCaseDelimitedLabel } from '@/utils/textPresentation';

const TIER_LABELS: Record<string, string> = {
  free: 'Community',
  relay: 'Relay',
  pro: 'Pro',
  pro_plus: 'Legacy Pro+',
  pro_annual: 'Pro Annual',
  lifetime: 'Lifetime',
  cloud: 'Cloud',
  msp: 'MSP',
  enterprise: 'Enterprise',
};

const SELF_HOSTED_PLAN_LABELS: Record<string, string> = {
  pro: 'Pulse Pro',
  pro_annual: 'Pulse Pro Annual',
  pro_plus: 'Legacy Pulse Pro+',
  lifetime: 'Pulse Pro Lifetime',
};

const LEGACY_PLAN_VERSION_LABELS: Record<string, string> = {
  pro_plus: 'Legacy Pro Plus',
};

const FEATURE_MIN_TIER_LABELS: Record<string, string> = {
  relay: 'Relay',
  mobile_app: 'Relay',
  push_notifications: 'Relay',
  multi_tenant: 'MSP',
};

export interface LicenseSubscriptionStatusPresentation {
  label: string;
  badgeClass: string;
}

export interface LicenseLoadingStateCopy {
  text: string;
}

export interface LicenseInlineNotice {
  tone: string;
  title: string;
  body: string;
}

export interface LicenseActionNotice extends LicenseInlineNotice {
  actionLabel: string;
}

export interface BillingAdminOrganizationBadge {
  label: string;
  badgeClass: string;
}

export interface SelfHostedActivationNoticeCopy {
  title: string;
  body: string;
}

export interface SelfHostedRecoveryPresentation {
  disclosureLabel: string;
  disclosureDescription: string;
  fieldLabel: string;
  fieldPlaceholder: string;
  helpTextBeforeTerms: string;
  helpTextAfterTerms: string;
  termsLabel: string;
  activateIdleLabel: string;
  activatePendingLabel: string;
  clearIdleLabel: string;
  clearPendingLabel: string;
  legacyNotice: SelfHostedActivationNoticeCopy;
}

export interface SelfHostedCurrentPlanPresentation {
  title: string;
  body: string;
  unlockedFeaturesLabel: string;
  unlockedFeatures: string[];
  includedExtrasLabel?: string;
  includedExtras: string[];
  supplementalBadges: string[];
  supplementalSummary?: string;
}

export interface SelfHostedActivationSuccessPresentation extends LicenseInlineNotice {
  highlightsLabel: string;
  highlights: string[];
}

export type SelfHostedActivationSuccessSource = 'manual' | 'purchase';

export interface SelfHostedPlanComparisonCardPresentation {
  title: string;
  body: string;
  highlights: string[];
}

export interface SelfHostedPlanComparisonPresentation {
  cards: SelfHostedPlanComparisonCardPresentation[];
}

const GRANDFATHERED_V5_PLAN_LABELS: Record<string, string> = {
  v5_lifetime_grandfathered: 'V5 Lifetime Grandfathered',
  v5_pro_monthly_grandfathered: 'V5 Pro Monthly (Grandfathered)',
  v5_pro_annual_grandfathered: 'V5 Pro Annual (Grandfathered)',
};

export const isGrandfatheredRecurringV5PlanVersion = (planVersion?: string | null): boolean => {
  const normalized = (planVersion || '').trim().toLowerCase();
  return (
    normalized === 'v5_pro_monthly_grandfathered' || normalized === 'v5_pro_annual_grandfathered'
  );
};

export const isUncappedGrandfatheredPlanVersion = (
  planVersion?: string | null,
  isLifetime?: boolean | null,
): boolean => {
  if (isLifetime) {
    return true;
  }
  return isGrandfatheredRecurringV5PlanVersion(planVersion);
};

const isActiveOrGraceSubscription = (subscriptionState?: string | null): boolean => {
  const normalized = (subscriptionState || '').trim().toLowerCase();
  return normalized === 'active' || normalized === 'grace';
};

export const hasActiveUncappedSelfHostedContinuity = ({
  planVersion,
  isLifetime,
  subscriptionState,
}: {
  planVersion?: string | null;
  isLifetime?: boolean | null;
  subscriptionState?: string | null;
}): boolean => {
  if (isLifetime) {
    return true;
  }
  return (
    isActiveOrGraceSubscription(subscriptionState) &&
    isGrandfatheredRecurringV5PlanVersion(planVersion)
  );
};

export const getDisplayableMonitoredSystemContinuity = ({
  continuity,
  planVersion,
  isLifetime,
  subscriptionState,
}: {
  continuity?: MonitoredSystemContinuityStatus | null;
  planVersion?: string | null;
  isLifetime?: boolean | null;
  subscriptionState?: string | null;
}): MonitoredSystemContinuityStatus | null => {
  if (!continuity) {
    return null;
  }
  if (hasActiveUncappedSelfHostedContinuity({ planVersion, isLifetime, subscriptionState })) {
    return null;
  }
  if (typeof continuity.effective_limit === 'number' && continuity.effective_limit <= 0) {
    return null;
  }
  return continuity;
};

export const getLicenseTierLabel = (tier?: string | null): string => {
  const normalized = (tier || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return TIER_LABELS[normalized] || titleCaseDelimitedLabel(normalized);
};

export const getSelfHostedPlanLabel = (tier?: string | null): string => {
  const normalized = (tier || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  return SELF_HOSTED_PLAN_LABELS[normalized] || getLicenseTierLabel(normalized);
};

export const getLicenseFeatureLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Unknown';
  const entry = getSelfHostedFeatureCatalogEntry(normalized);
  return entry?.comparisonName || titleCaseDelimitedLabel(normalized);
};

export const isDisplayableLicenseFeature = (feature?: string | null): boolean => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return false;
  return isDisplayableSelfHostedFeatureKey(normalized);
};

export const getFeatureMinTierLabel = (feature?: string | null): string => {
  const normalized = (feature || '').trim().toLowerCase();
  if (!normalized) return 'Pro';
  return FEATURE_MIN_TIER_LABELS[normalized] || 'Pro';
};

export const formatLicensePlanVersion = (value?: string | null): string | null => {
  const normalized = (value || '').trim();
  if (!normalized) return null;
  const grandfathered = GRANDFATHERED_V5_PLAN_LABELS[normalized.toLowerCase()];
  if (grandfathered) return grandfathered;
  const legacy = LEGACY_PLAN_VERSION_LABELS[normalized.toLowerCase()];
  if (legacy) return legacy;
  const canonical = CLOUD_PLAN_LABELS[normalized.toLowerCase()];
  if (canonical) return canonical;
  return titleCaseDelimitedLabel(normalized);
};

export const getGrandfatheredPriceContinuityNotice = (
  planVersion?: string | null,
  subscriptionState?: string | null,
): LicenseInlineNotice | null => {
  if (!isGrandfatheredRecurringV5PlanVersion(planVersion)) {
    return null;
  }

  const normalizedState = (subscriptionState || '').trim().toLowerCase();
  if (normalizedState !== 'active' && normalizedState !== 'grace') {
    return null;
  }

  return {
    tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
    title: 'Grandfathered v5 pricing',
    body: 'This migrated v5 Pro subscription keeps its existing recurring price until you cancel. Self-hosted monitoring and child-resource volume remain uncapped under the current v6 policy. If you cancel and return later, current v6 pricing applies for paid features.',
  };
};

export const getMonitoredSystemContinuityNotice = (
  continuity?: MonitoredSystemContinuityStatus | null,
  limit?: MonitoredSystemLimitUsageStatus | null,
  capacity?: MonitoredSystemCapacityStatus | null,
  context?: {
    planVersion?: string | null;
    isLifetime?: boolean | null;
    subscriptionState?: string | null;
  },
): LicenseInlineNotice | null => {
  const displayContinuity = getDisplayableMonitoredSystemContinuity({
    continuity,
    planVersion: context?.planVersion,
    isLifetime: context?.isLifetime,
    subscriptionState: context?.subscriptionState,
  });
  if (!displayContinuity) {
    return null;
  }
  const resolvedCapacity = resolveMonitoredSystemCapacityStatus(capacity, limit);

  if (displayContinuity.capture_pending) {
    if (!resolvedCapacity?.current_available) {
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Migration continuity verification pending',
        body: `Pulse is still verifying the grandfathered monitored-system floor for this migrated v5 installation. ${formatMonitoredSystemUsageUnavailableMessage(
          getMonitoredSystemLimitUnavailableReason(limit, capacity),
        )}`,
      };
    }

    if (resolvedCapacity.mode === 'over_limit_frozen') {
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Migration continuity verification pending',
        body: `Pulse is still verifying the grandfathered monitored-system floor for this migrated v5 installation. The finite policy includes ${displayContinuity.plan_limit}, while this installation is already monitoring ${resolvedCapacity.current}. Existing monitoring continues while additional monitored-system admissions pause until continuity capture finishes.`,
      };
    }

    return {
      tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
      title: 'Migration continuity verification pending',
      body: 'Pulse is still verifying the grandfathered monitored-system floor for this migrated v5 installation. Existing monitoring continues while Pulse finalizes the effective monitored-system limit.',
    };
  }

  if (!isMonitoredSystemLimitUsageAvailable(limit)) {
    return {
      tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
      title: 'Monitored-system usage unavailable',
      body: formatMonitoredSystemUsageUnavailableMessage(
        getMonitoredSystemLimitUnavailableReason(limit, capacity),
      ),
    };
  }

  if (
    typeof displayContinuity.grandfathered_floor === 'number' &&
    displayContinuity.grandfathered_floor > 0 &&
    displayContinuity.effective_limit > displayContinuity.plan_limit
  ) {
    return {
      tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
      title: 'Grandfathered monitored-system floor',
      body: `This migrated v5 installation keeps an effective monitored-system limit of ${displayContinuity.effective_limit}. The current plan includes ${displayContinuity.plan_limit}, and the observed legacy estate was grandfathered at ${displayContinuity.grandfathered_floor}.`,
    };
  }

  return null;
};

const getSelfHostedUnlockedFeatures = ({
  entitlements,
  displayableCapabilities,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
}): string[] => {
  const current = entitlements;
  if (!current) {
    return [];
  }

  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(current.tier);
  if (planDefinition) {
    return [...planDefinition.entitlementHighlights];
  }
  return displayableCapabilities;
};

const getSelfHostedIncludedExtras = ({
  entitlements,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
}): string[] => {
  const current = entitlements;
  if (!current) {
    return [];
  }

  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(current.tier);
  return [...(planDefinition?.includedExtras ?? [])];
};

const getSelfHostedActivationHighlights = ({
  entitlements,
  displayableCapabilities,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
}): string[] => {
  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(entitlements?.tier);
  const prioritized = [
    ...(planDefinition?.entitlementHighlights ?? []),
    ...(planDefinition?.includedExtras ?? []),
  ];
  const unlockedFeatures = getSelfHostedUnlockedFeatures({
    entitlements,
    displayableCapabilities,
  });
  const highlights: string[] = [];

  for (const feature of [...prioritized, ...unlockedFeatures]) {
    if (!feature || highlights.includes(feature)) {
      continue;
    }
    highlights.push(feature);
    if (highlights.length >= 8) {
      break;
    }
  }

  return highlights;
};

const getSelfHostedPlanComparisonHighlights = (
  planDefinition: ReturnType<typeof getSelfHostedPlanDefinitionForBillingTier>,
): string[] => {
  if (!planDefinition) {
    return [];
  }
  const highlights: string[] = [];
  for (const feature of [
    ...planDefinition.entitlementHighlights,
    ...planDefinition.includedExtras,
  ]) {
    if (!feature || highlights.includes(feature)) {
      continue;
    }
    highlights.push(feature);
    if (highlights.length >= 8) {
      break;
    }
  }
  return highlights;
};

const getSelfHostedActivePlanSummary = (
  planLabel: string,
  planDefinition: ReturnType<typeof getSelfHostedPlanDefinitionForBillingTier>,
): string | null => {
  switch (planDefinition?.tier) {
    case 'community':
      return `${planLabel} is active on this instance. It includes self-hosted monitoring, 7-day metric history, Pulse Patrol (BYOK), and update alerts.`;
    case 'relay':
      return `${planLabel} is active on this instance. Remote access, mobile, push, and longer history are unlocked right now.`;
    case 'pro':
      return `${planLabel} is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are unlocked right now.`;
    default:
      return null;
  }
};

export const getSelfHostedPlanComparisonPresentation = ({
  entitlements,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
}): SelfHostedPlanComparisonPresentation => {
  const normalizedTier = (entitlements?.tier || '').trim().toLowerCase();
  const comparisonTiers =
    normalizedTier === 'relay'
      ? ['pro']
      : normalizedTier === 'free' || normalizedTier === 'community' || !normalizedTier
        ? ['relay', 'pro']
        : [];

  return {
    cards: comparisonTiers
      .map((tier) => {
        const definition = getSelfHostedPlanDefinitionForBillingTier(tier);
        if (!definition) {
          return null;
        }
        return {
          title: `What ${getSelfHostedPlanLabel(tier)} adds`,
          body: definition.comparisonSummary,
          highlights: getSelfHostedPlanComparisonHighlights(definition),
        };
      })
      .filter((card): card is SelfHostedPlanComparisonCardPresentation => card !== null),
  };
};

export const getSelfHostedCurrentPlanPresentation = ({
  entitlements,
  displayableCapabilities,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
}): SelfHostedCurrentPlanPresentation => {
  const current = entitlements;
  if (!current) {
    return {
      title: 'Current plan: Unknown',
      body: 'Pulse is still loading the current self-hosted entitlement for this instance.',
      unlockedFeaturesLabel: 'Unlocked on this instance',
      unlockedFeatures: [],
      includedExtras: [],
      supplementalBadges: [],
    };
  }

  const normalizedState = (current.subscription_state || '').trim().toLowerCase();
  const normalizedTier = (current.tier || '').trim().toLowerCase();
  const planLabel = getSelfHostedPlanLabel(current.tier);
  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(current.tier);
  const hasUncappedContinuity = hasActiveUncappedSelfHostedContinuity({
    planVersion: current.plan_version,
    isLifetime: current.is_lifetime,
    subscriptionState: current.subscription_state,
  });
  const unlockedFeatures = getSelfHostedUnlockedFeatures({
    entitlements: current,
    displayableCapabilities,
  });
  const includedExtras = getSelfHostedIncludedExtras({
    entitlements: current,
  });
  const unlockedFeaturesLabel =
    normalizedTier === 'free' ? 'Included on this instance' : 'Primary capabilities';

  const supplementalBadges: string[] = [];
  const supplementalDetails: string[] = [];

  if (
    isActiveOrGraceSubscription(current.subscription_state) &&
    isGrandfatheredRecurringV5PlanVersion(current.plan_version)
  ) {
    supplementalBadges.push('Grandfathered price');
    supplementalDetails.push(
      'This migrated v5 subscription keeps its existing recurring price until cancellation. Self-hosted monitoring and child-resource volume remain uncapped under the current v6 policy.',
    );
  } else if (hasUncappedContinuity && current.is_lifetime) {
    supplementalBadges.push('Grandfathered lifetime');
    supplementalDetails.push(
      'This migrated lifetime install remains valid permanently, with uncapped self-hosted monitoring and child-resource volume.',
    );
  }

  const continuity = getDisplayableMonitoredSystemContinuity({
    continuity: current.monitored_system_continuity,
    planVersion: current.plan_version,
    isLifetime: current.is_lifetime,
    subscriptionState: current.subscription_state,
  });
  if (continuity?.capture_pending) {
    supplementalBadges.push('Continuity pending');
    supplementalDetails.push(
      'Pulse is still verifying the grandfathered monitored-system floor for this migrated v5 installation.',
    );
  } else if (
    continuity &&
    typeof continuity.grandfathered_floor === 'number' &&
    continuity.grandfathered_floor > 0 &&
    continuity.effective_limit > continuity.plan_limit
  ) {
    supplementalBadges.push('Grandfathered floor');
    supplementalDetails.push(
      `This installation keeps an effective monitored-system limit of ${continuity.effective_limit} from the observed legacy estate.`,
    );
  }

  if (normalizedState === 'trial') {
    return {
      title: `Current plan: ${planLabel} Trial`,
      body:
        unlockedFeatures.length > 0
          ? `${planLabel} trial capabilities are active on this instance right now.`
          : `${planLabel} trial entitlement is being confirmed for this instance.`,
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtrasLabel: includedExtras.length > 0 ? 'Included extras' : undefined,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
    };
  }

  if (normalizedTier === 'free') {
    return {
      title: 'Current plan: Community',
      body:
        getSelfHostedActivePlanSummary('Community', planDefinition) ||
        'Community is active on this instance. Self-hosted monitoring, 7-day metric history, Pulse Patrol (BYOK), and update alerts are included here.',
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
    };
  }

  if (normalizedState === 'active' || normalizedState === 'grace') {
    return {
      title: `Current plan: ${planLabel}`,
      body:
        getSelfHostedActivePlanSummary(planLabel, planDefinition) ||
        `${planLabel} is active on this instance. These capabilities are unlocked right now.`,
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtrasLabel: includedExtras.length > 0 ? 'Included extras' : undefined,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
    };
  }

  return {
    title: `Current plan: ${planLabel}`,
    body: 'Review the plan details below to confirm what this key unlocks on this instance.',
    unlockedFeaturesLabel: 'Unlocked on this instance',
    unlockedFeatures,
    includedExtras,
    supplementalBadges,
    supplementalSummary: supplementalDetails.join(' '),
  };
};

export const getSelfHostedActivationSuccessPresentation = ({
  entitlements,
  displayableCapabilities,
  source,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
  source: SelfHostedActivationSuccessSource | null;
}): SelfHostedActivationSuccessPresentation | null => {
  const current = entitlements;
  if (!source || !current) {
    return null;
  }

  const normalizedState = (current.subscription_state || '').trim().toLowerCase();
  const normalizedTier = (current.tier || '').trim().toLowerCase();
  if (normalizedTier === 'free') {
    return null;
  }

  if (normalizedState !== 'active' && normalizedState !== 'grace') {
    return null;
  }

  const planLabel = getSelfHostedPlanLabel(current.tier);
  const highlights = getSelfHostedActivationHighlights({
    entitlements: current,
    displayableCapabilities,
  });

  return {
    tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
    title: `${planLabel} is now active`,
    body:
      source === 'purchase'
        ? `Checkout completed and this instance is now running ${planLabel}.`
        : `The activation key was accepted and this instance is now running ${planLabel}.`,
    highlightsLabel: 'Available now on this instance',
    highlights,
  };
};

export const getCommercialMigrationActionText = (action?: string): string => {
  switch (action) {
    case 'retry_activation':
      return 'Retry activation from this instance.';
    case 'use_v6_activation_key':
      return 'Use the current v6 activation key for this purchase.';
    case 'enter_supported_v5_key':
      return 'Retry with the original v5 Pro/Lifetime key from this instance.';
    default:
      return 'Review the activation state from this instance before trying again.';
  }
};

export const getCommercialMigrationNotice = (
  migration?: CommercialMigrationStatus,
): LicenseInlineNotice | null => {
  if (!migration?.state) return null;

  const actionText = getCommercialMigrationActionText(migration.recommended_action);

  if (migration.state === 'pending') {
    let body =
      'Pulse detected a paid v5 license, but the automatic v6 exchange did not complete yet.';
    switch (migration.reason) {
      case 'exchange_rate_limited':
        body = 'Pulse detected a paid v5 license, but the v6 exchange is rate-limited right now.';
        break;
      case 'exchange_conflict':
        body =
          'Pulse detected a paid v5 license, but another v6 activation handoff is still settling.';
        break;
      case 'exchange_unavailable':
      default:
        break;
    }

    return {
      tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
      title: 'v5 license migration pending',
      body: `${body} ${actionText}`,
    };
  }

  let body = 'Pulse detected a paid v5 license, but it could not be migrated automatically.';
  switch (migration.reason) {
    case 'exchange_invalid':
      body = 'Pulse detected a paid v5 license, but that key was rejected during v6 migration.';
      break;
    case 'exchange_malformed':
      body = 'Pulse detected a v5-looking key, but it is malformed and cannot be migrated.';
      break;
    case 'exchange_revoked':
      body =
        'Pulse detected a paid v5 license, but that key is no longer eligible for automatic migration.';
      break;
    case 'exchange_non_migratable':
      body = 'Pulse detected a paid v5 license, but it is not eligible for automatic v6 migration.';
      break;
    case 'exchange_unsupported':
      body = 'Pulse detected a key that is not a supported v5 Pro/Lifetime migration input.';
      break;
    default:
      break;
  }

  return {
    tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
    title: 'v5 license migration needs attention',
    body: `${body} ${actionText}`,
  };
};

export const getPurchaseActivationNotice = (result?: string | null): LicenseInlineNotice | null => {
  switch ((result || '').trim().toLowerCase()) {
    case 'activated':
      return {
        tone: 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
        title: 'Plan activated',
        body: 'Pulse finished checkout and activated this instance automatically. The plan state below is live.',
      };
    case 'cancelled':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Checkout cancelled',
        body: 'Checkout was cancelled before completion. The current plan state below is unchanged until you start the upgrade again.',
      };
    case 'expired':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Upgrade return expired',
        body: 'That secure checkout return link expired or was already used. Start the upgrade again from this instance if you still need it.',
      };
    case 'failed':
      return {
        tone: 'border-red-200 dark:border-red-900 bg-red-50 dark:bg-red-900 text-red-900 dark:text-red-100',
        title: 'Activation needs attention',
        body: 'Checkout completed, but Pulse could not finish local activation automatically. Review the plan state below, then open recovery if you already have a key from this purchase.',
      };
    case 'unavailable':
      return {
        tone: 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100',
        title: 'Pulse Account unavailable',
        body: 'Pulse could not open the Pulse Account upgrade flow right now. The current plan state below is unchanged. Retry from this instance in a moment, or use recovery below if you already have a key.',
      };
    default:
      return null;
  }
};

export const getLicenseSubscriptionStatusPresentation = (
  state?: string | null,
): LicenseSubscriptionStatusPresentation => {
  switch ((state || '').trim().toLowerCase()) {
    case 'trial':
      return {
        label: 'Trial',
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      };
    case 'active':
      return {
        label: 'Active',
        badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      };
    case 'grace':
      return {
        label: 'Grace Period',
        badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-300',
      };
    case 'suspended':
      return {
        label: 'Suspended',
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
      };
    case 'canceled':
    case 'expired':
      return {
        label: 'Expired',
        badgeClass: 'bg-red-100 text-red-700 dark:bg-red-900 dark:text-red-300',
      };
    default:
      return {
        label: 'Unknown',
        badgeClass: 'bg-surface-alt text-muted',
      };
  }
};

export const getSelfHostedCurrentPlanStatusPresentation = (
  entitlements?: Pick<LicenseCommercialEntitlements, 'tier' | 'subscription_state'> | null,
): LicenseSubscriptionStatusPresentation => {
  const normalizedTier = (entitlements?.tier || '').trim().toLowerCase();
  if (normalizedTier === 'free' || normalizedTier === 'community') {
    return {
      label: 'Community',
      badgeClass: 'bg-surface text-base-content border border-border',
    };
  }
  return getLicenseSubscriptionStatusPresentation(entitlements?.subscription_state);
};

export const getLicenseStatusLoadingState = (): LicenseLoadingStateCopy => ({
  text: 'Loading license status...',
});

export const getNoActiveProLicenseState = (): LicenseLoadingStateCopy => ({
  text: 'No Pro license is active.',
});

export const SELF_HOSTED_RECOVERY_PRESENTATION: SelfHostedRecoveryPresentation = {
  disclosureLabel: 'Redeem existing key',
  disclosureDescription:
    'Use this only if you already have an activation key or need to recover a legacy self-hosted purchase on this instance.',
  fieldLabel: 'License or Activation Key',
  fieldPlaceholder: 'Paste your license key or activation key',
  helpTextBeforeTerms:
    'Paste the Pulse v6 activation key shown on the hosted checkout success page. A backup copy is also sent by email, but the hosted success page is the primary handoff. You can also paste a legacy Pulse v5 Pro/Lifetime license key and Pulse will exchange it automatically during activation when migration is available. By activating a license, you agree to the',
  helpTextAfterTerms: '.',
  termsLabel: 'Terms of Service',
  activateIdleLabel: 'Activate License',
  activatePendingLabel: 'Activating...',
  clearIdleLabel: 'Clear License',
  clearPendingLabel: 'Clearing...',
  legacyNotice: {
    title: 'Legacy v5 license detected',
    body: 'Pulse will try to exchange this key into the v6 activation model automatically. If the exchange cannot complete immediately, retry from this panel or use the self-serve retrieval flow to get the current v6 activation key.',
  },
};

export const getOrganizationBillingLicenseStatusLabel = (
  status?: Pick<LicenseStatus, 'valid' | 'in_grace_period'> | null,
): string => {
  if (!status?.valid) return 'No License';
  return status.in_grace_period ? 'Grace Period' : 'Active';
};

export const getBillingAdminTrialStatus = (
  state?: Pick<BillingState, 'subscription_state' | 'trial_started_at' | 'trial_ends_at'> | null,
): string => {
  if (!state) return 'Loading...';

  const subscriptionState = (state.subscription_state || '').toLowerCase();
  if (subscriptionState !== 'trial' && !state.trial_ends_at && !state.trial_started_at) {
    return 'No trial';
  }

  const started = formatUnixSeconds(state.trial_started_at);
  const ends = formatUnixSeconds(state.trial_ends_at);
  if (subscriptionState === 'trial') {
    return `Trial (ends ${ends})`;
  }
  return `Trial (started ${started}, ends ${ends})`;
};

export const getBillingAdminOrganizationBadges = (
  organization: Pick<HostedOrganizationSummary, 'soft_deleted' | 'suspended'>,
): BillingAdminOrganizationBadge[] => {
  const badges: BillingAdminOrganizationBadge[] = [];
  if (organization.soft_deleted) {
    badges.push({
      label: 'soft-deleted',
      badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    });
  }
  if (organization.suspended && !organization.soft_deleted) {
    badges.push({
      label: 'suspended',
      badgeClass: 'bg-amber-100 text-amber-700 dark:bg-amber-900 dark:text-amber-200',
    });
  }
  return badges;
};

export const getBillingAdminStateUpdateSuccessMessage = (
  nextState: 'suspended' | 'active',
): string =>
  nextState === 'suspended' ? 'Organization billing suspended' : 'Organization billing activated';

export const BILLING_ADMIN_EMPTY_STATE = 'No organizations found.';

function formatUnixSeconds(value?: number | null): string {
  if (!value || value <= 0) return 'N/A';
  const date = new Date(value * 1000);
  if (Number.isNaN(date.getTime())) return String(value);
  return date.toLocaleString();
}
