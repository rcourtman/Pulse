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
      runtime: '',
      context: 'cluster-a',
      host: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });

    expect(WORKLOADS_QUERY_PARAMS.type).toBe('type');
    expect(WORKLOADS_QUERY_PARAMS.runtime).toBe('runtime');
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
      provider: 'proxmox-pbs',
      cluster: 'cluster-main',
      namespace: 'tenant-a',
      mode: 'remote',
      status: 'failed',
      verification: 'verified',
      scope: 'workload',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });
    const url = new URL(href, 'http://localhost');
    expect(url.pathname).toBe('/backups');
    expect(url.searchParams.get('provider')).toBe('proxmox-pbs');
    expect(url.searchParams.get('cluster')).toBe('cluster-main');
    expect(url.searchParams.get('namespace')).toBe('tenant-a');
    expect(url.searchParams.get('mode')).toBe('remote');
    expect(url.searchParams.get('scope')).toBe('workload');
    expect(url.searchParams.get('status')).toBe('failed');
    expect(url.searchParams.get('verification')).toBe('verified');
    expect(url.searchParams.get('node')).toBe('cluster-main-pve1');
    expect(url.searchParams.get('q')).toBe('node:pve1');

    const parsed = parseBackupsLinkSearch(href.slice('/backups'.length));
    expect(parsed).toEqual({
      view: '',
      rollupId: '',
      provider: 'proxmox-pbs',
      cluster: 'cluster-main',
      namespace: 'tenant-a',
      mode: 'remote',
      scope: 'workload',
      status: 'failed',
      verification: 'verified',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });

    expect(BACKUPS_QUERY_PARAMS.provider).toBe('provider');
    expect(BACKUPS_QUERY_PARAMS.cluster).toBe('cluster');
    expect(BACKUPS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(BACKUPS_QUERY_PARAMS.mode).toBe('mode');
    expect(BACKUPS_QUERY_PARAMS.scope).toBe('scope');
    expect(BACKUPS_QUERY_PARAMS.verification).toBe('verification');
    expect(BACKUPS_QUERY_PARAMS.query).toBe('q');
    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });

});
