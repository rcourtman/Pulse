import { describe, expect, it } from 'vitest';
import { getActiveTabForPath } from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/unknown')).toBeNull();
    expect(getActiveTabForPath('/dashboard')).toBeNull();
    expect(getActiveTabForPath('/proxmox')).toBe('proxmox');
    expect(getActiveTabForPath('/proxmox/storage')).toBe('proxmox');
    expect(getActiveTabForPath('/infrastructure')).toBe('infrastructure');
    expect(getActiveTabForPath('/workloads?type=pod')).toBe('workloads');
    expect(getActiveTabForPath('/storage')).toBe('storage');
    expect(getActiveTabForPath('/ceph')).toBe('storage');
    expect(getActiveTabForPath('/recovery')).toBe('recovery');
    expect(getActiveTabForPath('/alerts/open')).toBe('alerts');
    expect(getActiveTabForPath('/patrol')).toBe('ai');
    expect(getActiveTabForPath('/ai')).toBe('ai');
    expect(getActiveTabForPath('/operations')).toBe('settings');
    expect(getActiveTabForPath('/operations/diagnostics')).toBe('settings');
    expect(getActiveTabForPath('/operations/logs')).toBe('settings');
    expect(getActiveTabForPath('/settings/security')).toBe('settings');
  });
});
