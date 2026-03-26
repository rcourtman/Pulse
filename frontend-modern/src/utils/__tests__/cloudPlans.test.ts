import { describe, expect, it } from 'vitest';

import {
  CLOUD_COMMERCIAL_PRESENTATION,
  CLOUD_PLAN_BY_TIER,
  getCloudPlanPricePresentation,
} from '@/utils/cloudPlans';

describe('cloudPlans', () => {
  it('keeps founding-rate display in the shared cloud pricing contract', () => {
    expect(getCloudPlanPricePresentation(CLOUD_PLAN_BY_TIER.starter)).toEqual({
      monthlyPrice: '$19',
      cadence: '/month',
      annualSummary: 'or $249/year (save 29%)',
      compareAtMonthlyPrice: '$29',
      campaignBadge: 'Founding rate',
    });
  });

  it('keeps non-founding tiers on their standard monthly display', () => {
    expect(getCloudPlanPricePresentation(CLOUD_PLAN_BY_TIER.power)).toEqual({
      monthlyPrice: '$49',
      cadence: '/month',
      annualSummary: 'or $449/year (save 24%)',
      compareAtMonthlyPrice: undefined,
      campaignBadge: undefined,
    });
  });

  it('keeps shared cloud commercial copy in the common contract', () => {
    expect(CLOUD_COMMERCIAL_PRESENTATION).toEqual({
      pageTitle: 'Pulse Cloud',
      pageDescription: 'Managed Pulse hosting with Pro features included.',
      includedInAllHeading: 'Included in every Cloud plan',
      includedInAllItems: [
        'All Pro features',
        'Managed hosting',
        'Daily backups',
        'Secure agent connectivity via Relay',
        'Mobile app access and push notifications',
        'Dedicated workspace URL',
      ],
      setupHeading: 'Setup',
      setupSteps: [
        'Create your workspace. No credit card is required for the trial.',
        'Install the Pulse agent on any Linux machine.',
        'Connect systems, review findings, and configure alerts.',
      ],
    });
  });
});
