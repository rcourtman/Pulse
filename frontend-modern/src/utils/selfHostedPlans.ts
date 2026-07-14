import {
  SELF_HOSTED_FEATURE_CATALOG,
  getSelfHostedFeaturesForRole,
} from './selfHostedFeatureCatalog.generated';

export type SelfHostedTierKey = 'community' | 'relay' | 'pro';

export interface SelfHostedPlanDefinition {
  tier: SelfHostedTierKey;
  name: string;
  price: string;
  subline: string;
  metricHistoryDays: number;
  billingExtrasSummary: string;
  entitlementSummary: string;
  entitlementHighlights: readonly string[];
  includedExtras: readonly string[];
  comparisonSummary: string;
  highlights: readonly string[];
}

export interface SelfHostedFeatureRow {
  key: string;
  name: string;
  community: boolean | string;
  relay: boolean | string;
  pro: boolean | string;
}

export interface SelfHostedLinkCTA {
  preface: string;
  label: string;
  href: string;
}

export interface SelfHostedCommercialPresentation {
  pageTitle: string;
  pageDescription: string;
  currentPlanLabel: string;
  includedLabel: string;
  freeLabel: string;
  buyRelayLabel: string;
  upgradeToProLabel: string;
  featureComparisonHeading: string;
  footerLinks: readonly SelfHostedLinkCTA[];
}

const getTierMetricHistoryHighlight = (tier: SelfHostedTierKey, metricHistoryDays: number) =>
  tier === 'relay' || tier === 'pro' ? `${metricHistoryDays}-day metric history` : null;

const getTierEntitlementHighlights = (
  tier: SelfHostedTierKey,
  metricHistoryDays: number,
): readonly string[] => {
  const highlights = getSelfHostedFeaturesForRole(tier, 'primary_pillar').map((entry) =>
    entry.key === 'long_term_metrics'
      ? `${metricHistoryDays}-day metric history`
      : entry.comparisonName,
  );
  const metricHistoryHighlight = getTierMetricHistoryHighlight(tier, metricHistoryDays);
  if (!metricHistoryHighlight || highlights.includes(metricHistoryHighlight)) {
    return highlights;
  }
  return [...highlights, metricHistoryHighlight];
};

const getTierIncludedExtras = (tier: SelfHostedTierKey): readonly string[] =>
  getSelfHostedFeaturesForRole(tier, 'included_extra').map((entry) => entry.displayName);

const SELF_HOSTED_PLAN_DEFAULT_ENTITLEMENT_LABELS: Record<SelfHostedTierKey, string> = {
  community: 'Community',
  relay: 'Relay',
  pro: 'Pulse Pro',
};

export function getSelfHostedPlanEntitlementSummary(
  tier: SelfHostedTierKey,
  planLabel = SELF_HOSTED_PLAN_DEFAULT_ENTITLEMENT_LABELS[tier],
): string {
  switch (tier) {
    case 'community':
      return `${planLabel} is active on this instance. It includes self-hosted monitoring, 7-day metric history, watch-only Patrol, update alerts, and SSO.`;
    case 'relay':
      return `${planLabel} is active on this instance. It includes remote web access, Pulse Mobile pairing, push notifications, and 14-day metric history.`;
    case 'pro':
      return `${planLabel} is active on this instance. It includes Relay connectivity, Pulse Mobile pairing, push notifications, Patrol modes (Ask first, Safe auto-fix, Autopilot), 90-day metric history, RBAC, audit logging, reporting, and agent profiles.`;
  }
}

const buildSelfHostedComparisonFeatureRows = (): readonly SelfHostedFeatureRow[] =>
  SELF_HOSTED_FEATURE_CATALOG.filter((entry) => entry.showInComparisonTable).map((entry) => ({
    key: entry.key,
    name: entry.comparisonName,
    community: entry.includedIn.community,
    relay: entry.includedIn.relay,
    pro: entry.includedIn.pro,
  }));

