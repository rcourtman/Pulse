import { describe, expect, it } from 'vitest';

import { buildSelfHostedCommercialPlanModel } from '../commercialBillingModel';
import { SELF_HOSTED_PLAN_BY_TIER } from '../selfHostedPlans';

const createBaseInput = () => ({
  licensedEmail: 'owner@example.com',
  statusLabel: 'Active',
  tierLabel: 'Pro',
  planTerms: 'Pro Monthly',
  expires: '12/31/2026',
  daysRemaining: 123,
  monitoredSystemsSummary: 'Unlimited',
  capacityStatusSummary: 'Unlimited',
  maxMonitoredSystems: 'Unlimited' as const,
});

describe('commercialBillingModel', () => {
  it('uses shared current retail plan metadata without capacity entitlement copy', () => {
    const model = buildSelfHostedCommercialPlanModel({
      ...createBaseInput(),
      retailPlanDefinition: SELF_HOSTED_PLAN_BY_TIER.pro,
    });

    expect(model.summary).toEqual([
      { label: 'Core Monitoring', value: 'Included' },
      { label: 'Metric History', value: '90 days' },
      { label: 'Included Extras', value: 'Analysis, remediation, and admin controls' },
    ]);
    expect(model.details.map((item) => item.label)).toEqual([
      'Tier',
      'Licensed Email',
      'Plan Terms',
      'Expires',
      'Days Remaining',
    ]);
  });

  it('keeps grandfathered continuity focused on unmetered core monitoring', () => {
    const model = buildSelfHostedCommercialPlanModel({
      ...createBaseInput(),
      planTerms: 'V5 Pro Monthly (Grandfathered)',
      retailPlanDefinition: null,
    });

    expect(model.summary).toEqual([
      { label: 'Core Monitoring', value: 'Included' },
      { label: 'Plan Status', value: 'Active' },
    ]);
    expect(model.details.map((item) => item.label)).not.toContain('Included Monitored Systems');
    expect(model.summary.map((item) => item.label)).not.toContain('Guest Capacity');
  });

  it('keeps bounded monitored-system details on legacy fallback paths', () => {
    const model = buildSelfHostedCommercialPlanModel({
      ...createBaseInput(),
      monitoredSystemsSummary: '7 monitored systems',
      capacityStatusSummary: '3 remaining',
      maxMonitoredSystems: 10,
      retailPlanDefinition: null,
    });

    expect(model.summary).toEqual([
      { label: 'Monitored Systems', value: '7 monitored systems' },
      { label: 'Capacity Status', value: '3 remaining' },
      { label: 'Plan Status', value: 'Active' },
    ]);
    expect(model.details.map((item) => item.label)).toContain('Included Monitored Systems');
    expect(model.details.map((item) => item.label)).not.toContain('Guest Capacity');
  });
});
