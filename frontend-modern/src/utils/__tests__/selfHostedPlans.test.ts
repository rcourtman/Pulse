import { describe, expect, it } from 'vitest';

import {
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
      'Pro+',
    ]);

    expect(SELF_HOSTED_PLAN_BY_TIER.community.monitoredSystems).toBe(5);
    expect(SELF_HOSTED_PLAN_BY_TIER.relay.monitoredSystems).toBe(8);
    expect(SELF_HOSTED_PLAN_BY_TIER.pro.monitoredSystems).toBe(15);
    expect(SELF_HOSTED_PLAN_BY_TIER.proPlus.monitoredSystems).toBe(50);

    const hostsRow = SELF_HOSTED_FEATURE_ROWS.find((row) => row.key === 'hosts');
    expect(hostsRow).toEqual({
      key: 'hosts',
      name: 'Monitored Systems',
      community: '5',
      relay: '8',
      pro: '15',
      proPlus: '50',
    });
  });
});
