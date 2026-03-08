import { describe, expect, it } from 'vitest';
import { getAIProviderHealthPresentation } from '@/utils/aiProviderHealthPresentation';

describe('getAIProviderHealthPresentation', () => {
  it('returns canonical provider-health presentation for each status', () => {
    expect(getAIProviderHealthPresentation('ok')).toEqual({
      label: 'Healthy',
      badgeClass: 'bg-green-100 dark:bg-green-900 text-green-700 dark:text-green-300',
    });

    expect(getAIProviderHealthPresentation('error')).toEqual({
      label: 'Issue',
      badgeClass: 'bg-red-100 dark:bg-red-900 text-red-700 dark:text-red-300',
    });

    expect(getAIProviderHealthPresentation('checking')).toEqual({
      label: 'Checking...',
      badgeClass: 'bg-blue-100 dark:bg-blue-900 text-blue-700 dark:text-blue-300',
    });

    expect(getAIProviderHealthPresentation('not_configured')).toEqual({
      label: 'Not checked',
      badgeClass: 'bg-surface-hover text-muted',
    });
  });
});
