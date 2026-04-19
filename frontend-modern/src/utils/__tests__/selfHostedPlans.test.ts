import { describe, expect, it } from 'vitest';

import {
  getSelfHostedPlanDefinitionForBillingTier,
  SELF_HOSTED_COMMERCIAL_PRESENTATION,
  SELF_HOSTED_FEATURE_ROWS,
  SELF_HOSTED_PLAN_BY_TIER,
  SELF_HOSTED_PLAN_DEFINITIONS,
} from '../selfHostedPlans';

describe('selfHostedPlans', () => {
  it('keeps self-hosted plan limits aligned across tier cards and comparison rows', () => {
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
      name: 'Core Monitoring Scope',
      community: 'Unlimited',
      relay: 'Unlimited',
      pro: 'Unlimited',
    });
  });

  it('keeps shared self-hosted commercial copy in the common contract', () => {
    expect(SELF_HOSTED_COMMERCIAL_PRESENTATION).toEqual({
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
    });
  });

  it('keeps Community quickstart copy scoped to Patrol activation support', () => {
    expect(SELF_HOSTED_PLAN_BY_TIER.community.subline).toBe('Unlimited self-hosted monitoring');
    expect(SELF_HOSTED_PLAN_BY_TIER.community.billingExtrasSummary).toBe(
      'Patrol, alerts, and OIDC',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.billingExtrasSummary).toBe(
      'Remote access, mobile, and push',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.billingExtrasSummary).toBe(
      'AI operations and advanced admin',
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.community.highlights).toEqual(
      expect.arrayContaining([
        'Unlimited self-hosted monitoring',
        'Pulse Patrol (BYOK)',
        'Patrol quickstart after activation or trial: 25 runs, no API key',
      ]),
    );
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.highlights).toContain('14-day metric history');
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.highlights).toContain('Unlimited self-hosted monitoring');
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
    expect(getSelfHostedPlanDefinitionForBillingTier('relay')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.relay,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('pro')).toBe(
      SELF_HOSTED_PLAN_BY_TIER.pro,
    );
    expect(getSelfHostedPlanDefinitionForBillingTier('enterprise')).toBeNull();
  });
});
