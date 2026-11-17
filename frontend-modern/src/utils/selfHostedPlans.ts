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
  billingExtrasSummary: string;
  entitlementSummary: string;
  entitlementHighlights: readonly string[];
  includedExtras: readonly string[];
  comparisonSummary: string;
  highlights: readonly string[];
  // Short line under the plan name on the in-app comparison card.
  comparisonTagline?: string;
  // Feature groups for the in-app comparison card; falls back to highlights.
  comparisonGroups?: readonly SelfHostedPlanComparisonGroup[];
}

export interface SelfHostedPlanComparisonGroup {
  title: string;
  items: readonly string[];
}

const PRO_COMPARISON_GROUPS: readonly SelfHostedPlanComparisonGroup[] = [
  {
    title: 'For your team',
    items: ['3 named team seats', 'Roles and permissions (RBAC)', 'Audit logging'],
  },
  {
    title: 'For larger estates',
    items: [
      'Reporting and agent profiles',
      'External probes from another site',
      'Commercial support, typically within two business days',
    ],
  },
  {
    title: 'Patrol',
    items: [
      'Patrol modes: Ask first, Safe auto-fix, or Autopilot',
      'Patrol fixes issues and verifies the result',
    ],
  },
];

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

const getTierEntitlementHighlights = (tier: SelfHostedTierKey): readonly string[] =>
  getSelfHostedFeaturesForRole(tier, 'primary_pillar').map((entry) => entry.comparisonName);

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
      return `${planLabel} is active on this instance. It includes self-hosted monitoring, 90-day metric history, watch-only Patrol, update alerts, and SSO.`;
    case 'relay':
      return `${planLabel} is active on this instance. It includes Pulse Mobile pairing and push notifications until 31 March 2027.`;
    case 'pro':
      return `${planLabel} is active on this instance. It includes Patrol modes (Ask first, Safe auto-fix, Autopilot), RBAC, audit logging, reporting, agent profiles, and Pulse Mobile pairing until 31 March 2027.`;
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
    billingExtrasSummary: 'Watch-only Patrol, alerts, and SSO',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('community'),
    entitlementHighlights: [
      'Real-time monitoring',
      '90-day metric history',
      'Watch-only Patrol',
      'Update alerts',
    ],
    includedExtras: [],
    comparisonSummary:
      'Community covers self-hosted monitoring and watch-only Patrol on this instance.',
    highlights: [
      'Real-time monitoring',
      '90-day metric history',
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
    billingExtrasSummary: 'Pulse Mobile pairing and push',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('relay'),
    entitlementHighlights: getTierEntitlementHighlights('relay'),
    includedExtras: [],
    comparisonSummary:
      'Retired plan: Pulse Mobile pairing and push notifications until 31 March 2027.',
    highlights: ['Pulse Mobile pairing until 31 March 2027', 'Push notifications'],
  },
  {
    tier: 'pro',
    name: 'Pro',
    price: '$79/year',
    subline: 'or $8.99/month',
    billingExtrasSummary: 'Patrol modes and team controls',
    entitlementSummary: getSelfHostedPlanEntitlementSummary('pro'),
    entitlementHighlights: getTierEntitlementHighlights('pro'),
    includedExtras: getTierIncludedExtras('pro'),
    // Pro is sold as the plan for teams and larger estates: organisation
    // features lead, Patrol autonomy follows.
    comparisonSummary:
      'For small teams and larger estates: team access with audit logging, reporting, checks from outside your network, and support, plus Patrol fixes.',
    comparisonTagline: 'For small teams and larger estates',
    comparisonGroups: PRO_COMPARISON_GROUPS,
    highlights: PRO_COMPARISON_GROUPS.flatMap((group) => group.items),
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
  ...buildSelfHostedComparisonFeatureRows(),
] as const;
