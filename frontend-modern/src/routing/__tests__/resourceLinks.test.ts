import { describe, expect, it } from 'vitest';
import {
  PMG_THRESHOLDS_PATH,
  RECOVERY_QUERY_PARAMS,
  buildRecoveryPath,
  buildInfrastructurePath,
  buildStoragePath,
  buildWorkloadsPath,
  parseRecoveryLinkSearch,
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
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });
    expect(href).toBe(
      '/workloads?type=pod&context=cluster-a&agent=worker-1&resource=cluster-a%3Aworker-1%3A101',
    );

    const parsed = parseWorkloadsLinkSearch(href.slice('/workloads'.length));
    expect(parsed).toEqual({
      type: 'pod',
      runtime: '',
      context: 'cluster-a',
      namespace: '',
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });

    expect(WORKLOADS_QUERY_PARAMS.type).toBe('type');
    expect(WORKLOADS_QUERY_PARAMS.runtime).toBe('runtime');
    expect(WORKLOADS_QUERY_PARAMS.context).toBe('context');
    expect(WORKLOADS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(WORKLOADS_QUERY_PARAMS.agent).toBe('agent');
    expect(WORKLOADS_QUERY_PARAMS.resource).toBe('resource');
  });

  it('canonicalizes legacy workloads type aliases when building links', () => {
    expect(buildWorkloadsPath({ type: 'docker', agent: 'runtime-1' })).toBe(
      '/workloads?type=app-container&agent=runtime-1',
    );
    expect(buildWorkloadsPath({ type: 'kubernetes', context: 'cluster-a' })).toBe(
      '/workloads?type=pod&context=cluster-a',
    );
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
    expect(href).toBe(
      '/storage?tab=disks&group=storage&source=pbs&status=available&node=cluster-main-pve1&q=local-lvm&resource=storage-1&sort=usage&order=desc',
    );

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

  it('builds and parses recovery query params', () => {
    const href = buildRecoveryPath({
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
    expect(url.pathname).toBe('/recovery');
    expect(url.searchParams.get('provider')).toBe('proxmox-pbs');
    expect(url.searchParams.get('cluster')).toBe('cluster-main');
    expect(url.searchParams.get('namespace')).toBe('tenant-a');
    expect(url.searchParams.get('mode')).toBe('remote');
    expect(url.searchParams.get('scope')).toBe('workload');
    expect(url.searchParams.get('status')).toBe('failed');
    expect(url.searchParams.get('verification')).toBe('verified');
    expect(url.searchParams.get('node')).toBe('cluster-main-pve1');
    expect(url.searchParams.get('q')).toBe('node:pve1');

    const parsed = parseRecoveryLinkSearch(href.slice('/recovery'.length));
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

    expect(RECOVERY_QUERY_PARAMS.provider).toBe('provider');
    expect(RECOVERY_QUERY_PARAMS.cluster).toBe('cluster');
    expect(RECOVERY_QUERY_PARAMS.namespace).toBe('namespace');
    expect(RECOVERY_QUERY_PARAMS.mode).toBe('mode');
    expect(RECOVERY_QUERY_PARAMS.scope).toBe('scope');
    expect(RECOVERY_QUERY_PARAMS.verification).toBe('verification');
    expect(RECOVERY_QUERY_PARAMS.query).toBe('q');

    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });
});
