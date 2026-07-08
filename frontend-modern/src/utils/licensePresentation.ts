import type { BillingState, HostedOrganizationSummary } from '@/api/billingAdmin';
import type {
  CommercialMigrationStatus,
  LicenseCommercialEntitlements,
  LicenseStatus,
} from '@/api/license';
import { CLOUD_PLAN_LABELS } from '@/utils/cloudPlans';
import {
  getSelfHostedPlanDefinitionForBillingTier,
  getSelfHostedPlanEntitlementSummary,
} from '@/utils/selfHostedPlans';
import { PATROL_CONTROL_PATH, PATROL_CONTROL_PATH_WITH_STARTER } from '@/routing/resourceLinks';
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

export interface BillingAdminOrganizationBadge {
  label: string;
  badgeClass: string;
}

export interface SelfHostedActivationNoticeCopy {
  title: string;
  body: string;
}

export interface LicenseExternalActionCopy {
  actionLabel: string;
  actionUrl: string;
}

export type SelfHostedPatrolControlActionIntent = 'patrol_control';

export interface SelfHostedPatrolControlAction extends LicenseExternalActionCopy {
  actionIntent: SelfHostedPatrolControlActionIntent;
}

export interface SelfHostedRecoveryPresentation {
  disclosureLabel: string;
  disclosureDescription: string;
  fieldLabel: string;
  fieldPlaceholder: string;
  helpTextBeforeTerms: string;
  helpTextAfterTerms: string;
  termsLabel: string;
  privateRuntimeNotice: SelfHostedActivationNoticeCopy & LicenseExternalActionCopy;
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
  privateRuntimeAction?: LicenseExternalActionCopy;
  patrolControlAction?: SelfHostedPatrolControlAction;
}

export interface SelfHostedPlanStatusItem {
  label: string;
  statusLabel: string;
  state: 'active' | 'partial' | 'missing';
  detail: string;
}

export interface SelfHostedPlanStatusPresentation {
  title: string;
  body: string;
  items: SelfHostedPlanStatusItem[];
}

export interface SelfHostedPatrolOperatorStatus {
  nextAction?: string;
  progressLabel?: string;
  patrolControlOperationsLoopStarterCount?: number;
  patrolControlCompletedOperationsLoopCount?: number;
  patrolControlResolvedOperationsLoopCount?: number;
  patrolControlValueState?:
    | 'not_started'
    | 'in_progress'
    | 'governed_decision_recorded'
    | 'verified_needs_mcp'
    | 'verified';
  patrolAutonomyOperationsLoopStarterCount?: number;
  patrolAutonomyCompletedOperationsLoopCount?: number;
  patrolAutonomyResolvedOperationsLoopCount?: number;
  patrolAutonomyValueState?:
    | 'not_started'
    | 'in_progress'
    | 'governed_decision_recorded'
    | 'verified_needs_mcp'
    | 'verified';
  proActivationOperationsLoopStarterCount?: number;
  proActivationCompletedOperationsLoopCount?: number;
  proActivationResolvedOperationsLoopCount?: number;
  proActivationValueProofState?:
    | 'not_started'
    | 'in_progress'
    | 'governed_decision_recorded'
    | 'verified_needs_mcp'
    | 'verified';
  externalAgentReady: boolean;
}

