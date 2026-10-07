import { describe, expect, it } from 'vitest';
import {
  buildPatrolAttentionPath,
  API_TOKEN_CREATE_ANCHOR,
  API_TOKEN_PRESET_QUERY_PARAM,
  DOCKER_PATH,
  DOCKER_QUERY_PARAMS,
  EXTERNAL_AGENT_SETUP_ANCHOR,
  EXTERNAL_AGENT_SETUP_PATH,
  KUBERNETES_PATH,
  KUBERNETES_QUERY_PARAMS,
  PMG_THRESHOLDS_PATH,
  PATROL_AUTONOMY_OPERATIONS_LOOP_PATH,
  PATROL_CONTROL_ANCHOR,
  PATROL_CONTROL_PATH,
  PATROL_CONTROL_PATH_WITH_STARTER,
  PATROL_CONTROL_STARTER,
  PATROL_CONTROL_STARTER_QUERY_PARAM,
  PATROL_CONTROL_OPERATIONS_LOOP_PATH,
  PATROL_OPERATIONS_LOOP_ANCHOR,
  PATROL_OPERATIONS_LOOP_PATROL_AUTONOMY_STARTER,
  PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
  PATROL_OPERATIONS_LOOP_PRO_ACTIVATION_STARTER,
  PATROL_OPERATIONS_LOOP_PATH,
  PATROL_OPERATIONS_LOOP_STARTER_QUERY_PARAM,
  PATROL_PRO_ACTIVATION_OPERATIONS_LOOP_PATH,
  PATROL_PATH,
  PULSE_MCP_SETUP_ANCHOR,
  PULSE_MCP_LEGACY_SETUP_PATH,
  PULSE_MCP_SETUP_PATH,
  PULSE_MCP_TOKEN_SETUP_PATH,
  PULSE_INTELLIGENCE_AGENT_TOKEN_PRESET,
  SETTINGS_PULSE_INTELLIGENCE_ASSISTANT_PATH,
  SETTINGS_PULSE_INTELLIGENCE_DISCOVERY_PATH,
  SETTINGS_PULSE_INTELLIGENCE_PATH,
  isExternalAgentSetupHash,
  PROXMOX_BACKUPS_QUERY_PARAMS,
  PROXMOX_BACKUPS_PATH,
  PROXMOX_BACKUPS_DEFAULT_VIEW,
  PROXMOX_DEFAULT_TAB,
  PROXMOX_PATH,
  SETTINGS_API_ACCESS_PATH,
  STANDALONE_DEFAULT_TAB,
  STANDALONE_PATH,
  STANDALONE_QUERY_PARAMS,
  TRUENAS_PATH,
  VMWARE_PATH,
  buildPatrolControlPath,
  buildPatrolOperationsLoopPath,
  buildDockerPath,
  buildDockerRouteSearch,
  buildKubernetesPath,
  buildProxmoxPath,
  buildProxmoxBackupsPath,
  buildStandalonePath,
  buildStorageRouteSearch,
  buildTrueNASPath,
  buildVmwarePath,
  buildWorkloadsRouteSearch,
  parsePatrolAttentionItemId,
  parsePatrolControlStarter,
  parsePatrolOperationsLoopStarter,
  parseStorageLinkSearch,
  parseWorkloadsLinkSearch,
  STORAGE_QUERY_PARAMS,
  WORKLOADS_QUERY_PARAMS,
} from '@/routing/resourceLinks';
import * as resourceLinks from '@/routing/resourceLinks';

