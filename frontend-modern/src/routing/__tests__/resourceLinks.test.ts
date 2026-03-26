import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  PMG_THRESHOLDS_PATH,
  RECOVERY_QUERY_PARAMS,
  buildInfrastructureHrefForWorkload,
  buildRecoveryPath,
  buildInfrastructurePath,
  buildInfrastructureResourceHref,
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

const baseGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: 'guest-1',
  vmid: 101,
  name: 'guest-1',
  node: 'node-1',
  instance: 'cluster-a',
  status: 'running',
  type: 'vm',
  cpu: 0,
  cpus: 2,
  memory: { total: 0, used: 0, free: 0, usage: 0 },
  disk: { total: 0, used: 0, free: 0, usage: 0 },
  networkIn: 0,
  networkOut: 0,
  diskRead: 0,
  diskWrite: 0,
  uptime: 0,
  template: false,
  lastBackup: 0,
  tags: [],
  lock: '',
  lastSeen: new Date().toISOString(),
  ...overrides,
});

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

  it('canonicalizes infrastructure source aliases when building and parsing links', () => {
    expect(buildInfrastructurePath({ source: 'proxmox', query: 'pve1' })).toBe(
      '/infrastructure?source=proxmox-pve&q=pve1',
    );
    expect(parseInfrastructureLinkSearch('?source=pbs&q=archive')).toEqual({
      source: 'proxmox-pbs',
      query: 'archive',
      resource: '',
    });
  });

  it('builds canonical infrastructure resource links', () => {
    expect(buildInfrastructureResourceHref(' resource-123 ')).toBe(
      '/infrastructure?resource=resource-123',
    );
    expect(buildInfrastructureResourceHref('')).toBeNull();
  });

  it('maps vm workloads to proxmox infrastructure source with node query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'vm',
        workloadType: 'vm',
        node: 'pve1',
        instance: 'cluster-main',
      }),
    );
    expect(href).toBe('/infrastructure?source=proxmox-pve&q=pve1');
  });

  it('maps app-container workloads to docker infrastructure source with context query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'app-container',
        workloadType: 'app-container',
        contextLabel: 'docker-host-1',
      }),
    );
    expect(href).toBe('/infrastructure?source=docker&q=docker-host-1');
  });

  it('maps pod workloads to kubernetes infrastructure source with cluster query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'pod',
        workloadType: 'pod',
        contextLabel: 'cluster-a',
      }),
    );
    expect(href).toBe('/infrastructure?source=kubernetes&q=cluster-a');
  });

  it('defaults unknown workload types to proxmox infrastructure compatibility mapping', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'unknown',
        workloadType: undefined,
      }),
    );
    expect(href).toBe('/infrastructure?source=proxmox-pve&q=node-1');
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
      '/storage?tab=disks&group=storage&source=proxmox-pbs&status=available&node=cluster-main-pve1&q=local-lvm&resource=storage-1&sort=usage&order=desc',
    );

    const parsed = parseStorageLinkSearch(href.slice('/storage'.length));
    expect(parsed).toEqual({
      tab: 'disks',
      group: 'storage',
      source: 'proxmox-pbs',
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

  it('canonicalizes legacy storage source aliases when parsing links', () => {
    expect(parseStorageLinkSearch('?source=pbs')).toMatchObject({ source: 'proxmox-pbs' });
    expect(parseStorageLinkSearch('?source=proxmox')).toMatchObject({ source: 'proxmox-pve' });
  });

  it('builds and parses recovery query params', () => {
    const href = buildRecoveryPath({
      view: 'events',
      platform: 'proxmox-pbs',
      stale: '1',
      range: '7',
      cluster: 'cluster-main',
      day: '2026-02-13',
      namespace: 'tenant-a',
      mode: 'remote',
      itemType: 'vm',
      status: 'failed',
      verification: 'verified',
      scope: 'workload',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });
    const url = new URL(href, 'http://localhost');
    expect(url.pathname).toBe('/recovery');
    expect(url.searchParams.get('view')).toBe('events');
    expect(url.searchParams.get('platform')).toBe('proxmox-pbs');
    expect(url.searchParams.get('stale')).toBe('1');
    expect(url.searchParams.get('range')).toBe('7');
    expect(url.searchParams.get('cluster')).toBe('cluster-main');
    expect(url.searchParams.get('day')).toBe('2026-02-13');
    expect(url.searchParams.get('namespace')).toBe('tenant-a');
    expect(url.searchParams.get('mode')).toBe('remote');
    expect(url.searchParams.get('itemType')).toBe('vm');
    expect(url.searchParams.get('scope')).toBe('workload');
    expect(url.searchParams.get('status')).toBe('failed');
    expect(url.searchParams.get('verification')).toBe('verified');
    expect(url.searchParams.get('node')).toBe('cluster-main-pve1');
    expect(url.searchParams.get('q')).toBe('node:pve1');

    const parsed = parseRecoveryLinkSearch(href.slice('/recovery'.length));
    expect(parsed).toEqual({
      rollupId: '',
      view: 'events',
      platform: 'proxmox-pbs',
      stale: '1',
      range: '7',
      cluster: 'cluster-main',
      day: '2026-02-13',
      namespace: 'tenant-a',
      mode: 'remote',
      itemType: 'vm',
      scope: 'workload',
      status: 'failed',
      verification: 'verified',
      node: 'cluster-main-pve1',
      query: 'node:pve1',
    });

    expect(RECOVERY_QUERY_PARAMS.platform).toBe('platform');
    expect(RECOVERY_QUERY_PARAMS.view).toBe('view');
    expect(RECOVERY_QUERY_PARAMS.stale).toBe('stale');
    expect(RECOVERY_QUERY_PARAMS.range).toBe('range');
    expect(RECOVERY_QUERY_PARAMS.cluster).toBe('cluster');
    expect(RECOVERY_QUERY_PARAMS.day).toBe('day');
    expect(RECOVERY_QUERY_PARAMS.namespace).toBe('namespace');
    expect(RECOVERY_QUERY_PARAMS.mode).toBe('mode');
    expect(RECOVERY_QUERY_PARAMS.itemType).toBe('itemType');
    expect(RECOVERY_QUERY_PARAMS.scope).toBe('scope');
    expect(RECOVERY_QUERY_PARAMS.verification).toBe('verification');
    expect(RECOVERY_QUERY_PARAMS.query).toBe('q');

    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });

  it('canonicalizes recovery platform aliases when building and parsing links', () => {
    expect(buildRecoveryPath({ platform: 'pbs', mode: 'remote' })).toBe(
      '/recovery?platform=proxmox-pbs&mode=remote',
    );
    expect(parseRecoveryLinkSearch('?provider=proxmox&mode=local')).toMatchObject({
      platform: 'proxmox-pve',
      mode: 'local',
    });
    expect(parseRecoveryLinkSearch('?itemType=proxmox-vm')).toMatchObject({
      itemType: 'vm',
    });
  });

  it('canonicalizes stale-only recovery route flags to the owned query shape', () => {
    expect(buildRecoveryPath({ stale: 'true', platform: 'proxmox-pve' })).toBe(
      '/recovery?platform=proxmox-pve&stale=1',
    );
    expect(parseRecoveryLinkSearch('?stale=%201%20')).toMatchObject({ stale: '1' });
  });

  it('preserves explicit recovery chart range values in route state', () => {
    const href = buildRecoveryPath({ range: '30', platform: 'proxmox-pve' });
    const url = new URL(href, 'http://localhost');
    expect(url.pathname).toBe('/recovery');
    expect(url.searchParams.get('platform')).toBe('proxmox-pve');
    expect(url.searchParams.get('range')).toBe('30');
    expect(parseRecoveryLinkSearch('?range=90')).toMatchObject({ range: '90' });
  });
});
