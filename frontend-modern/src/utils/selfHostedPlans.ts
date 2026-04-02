export type SelfHostedTierKey = 'community' | 'relay' | 'pro' | 'proPlus';

export interface SelfHostedPlanDefinition {
  tier: SelfHostedTierKey;
  name: string;
  price: string;
  subline: string;
  monitoredSystems: number;
  metricHistoryDays: number;
  highlights: readonly string[];
}

export interface SelfHostedFeatureRow {
  key: string;
  name: string;
  community: boolean | string;
  relay: boolean | string;
  pro: boolean | string;
  proPlus: boolean | string;
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
  upgradeToProPlusLabel: string;
  startTrialLabel: string;
  featureComparisonHeading: string;
  footerLinks: readonly SelfHostedLinkCTA[];
}

export const SELF_HOSTED_PLAN_DEFINITIONS: readonly SelfHostedPlanDefinition[] = [
  {
    tier: 'community',
    name: 'Community',
    price: 'Free forever',
    subline: 'Monitor up to 5 systems for free',
    monitoredSystems: 5,
    metricHistoryDays: 7,
    highlights: [
      'Real-time monitoring',
      '7-day metric history',
      'AI Patrol (BYOK)',
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
    monitoredSystems: 8,
    metricHistoryDays: 14,
    highlights: [
      'Everything in Community',
      'Remote access via Relay',
      'Mobile app access',
      'Push notifications',
      'Custom URL (yourlab.pulserelay.pro)',
      '8 monitored systems · 14-day history',
    ],
  },
  {
    tier: 'pro',
    name: 'Pro',
    price: '$8.99/month',
    subline: 'or $79/year',
    monitoredSystems: 15,
    metricHistoryDays: 90,
    highlights: [
      'Everything in Relay',
      'AI Auto-Fix & investigation',
      'AI alert analysis',
      'Kubernetes AI analysis',
      '90-day metric history',
      'RBAC, audit logging, SAML SSO',
      'Agent profiles · PDF/CSV reports',
      '15 monitored systems',
    ],
  },
  {
    tier: 'proPlus',
    name: 'Pro+',
    price: '$14.99/month',
    subline: 'or $129/year',
    monitoredSystems: 50,
    metricHistoryDays: 90,
    highlights: ['Everything in Pro', '50 monitored systems'],
  },
] as const;

export const SELF_HOSTED_PLAN_BY_TIER: Record<SelfHostedTierKey, SelfHostedPlanDefinition> = {
  community: SELF_HOSTED_PLAN_DEFINITIONS[0],
  relay: SELF_HOSTED_PLAN_DEFINITIONS[1],
  pro: SELF_HOSTED_PLAN_DEFINITIONS[2],
  proPlus: SELF_HOSTED_PLAN_DEFINITIONS[3],
};

export const SELF_HOSTED_COMMERCIAL_PRESENTATION: SelfHostedCommercialPresentation = {
  pageTitle: 'Pricing',
  pageDescription: 'Core monitoring stays free. Relay adds remote access, and Pro unlocks AI operations.',
  mostPopularBadge: 'Most Popular',
  currentPlanLabel: 'Current Plan',
  includedLabel: 'Included',
  freeLabel: 'Free',
  buyRelayLabel: 'Buy Relay',
  upgradeToProLabel: 'Upgrade to Pro',
  upgradeToProPlusLabel: 'Upgrade to Pro+',
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
    {
      preface: 'Need more than 50 monitored systems?',
      label: 'Contact us',
      href: 'mailto:hello@pulserelay.pro?subject=Pulse%20Enterprise%20Inquiry',
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
    proPlus: true,
  },
  {
    key: 'hosts',
    name: 'Monitored Systems',
    community: String(SELF_HOSTED_PLAN_BY_TIER.community.monitoredSystems),
    relay: String(SELF_HOSTED_PLAN_BY_TIER.relay.monitoredSystems),
    pro: String(SELF_HOSTED_PLAN_BY_TIER.pro.monitoredSystems),
    proPlus: String(SELF_HOSTED_PLAN_BY_TIER.proPlus.monitoredSystems),
  },
  {
    key: 'history',
    name: 'Metric History',
    community: `${SELF_HOSTED_PLAN_BY_TIER.community.metricHistoryDays} days`,
    relay: `${SELF_HOSTED_PLAN_BY_TIER.relay.metricHistoryDays} days`,
    pro: `${SELF_HOSTED_PLAN_BY_TIER.pro.metricHistoryDays} days`,
    proPlus: `${SELF_HOSTED_PLAN_BY_TIER.proPlus.metricHistoryDays} days`,
  },
  {
    key: 'update_alerts',
    name: 'Update Alerts (Container/Package)',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'sso',
    name: 'Basic SSO (OIDC)',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'relay',
    name: 'Remote Access (Relay)',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'mobile_app',
    name: 'Mobile App Access',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'push_notifications',
    name: 'Push Notifications',
    community: false,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'ai_patrol',
    name: 'AI Patrol (Background Health Checks)',
    community: true,
    relay: true,
    pro: true,
    proPlus: true,
  },
  {
    key: 'ai_autofix',
    name: 'AI Patrol Auto-Fix',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'ai_alerts',
    name: 'AI Alert Analysis',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'kubernetes_ai',
    name: 'Kubernetes AI Analysis',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'rbac',
    name: 'Role-Based Access Control (RBAC)',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'audit_logging',
    name: 'Audit Logging',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'advanced_sso',
    name: 'Advanced SSO (SAML/Multi-Provider)',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'agent_profiles',
    name: 'Centralized Agent Profiles',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
  {
    key: 'advanced_reporting',
    name: 'PDF/CSV Reporting',
    community: false,
    relay: false,
    pro: true,
    proPlus: true,
  },
] as const;
