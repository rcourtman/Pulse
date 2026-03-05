import { describe, expect, it } from 'vitest';
import { getActiveTabForPath } from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/dashboard')).toBe('dashboard');
    expect(getActiveTabForPath('/infrastructure')).toBe('infrastructure');
    expect(getActiveTabForPath('/workloads?type=pod')).toBe('workloads');
    expect(getActiveTabForPath('/storage')).toBe('storage');
    expect(getActiveTabForPath('/ceph')).toBe('storage');
    expect(getActiveTabForPath('/recovery')).toBe('recovery');
    expect(getActiveTabForPath('/alerts/open')).toBe('alerts');
    expect(getActiveTabForPath('/ai')).toBe('ai');
    expect(getActiveTabForPath('/operations')).toBe('operations');
    expect(getActiveTabForPath('/operations/diagnostics')).toBe('operations');
    expect(getActiveTabForPath('/operations/logs')).toBe('operations');
    expect(getActiveTabForPath('/settings/security')).toBe('settings');
  });
});