export const SELF_HOSTED_PLAN_DEFINITIONS: readonly SelfHostedPlanDefinition[] = [
  {
    tier: 'community',
    name: 'Community',
    price: 'Free forever',
    subline: 'Core monitoring included',
    metricHistoryDays: 7,
    billingExtrasSummary: 'Watch-only Patrol, alerts, and SSO',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('community'),
    entitlementHighlights: [
      'Real-time monitoring',
      '7-day metric history',
      'Watch-only Patrol',
      'Update alerts',
    ],
    includedExtras: [],
    comparisonSummary:
      'Community covers self-hosted monitoring and watch-only Patrol on this instance.',
    highlights: [
      'Real-time monitoring',
      '7-day metric history',
      'Watch-only Patrol',
      'Update alerts',
      'SSO (OIDC/SAML)',
      'Community support',
    ],
  },
  {
    tier: 'relay',
    name: 'Relay',
    price: '$39/year',
    subline: 'or $4.99/month',
    metricHistoryDays: 14,
    billingExtrasSummary: 'Remote web access, pairing, and push',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('relay'),
    entitlementHighlights: getTierEntitlementHighlights('relay', 14),
    includedExtras: [],
    comparisonSummary:
      'Remote web access, Pulse Mobile pairing, push notifications, and 14-day metric history.',
    highlights: [
      'Secure access when you are away from your network',
      'Remote web access via Relay',
      'Pulse Mobile pairing',
      'Push notifications',
      'No inbound ports required',
      '14-day metric history',
    ],
  },
  {
    tier: 'pro',
    name: 'Pro',
    price: '$79/year',
    subline: 'or $8.99/month',
    metricHistoryDays: 90,
    billingExtrasSummary: 'Patrol modes, history, and team controls',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('pro'),
    entitlementHighlights: getTierEntitlementHighlights('pro', 90),
    includedExtras: getTierIncludedExtras('pro'),
    comparisonSummary:
      'Patrol investigates issues, applies safe fixes, and verifies the result. Relay connectivity is included, plus 90-day metric history and team controls.',
    highlights: [
      'Relay connectivity, Pulse Mobile pairing, and push notifications included',
      'Patrol modes: Ask first, Safe auto-fix, or Autopilot',
      'Patrol investigates issues and explains the root cause',
      'Patrol applies safe fixes and verifies the result',
      '90-day metric history',
      'Team controls: RBAC, audit logging, reporting, and agent profiles',
    ],
  },
] as const;

export const SELF_HOSTED_PLAN_BY_TIER: Record<SelfHostedTierKey, SelfHostedPlanDefinition> = {
  community: SELF_HOSTED_PLAN_DEFINITIONS[0],
  relay: SELF_HOSTED_PLAN_DEFINITIONS[1],
  pro: SELF_HOSTED_PLAN_DEFINITIONS[2],
};

export function getSelfHostedPlanDefinitionForBillingTier(
  tier?: string | null,
): SelfHostedPlanDefinition | null {
  switch ((tier || '').trim().toLowerCase()) {
    case 'community':
    case 'free':
      return SELF_HOSTED_PLAN_BY_TIER.community;
    case 'relay':
      return SELF_HOSTED_PLAN_BY_TIER.relay;
    case 'pro':
    case 'pro_annual':
    case 'pro_plus':
    case 'lifetime':
      return SELF_HOSTED_PLAN_BY_TIER.pro;
    default:
      return null;
  }
}

export const SELF_HOSTED_COMMERCIAL_PRESENTATION: SelfHostedCommercialPresentation = {
  pageTitle: 'Pricing',
  pageDescription: 'Self-hosted plans and included capabilities.',
  currentPlanLabel: 'Current Plan',
  includedLabel: 'Included',
  freeLabel: 'Free',
  buyRelayLabel: 'Buy Relay',
  upgradeToProLabel: 'Choose Pro',
  featureComparisonHeading: 'Feature Comparison',
  footerLinks: [
    {
      preface: 'Need managed hosting?',
      label: 'Request managed hosting',
      href: 'mailto:support@pulserelay.pro?subject=Pulse%20Managed%20Hosting',
    },
    {
      preface: 'Managing clients?',
      label: 'See MSP plans',
      href: 'mailto:hello@pulserelay.pro?subject=Pulse%20MSP%20Inquiry',
    },
  ],
} as const;

export const SELF_HOSTED_FEATURE_ROWS: readonly SelfHostedFeatureRow[] = [
  {
    key: 'monitoring',
    name: 'Real-time Monitoring',
    community: true,
    relay: true,
    pro: true,
  },
  {
    key: 'core_monitoring_scope',
    name: 'Core Monitoring',
    community: 'Included',
    relay: 'Included',
    pro: 'Included',
  },
  {
    key: 'history',
    name: 'Metric History',
    community: `${SELF_HOSTED_PLAN_BY_TIER.community.metricHistoryDays} days`,
    relay: `${SELF_HOSTED_PLAN_BY_TIER.relay.metricHistoryDays} days`,
    pro: `${SELF_HOSTED_PLAN_BY_TIER.pro.metricHistoryDays} days`,
  },
  ...buildSelfHostedComparisonFeatureRows(),
] as const;
