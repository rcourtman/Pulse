export type SelfHostedTierKey = 'community' | 'relay' | 'pro';

export interface SelfHostedPlanDefinition {
  tier: SelfHostedTierKey;
  name: string;
  price: string;
  subline: string;
  metricHistoryDays: number;
  billingExtrasSummary: string;
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
  startTrialLabel: string;
  featureComparisonHeading: string;
  footerLinks: readonly SelfHostedLinkCTA[];
}

export const SELF_HOSTED_PLAN_DEFINITIONS: readonly SelfHostedPlanDefinition[] = [
  {
    tier: 'community',
    name: 'Community',
    price: 'Free forever',
    subline: 'Unlimited self-hosted monitoring',
    metricHistoryDays: 7,
    billingExtrasSummary: 'Patrol, alerts, and OIDC',
    highlights: [
      'Real-time monitoring',
      'Unlimited self-hosted monitoring',
      '7-day metric history',
      'Pulse Patrol (BYOK)',
      'Patrol quickstart after activation or trial: 25 runs, no API key',
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
    billingExtrasSummary: 'AI operations and advanced admin',
    highlights: [
      'Everything in Relay',
      'Patrol Auto-Fix & investigation',
      'Pulse Alert Analysis',
      'Kubernetes Insights',
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
      return SELF_HOSTED_PLAN_BY_TIER.pro;
    default:
      return null;
  }
}

export const SELF_HOSTED_COMMERCIAL_PRESENTATION: SelfHostedCommercialPresentation = {
  pageTitle: 'Pricing',
  pageDescription:
    'Core monitoring stays free for self-hosted Pulse. Relay adds remote access and mobile convenience, while Pro unlocks AI operations, automation, governance, and longer history.',
  mostPopularBadge: 'Most Popular',
  currentPlanLabel: 'Current Plan',
  includedLabel: 'Included',
  freeLabel: 'Free',
  buyRelayLabel: 'Buy Relay',
  upgradeToProLabel: 'Upgrade to Pro',
  startTrialLabel: 'Start Free 14-day Trial',
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
  {
    key: 'update_alerts',
    name: 'Update Alerts (Container/Package)',
    community: true,
    relay: true,
    pro: true,
  },
  {
    key: 'sso',
    name: 'Basic SSO (OIDC)',
    community: true,
    relay: true,
    pro: true,
  },
  {
    key: 'relay',
    name: 'Remote Access (Relay)',
    community: false,
    relay: true,
    pro: true,
  },
  {
    key: 'mobile_app',
    name: 'Mobile App Access',
    community: false,
    relay: true,
    pro: true,
  },
  {
    key: 'push_notifications',
    name: 'Push Notifications',
    community: false,
    relay: true,
    pro: true,
  },
  {
    key: 'ai_patrol',
    name: 'Pulse Patrol',
    community: true,
    relay: true,
    pro: true,
  },
  {
    key: 'ai_autofix',
    name: 'Patrol Auto-Fix',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'ai_alerts',
    name: 'Pulse Alert Analysis',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'kubernetes_ai',
    name: 'Kubernetes Insights',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'rbac',
    name: 'Role-Based Access Control (RBAC)',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'audit_logging',
    name: 'Audit Logging',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'advanced_sso',
    name: 'Advanced SSO (SAML/Multi-Provider)',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'agent_profiles',
    name: 'Centralized Agent Profiles',
    community: false,
    relay: false,
    pro: true,
  },
  {
    key: 'advanced_reporting',
    name: 'PDF/CSV Reporting',
    community: false,
    relay: false,
    pro: true,
  },
] as const;
