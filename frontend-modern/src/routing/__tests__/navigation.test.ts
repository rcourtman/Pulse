import { describe, expect, it } from 'vitest';
import { getActiveTabForPath } from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/unknown')).toBeNull();
    expect(getActiveTabForPath('/dashboard')).toBeNull();
    expect(getActiveTabForPath('/agents')).toBe('agents');
    expect(getActiveTabForPath('/agents/overview')).toBe('agents');
    expect(getActiveTabForPath('/proxmox')).toBe('proxmox');
    expect(getActiveTabForPath('/proxmox/storage')).toBe('proxmox');
    // Legacy top-level routes (/infrastructure, /workloads, /storage,
    // /recovery, /ceph) were retired when primary nav moved to platform-first.
    expect(getActiveTabForPath('/infrastructure')).toBeNull();
    expect(getActiveTabForPath('/workloads?type=pod')).toBeNull();
    expect(getActiveTabForPath('/storage')).toBeNull();
    expect(getActiveTabForPath('/ceph')).toBeNull();
    expect(getActiveTabForPath('/recovery')).toBeNull();
    expect(getActiveTabForPath('/alerts/open')).toBe('alerts');
    expect(getActiveTabForPath('/patrol')).toBe('ai');
    expect(getActiveTabForPath('/ai')).toBe('ai');
    expect(getActiveTabForPath('/operations')).toBe('settings');
    expect(getActiveTabForPath('/operations/diagnostics')).toBe('settings');
    expect(getActiveTabForPath('/operations/logs')).toBe('settings');
    expect(getActiveTabForPath('/settings/security')).toBe('settings');
  });
});
