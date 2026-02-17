import { describe, expect, it } from 'vitest';
import {
  buildLegacyRedirectTarget,
  getActiveTabForPath,
  mergeRedirectQueryParams,
  readLegacyMigrationSource,
} from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/dashboard', 'unified')).toBe('dashboard');
    expect(getActiveTabForPath('/infrastructure', 'unified')).toBe('infrastructure');
    expect(getActiveTabForPath('/workloads?type=k8s', 'unified')).toBe('workloads');
    expect(getActiveTabForPath('/storage', 'unified')).toBe('storage');
    expect(getActiveTabForPath('/ceph', 'unified')).toBe('storage');
    expect(getActiveTabForPath('/backups', 'unified')).toBe('backups');
    expect(getActiveTabForPath('/replication', 'unified')).toBe('backups');
    expect(getActiveTabForPath('/kubernetes', 'unified')).toBe('workloads');
    expect(getActiveTabForPath('/mail', 'unified')).toBe('infrastructure');
    expect(getActiveTabForPath('/services', 'unified')).toBe('infrastructure');
    expect(getActiveTabForPath('/alerts/open', 'unified')).toBe('alerts');
    expect(getActiveTabForPath('/settings/security', 'unified')).toBe('settings');
  });

  it('maps paths to the correct primary tab in classic navigation mode', () => {
    expect(getActiveTabForPath('/infrastructure', 'classic')).toBe('infrastructure');
    expect(getActiveTabForPath('/infrastructure?source=proxmox', 'classic')).toBe('proxmox');
    expect(getActiveTabForPath('/infrastructure?source=pve', 'classic')).toBe('proxmox');
    expect(getActiveTabForPath('/infrastructure?source=agent', 'classic')).toBe('hosts');
    expect(getActiveTabForPath('/infrastructure?source=docker', 'classic')).toBe('docker');
    expect(getActiveTabForPath('/infrastructure?source=pmg', 'classic')).toBe('services');
    expect(getActiveTabForPath('/infrastructure?source=proxmox,agent', 'classic')).toBe('infrastructure');

    expect(getActiveTabForPath('/workloads', 'classic')).toBe('workloads');
    expect(getActiveTabForPath('/workloads?type=docker', 'classic')).toBe('containers');
    expect(getActiveTabForPath('/workloads?type=k8s', 'classic')).toBe('kubernetes');

    expect(getActiveTabForPath('/kubernetes', 'classic')).toBe('kubernetes');
    expect(getActiveTabForPath('/servers', 'classic')).toBe('hosts');
    expect(getActiveTabForPath('/services', 'classic')).toBe('services');
  });

  it('appends migration metadata to legacy redirect targets', () => {
    expect(buildLegacyRedirectTarget('/infrastructure', 'hosts')).toBe(
      '/infrastructure?migrated=1&from=hosts',
    );
    expect(buildLegacyRedirectTarget('/workloads?type=k8s', 'kubernetes')).toBe(
      '/workloads?type=k8s&migrated=1&from=kubernetes',
    );
    expect(buildLegacyRedirectTarget('/infrastructure?source=docker', 'docker')).toBe(
      '/infrastructure?source=docker&migrated=1&from=docker',
    );
  });

  it('reads migration source from query parameters', () => {
    expect(readLegacyMigrationSource('?migrated=1&from=kubernetes')).toBe('kubernetes');
    expect(readLegacyMigrationSource('?migrated=1&from=mail')).toBe('mail');
    expect(readLegacyMigrationSource('?from=mail')).toBeNull();
    expect(readLegacyMigrationSource('?migrated=1&from=unknown')).toBeNull();
  });

  it('merges legacy route query params into redirect targets without overriding canonical params', () => {
    expect(
      mergeRedirectQueryParams(
        '/workloads?type=k8s&migrated=1&from=kubernetes',
        '?context=cluster-a&type=docker&migrated=0',
      ),
    ).toBe('/workloads?type=k8s&migrated=1&from=kubernetes&context=cluster-a');

    expect(
      mergeRedirectQueryParams('/infrastructure?source=pmg&migrated=1&from=services', '?search=mail'),
    ).toBe('/infrastructure?source=pmg&migrated=1&from=services&search=mail');
  });
});
