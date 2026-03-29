import { describe, expect, it } from 'vitest';
import {
  getInfrastructureEmptyState,
  getInfrastructureFilterEmptyState,
  getInfrastructureLoadFailureState,
} from '@/utils/infrastructureEmptyStatePresentation';

describe('infrastructureEmptyStatePresentation', () => {
  it('returns the infrastructure onboarding empty state', () => {
    expect(getInfrastructureEmptyState()).toEqual({
      title: 'No infrastructure resources yet',
      description:
        'Start by opening Settings → Infrastructure → Install on a host and adding the first system you want Pulse to monitor. If you prefer an API-backed platform integration instead, use Platform connections.',
      actionLabel: 'Open Infrastructure Install',
    });
  });

  it('returns the filtered infrastructure empty state', () => {
    expect(getInfrastructureFilterEmptyState()).toEqual({
      title: 'No resources match filters',
      description: 'Try adjusting the search, source, or status filters.',
      actionLabel: 'Clear filters',
    });
  });

  it('returns the infrastructure load failure state', () => {
    expect(getInfrastructureLoadFailureState()).toEqual({
      title: 'Unable to load infrastructure',
      description: 'We couldn’t fetch unified resources. Check connectivity or retry.',
      actionLabel: 'Retry',
    });
  });
});
