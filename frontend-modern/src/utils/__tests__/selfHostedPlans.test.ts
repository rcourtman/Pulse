import { describe, expect, it } from 'vitest';

import {
  getSelfHostedPlanEntitlementSummary,
  getSelfHostedPlanDefinitionForBillingTier,
  SELF_HOSTED_COMMERCIAL_PRESENTATION,
  SELF_HOSTED_FEATURE_ROWS,
  SELF_HOSTED_PLAN_BY_TIER,
  SELF_HOSTED_PLAN_DEFINITIONS,
} from '../selfHostedPlans';
import {
  getSelfHostedFeaturesForRole,
  SELF_HOSTED_FEATURE_CATALOG,
} from '../selfHostedFeatureCatalog.generated';

const primaryPillarClaimLabels = (tier: 'relay' | 'pro', metricHistoryDays: number) =>
  getSelfHostedFeaturesForRole(tier, 'primary_pillar').map((entry) =>
    entry.key === 'long_term_metrics'
      ? `${metricHistoryDays}-day metric history`
      : entry.comparisonName,
  );

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
        'Self-hosted Pulse includes core monitoring for free. Relay adds secure remote access to the Pulse web UI, Pulse Mobile pairing, and push notifications, while Pro adds root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras.',
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
      'Patrol, alerts, and SSO',
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
      'Remote web access, pairing, and push',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementSummary).toContain(
      'Remote web access, Pulse Mobile pairing, push notifications, and longer history are available right now.',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.comparisonSummary).toContain(
      'Reach your Pulse web UI securely from anywhere',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementHighlights).toEqual([
      'Pulse Relay (Remote Access)',
      'Pulse Mobile Pairing',
      'Push Notifications',
      '14-day metric history',
    ]);
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.includedExtras).toEqual([]);
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.highlights).toContain('Pulse Mobile pairing');
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.highlights).toContain('No inbound ports required');
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.highlights.join('\n')).not.toMatch(
      /yourlab\.pulserelay\.pro|custom\s+(?:url|subdomain|domain)/i,
    );
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

  it('derives paid self-hosted feature claims from the generated catalog', () => {
    expect(SELF_HOSTED_PLAN_BY_TIER.community.entitlementSummary).toBe(
      getSelfHostedPlanEntitlementSummary('community'),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementSummary).toBe(
      getSelfHostedPlanEntitlementSummary('relay'),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.entitlementSummary).toBe(
      getSelfHostedPlanEntitlementSummary('pro'),
    );
    expect(getSelfHostedPlanEntitlementSummary('pro', 'Legacy Pulse Pro+')).toBe(
      'Legacy Pulse Pro+ is active on this instance. Root-cause analysis, safe remediation workflows, 90-day history, and admin/reporting extras are available right now.',
    );

    expect(SELF_HOSTED_PLAN_BY_TIER.relay.entitlementHighlights).toEqual(
      primaryPillarClaimLabels('relay', SELF_HOSTED_PLAN_BY_TIER.relay.metricHistoryDays),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.entitlementHighlights).toEqual(
      primaryPillarClaimLabels('pro', SELF_HOSTED_PLAN_BY_TIER.pro.metricHistoryDays),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.includedExtras).toEqual(
      getSelfHostedFeaturesForRole('pro', 'included_extra').map((entry) => entry.displayName),
    );

    const displayablePaidFeatures = SELF_HOSTED_FEATURE_CATALOG.filter(
      (entry) =>
        entry.displayableInSelfHostedPlan &&
        !entry.includedIn.community &&
        (entry.includedIn.relay || entry.includedIn.pro),
    ).map((entry) => entry.key);
    expect(displayablePaidFeatures).toEqual([
      'relay',
      'mobile_app',
      'push_notifications',
      'ai_alerts',
      'ai_autofix',
      'long_term_metrics',
      'rbac',
      'audit_logging',
      'advanced_reporting',
      'agent_profiles',
    ]);
    expect(
      SELF_HOSTED_PLAN_DEFINITIONS.map((plan) => ({
        tier: plan.tier,
        historyDays: plan.metricHistoryDays,
        extras: plan.billingExtrasSummary,
      })),
    ).toEqual([
      { tier: 'community', historyDays: 7, extras: 'Patrol, alerts, and SSO' },
      {
        tier: 'relay',
        historyDays: 14,
        extras: 'Remote web access, pairing, and push',
      },
      { tier: 'pro', historyDays: 90, extras: 'Analysis, remediation, and admin controls' },
    ]);
    expect(SELF_HOSTED_FEATURE_ROWS.find((row) => row.key === 'history')).toEqual({
      key: 'history',
      name: 'Metric History',
      community: '7 days',
      relay: '14 days',
      pro: '90 days',
    });

    const paidClaimText = [
      SELF_HOSTED_COMMERCIAL_PRESENTATION.pageDescription,
      SELF_HOSTED_PLAN_BY_TIER.relay.entitlementSummary,
      SELF_HOSTED_PLAN_BY_TIER.relay.comparisonSummary,
      ...SELF_HOSTED_PLAN_BY_TIER.relay.entitlementHighlights,
      ...SELF_HOSTED_PLAN_BY_TIER.relay.highlights,
      SELF_HOSTED_PLAN_BY_TIER.pro.entitlementSummary,
      SELF_HOSTED_PLAN_BY_TIER.pro.comparisonSummary,
      ...SELF_HOSTED_PLAN_BY_TIER.pro.entitlementHighlights,
      ...SELF_HOSTED_PLAN_BY_TIER.pro.includedExtras,
      ...SELF_HOSTED_PLAN_BY_TIER.pro.highlights,
    ].join('\n');

    for (const hiddenEntry of SELF_HOSTED_FEATURE_CATALOG.filter(
      (entry) => !entry.displayableInSelfHostedPlan,
    )) {
      expect(paidClaimText).not.toContain(hiddenEntry.displayName);
      expect(paidClaimText).not.toContain(hiddenEntry.comparisonName);
    }
    expect(paidClaimText).not.toMatch(
      /unlimited|monitoring capacity|guest capacity|hosted quickstart|trial/i,
    );
    expect(paidClaimText).not.toMatch(
      /yourlab\.pulserelay\.pro|custom\s+(?:url|subdomain|domain)/i,
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
