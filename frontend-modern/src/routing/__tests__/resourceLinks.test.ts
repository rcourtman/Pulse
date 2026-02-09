import { describe, expect, it } from 'vitest';
import {
  BACKUPS_QUERY_PARAMS,
  PMG_THRESHOLDS_PATH,
  buildBackupsPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
  parseBackupsLinkSearch,
  INFRASTRUCTURE_QUERY_PARAMS,
  parseStorageLinkSearch,
  parseInfrastructureLinkSearch,
  parseWorkloadsLinkSearch,
  STORAGE_QUERY_PARAMS,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';

describe('resource link routing contract', () => {
  it('builds and parses workloads query params', () => {
    const href = buildWorkloadsPath({
      type: 'k8s',
      context: 'cluster-a',
      host: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });
    expect(href).toBe('/workloads?type=k8s&context=cluster-a&host=worker-1&resource=cluster-a%3Aworker-1%3A101');

    const parsed = parseWorkloadsLinkSearch(href.slice('/workloads'.length));
    expect(parsed).toEqual({
      type: 'k8s',
      context: 'cluster-a',
      host: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });

    expect(WORKLOADS_QUERY_PARAMS.type).toBe('type');
    expect(WORKLOADS_QUERY_PARAMS.context).toBe('context');
    expect(WORKLOADS_QUERY_PARAMS.host).toBe('host');
    expect(WORKLOADS_QUERY_PARAMS.resource).toBe('resource');
  });

  it('builds and parses infrastructure query params', () => {
    const href = buildInfrastructurePath({
      source: 'docker',
      query: 'docker-host-1',
      resource: 'docker-host-1',
    });
    expect(href).toBe('/infrastructure?source=docker&q=docker-host-1&resource=docker-host-1');

    const parsed = parseInfrastructureLinkSearch(href.slice('/infrastructure'.length));
    expect(parsed).toEqual({
      source: 'docker',
      query: 'docker-host-1',
      resource: 'docker-host-1',
    });

    expect(INFRASTRUCTURE_QUERY_PARAMS.source).toBe('source');
    expect(INFRASTRUCTURE_QUERY_PARAMS.query).toBe('q');
    expect(INFRASTRUCTURE_QUERY_PARAMS.resource).toBe('resource');
  });

  it('supports legacy infrastructure search query param parsing', () => {
    expect(parseInfrastructureLinkSearch('?search=legacy-host')).toEqual({
      source: '',
      query: 'legacy-host',
      resource: '',
    });
  });

  it('builds and parses storage query params', () => {
    const href = buildStoragePath({
      tab: 'disks',
      group: 'storage',
      source: 'pbs',
      status: 'available',
      node: 'cluster-main-pve1',
      query: 'local-lvm',
      resource: 'storage-1',
      sort: 'usage',
      order: 'desc',
    });
    expect(href).toBe('/storage?tab=disks&group=storage&source=pbs&status=available&node=cluster-main-pve1&q=local-lvm&resource=storage-1&sort=usage&order=desc');

    const parsed = parseStorageLinkSearch(href.slice('/storage'.length));
    expect(parsed).toEqual({
      tab: 'disks',
      group: 'storage',
      source: 'pbs',
      status: 'available',
      node: 'cluster-main-pve1',
      query: 'local-lvm',
      resource: 'storage-1',
      sort: 'usage',
      order: 'desc',
    });

    expect(STORAGE_QUERY_PARAMS.tab).toBe('tab');
    expect(STORAGE_QUERY_PARAMS.group).toBe('group');
    expect(STORAGE_QUERY_PARAMS.query).toBe('q');
    expect(STORAGE_QUERY_PARAMS.resource).toBe('resource');
    expect(STORAGE_QUERY_PARAMS.sort).toBe('sort');
    expect(STORAGE_QUERY_PARAMS.order).toBe('order');
  });

  it('supports legacy storage search query param parsing', () => {
    expect(parseStorageLinkSearch('?search=ceph')).toEqual({
      tab: '',
      group: '',
      source: '',
      status: '',
      node: '',
      query: 'ceph',
      resource: '',
      sort: '',
      order: '',
    });
  });

  it('builds and parses backups query params', () => {
    const href = buildBackupsPath({
      guestType: 'vm',
      source: 'pbs',
      namespace: 'tenant-a',
      backupType: 'remote',
      status: 'verified',
      group: 'guest',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });
    expect(href).toBe('/backups?type=vm&source=pbs&namespace=tenant-a&backupType=remote&status=verified&group=guest&node=cluster-main-pve1&q=node%3Apve1');

    const parsed = parseBackupsLinkSearch(href.slice('/backups'.length));
    expect(parsed).toEqual({
      guestType: 'vm',
      source: 'pbs',
      namespace: 'tenant-a',
      backupType: 'remote',
      status: 'verified',
      group: 'guest',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });

    expect(BACKUPS_QUERY_PARAMS.guestType).toBe('type');
    expect(BACKUPS_QUERY_PARAMS.source).toBe('source');
    expect(BACKUPS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(BACKUPS_QUERY_PARAMS.backupType).toBe('backupType');
    expect(BACKUPS_QUERY_PARAMS.query).toBe('q');
    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });

  it('supports legacy backups search query param parsing', () => {
    expect(parseBackupsLinkSearch('?search=vm-101')).toEqual({
      guestType: '',
      source: '',
      namespace: '',
      backupType: '',
      status: '',
      group: '',
      node: '',
      query: 'vm-101',
    });
  });

});