describe('resource link routing contract', () => {
  it('keeps Patrol links on the canonical Patrol route', () => {
    expect(PATROL_PATH).toBe('/patrol');
    expect(PATROL_CONTROL_ANCHOR).toBe('patrol-control');
    expect(PATROL_CONTROL_STARTER_QUERY_PARAM).toBe('patrolControlStarter');
    expect(PATROL_CONTROL_STARTER).toBe('patrol_control');
    expect(PATROL_CONTROL_PATH).toBe('/patrol#patrol-control');
    expect(PATROL_CONTROL_PATH_WITH_STARTER).toBe(
      '/patrol?patrolControlStarter=patrol_control#patrol-control',
    );
    expect(PATROL_OPERATIONS_LOOP_ANCHOR).toBe('operations-loop');
    expect(PATROL_OPERATIONS_LOOP_STARTER_QUERY_PARAM).toBe('operationsLoopStarter');
    expect(PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER).toBe('patrol_control');
    expect(PATROL_OPERATIONS_LOOP_PATROL_AUTONOMY_STARTER).toBe('patrol_autonomy');
    expect(PATROL_OPERATIONS_LOOP_PRO_ACTIVATION_STARTER).toBe('pulse_pro_activation');
    expect(PATROL_OPERATIONS_LOOP_PATH).toBe(PATROL_CONTROL_PATH);
    expect(PATROL_CONTROL_OPERATIONS_LOOP_PATH).toBe(PATROL_CONTROL_PATH_WITH_STARTER);
    expect(PATROL_AUTONOMY_OPERATIONS_LOOP_PATH).toBe(
      '/patrol?patrolControlStarter=patrol_control#patrol-control',
    );
    expect(PATROL_PRO_ACTIVATION_OPERATIONS_LOOP_PATH).toBe(
      '/patrol?patrolControlStarter=patrol_control#patrol-control',
    );
    expect(
      buildPatrolControlPath({
        starter: PATROL_CONTROL_STARTER,
      }),
    ).toBe(PATROL_CONTROL_PATH_WITH_STARTER);
    expect(
      buildPatrolControlPath({
        starter: PATROL_OPERATIONS_LOOP_PRO_ACTIVATION_STARTER,
      }),
    ).toBe(PATROL_CONTROL_PATH_WITH_STARTER);
    expect(
      buildPatrolControlPath({
        starter: null,
      }),
    ).toBe(PATROL_CONTROL_PATH);
    expect(parsePatrolControlStarter('?patrolControlStarter=patrol_control')).toBe(
      PATROL_CONTROL_STARTER,
    );
    expect(
      buildPatrolOperationsLoopPath({
        starter: PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
      }),
    ).toBe(PATROL_CONTROL_OPERATIONS_LOOP_PATH);
    expect(
      buildPatrolOperationsLoopPath({
        starter: PATROL_OPERATIONS_LOOP_PRO_ACTIVATION_STARTER,
      }),
    ).toBe(PATROL_PRO_ACTIVATION_OPERATIONS_LOOP_PATH);
    expect(parsePatrolOperationsLoopStarter('?operationsLoopStarter=patrol_control')).toBe(
      PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
    );
    expect(parsePatrolOperationsLoopStarter('?operationsLoopStarter=patrol_autonomy')).toBe(
      PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
    );
    expect(parsePatrolOperationsLoopStarter('?operationsLoopStarter=pulse_pro_activation')).toBe(
      PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
    );
    expect(parsePatrolOperationsLoopStarter('?patrolControlStarter=patrol_control')).toBe(
      PATROL_OPERATIONS_LOOP_PATROL_CONTROL_STARTER,
    );
    expect(parsePatrolOperationsLoopStarter('?operationsLoopStarter=pulse_patrol')).toBe('');
  });

  it('keeps external agent setup in Pulse Intelligence while token creation stays in API Access', () => {
    expect(SETTINGS_API_ACCESS_PATH).toBe('/settings/security/api');
    expect(SETTINGS_PULSE_INTELLIGENCE_PATH).toBe('/settings/pulse-intelligence');
    expect(SETTINGS_PULSE_INTELLIGENCE_ASSISTANT_PATH).toBe(
      '/settings/pulse-intelligence/assistant',
    );
    expect(SETTINGS_PULSE_INTELLIGENCE_DISCOVERY_PATH).toBe(
      '/settings/pulse-intelligence/discovery',
    );
    expect(EXTERNAL_AGENT_SETUP_ANCHOR).toBe('external-agent-setup');
    expect(EXTERNAL_AGENT_SETUP_PATH).toBe(
      '/settings/pulse-intelligence/assistant#external-agent-setup',
    );
    expect(PULSE_MCP_SETUP_ANCHOR).toBe('pulse-mcp-setup');
    expect(PULSE_MCP_SETUP_PATH).toBe(EXTERNAL_AGENT_SETUP_PATH);
    expect(PULSE_MCP_LEGACY_SETUP_PATH).toBe('/settings/security/api#pulse-mcp-setup');
    expect(isExternalAgentSetupHash('#external-agent-setup')).toBe(true);
    expect(isExternalAgentSetupHash('#pulse-mcp-setup')).toBe(true);
    expect(isExternalAgentSetupHash('#api-token-create')).toBe(false);
    expect(API_TOKEN_CREATE_ANCHOR).toBe('api-token-create');
    expect(API_TOKEN_PRESET_QUERY_PARAM).toBe('tokenPreset');
    expect(PULSE_INTELLIGENCE_AGENT_TOKEN_PRESET).toBe('pulse-intelligence-agent');
    expect(PULSE_MCP_TOKEN_SETUP_PATH).toBe(
      '/settings/security/api?tokenPreset=pulse-intelligence-agent#api-token-create',
    );
  });

  it('builds canonical Proxmox platform tab paths', () => {
    expect(PROXMOX_PATH).toBe('/proxmox');
    expect(PROXMOX_DEFAULT_TAB).toBe('overview');
    expect(PROXMOX_BACKUPS_PATH).toBe('/proxmox/backups');
    expect(PROXMOX_BACKUPS_DEFAULT_VIEW).toBe('date');
    expect(PROXMOX_BACKUPS_QUERY_PARAMS).toEqual({
      query: 'q',
      view: 'view',
      node: 'node',
      type: 'type',
      source: 'source',
      location: 'location',
      posture: 'posture',
      day: 'day',
    });
    expect(buildProxmoxPath()).toBe('/proxmox/overview');
    expect(buildProxmoxPath('/storage/')).toBe('/proxmox/storage');
    expect(buildProxmoxPath('')).toBe('/proxmox');
    expect(buildProxmoxBackupsPath()).toBe('/proxmox/backups/date');
    expect(buildProxmoxBackupsPath('coverage')).toBe('/proxmox/backups/coverage');
  });

  it('builds canonical Machines, container runtime, Kubernetes, TrueNAS, and vSphere tab paths', () => {
    expect(STANDALONE_PATH).toBe('/standalone');
    expect(STANDALONE_DEFAULT_TAB).toBe('machines');
    expect(STANDALONE_QUERY_PARAMS).toEqual({ query: 'q', status: 'status', view: 'view' });
    expect(buildStandalonePath()).toBe('/standalone/machines');
    expect(buildStandalonePath('')).toBe('/standalone');

    expect(DOCKER_PATH).toBe('/docker');
    expect(buildDockerPath()).toBe('/docker/overview');
    expect(buildDockerPath('containers')).toBe('/docker/containers');
    expect(buildDockerPath('')).toBe('/docker');
    expect(DOCKER_QUERY_PARAMS.host).toBe('host');
    expect(buildDockerRouteSearch({ host: 'frigate.mist-stork.ts.net' })).toBe(
      '?host=frigate.mist-stork.ts.net',
    );
    expect(buildDockerRouteSearch({ host: ' host with spaces ' })).toBe('?host=host+with+spaces');
    expect(buildDockerRouteSearch({ host: '' })).toBe('');

    expect(KUBERNETES_PATH).toBe('/kubernetes');
    expect(buildKubernetesPath()).toBe('/kubernetes/overview');
    expect(buildKubernetesPath('workloads')).toBe('/kubernetes/workloads');
    expect(buildKubernetesPath('services')).toBe('/kubernetes/services');
    expect(buildKubernetesPath('configuration')).toBe('/kubernetes/configuration');
    expect(buildKubernetesPath('pods')).toBe('/kubernetes/pods');
    expect(KUBERNETES_QUERY_PARAMS.cluster).toBe('cluster');

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
    const search = buildWorkloadsRouteSearch({
      type: 'k8s',
      platform: 'kubernetes',
      context: 'cluster-a',
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
    });
    expect(search).toBe(
      '?type=pod&platform=kubernetes&context=cluster-a&agent=worker-1&resource=cluster-a%3Aworker-1%3A101',
    );

    const parsed = parseWorkloadsLinkSearch(search);
    expect(parsed).toEqual({
      type: 'pod',
      platform: 'kubernetes',
      runtime: '',
      context: 'cluster-a',
      namespace: '',
      cluster: '',
      agent: 'worker-1',
      resource: 'cluster-a:worker-1:101',
      summaryGroup: '',
    });

    expect(WORKLOADS_QUERY_PARAMS.type).toBe('type');
    expect(WORKLOADS_QUERY_PARAMS.platform).toBe('platform');
    expect(WORKLOADS_QUERY_PARAMS.runtime).toBe('runtime');
    expect(WORKLOADS_QUERY_PARAMS.context).toBe('context');
    expect(WORKLOADS_QUERY_PARAMS.namespace).toBe('namespace');
    expect(WORKLOADS_QUERY_PARAMS.cluster).toBe('cluster');
    expect(WORKLOADS_QUERY_PARAMS.agent).toBe('agent');
    expect(WORKLOADS_QUERY_PARAMS.resource).toBe('resource');
    expect(WORKLOADS_QUERY_PARAMS.summaryGroup).toBe('summaryGroup');
  });

  it('canonicalizes legacy workloads type aliases when building route search', () => {
    expect(
      buildWorkloadsRouteSearch({ type: 'docker', platform: 'docker', agent: 'runtime-1' }),
    ).toBe('?type=app-container&platform=docker&agent=runtime-1');
    expect(
      buildWorkloadsRouteSearch({
        type: 'kubernetes',
        platform: 'kubernetes',
        context: 'cluster-a',
      }),
    ).toBe('?type=pod&platform=kubernetes&context=cluster-a');
  });

  it('does not expose retired aggregate route builders', () => {
    expect(resourceLinks).not.toHaveProperty('buildWorkloadsPath');
    expect(resourceLinks).not.toHaveProperty('buildStoragePath');
    expect(resourceLinks).not.toHaveProperty('buildRecoveryPath');
  });

  it('keeps no recovery query serializer after the aggregate Recovery page', () => {
    // The aggregate Recovery page and its route-state owner were deleted on
    // 2026-05-26. The Proxmox Backups tab owns its query through
    // PROXMOX_BACKUPS_QUERY_PARAMS and the TrueNAS Protection tab keeps its
    // filters out of the URL, so a shared recovery serializer has no reader.
    expect(resourceLinks).not.toHaveProperty('RECOVERY_QUERY_PARAMS');
    expect(resourceLinks).not.toHaveProperty('buildRecoveryRouteSearch');
    expect(resourceLinks).not.toHaveProperty('parseRecoveryLinkSearch');
    expect(Object.keys(resourceLinks).filter((name) => /recovery/i.test(name))).toEqual([]);
  });

  it('builds and parses storage query params', () => {
    const search = buildStorageRouteSearch({
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
    expect(search).toBe(
      '?tab=disks&group=storage&source=proxmox-pbs&status=available&diskRole=nvme-disk&diskGroup=data&node=cluster-main-pve1&q=local-lvm&resource=storage-1&sort=usage&order=desc',
    );

    const parsed = parseStorageLinkSearch(search);
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

  it('exposes the mail gateway thresholds path', () => {
    expect(PMG_THRESHOLDS_PATH).toBe('/alerts/thresholds/mail-gateway');
  });

  it('deep-links one stable Patrol attention item without changing the route owner', () => {
    expect(buildPatrolAttentionPath('record/one')).toBe('/patrol?attention=record%2Fone');
    expect(buildPatrolAttentionPath('')).toBe('/patrol');
    expect(parsePatrolAttentionItemId('?attention=record%2Fone')).toBe('record/one');
  });
});
