import { describe, expect, it } from 'vitest';
import {
  getUnifiedAgentLookupStatusPresentation,
  getUnifiedAgentStatusPresentation,
} from '@/utils/unifiedAgentStatusPresentation';

describe('unifiedAgentStatusPresentation', () => {
  it('returns canonical monitored-agent status presentation', () => {
    expect(getUnifiedAgentStatusPresentation('active', 'online')).toEqual({
      badgeClass: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
      label: 'online',
    });

    expect(getUnifiedAgentStatusPresentation('removed', 'offline')).toEqual({
      badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
      label: 'Monitoring stopped',
    });
  });

  it('returns canonical lookup status presentation', () => {
    expect(getUnifiedAgentLookupStatusPresentation(true)).toEqual({
      badgeClass: 'bg-green-100 text-green-700 dark:bg-green-900 dark:text-green-300',
      label: 'Connected',
    });

    expect(getUnifiedAgentLookupStatusPresentation(false)).toEqual({
      badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
      label: 'Not reporting yet',
    });
  });
});
