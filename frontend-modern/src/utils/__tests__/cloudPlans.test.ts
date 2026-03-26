import { describe, expect, it } from 'vitest';

import {
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
    });
  });

  it('keeps non-founding tiers on their standard monthly display', () => {
    expect(getCloudPlanPricePresentation(CLOUD_PLAN_BY_TIER.power)).toEqual({
      monthlyPrice: '$49',
      cadence: '/month',
      annualSummary: 'or $449/year (save 24%)',
      compareAtMonthlyPrice: undefined,
    });
  });
});
