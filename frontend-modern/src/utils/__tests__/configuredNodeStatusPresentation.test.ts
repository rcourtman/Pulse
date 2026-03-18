import { describe, expect, it } from 'vitest';
import {
  resolveConfiguredInstanceStatusIndicator,
  resolveConfiguredNodeStatusIndicator,
} from '@/utils/configuredNodeStatusPresentation';

describe('configuredNodeStatusPresentation', () => {
  it('resolves configured/live/connection state into canonical status indicators', () => {
    expect(
      resolveConfiguredNodeStatusIndicator({
        configuredStatus: 'connected',
      }),
    ).toMatchObject({ variant: 'success', label: 'Online' });

    expect(
      resolveConfiguredNodeStatusIndicator({
        configuredStatus: 'pending',
      }),
    ).toMatchObject({ variant: 'warning', label: 'Pending' });

    expect(
      resolveConfiguredNodeStatusIndicator({
        configuredStatus: 'connected',
        connectionHealth: 'degraded',
      }),
    ).toMatchObject({ variant: 'warning', label: 'Degraded' });

    expect(
      resolveConfiguredNodeStatusIndicator({
        configuredStatus: 'connected',
        connectionHealth: 'error',
      }),
    ).toMatchObject({ variant: 'danger', label: 'Offline' });
  });

  it('resolves PBS/PMG-like instance state through the shared helper', () => {
    expect(
      resolveConfiguredInstanceStatusIndicator(
        { name: 'pbs-1', status: 'connected' } as never,
        [{ name: 'pbs-1', status: 'online', connectionHealth: 'healthy' }],
      ),
    ).toMatchObject({ variant: 'success', label: 'Online' });

    expect(
      resolveConfiguredInstanceStatusIndicator(
        { name: 'pmg-1', status: 'connected' } as never,
        [{ name: 'pmg-1', status: 'offline', connectionHealth: 'error' }],
      ),
    ).toMatchObject({ variant: 'danger', label: 'Offline' });
  });
});
