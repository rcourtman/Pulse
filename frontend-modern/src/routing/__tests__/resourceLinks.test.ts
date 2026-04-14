import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  AI_PATROL_PATH,
  PMG_THRESHOLDS_PATH,
  PATROL_PATH,
  RECOVERY_QUERY_PARAMS,
  buildInfrastructureResourceLink,
  buildInfrastructureHrefForWorkload,
  buildRecoveryPath,
  buildRecoveryHrefForResource,
  buildInfrastructurePath,
  buildInfrastructureResourceHref,
  buildResolvedResourceSurfaceLinks,
  buildResourceSurfaceLinksForResource,
  buildStorageHrefForResource,
  buildStoragePath,
  buildWorkloadsHrefForResource,
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
  it('keeps Patrol links on the canonical Patrol route', () => {
    expect(PATROL_PATH).toBe('/patrol');
    expect(AI_PATROL_PATH).toBe(PATROL_PATH);
  });

  it('builds and parses workloads query params', () => {
    const href = buildWorkloadsPath({
      type: 'k8s',
      platform: 'kubernetes',
      context: 'cluster-a',
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });
    expect(href).toBe(
      '/workloads?type=pod&platform=kubernetes&context=cluster-a&agent=worker-1&resource=cluster-a%3Aworker-1%3A101',
    );

    const parsed = parseWorkloadsLinkSearch(href.slice('/workloads'.length));
    expect(parsed).toEqual({
      type: 'pod',
      platform: 'kubernetes',
      runtime: '',
      context: 'cluster-a',
      namespace: '',
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
      summaryGroup: '',
    });

    expect(WORKLOADS_QUERY_PARAMS.type).toBe('type');
    expect(WORKLOADS_QUERY_PARAMS.platform).toBe('platform');
    expect(WORKLOADS_QUERY_PARAMS.runtime).toBe('runtime');
    expect(WORKLOADS_QUERY_PARAMS.context).toBe('context');
    expect(WORKLOADS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(WORKLOADS_QUERY_PARAMS.agent).toBe('agent');
    expect(WORKLOADS_QUERY_PARAMS.resource).toBe('resource');
    expect(WORKLOADS_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes legacy workloads type aliases when building links', () => {
    expect(buildWorkloadsPath({ type: 'docker', platform: 'docker', agent: 'runtime-1' })).toBe(
      '/workloads?type=app-container&platform=docker&agent=runtime-1',
    );
    expect(buildWorkloadsPath({ type: 'kubernetes', platform: 'kubernetes', context: 'cluster-a' })).toBe(
      '/workloads?type=pod&platform=kubernetes&context=cluster-a',
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
      summaryGroup: '',
    });

    expect(INFRASTRUCTURE_QUERY_PARAMS.source).toBe('source');
    expect(INFRASTRUCTURE_QUERY_PARAMS.query).toBe('q');
    expect(INFRASTRUCTURE_QUERY_PARAMS.resource).toBe('resource');
    expect(INFRASTRUCTURE_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes infrastructure source aliases when building and parsing links', () => {
    expect(buildInfrastructurePath({ source: 'proxmox', query: 'pve1' })).toBe(
      '/infrastructure?source=proxmox-pve&q=pve1',
    );
    expect(parseInfrastructureLinkSearch('?source=pbs&q=archive')).toEqual({
      source: 'proxmox-pbs',
      query: 'archive',
      resource: '',
      summaryGroup: '',
    });
  });

  it('builds canonical infrastructure resource links', () => {
    expect(buildInfrastructureResourceHref(' resource-123 ')).toBe(
      '/infrastructure?resource=resource-123',
    );
    expect(buildInfrastructureResourceHref('')).toBeNull();
  });

  it('builds canonical infrastructure resource link metadata', () => {
    expect(buildInfrastructureResourceLink(' truenas-main ', 'TrueNAS Main')).toEqual({
      href: '/infrastructure?resource=truenas-main',
      label: 'Open in Infrastructure',
      compactLabel: 'Infrastructure',
      ariaLabel: 'Open related infrastructure for TrueNAS Main',
    });
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
        platformType: 'docker',
        contextLabel: 'docker-host-1',
      }),
    );
    expect(href).toBe('/infrastructure?source=docker&q=docker-host-1');
  });

  it('maps TrueNAS app-container workloads to the TrueNAS infrastructure source', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'app-container',
        workloadType: 'app-container',
        platformType: 'truenas',
        contextLabel: 'truenas-main',
      }),
    );
    expect(href).toBe('/infrastructure?source=truenas&q=truenas-main');
  });

  it('builds TrueNAS workloads links from canonical unified resources', () => {
    expect(
      buildWorkloadsHrefForResource({
        id: 'truenas-main',
        type: 'agent',
        name: 'truenas-main',
        displayName: 'TrueNAS Main',
        platformId: 'truenas-main',
        platformType: 'truenas',
        sourceType: 'hybrid',
        status: 'online',
        lastSeen: Date.now(),
      } as any),
    ).toBe('/workloads?type=app-container&platform=truenas&agent=truenas-main');
  });

  it('builds exact workloads links for TrueNAS app-container resources', () => {
    expect(
      buildWorkloadsHrefForResource({
        id: 'app-container:truenas-main:nextcloud',
        type: 'app-container',
        name: 'nextcloud',
        displayName: 'Nextcloud',
        parentId: 'truenas-main',
        platformId: 'truenas-main',
        platformType: 'truenas',
        sourceType: 'api',
        status: 'running',
        lastSeen: Date.now(),
      } as any),
    ).toBe(
      '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
    );
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
      summaryGroup: '',
    });

    expect(STORAGE_QUERY_PARAMS.tab).toBe('tab');
    expect(STORAGE_QUERY_PARAMS.group).toBe('group');
    expect(STORAGE_QUERY_PARAMS.query).toBe('q');
    expect(STORAGE_QUERY_PARAMS.resource).toBe('resource');
    expect(STORAGE_QUERY_PARAMS.sort).toBe('sort');
    expect(STORAGE_QUERY_PARAMS.order).toBe('order');
    expect(STORAGE_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('builds storage deep links for exact TrueNAS storage resources', () => {
    const href = buildStorageHrefForResource({
      id: 'storage-truenas-display',
      type: 'storage',
      name: 'tank',
      displayName: 'tank',
      platformId: 'truenas-1',
      platformType: 'truenas',
      sourceType: 'api',
      status: 'online',
      lastSeen: Date.now(),
      storage: { platform: 'truenas', type: 'zfs-pool' },
    } as any);

    expect(href).toBe('/storage?source=truenas&resource=storage-truenas-display');
  });

  it('builds storage deep links for TrueNAS physical disks on the disks tab', () => {
    const href = buildStorageHrefForResource({
      id: 'disk:truenas-main:sda',
      type: 'physical_disk',
      name: 'sda',
      displayName: 'Seagate IronWolf',
      parentId: 'truenas-main',
      platformId: 'truenas-main',
      platformType: 'truenas',
      sourceType: 'api',
      status: 'online',
      lastSeen: Date.now(),
      physicalDisk: {
        serial: '',
      },
    } as any);

    expect(href).toBe(
      '/storage?tab=disks&source=truenas&node=truenas-main&resource=disk%3Atruenas-main%3Asda',
    );
  });

  it('builds storage deep links for top-level TrueNAS systems', () => {
    const href = buildStorageHrefForResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformId: 'truenas-main',
      platformType: 'truenas',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: Date.now(),
    } as any);

    expect(href).toBe('/storage?source=truenas&node=truenas-main');
  });

  it('builds storage deep links for hybrid agent resources with merged truenas sources', () => {
    const href = buildStorageHrefForResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformId: 'truenas-main',
      platformType: 'agent',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: Date.now(),
      platformData: {
        sources: ['agent', 'truenas'],
      },
    } as any);

    expect(href).toBe('/storage?source=truenas&node=truenas-main');
  });

  it('builds recovery deep links for top-level TrueNAS systems', () => {
    const href = buildRecoveryHrefForResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformId: 'truenas-main',
      platformType: 'truenas',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: Date.now(),
    } as any);

    expect(href).toBe('/recovery?platform=truenas&node=truenas-main');
  });

  it('builds recovery deep links for hybrid agent resources with merged truenas sources', () => {
    const href = buildRecoveryHrefForResource({
      id: 'truenas-main',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'TrueNAS Main',
      platformId: 'truenas-main',
      platformType: 'agent',
      sourceType: 'hybrid',
      status: 'online',
      lastSeen: Date.now(),
      platformData: {
        sources: ['agent', 'truenas'],
      },
    } as any);

    expect(href).toBe('/recovery?platform=truenas&node=truenas-main');
  });

  it('builds canonical shared surface links for top-level truenas systems', () => {
    expect(
      buildResourceSurfaceLinksForResource(
        {
          id: 'truenas-main',
          type: 'agent',
          name: 'truenas-main',
          displayName: 'TrueNAS Main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          sourceType: 'hybrid',
          status: 'online',
          lastSeen: Date.now(),
          platformData: { sources: ['truenas'] },
        } as any,
        'TrueNAS Main',
      ),
    ).toEqual([
      {
        href: '/workloads?type=app-container&platform=truenas&agent=truenas-main',
        label: 'Open in Workloads',
        compactLabel: 'Workloads',
        ariaLabel: 'Open related workloads for TrueNAS Main',
      },
      {
        href: '/storage?source=truenas&node=truenas-main',
        label: 'Open in Storage',
        compactLabel: 'Storage',
        ariaLabel: 'Open related storage for TrueNAS Main',
      },
      {
        href: '/recovery?platform=truenas&node=truenas-main',
        label: 'Open in Recovery',
        compactLabel: 'Recovery',
        ariaLabel: 'Open related recovery for TrueNAS Main',
      },
    ]);
  });

  it('builds canonical surface links for exact TrueNAS app-container resources', () => {
    expect(
      buildResolvedResourceSurfaceLinks({
        resourceId: 'app-container:truenas-main:nextcloud',
        displayName: 'Nextcloud',
        resource: {
          id: 'app-container:truenas-main:nextcloud',
          type: 'app-container',
          name: 'nextcloud',
          displayName: 'Nextcloud',
          parentId: 'truenas-main',
          platformId: 'truenas-main',
          platformType: 'truenas',
          sourceType: 'api',
          status: 'running',
          lastSeen: Date.now(),
        } as any,
      }),
    ).toEqual([
      {
        href: '/infrastructure?resource=app-container%3Atruenas-main%3Anextcloud',
        label: 'Open in Infrastructure',
        compactLabel: 'Infrastructure',
        ariaLabel: 'Open related infrastructure for Nextcloud',
      },
      {
        href: '/workloads?type=app-container&platform=truenas&agent=truenas-main&resource=app-container%3Atruenas-main%3Anextcloud',
        label: 'Open in Workloads',
        compactLabel: 'Workloads',
        ariaLabel: 'Open related workloads for Nextcloud',
      },
    ]);
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
    const parsed = parseRecoveryLinkSearch('?provider=proxmox&mode=local');
    expect(parsed).toMatchObject({
      platform: 'proxmox-pve',
      mode: 'local',
    });
    expect(buildRecoveryPath(parsed)).toBe('/recovery?platform=proxmox-pve&mode=local');
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
