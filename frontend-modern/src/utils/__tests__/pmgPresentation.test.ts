import { describe, expect, it } from 'vitest';
import {
  getPMGDisconnectedState,
  getPMGSearchEmptyState,
  PMG_DISCONNECTED_STATE_TITLE,
  PMG_EMPTY_STATE_DESCRIPTION,
  PMG_EMPTY_STATE_TITLE,
  PMG_LOADING_STATE_DESCRIPTION,
  PMG_LOADING_STATE_TITLE,
  PMG_SEARCH_PLACEHOLDER,
} from '../pmgPresentation';

describe('pmgPresentation', () => {
  it('exports canonical PMG empty-state copy', () => {
    expect(PMG_EMPTY_STATE_TITLE).toBe('No Mail Gateways configured');
    expect(PMG_EMPTY_STATE_DESCRIPTION).toBe(
      'Add a Proxmox Mail Gateway via Settings → Infrastructure to start collecting mail analytics and security metrics.',
    );
    expect(PMG_EMPTY_STATE_DESCRIPTION).not.toContain('Settings → Infrastructure → Proxmox');
    expect(PMG_LOADING_STATE_TITLE).toBe('Loading mail gateway data...');
    expect(PMG_LOADING_STATE_DESCRIPTION).toBe('Connecting to the monitoring service.');
    expect(PMG_DISCONNECTED_STATE_TITLE).toBe('Connection lost');
    expect(PMG_SEARCH_PLACEHOLDER).toBe('Search gateways...');
  });

  it('exports canonical PMG disconnected and search state helpers', () => {
    expect(getPMGDisconnectedState(true)).toEqual({
      title: 'Connection lost',
      description: 'Attempting to reconnect…',
      actionLabel: undefined,
    });
    expect(getPMGDisconnectedState(false)).toEqual({
      title: 'Connection lost',
      description: 'Unable to connect to the backend server',
      actionLabel: 'Reconnect now',
    });
    expect(getPMGSearchEmptyState('edge')).toEqual({
      description: 'No gateways match "edge"',
      actionLabel: 'Clear search',
    });
  });
});
