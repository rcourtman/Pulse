import { describe, expect, it } from 'vitest';
import {
  buildLegacyRedirectTarget,
  getActiveTabForPath,
  mergeRedirectQueryParams,
  readLegacyMigrationSource,
} from '../navigation';

describe('navigation routing helpers', () => {
  it('maps paths to the correct primary tab', () => {
    expect(getActiveTabForPath('/infrastructure')).toBe('infrastructure');
    expect(getActiveTabForPath('/workloads?type=k8s')).toBe('workloads');
    expect(getActiveTabForPath('/storage-v2')).toBe('storage-v2');
    expect(getActiveTabForPath('/storage')).toBe('storage');
    expect(getActiveTabForPath('/ceph')).toBe('storage');
    expect(getActiveTabForPath('/backups-v2')).toBe('backups-v2');
    expect(getActiveTabForPath('/backups')).toBe('backups');
    expect(getActiveTabForPath('/replication')).toBe('backups');
    expect(getActiveTabForPath('/kubernetes')).toBe('workloads');
    expect(getActiveTabForPath('/mail')).toBe('infrastructure');
    expect(getActiveTabForPath('/services')).toBe('infrastructure');
    expect(getActiveTabForPath('/alerts/open')).toBe('alerts');
    expect(getActiveTabForPath('/settings/security')).toBe('settings');
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

describe('alias path tab mapping (scheduled for consolidation)', () => {
  it('maps /storage-v2 to storage-v2 tab (alias, consolidates to storage in SB5-05)', () => {
    expect(getActiveTabForPath('/storage-v2')).toBe('storage-v2');
  });

  it('maps /backups-v2 to backups-v2 tab (alias, consolidates to backups in SB5-05)', () => {
    expect(getActiveTabForPath('/backups-v2')).toBe('backups-v2');
  });

  it('canonical paths /storage and /backups remain stable', () => {
    expect(getActiveTabForPath('/storage')).toBe('storage');
    expect(getActiveTabForPath('/backups')).toBe('backups');
  });
});