export interface SelfHostedActivationSuccessPresentation extends LicenseInlineNotice {
  highlightsLabel: string;
  highlights: string[];
  actionLabel?: string;
  actionUrl?: string;
  actionIntent?: SelfHostedPatrolControlActionIntent;
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

const isActiveOrGraceSubscription = (subscriptionState?: string | null): boolean => {
  const normalized = (subscriptionState || '').trim().toLowerCase();
  return normalized === 'active' || normalized === 'grace';
};

const PRO_RUNTIME_REQUIRED_TIERS = new Set([
  'pro',
  'pro_annual',
  'pro_plus',
  'lifetime',
  'enterprise',
]);

export const PULSE_PRO_DOWNLOAD_URL = 'https://pulserelay.pro/download.html';
export const PULSE_PRO_RETRIEVE_LICENSE_URL = 'https://pulserelay.pro/retrieve-license';
export const PATROL_CONTROL_STARTER_URL = PATROL_CONTROL_PATH_WITH_STARTER;

const PATROL_CONTROL_CAPABILITIES = new Set(['ai_autofix']);

const hasPatrolControlCapability = (entitlements?: LicenseCommercialEntitlements | null): boolean =>
  Boolean(
    entitlements?.capabilities?.some((capability) =>
      PATROL_CONTROL_CAPABILITIES.has(capability.trim().toLowerCase()),
    ),
  );

const normalizeStatusCount = (value?: number | null): number => {
  if (typeof value !== 'number' || !Number.isFinite(value)) {
    return 0;
  }
  return Math.max(0, Math.trunc(value));
};

const getFirstPartyPatrolControlCount = (
  patrolControlCount?: number | null,
  patrolAutonomyCompatibilityCount?: number | null,
  proActivationAliasCount?: number | null,
): number => {
  const aggregatePatrolControlCount = normalizeStatusCount(
    patrolControlCount ?? patrolAutonomyCompatibilityCount,
  );
  return Math.max(0, aggregatePatrolControlCount - normalizeStatusCount(proActivationAliasCount));
};

const getPatrolControlStarterCount = (status?: SelfHostedPatrolOperatorStatus | null): number =>
  getFirstPartyPatrolControlCount(
    status?.patrolControlOperationsLoopStarterCount,
    status?.patrolAutonomyOperationsLoopStarterCount,
    status?.proActivationOperationsLoopStarterCount,
  );

const getPatrolControlCompletedCount = (status?: SelfHostedPatrolOperatorStatus | null): number =>
  getFirstPartyPatrolControlCount(
    status?.patrolControlCompletedOperationsLoopCount,
    status?.patrolAutonomyCompletedOperationsLoopCount,
    status?.proActivationCompletedOperationsLoopCount,
  );

const getPatrolControlResolvedCount = (status?: SelfHostedPatrolOperatorStatus | null): number =>
  getFirstPartyPatrolControlCount(
    status?.patrolControlResolvedOperationsLoopCount,
    status?.patrolAutonomyResolvedOperationsLoopCount,
    status?.proActivationResolvedOperationsLoopCount,
  );

const getPatrolControlValueState = (
  status?: SelfHostedPatrolOperatorStatus | null,
): SelfHostedPatrolOperatorStatus['patrolControlValueState'] =>
  getPatrolControlStarterCount(status) > 0 ||
  getPatrolControlCompletedCount(status) > 0 ||
  getPatrolControlResolvedCount(status) > 0
    ? (status?.patrolControlValueState ?? status?.patrolAutonomyValueState)
    : undefined;

const hasVerifiedPatrolOperatorOutcome = (
  status?: SelfHostedPatrolOperatorStatus | null,
): boolean => {
  const resolvedCount = getPatrolControlResolvedCount(status);
  const valueProofState = getPatrolControlValueState(status);
  return (
    valueProofState === 'verified' ||
    valueProofState === 'verified_needs_mcp' ||
    (!valueProofState && resolvedCount > 0)
  );
};

const hasGovernedDecisionOnlyPatrolOperatorOutcome = (
  status?: SelfHostedPatrolOperatorStatus | null,
): boolean => {
  const completedCount = getPatrolControlCompletedCount(status);
  const resolvedCount = getPatrolControlResolvedCount(status);
  const valueProofState = getPatrolControlValueState(status);
  return (
    valueProofState === 'governed_decision_recorded' ||
    (!valueProofState && completedCount > 0 && resolvedCount === 0)
  );
};

type PatrolControlActionStage = 'set' | 'continue' | 'decision';

const getPatrolControlActionStage = (
  status?: SelfHostedPatrolOperatorStatus | null,
): PatrolControlActionStage => {
  if (hasGovernedDecisionOnlyPatrolOperatorOutcome(status)) {
    return 'decision';
  }
  const completedCount = getPatrolControlCompletedCount(status);
  const resolvedCount = getPatrolControlResolvedCount(status);
  const starterCount = getPatrolControlStarterCount(status);
  if (
    !hasVerifiedPatrolOperatorOutcome(status) &&
    (starterCount > 0 || completedCount > 0 || resolvedCount > 0)
  ) {
    return 'continue';
  }

  return 'set';
};

const getPatrolControlActionLabel = (status?: SelfHostedPatrolOperatorStatus | null): string => {
  switch (getPatrolControlActionStage(status)) {
    case 'decision':
      return 'Review Patrol decision';
    case 'continue':
      return 'Open Patrol';
    case 'set':
      return PATROL_CONTROL_ACTION.actionLabel;
  }
};

const getPatrolControlActionUrl = (status?: SelfHostedPatrolOperatorStatus | null): string => {
  const stage = getPatrolControlActionStage(status);
  if (stage === 'decision') {
    return PATROL_CONTROL_PATH;
  }
  return PATROL_CONTROL_STARTER_URL;
};

const PATROL_CONTROL_ACTION: SelfHostedPatrolControlAction = {
  actionLabel: 'Choose Patrol mode',
  actionUrl: PATROL_CONTROL_STARTER_URL,
  actionIntent: 'patrol_control',
};

const isActivePaidRuntimeState = (subscriptionState?: string | null): boolean => {
  const normalized = (subscriptionState || '').trim().toLowerCase();
  return normalized === 'active' || normalized === 'grace' || normalized === 'trial';
};

export const requiresPulseProRuntime = (
  entitlements?: Pick<
    LicenseCommercialEntitlements,
    'hosted_mode' | 'subscription_state' | 'tier'
  > | null,
): boolean => {
  if (!entitlements || entitlements.hosted_mode) {
    return false;
  }
  const normalizedTier = (entitlements.tier || '').trim().toLowerCase();
  return (
    PRO_RUNTIME_REQUIRED_TIERS.has(normalizedTier) &&
    isActivePaidRuntimeState(entitlements.subscription_state)
  );
};

export const hasPulseProRuntime = (
  entitlements?: Pick<LicenseCommercialEntitlements, 'runtime'> | null,
): boolean => (entitlements?.runtime?.build || '').trim().toLowerCase() === 'pro';

const getPulseProRuntimeBuild = (
  entitlements?: Pick<LicenseCommercialEntitlements, 'runtime'> | null,
): string => (entitlements?.runtime?.build || '').trim().toLowerCase();

export const hasPulseProRuntimeMismatch = (
  entitlements?: Pick<
    LicenseCommercialEntitlements,
    'hosted_mode' | 'runtime' | 'subscription_state' | 'tier'
  > | null,
): boolean => {
  return requiresPulseProRuntime(entitlements) && getPulseProRuntimeBuild(entitlements) !== 'pro';
};

const getPulseProRuntimeMismatchSummary = (
  entitlements?: Pick<LicenseCommercialEntitlements, 'runtime'> | null,
): string =>
  getPulseProRuntimeBuild(entitlements) === 'community'
    ? 'running the community runtime'
    : 'not reporting the private Pulse Pro runtime';

const getPulseProRuntimeMismatchDetail = (
  entitlements?: Pick<LicenseCommercialEntitlements, 'runtime'> | null,
): string =>
  getPulseProRuntimeBuild(entitlements) === 'community'
    ? `This install reports the community runtime. Open ${PULSE_PRO_DOWNLOAD_URL} with your activation key (starts ppk_live_) and install the private Pulse Pro runtime; if you do not have an activation key, issue one at ${PULSE_PRO_RETRIEVE_LICENSE_URL} with your purchase email. Public GitHub releases and the public Docker image do not include Pro-only runtime hooks.`
    : `This install is not reporting a Pulse Pro runtime identity. Open ${PULSE_PRO_DOWNLOAD_URL} with your activation key (starts ppk_live_) and install the private Pulse Pro runtime; if you do not have an activation key, issue one at ${PULSE_PRO_RETRIEVE_LICENSE_URL} with your purchase email. Public GitHub releases and the public Docker image do not include Pro-only runtime hooks.`;

const getPatrolControlAction = ({
  entitlements,
  patrolOperatorStatus,
  planDefinition,
  subscriptionState,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  patrolOperatorStatus?: SelfHostedPatrolOperatorStatus | null;
  planDefinition: ReturnType<typeof getSelfHostedPlanDefinitionForBillingTier>;
  subscriptionState?: string | null;
}): SelfHostedPatrolControlAction | undefined => {
  const normalizedState = (subscriptionState || '').trim().toLowerCase();
  if (normalizedState !== 'active' && normalizedState !== 'grace' && normalizedState !== 'trial') {
    return undefined;
  }
  if (planDefinition?.tier !== 'pro') {
    return undefined;
  }
  if (!hasPatrolControlCapability(entitlements)) {
    return undefined;
  }
  if (hasPulseProRuntimeMismatch(entitlements)) {
    return undefined;
  }
  return {
    ...PATROL_CONTROL_ACTION,
    actionLabel: getPatrolControlActionLabel(patrolOperatorStatus),
    actionUrl: getPatrolControlActionUrl(patrolOperatorStatus),
  };
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
    body: 'This migrated v5 Pro subscription keeps its existing recurring price until you cancel. Self-hosted monitoring and child-resource volume are not metered in current v6 self-hosted packaging. If you cancel and return later, current v6 pricing applies for paid features.',
  };
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
  for (const feature of planDefinition.highlights) {
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
  patrolOperatorStatus?: SelfHostedPatrolOperatorStatus | null,
): string | null => {
  if (!planDefinition) {
    return null;
  }
  if (planDefinition.tier === 'pro') {
    switch (getPatrolControlActionStage(patrolOperatorStatus)) {
      case 'decision':
        return `${planLabel} is active on this instance. Review the current Patrol decision.`;
      case 'continue':
        return `${planLabel} is active on this instance. Open Patrol to continue current work.`;
      case 'set':
        break;
    }
  }
  return getSelfHostedPlanEntitlementSummary(planDefinition.tier, planLabel);
};

const getSelfHostedActivationPatrolControlBody = ({
  actionStage,
  planLabel,
  source,
}: {
  actionStage: PatrolControlActionStage;
  planLabel: string;
  source: SelfHostedActivationSuccessSource;
}): string => {
  const prefix =
    source === 'purchase'
      ? `Checkout completed and ${planLabel} is active.`
      : `The license key was accepted and ${planLabel} is active.`;

  switch (actionStage) {
    case 'decision':
      return `${prefix} Review the current Patrol decision.`;
    case 'continue':
      return `${prefix} Open Patrol to continue current work.`;
    case 'set':
      return `${prefix} Choose Patrol mode.`;
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
          title: `${getSelfHostedPlanLabel(tier)} plan`,
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
  patrolOperatorStatus,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
  patrolOperatorStatus?: SelfHostedPatrolOperatorStatus | null;
}): SelfHostedCurrentPlanPresentation => {
  const current = entitlements;
  if (!current) {
    return {
      title: 'Current plan: Unknown',
      body: 'Pulse is still loading the current self-hosted plan state for this instance.',
      unlockedFeaturesLabel: 'Available on this instance',
      unlockedFeatures: [],
      includedExtras: [],
      supplementalBadges: [],
    };
  }

  const normalizedState = (current.subscription_state || '').trim().toLowerCase();
  const normalizedTier = (current.tier || '').trim().toLowerCase();
  const planLabel = getSelfHostedPlanLabel(current.tier);
  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(current.tier);
  const unlockedFeatures = getSelfHostedUnlockedFeatures({
    entitlements: current,
    displayableCapabilities,
  });
  const includedExtras = getSelfHostedIncludedExtras({
    entitlements: current,
  });
  const runtimeMismatch = hasPulseProRuntimeMismatch(current);
  const unlockedFeaturesLabel =
    normalizedTier === 'free' ? 'Included on this instance' : 'Primary capabilities';
  const patrolControlAction = getPatrolControlAction({
    entitlements: current,
    patrolOperatorStatus,
    planDefinition,
    subscriptionState: current.subscription_state,
  });

  const supplementalBadges: string[] = [];
  const supplementalDetails: string[] = [];

  if (
    isActiveOrGraceSubscription(current.subscription_state) &&
    isGrandfatheredRecurringV5PlanVersion(current.plan_version)
  ) {
    supplementalBadges.push('Grandfathered price');
    supplementalDetails.push(
      'This migrated v5 subscription keeps its existing recurring price until cancellation. Self-hosted monitoring and child-resource volume are not metered in current v6 self-hosted packaging.',
    );
  } else if (current.is_lifetime) {
    supplementalBadges.push('Grandfathered lifetime');
    supplementalDetails.push(
      'This migrated lifetime install remains valid permanently, and self-hosted monitoring plus child-resource volume are not metered in current v6 self-hosted packaging.',
    );
  }

  if (normalizedState === 'trial') {
    if (runtimeMismatch) {
      supplementalBadges.push('Pro runtime missing');
      supplementalDetails.unshift(
        'Public GitHub releases and the public Docker image are community builds. Open Pulse Pro downloads with your activation key to install the private Pulse Pro runtime and test Pro-only runtime hooks during the trial.',
      );
    }
    return {
      title: `Current plan: ${planLabel} Trial`,
      body: runtimeMismatch
        ? `${planLabel} trial entitlement is active, but this install is ${getPulseProRuntimeMismatchSummary(current)}. Open Pulse Pro downloads with your activation key and install the private Pulse Pro runtime to use Pro-only features.`
        : unlockedFeatures.length > 0
          ? `${planLabel} trial capabilities are active on this instance right now.`
          : `${planLabel} trial entitlement is being confirmed for this instance.`,
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtrasLabel: includedExtras.length > 0 ? 'Included extras' : undefined,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
      ...(patrolControlAction ? { patrolControlAction } : {}),
      ...(runtimeMismatch
        ? {
            privateRuntimeAction: {
              actionLabel: 'Open Pulse Pro downloads',
              actionUrl: PULSE_PRO_DOWNLOAD_URL,
            },
          }
        : {}),
    };
  }

  if (normalizedTier === 'free') {
    return {
      title: 'Current plan: Community',
      body:
        getSelfHostedActivePlanSummary('Community', planDefinition) ||
        'Community is active on this instance. Self-hosted monitoring, 7-day metric history, watch-only Patrol, and update alerts are included here.',
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
    };
  }

  if (normalizedState === 'active' || normalizedState === 'grace') {
    if (runtimeMismatch) {
      supplementalBadges.push('Pro runtime missing');
      supplementalDetails.unshift(
        'Public GitHub releases and the public Docker image are community builds. They can accept the license, but Pro-only runtime hooks are only in the private Pulse Pro download.',
      );
      return {
        title: `Current plan: ${planLabel}`,
        body: `${planLabel} is active, but this install is ${getPulseProRuntimeMismatchSummary(current)}. Open Pulse Pro downloads with your activation key and install the private Pulse Pro runtime to use Pro-only features such as Patrol mode, Audit Log, Audit Webhooks, and RBAC.`,
        unlockedFeaturesLabel,
        unlockedFeatures,
        includedExtrasLabel: includedExtras.length > 0 ? 'Included extras' : undefined,
        includedExtras,
        supplementalBadges,
        supplementalSummary: supplementalDetails.join(' '),
        privateRuntimeAction: {
          actionLabel: 'Open Pulse Pro downloads',
          actionUrl: PULSE_PRO_DOWNLOAD_URL,
        },
      };
    }

    return {
      title: `Current plan: ${planLabel}`,
      body:
        getSelfHostedActivePlanSummary(planLabel, planDefinition, patrolOperatorStatus) ||
        `${planLabel} is active on this instance. These capabilities are available right now.`,
      unlockedFeaturesLabel,
      unlockedFeatures,
      includedExtrasLabel: includedExtras.length > 0 ? 'Included extras' : undefined,
      includedExtras,
      supplementalBadges,
      supplementalSummary: supplementalDetails.join(' '),
      ...(patrolControlAction ? { patrolControlAction } : {}),
    };
  }

  return {
    title: `Current plan: ${planLabel}`,
    body: 'Review the plan details below to confirm what this key enables on this instance.',
    unlockedFeaturesLabel: 'Available on this instance',
    unlockedFeatures,
    includedExtras,
    supplementalBadges,
    supplementalSummary: supplementalDetails.join(' '),
  };
};

const getCapabilityStatusState = (
  capabilities: Set<string>,
  requiredCapabilities: readonly string[],
): SelfHostedPlanStatusItem['state'] => {
  const presentCount = requiredCapabilities.filter((capability) =>
    capabilities.has(capability),
  ).length;
  if (presentCount === requiredCapabilities.length) {
    return 'active';
  }
  return presentCount > 0 ? 'partial' : 'missing';
};

const getStatusStateLabel = (state: SelfHostedPlanStatusItem['state']): string => {
  switch (state) {
    case 'active':
      return 'Active';
    case 'partial':
      return 'Partial';
    case 'missing':
      return 'Needs attention';
  }
};

const buildCapabilityStatusItem = ({
  capabilities,
  label,
  requiredCapabilities,
  activeDetail,
  partialDetail,
  missingDetail,
}: {
  capabilities: Set<string>;
  label: string;
  requiredCapabilities: readonly string[];
  activeDetail: string;
  partialDetail: string;
  missingDetail: string;
}): SelfHostedPlanStatusItem => {
  const state = getCapabilityStatusState(capabilities, requiredCapabilities);
  return {
    label,
    state,
    statusLabel: getStatusStateLabel(state),
    detail: state === 'active' ? activeDetail : state === 'partial' ? partialDetail : missingDetail,
  };
};

export const getSelfHostedPlanStatusPresentation = (
  entitlements?: LicenseCommercialEntitlements | null,
  _patrolOperatorStatus?: SelfHostedPatrolOperatorStatus | null,
): SelfHostedPlanStatusPresentation | null => {
  const planDefinition = getSelfHostedPlanDefinitionForBillingTier(entitlements?.tier);
  if (!entitlements || !planDefinition || planDefinition.tier === 'community') {
    return null;
  }
  if (typeof entitlements.max_history_days !== 'number') {
    return null;
  }

  const normalizedState = (entitlements.subscription_state || '').trim().toLowerCase();
  if (normalizedState !== 'active' && normalizedState !== 'grace' && normalizedState !== 'trial') {
    return null;
  }

  const capabilities = new Set(
    (entitlements.capabilities || []).map((capability) => capability.trim().toLowerCase()),
  );
  const isProPlan = planDefinition.tier === 'pro';
  const items: SelfHostedPlanStatusItem[] = [];
  if (requiresPulseProRuntime(entitlements)) {
    const hasProRuntime = hasPulseProRuntime(entitlements);
    items.push({
      label: 'Pulse Pro runtime',
      state: hasProRuntime ? 'active' : 'missing',
      statusLabel: hasProRuntime ? 'Active' : 'Needs attention',
      detail: hasProRuntime
        ? 'This install is running the private Pulse Pro runtime required for Patrol mode and Pro-only actions.'
        : getPulseProRuntimeMismatchDetail(entitlements),
    });
  }
  items.push(
    buildCapabilityStatusItem({
      capabilities,
      label: 'Remote access, pairing, and push',
      requiredCapabilities: ['relay', 'mobile_app', 'push_notifications'],
      activeDetail:
        'Relay, Pulse Mobile pairing, and push notifications are available on this instance.',
      partialDetail:
        'Some remote-access capabilities are available. Refresh the plan or open recovery if remote access, Pulse Mobile pairing, or push stays unavailable.',
      missingDetail:
        'Remote access, Pulse Mobile pairing, or push notifications are not available yet. Refresh the plan or open recovery before relying on Relay.',
    }),
  );

  const requiredHistoryDays = planDefinition.metricHistoryDays;
  const actualHistoryDays = entitlements.max_history_days;
  const historyState =
    actualHistoryDays >= requiredHistoryDays
      ? 'active'
      : actualHistoryDays > 0
        ? 'partial'
        : 'missing';
  items.push({
    label: `${requiredHistoryDays}-day metric history`,
    state: historyState,
    statusLabel: getStatusStateLabel(historyState),
    detail:
      historyState === 'active'
        ? isProPlan
          ? `Patrol has ${actualHistoryDays} days of metric history available for investigation context.`
          : `This instance has ${actualHistoryDays} days of metric history available.`
        : historyState === 'partial'
          ? `This instance reports ${actualHistoryDays} days of metric history, below the expected ${requiredHistoryDays} days.`
          : `This instance does not have metric-history access yet; expected ${requiredHistoryDays} days.`,
  });

  if (planDefinition.tier === 'pro') {
    items.push(
      buildCapabilityStatusItem({
        capabilities,
        label: 'Patrol investigation and remediation',
        requiredCapabilities: ['ai_alerts', 'ai_autofix'],
        activeDetail:
          'Patrol can investigate alert-triggered issues and handle governed actions within the selected Patrol mode.',
        partialDetail:
          'Some Patrol capability is available, but investigation or remediation may not be fully enabled yet.',
        missingDetail:
          'Patrol investigation or remediation capability is missing. Refresh the plan or open recovery before relying on Patrol to handle issues.',
      }),
      buildCapabilityStatusItem({
        capabilities,
        label: 'Team and admin controls',
        requiredCapabilities: ['rbac', 'audit_logging', 'advanced_reporting', 'agent_profiles'],
        activeDetail:
          'RBAC, audit logging, reporting, and agent profiles are available for governed operations.',
        partialDetail:
          'Some team/admin capabilities are available. Refresh the plan or open recovery if any admin controls stay unavailable.',
        missingDetail:
          'Team/admin controls are not available yet. Refresh the plan or open recovery before relying on this Pro install.',
      }),
    );
  }

  const planLabel = getSelfHostedPlanLabel(entitlements.tier);
  return {
    title: isProPlan ? 'Capability details' : `${planLabel} status`,
    body: isProPlan
      ? 'Open this only when a Pro capability looks unavailable. Normal setup is choosing Patrol mode.'
      : 'These checks show the capabilities this instance can use right now, based on its entitlement and runtime payloads.',
    items,
  };
};

export const getSelfHostedActivationSuccessPresentation = ({
  entitlements,
  displayableCapabilities,
  patrolOperatorStatus,
  source,
}: {
  entitlements?: LicenseCommercialEntitlements | null;
  displayableCapabilities: string[];
  patrolOperatorStatus?: SelfHostedPatrolOperatorStatus | null;
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
  const runtimeMismatch = hasPulseProRuntimeMismatch(current);
  const patrolControlAction = getPatrolControlAction({
    entitlements: current,
    patrolOperatorStatus,
    planDefinition: getSelfHostedPlanDefinitionForBillingTier(current.tier),
    subscriptionState: current.subscription_state,
  });
  const patrolControlActionStage = getPatrolControlActionStage(patrolOperatorStatus);

  return {
    tone: runtimeMismatch
      ? 'border-amber-200 dark:border-amber-900 bg-amber-50 dark:bg-amber-900 text-amber-900 dark:text-amber-100'
      : 'border-green-200 dark:border-green-900 bg-green-50 dark:bg-green-900 text-green-900 dark:text-green-100',
    title: runtimeMismatch ? `${planLabel} license is active` : `${planLabel} is now active`,
    body:
      source === 'purchase'
        ? runtimeMismatch
          ? `Checkout completed and the license is active. This install is ${getPulseProRuntimeMismatchSummary(current)}, so open Pulse Pro downloads with your activation key and install the private Pulse Pro runtime before using Pro-only features.`
          : patrolControlAction
            ? getSelfHostedActivationPatrolControlBody({
                actionStage: patrolControlActionStage,
                planLabel,
                source,
              })
            : `Checkout completed and this instance is now running ${planLabel}.`
        : runtimeMismatch
          ? `The license key was accepted. This install is ${getPulseProRuntimeMismatchSummary(current)}, so open Pulse Pro downloads with your activation key and install the private Pulse Pro runtime before using Pro-only features.`
          : patrolControlAction
            ? getSelfHostedActivationPatrolControlBody({
                actionStage: patrolControlActionStage,
                planLabel,
                source,
              })
            : `The license key was accepted and this instance is now running ${planLabel}.`,
    highlightsLabel: runtimeMismatch ? 'Licensed capabilities' : 'Available now on this instance',
    highlights,
    ...(runtimeMismatch
      ? {
          actionLabel: 'Open Pulse Pro downloads',
          actionUrl: PULSE_PRO_DOWNLOAD_URL,
        }
      : patrolControlAction
        ? {
            actionLabel: patrolControlAction.actionLabel,
            actionUrl: patrolControlAction.actionUrl,
            actionIntent: patrolControlAction.actionIntent,
          }
        : {}),
  };
};

export const getCommercialMigrationActionText = (action?: string): string => {
  switch (action) {
    case 'retry_activation':
      return 'Retry from this instance.';
    case 'use_v6_activation_key':
      return 'Use the current v6 key for this purchase.';
    case 'enter_supported_v5_key':
      return 'Retry with the original v5 Pro/Lifetime key from this instance.';
    case 'free_installation_slot':
      return 'Contact support@pulserelay.pro to release an installation you no longer use or to raise the limit.';
    case 'retrieve_current_key':
      return 'Retrieve your current key at pulserelay.pro/retrieve-license and paste it here.';
    case 'allow_license_egress':
      return 'Allow outbound HTTPS to license.pulserelay.pro, then Pulse will keep retrying automatically.';
    default:
      return 'Review the plan state from this instance before trying again.';
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
          'Pulse detected a paid v5 license, but another v6 license handoff is still settling.';
        break;
      case 'exchange_connectivity_required':
        body =
          'Pulse has not been able to reach license.pulserelay.pro for over a day. Paid v6 features require periodic outbound HTTPS to that host. Core monitoring keeps running; paid features stay on Community until connectivity is allowed. See docs/UPGRADE_v6.md for the connectivity policy.';
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
    case 'exchange_installation_limit':
      body =
        'Pulse detected a paid v5 license, but that key is already active on its maximum number of v6 installations, so this instance cannot activate until a slot is freed.';
      break;
    case 'exchange_invalid':
      body =
        'Pulse detected a paid v5 license, but that key was rejected during v6 migration. Retrieve your current key at pulserelay.pro/retrieve-license if this purchase is still active.';
      break;
    case 'exchange_stale_key':
      body =
        'Pulse detected a paid v5 license, but this key has been superseded by a renewal.';
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
    case 'persisted_license_unreadable':
      body =
        'Pulse found a saved v5 license from a previous installation, but it could not be read on this system.';
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
        title: 'Plan needs attention',
        body: 'Checkout completed, but this instance could not apply the plan automatically. Review the current plan below, then open recovery if you already have a key from this purchase.',
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

export const getNoActiveSelfHostedActivationState = (): LicenseLoadingStateCopy => ({
  text: 'Community is ready to use on this instance.',
});

export const SELF_HOSTED_RECOVERY_PRESENTATION: SelfHostedRecoveryPresentation = {
  disclosureLabel: 'Manual key recovery',
  disclosureDescription:
    'Normal checkout handles this automatically. Open this only if you already have a key or need to recover an older purchase.',
  fieldLabel: 'License key',
  fieldPlaceholder: 'Paste your license key',
  helpTextBeforeTerms:
    'Paste the key from the hosted checkout success page or email. Legacy Pulse v5 Pro/Lifetime keys can be migrated when supported. By applying a license, you agree to the',
  helpTextAfterTerms: '.',
  termsLabel: 'Terms of Service',
  privateRuntimeNotice: {
    title: 'Paid Docker and Linux installs use a private runtime',
    body: 'Public GitHub releases and the public Docker image are community builds. Pro-only runtime hooks require the private Pulse Pro Docker image or Linux archive.',
    actionLabel: 'Open Pulse Pro downloads',
    actionUrl: PULSE_PRO_DOWNLOAD_URL,
  },
  activateIdleLabel: 'Apply key',
  activatePendingLabel: 'Applying...',
  clearIdleLabel: 'Clear key',
  clearPendingLabel: 'Clearing...',
  legacyNotice: {
    title: 'Legacy v5 license detected',
    body: 'Pulse will try to migrate this key automatically. If migration is still pending, retry here or use self-serve retrieval to get the current v6 key.',
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
