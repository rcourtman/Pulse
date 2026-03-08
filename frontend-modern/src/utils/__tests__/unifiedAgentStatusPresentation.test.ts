import { describe, expect, it } from 'vitest';
import {
  ALLOW_RECONNECT_LABEL,
  getUnifiedAgentStatusPresentation,
  MONITORING_STOPPED_STATUS_LABEL,
} from '@/utils/unifiedAgentStatusPresentation';

describe('unifiedAgentStatusPresentation', () => {
  it('exports canonical unified-agent status labels', () => {
    expect(MONITORING_STOPPED_STATUS_LABEL).toBe('Monitoring stopped');
    expect(ALLOW_RECONNECT_LABEL).toBe('Allow reconnect');
  });

  it('maps removed monitoring state to the stopped presentation', () => {
    expect(getUnifiedAgentStatusPresentation('removed', 'offline')).toEqual({
      badgeClass: 'bg-amber-100 text-amber-800 dark:bg-amber-900 dark:text-amber-200',
      label: 'Monitoring stopped',
    });
  });

  it('maps connected health states to the success presentation', () => {
    expect(getUnifiedAgentStatusPresentation('active', 'healthy')).toEqual({
      badgeClass: 'bg-green-100 text-green-800 dark:bg-green-900 dark:text-green-300',
      label: 'healthy',
    });
  });

  it('falls back unknown active states to a muted presentation', () => {
    expect(getUnifiedAgentStatusPresentation('active', 'degraded')).toEqual({
      badgeClass: 'bg-surface-alt text-base-content',
      label: 'degraded',
    });
  });
});
