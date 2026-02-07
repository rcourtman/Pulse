import { describe, expect, it } from 'vitest';
import {
  BACKUPS_V2_PATH,
  BACKUPS_QUERY_PARAMS,
  PMG_THRESHOLDS_PATH,
  buildBackupsPath,
  buildBackupsV2Path,
  buildInfrastructurePath,
  buildStoragePath,
  buildStorageV2Path,
  buildWorkloadsPath,
  parseBackupsLinkSearch,
  INFRASTRUCTURE_QUERY_PARAMS,
  parseStorageLinkSearch,
  parseInfrastructureLinkSearch,
  STORAGE_V2_PATH,
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
    });
    expect(href).toBe('/storage?tab=disks&group=storage&source=pbs&status=available&node=cluster-main-pve1&q=local-lvm&resource=storage-1');

    const parsed = parseStorageLinkSearch(href.slice('/storage'.length));
    expect(parsed).toEqual({
      tab: 'disks',
      group: 'storage',
      source: 'pbs',
      status: 'available',
      node: 'cluster-main-pve1',
      query: 'local-lvm',
      resource: 'storage-1',
    });

    expect(STORAGE_QUERY_PARAMS.tab).toBe('tab');
    expect(STORAGE_QUERY_PARAMS.group).toBe('group');
    expect(STORAGE_QUERY_PARAMS.query).toBe('q');
    expect(STORAGE_QUERY_PARAMS.resource).toBe('resource');
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
    });
  });

  it('builds and parses backups query params', () => {
    const href = buildBackupsPath({
      guestType: 'vm',
      source: 'pbs',
      backupType: 'remote',
      status: 'verified',
      group: 'guest',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });
    expect(href).toBe('/backups?type=vm&source=pbs&backupType=remote&status=verified&group=guest&node=cluster-main-pve1&q=node%3Apve1');

    const parsed = parseBackupsLinkSearch(href.slice('/backups'.length));
    expect(parsed).toEqual({
      guestType: 'vm',
      source: 'pbs',
      backupType: 'remote',
      status: 'verified',
      group: 'guest',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });

    expect(BACKUPS_QUERY_PARAMS.guestType).toBe('type');
    expect(BACKUPS_QUERY_PARAMS.source).toBe('source');
    expect(BACKUPS_QUERY_PARAMS.backupType).toBe('backupType');
    expect(BACKUPS_QUERY_PARAMS.query).toBe('q');
    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });

  it('supports legacy backups search query param parsing', () => {
    expect(parseBackupsLinkSearch('?search=vm-101')).toEqual({
      guestType: '',
      source: '',
      backupType: '',
      status: '',
      group: '',
      node: '',
      query: 'vm-101',
    });
  });

  it('builds v2 storage and backups paths with compatible query contracts', () => {
    expect(
      buildStorageV2Path({
        source: 'kubernetes',
        status: 'healthy',
        query: 'pvc',
      }),
    ).toBe('/storage-v2?source=kubernetes&status=healthy&q=pvc');

    expect(
      buildBackupsV2Path({
        source: 'proxmox-pbs',
        status: 'success',
        query: 'vmid:101',
      }),
    ).toBe('/backups-v2?source=proxmox-pbs&status=success&q=vmid%3A101');

    expect(buildStorageV2Path()).toBe(STORAGE_V2_PATH);
    expect(buildBackupsV2Path()).toBe(BACKUPS_V2_PATH);
  });
});
