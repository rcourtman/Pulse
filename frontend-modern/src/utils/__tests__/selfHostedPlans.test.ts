import { describe, expect, it } from 'vitest';

import {
  getSelfHostedPlanDefinitionForBillingTier,
  SELF_HOSTED_COMMERCIAL_PRESENTATION,
  SELF_HOSTED_FEATURE_ROWS,
  SELF_HOSTED_PLAN_BY_TIER,
  SELF_HOSTED_PLAN_DEFINITIONS,
} from '../selfHostedPlans';

describe('selfHostedPlans', () => {
  it('keeps core monitoring aligned as an included self-hosted baseline', () => {
    expect(SELF_HOSTED_PLAN_DEFINITIONS.map((tier) => tier.name)).toEqual([
      'Community',
      'Relay',
      'Pro',
    ]);

    const monitoringScopeRow = SELF_HOSTED_FEATURE_ROWS.find(
      (row) => row.key === 'core_monitoring_scope',
    );
    expect(monitoringScopeRow).toEqual({
      key: 'core_monitoring_scope',
      name: 'Core Monitoring',
      community: 'Included',
      relay: 'Included',
      pro: 'Included',
    });

    expect(SELF_HOSTED_FEATURE_ROWS).toEqual(
      expect.arrayContaining([
        {
          key: 'update_alerts',
          name: 'Update Alerts',
          community: true,
          relay: true,
          pro: true,
        },
        {
          key: 'relay',
          name: 'Pulse Relay (Remote Access)',
          community: false,
          relay: true,
          pro: true,
        },
      ]),
    );
    expect(SELF_HOSTED_FEATURE_ROWS.find((row) => row.key === 'kubernetes_ai')).toBeUndefined();
  });

  it('keeps shared self-hosted commercial copy in the common contract', () => {
    expect(SELF_HOSTED_COMMERCIAL_PRESENTATION).toEqual({
      pageTitle: 'Pricing',
      pageDescription:
        'Self-hosted Pulse includes core monitoring for free. Relay adds secure remote access and mobile convenience, while Pro adds root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras.',
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
          label: 'Open Pulse Account',
          href: 'https://cloud.pulserelay.pro/portal',
        },
        {
          preface: 'Managing clients?',
          label: 'See MSP plans',
          href: 'mailto:hello@pulserelay.pro?subject=Pulse%20MSP%20Inquiry',
        },
      ],
    });
  });

  it('keeps Community copy free-first and Pro copy focused on operational extras', () => {
    expect(SELF_HOSTED_PLAN_BY_TIER.community.subline).toBe('Core monitoring included');
    expect(SELF_HOSTED_PLAN_BY_TIER.community.billingExtrasSummary).toBe(
      'Patrol, alerts, and OIDC',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.entitlementSummary).toContain(
      'Community is active on this instance.',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.comparisonSummary).toBe(
      'Community covers self-hosted monitoring and core operations on this instance.',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.entitlementHighlights).toEqual([
      'Real-time monitoring',
      '7-day metric history',
      'Pulse Patrol (BYOK)',
      'Update alerts',
    ]);
    expect(SELF_HOSTED_PLAN_BY_TIER.community.includedExtras).toEqual([]);
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.billingExtrasSummary).toBe(
      'Remote access, mobile, and push',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementSummary).toContain(
      'Remote access, mobile, push, and longer history are available right now.',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.comparisonSummary).toContain(
      'Reach this Pulse instance securely from anywhere',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementHighlights).toEqual([
      'Pulse Relay (Remote Access)',
      'Mobile App Access',
      'Push Notifications',
      '14-day metric history',
    ]);
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.includedExtras).toEqual([]);
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.billingExtrasSummary).toBe(
      'Analysis, remediation, and admin controls',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.entitlementSummary).toContain(
      'Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.comparisonSummary).toContain(
      'safe remediation workflows, 90-day history',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.entitlementHighlights).toEqual([
      'Alert Root-Cause Analysis',
      'Safe Remediation Workflows',
      '90-day metric history',
    ]);
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.includedExtras).toEqual([
      'Advanced SSO (SAML/Multi-Provider)',
      'Role-Based Access Control (RBAC)',
      'Audit Logging',
      'PDF/CSV Reporting',
      'Centralized Agent Profiles',
    ]);
    expect(SELF_HOSTED_PLAN_BY_TIER.community.highlights).toEqual(
      expect.arrayContaining(['Real-time monitoring', 'Pulse Patrol (BYOK)']),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.highlights).not.toContain(
      'Hosted Patrol quickstart with activated entitlement: 25 runs, no API key',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.highlights).toContain('14-day metric history');
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.highlights.join('\n')).not.toMatch(
      /unlimited.*self-hosted.*monitoring/i,
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.highlights).not.toContain(
      'Patrol quickstart: 25 runs, no API key',
    );
  });

  it('maps current billing tiers back to the shared self-hosted plan definitions', () => {
    expect(getSelfHostedPlanDefinitionForBillingTier('free')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.community,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('community')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.community,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('relay')).toBe(SELF_HOSTED_PLAN_BY_TIER.relay);
    expect(getSelfHostedPlanDefinitionForBillingTier('pro')).toBe(SELF_HOSTED_PLAN_BY_TIER.pro);
    expect(getSelfHostedPlanDefinitionForBillingTier('pro_annual')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.pro,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('pro_plus')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.pro,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('lifetime')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.pro,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('enterprise')).toBeNull();
  });
});
