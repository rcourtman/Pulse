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
  mostPopularBadge: string;
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
    entry.key === 'long_term_metrics' ? `${metricHistoryDays}-day metric history` : entry.comparisonName,
  );
  const metricHistoryHighlight = getTierMetricHistoryHighlight(tier, metricHistoryDays);
  if (!metricHistoryHighlight || highlights.includes(metricHistoryHighlight)) {
    return highlights;
  }
  return [...highlights, metricHistoryHighlight];
};

const getTierIncludedExtras = (tier: SelfHostedTierKey): readonly string[] =>
  getSelfHostedFeaturesForRole(tier, 'included_extra').map((entry) => entry.displayName);

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
    subline: 'Unlimited self-hosted monitoring',
    metricHistoryDays: 7,
    billingExtrasSummary: 'Patrol, alerts, and OIDC',
    entitlementSummary:
      'Community is active on this instance. It includes self-hosted monitoring, 7-day metric history, Pulse Patrol (BYOK), and update alerts.',
    entitlementHighlights: [
      'Unlimited self-hosted monitoring',
      '7-day metric history',
      'Pulse Patrol (BYOK)',
      'Update alerts',
    ],
    includedExtras: [],
    comparisonSummary: 'Community covers self-hosted monitoring and core operations on this instance.',
    highlights: [
      'Real-time monitoring',
      'Unlimited self-hosted monitoring',
      '7-day metric history',
      'Pulse Patrol (BYOK)',
      'Hosted Patrol quickstart with activated entitlement: 25 runs, no API key',
      'Update alerts',
      'Basic SSO (OIDC)',
      'Community support',
    ],
  },
  {
    tier: 'relay',
    name: 'Relay',
    price: '$4.99/month',
    subline: 'or $39/year',
    metricHistoryDays: 14,
    billingExtrasSummary: 'Remote access, mobile, and push',
    entitlementSummary:
      'Relay is active on this instance. Remote access, mobile, push, and longer history are unlocked right now.',
    entitlementHighlights: getTierEntitlementHighlights('relay', 14),
    includedExtras: [],
    comparisonSummary:
      'Reach this Pulse instance securely from anywhere, check it from mobile, get push notifications, and keep 14 days of history.',
    highlights: [
      'Everything in Community',
      'Remote access via Relay',
      'Mobile app access',
      'Push notifications',
      'Custom URL (yourlab.pulserelay.pro)',
      '14-day metric history',
    ],
  },
  {
    tier: 'pro',
    name: 'Pro',
    price: '$8.99/month',
    subline: 'or $79/year',
    metricHistoryDays: 90,
    billingExtrasSummary: 'Root-cause analysis, remediation, and admin extras',
    entitlementSummary:
      'Pulse Pro is active on this instance. Root-cause analysis, safe remediation, and 90-day history are unlocked right now.',
    entitlementHighlights: getTierEntitlementHighlights('pro', 90),
    includedExtras: getTierIncludedExtras('pro'),
    comparisonSummary:
      'Move from monitoring into operations with root-cause answers, safe remediation, and 90-day history. Pulse Pro also includes SAML SSO, RBAC, audit logging, reporting, and agent profiles.',
    highlights: [
      'Everything in Relay',
      'Pulse Alert Analysis',
      'Patrol Auto-Fix',
      '90-day metric history',
      'RBAC, audit logging, SAML SSO',
      'Agent profiles · PDF/CSV reports',
      'Unlimited self-hosted monitoring',
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
  pageDescription:
    'Core monitoring is free for self-hosted Pulse. Relay adds secure remote access and mobile convenience, while Pro adds root-cause analysis, safe remediation, 90-day history, and admin/reporting extras.',
  mostPopularBadge: 'Most Popular',
  currentPlanLabel: 'Current Plan',
  includedLabel: 'Included',
  freeLabel: 'Free',
  buyRelayLabel: 'Buy Relay',
  upgradeToProLabel: 'Choose Pro',
  featureComparisonHeading: 'Feature Comparison',
  footerLinks: [
    {
      preface: 'Need managed hosting?',
      label: 'See Cloud plans',
      href: '/cloud',
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
    name: 'Core Monitoring Scope',
    community: 'Unlimited',
    relay: 'Unlimited',
    pro: 'Unlimited',
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
