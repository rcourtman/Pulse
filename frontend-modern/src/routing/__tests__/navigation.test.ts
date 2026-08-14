import { describe, expect, it } from 'vitest';
import { getActiveTabForPath } from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/unknown')).toBeNull();
    expect(getActiveTabForPath('/dashboard')).toBeNull();
    expect(getActiveTabForPath('/dashboard/explore')).toBeNull();
    expect(getActiveTabForPath('/explore')).toBeNull();
    expect(getActiveTabForPath('/home')).toBeNull();
    expect(getActiveTabForPath('/standalone')).toBe('standalone');
    expect(getActiveTabForPath('/standalone/machines')).toBe('standalone');
    expect(getActiveTabForPath('/agents')).toBeNull();
    expect(getActiveTabForPath('/proxmox')).toBe('proxmox');
    expect(getActiveTabForPath('/proxmox/storage')).toBe('proxmox');
    // Infrastructure, Ceph, and aggregate workspace URLs are not standalone
    // tabs. Platform/runtime pages own those workflows.
    expect(getActiveTabForPath('/infrastructure')).toBeNull();
    expect(getActiveTabForPath('/workloads?type=pod')).toBeNull();
    expect(getActiveTabForPath('/storage')).toBeNull();
    expect(getActiveTabForPath('/ceph')).toBeNull();
    expect(getActiveTabForPath('/recovery')).toBeNull();
    expect(getActiveTabForPath('/alerts/open')).toBe('alerts');
    expect(getActiveTabForPath('/patrol')).toBe('ai');
    expect(getActiveTabForPath('/ai')).toBeNull();
    expect(getActiveTabForPath('/operations')).toBeNull();
    expect(getActiveTabForPath('/operations/diagnostics')).toBeNull();
    expect(getActiveTabForPath('/operations/logs')).toBeNull();
    expect(getActiveTabForPath('/settings/security')).toBe('settings');
  });

  it('keeps Activity history as a stable subordinate route identity', () => {
    // AppLayout presents this route beneath Patrol, while route identity stays
    // distinct for the page title, Assistant context, and durable action review.
    expect(getActiveTabForPath('/actions')).toBe('actions');
    expect(getActiveTabForPath('/actions/history')).toBe('actions');
  });
});
