import { describe, expect, it } from 'vitest';
import {
  AGENTS_PATH,
  AGENTS_DEFAULT_TAB,
  AI_PATROL_PATH,
  DOCKER_PATH,
  KUBERNETES_PATH,
  PMG_THRESHOLDS_PATH,
  PATROL_PATH,
  PROXMOX_DEFAULT_TAB,
  PROXMOX_PATH,
  RECOVERY_QUERY_PARAMS,
  TRUENAS_PATH,
  VMWARE_PATH,
  buildAgentsPath,
  buildDockerPath,
  buildKubernetesPath,
  buildRecoveryPath,
  buildInfrastructurePath,
  buildProxmoxPath,
  buildStoragePath,
  buildTrueNASPath,
  buildVmwarePath,
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
  it('keeps Patrol links on the canonical Patrol route', () => {
    expect(PATROL_PATH).toBe('/patrol');
    expect(AI_PATROL_PATH).toBe(PATROL_PATH);
  });

  it('builds canonical Proxmox platform tab paths', () => {
    expect(PROXMOX_PATH).toBe('/proxmox');
    expect(PROXMOX_DEFAULT_TAB).toBe('overview');
    expect(buildProxmoxPath()).toBe('/proxmox/overview');
    expect(buildProxmoxPath('/storage/')).toBe('/proxmox/storage');
    expect(buildProxmoxPath('')).toBe('/proxmox');
  });

  it('builds canonical Agents, container runtime, Kubernetes, TrueNAS, and vSphere tab paths', () => {
    expect(AGENTS_PATH).toBe('/agents');
    expect(AGENTS_DEFAULT_TAB).toBe('overview');
    expect(buildAgentsPath()).toBe('/agents/overview');
    expect(buildAgentsPath('')).toBe('/agents');

    expect(DOCKER_PATH).toBe('/docker');
    expect(buildDockerPath()).toBe('/docker/overview');
    expect(buildDockerPath('containers')).toBe('/docker/containers');
    expect(buildDockerPath('')).toBe('/docker');

    expect(KUBERNETES_PATH).toBe('/kubernetes');
    expect(buildKubernetesPath()).toBe('/kubernetes/overview');
    expect(buildKubernetesPath('pods')).toBe('/kubernetes/pods');
    expect(buildKubernetesPath('deployments')).toBe('/kubernetes/deployments');
    expect(buildKubernetesPath('controllers')).toBe('/kubernetes/controllers');

    expect(TRUENAS_PATH).toBe('/truenas');
    expect(buildTrueNASPath()).toBe('/truenas/overview');
    expect(buildTrueNASPath('storage')).toBe('/truenas/storage');
    expect(buildTrueNASPath('services')).toBe('/truenas/services');
    expect(buildTrueNASPath('apps')).toBe('/truenas/apps');
    expect(buildTrueNASPath('vms')).toBe('/truenas/vms');
    expect(buildTrueNASPath('shares')).toBe('/truenas/shares');
    expect(buildTrueNASPath('protection')).toBe('/truenas/protection');

    expect(VMWARE_PATH).toBe('/vmware');
    expect(buildVmwarePath()).toBe('/vmware/overview');
    expect(buildVmwarePath('storage')).toBe('/vmware/storage');
    expect(buildVmwarePath('health')).toBe('/vmware/health');
    expect(buildVmwarePath('activity')).toBe('/vmware/activity');
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
    expect(
      buildWorkloadsPath({ type: 'kubernetes', platform: 'kubernetes', context: 'cluster-a' }),
    ).toBe('/workloads?type=pod&platform=kubernetes&context=cluster-a');
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

  it('builds and parses storage query params', () => {
    const href = buildStoragePath({
      tab: 'disks',
      group: 'storage',
      source: 'pbs',
      status: 'available',
      diskRole: 'nvme-disk',
      diskGroup: 'data',
      node: 'cluster-main-pve1',
      query: 'local-lvm',
      resource: 'storage-1',
      sort: 'usage',
      order: 'desc',
    });
    expect(href).toBe(
      '/storage?tab=disks&group=storage&source=proxmox-pbs&status=available&diskRole=nvme-disk&diskGroup=data&node=cluster-main-pve1&q=local-lvm&resource=storage-1&sort=usage&order=desc',
    );

    const parsed = parseStorageLinkSearch(href.slice('/storage'.length));
    expect(parsed).toEqual({
      tab: 'disks',
      group: 'storage',
      source: 'proxmox-pbs',
      status: 'available',
      diskRole: 'nvme-disk',
      diskGroup: 'data',
      node: 'cluster-main-pve1',
      query: 'local-lvm',
      resource: 'storage-1',
      sort: 'usage',
      order: 'desc',
      summaryGroup: '',
    });

    expect(STORAGE_QUERY_PARAMS.tab).toBe('tab');
    expect(STORAGE_QUERY_PARAMS.group).toBe('group');
    expect(STORAGE_QUERY_PARAMS.diskRole).toBe('diskRole');
    expect(STORAGE_QUERY_PARAMS.diskGroup).toBe('diskGroup');
    expect(STORAGE_QUERY_PARAMS.query).toBe('q');
    expect(STORAGE_QUERY_PARAMS.resource).toBe('resource');
    expect(STORAGE_QUERY_PARAMS.sort).toBe('sort');
    expect(STORAGE_QUERY_PARAMS.order).toBe('order');
    expect(STORAGE_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes legacy storage source aliases when parsing links', () => {
    expect(parseStorageLinkSearch('?source=pbs')).toMatchObject({ source: 'proxmox-pbs' });
    expect(parseStorageLinkSearch('?source=proxmox')).toMatchObject({ source: 'proxmox-pve' });
  });

  it('builds and parses recovery query params', () => {
    const href = buildRecoveryPath({
      view: 'events',
      platform: 'proxmox-pbs',
      state: 'stale',
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
    expect(url.searchParams.get('state')).toBe('stale');
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
      state: 'stale',
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
    expect(RECOVERY_QUERY_PARAMS.state).toBe('state');
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
