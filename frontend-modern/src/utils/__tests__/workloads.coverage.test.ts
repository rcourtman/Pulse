import { describe, expect, it } from 'vitest';

import {
  buildAppContainerMetadataId,
  getWorkloadPlatformScopes,
  hasDiscoverySupportForWorkload,
  isContainerWorkloadType,
  isContainerWorkloadViewMode,
  resolveDiscoveryTargetForWorkload,
  workloadMatchesPlatformScope,
  workloadMatchesViewMode,
} from '@/utils/workloads';
import type { ViewMode, WorkloadGuest, WorkloadType } from '@/types/workloads';
import type { ResourceDiscoveryTarget } from '@/types/resource';

// `name` is a required string on WorkloadGuest, but buildAppContainerMetadataId
// tolerates null/undefined at runtime (`(value || '').trim()`). This alias lets
// the malformed-input cases opt out of the contract intentionally.
type AppContainerMetadataIdInput = Pick<WorkloadGuest, 'dockerHostId' | 'name'>;


describe('buildAppContainerMetadataId', () => {
  it('builds a host-and-name id in the canonical app-container format', () => {
    expect(
      buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: 'grafana' }),
    ).toBe('app-container:docker-main:name:grafana');
  });

  it('strips leading slashes from the container name', () => {
    expect(
      buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '/grafana' }),
    ).toBe('app-container:docker-main:name:grafana');
  });

  it('strips multiple leading slashes from the container name', () => {
    expect(
      buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '///traefik' }),
    ).toBe('app-container:docker-main:name:traefik');
  });

  it('preserves internal slashes in the container name', () => {
    expect(
      buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '/stack/app-1' }),
    ).toBe('app-container:docker-main:name:stack/app-1');
  });

  it('trims surrounding whitespace from both parts', () => {
    expect(
      buildAppContainerMetadataId({ dockerHostId: '  docker-main  ', name: '  grafana  ' }),
    ).toBe('app-container:docker-main:name:grafana');
  });

  it('returns null when the docker host id is missing', () => {
    expect(buildAppContainerMetadataId({ dockerHostId: undefined, name: 'grafana' })).toBeNull();
    expect(
      buildAppContainerMetadataId({
        dockerHostId: null,
        name: 'grafana',
      } as unknown as AppContainerMetadataIdInput),
    ).toBeNull();
  });

  it('returns null when the docker host id is empty or whitespace', () => {
    expect(buildAppContainerMetadataId({ dockerHostId: '', name: 'grafana' })).toBeNull();
    expect(buildAppContainerMetadataId({ dockerHostId: '   ', name: 'grafana' })).toBeNull();
  });

  it('returns null when the container name is missing', () => {
    expect(
      buildAppContainerMetadataId({
        dockerHostId: 'docker-main',
        name: undefined,
      } as unknown as AppContainerMetadataIdInput),
    ).toBeNull();
    expect(
      buildAppContainerMetadataId({
        dockerHostId: 'docker-main',
        name: null,
      } as unknown as AppContainerMetadataIdInput),
    ).toBeNull();
  });

  it('returns null when the container name is empty or whitespace', () => {
    expect(buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '' })).toBeNull();
    expect(buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '   ' })).toBeNull();
  });

  it('returns null when the container name is only leading slashes', () => {
    expect(buildAppContainerMetadataId({ dockerHostId: 'docker-main', name: '//' })).toBeNull();
  });
});

describe('getWorkloadPlatformScopes', () => {
  it('normalizes and deduplicates explicit platform scopes', () => {
    expect(
      getWorkloadPlatformScopes({
        platformScopes: ['docker', 'docker', 'k8s'],
        platformType: 'proxmox-pve',
      }),
    ).toEqual(['docker', 'kubernetes']);
  });

  it('lowercases and trims scope values', () => {
    expect(
      getWorkloadPlatformScopes({
        platformScopes: ['  Docker  ', 'Proxmox-PVE'],
        platformType: undefined,
      }),
    ).toEqual(['docker', 'proxmox-pve']);
  });

  it('resolves known aliases to their canonical platform id', () => {
    expect(
      getWorkloadPlatformScopes({
        platformScopes: ['pve', 'vmware', 'hyper-v'],
        platformType: undefined,
      }),
    ).toEqual(['proxmox-pve', 'vmware-vsphere', 'microsoft-hyperv']);
  });

  it('drops the all sentinel and empty scope values', () => {
    expect(
      getWorkloadPlatformScopes({
        platformScopes: ['all', '', '  ', 'docker'],
        platformType: undefined,
      }),
    ).toEqual(['docker']);
  });

  it('falls back to the platform type when no scopes are provided', () => {
    expect(getWorkloadPlatformScopes({ platformScopes: undefined, platformType: 'docker' })).toEqual(
      ['docker'],
    );
    expect(getWorkloadPlatformScopes({ platformScopes: [], platformType: 'K8s' })).toEqual([
      'kubernetes',
    ]);
  });

  it('returns an empty array when neither scopes nor platform type resolve', () => {
    expect(getWorkloadPlatformScopes({ platformScopes: undefined, platformType: undefined })).toEqual(
      [],
    );
    expect(getWorkloadPlatformScopes({ platformScopes: ['all'], platformType: '' })).toEqual([]);
  });

  it('does not fall back to platform type when at least one scope resolves', () => {
    expect(
      getWorkloadPlatformScopes({
        platformScopes: ['docker'],
        platformType: 'proxmox-pve',
      }),
    ).toEqual(['docker']);
  });
});

describe('workloadMatchesPlatformScope', () => {
  it('matches a workload scope verbatim', () => {
    expect(
      workloadMatchesPlatformScope({ platformScopes: ['docker'], platformType: undefined }, 'docker'),
    ).toBe(true);
  });

  it('matches a query alias to a canonical scope stored on the guest', () => {
    expect(
      workloadMatchesPlatformScope(
        { platformScopes: ['kubernetes'], platformType: undefined },
        'k8s',
      ),
    ).toBe(true);
    expect(
      workloadMatchesPlatformScope(
        { platformScopes: ['proxmox-pve'], platformType: undefined },
        'pve',
      ),
    ).toBe(true);
  });

  it('matches case-insensitively', () => {
    expect(
      workloadMatchesPlatformScope({ platformScopes: ['Docker'], platformType: undefined }, 'DOCKER'),
    ).toBe(true);
  });

  it('returns false when the scope is not part of the workload scopes', () => {
    expect(
      workloadMatchesPlatformScope(
        { platformScopes: ['docker'], platformType: undefined },
        'proxmox-pve',
      ),
    ).toBe(false);
  });

  it('returns true when the scope is undefined, null, empty, or whitespace', () => {
    const guest = { platformScopes: ['docker'], platformType: undefined };
    expect(workloadMatchesPlatformScope(guest, undefined)).toBe(true);
    expect(workloadMatchesPlatformScope(guest, null)).toBe(true);
    expect(workloadMatchesPlatformScope(guest, '')).toBe(true);
    expect(workloadMatchesPlatformScope(guest, '   ')).toBe(true);
  });

  it('returns true when the scope is the all sentinel (case-insensitive)', () => {
    const guest = { platformScopes: ['docker'], platformType: undefined };
    expect(workloadMatchesPlatformScope(guest, 'all')).toBe(true);
    expect(workloadMatchesPlatformScope(guest, 'ALL')).toBe(true);
  });

  it('matches against the platform type fallback when scopes are absent', () => {
    expect(
      workloadMatchesPlatformScope({ platformScopes: undefined, platformType: 'docker' }, 'docker'),
    ).toBe(true);
  });
});

describe('isContainerWorkloadType', () => {
  it('returns true for system-container and app-container', () => {
    expect(isContainerWorkloadType('system-container')).toBe(true);
    expect(isContainerWorkloadType('app-container')).toBe(true);
  });

  it('returns false for vm and pod', () => {
    expect(isContainerWorkloadType('vm')).toBe(false);
    expect(isContainerWorkloadType('pod')).toBe(false);
  });

  it('returns false for unknown workload type values', () => {
    expect(isContainerWorkloadType('unknown' as WorkloadType)).toBe(false);
    expect(isContainerWorkloadType('container' as WorkloadType)).toBe(false);
    expect(isContainerWorkloadType('' as WorkloadType)).toBe(false);
  });
});

describe('isContainerWorkloadViewMode', () => {
  it('returns true for the container, system-container, and app-container view modes', () => {
    expect(isContainerWorkloadViewMode('container')).toBe(true);
    expect(isContainerWorkloadViewMode('system-container')).toBe(true);
    expect(isContainerWorkloadViewMode('app-container')).toBe(true);
  });

  it('returns false for all, vm, and pod view modes', () => {
    expect(isContainerWorkloadViewMode('all')).toBe(false);
    expect(isContainerWorkloadViewMode('vm')).toBe(false);
    expect(isContainerWorkloadViewMode('pod')).toBe(false);
  });

  it('returns false for unknown view mode values', () => {
    expect(isContainerWorkloadViewMode('docker' as ViewMode)).toBe(false);
    expect(isContainerWorkloadViewMode('' as ViewMode)).toBe(false);
  });
});

describe('workloadMatchesViewMode', () => {
  it('returns true for every workload type when view mode is all', () => {
    const types: WorkloadType[] = ['vm', 'system-container', 'app-container', 'pod'];
    for (const workloadType of types) {
      expect(workloadMatchesViewMode(workloadType, 'all')).toBe(true);
    }
  });

  it('matches both system-container and app-container against the container view mode', () => {
    expect(workloadMatchesViewMode('system-container', 'container')).toBe(true);
    expect(workloadMatchesViewMode('app-container', 'container')).toBe(true);
  });

  it('does not match vm or pod against the container view mode', () => {
    expect(workloadMatchesViewMode('vm', 'container')).toBe(false);
    expect(workloadMatchesViewMode('pod', 'container')).toBe(false);
  });

  it('matches a workload type only against its own exact view mode', () => {
    expect(workloadMatchesViewMode('vm', 'vm')).toBe(true);
    expect(workloadMatchesViewMode('system-container', 'system-container')).toBe(true);
    expect(workloadMatchesViewMode('app-container', 'app-container')).toBe(true);
    expect(workloadMatchesViewMode('pod', 'pod')).toBe(true);
  });

  it('does not match a workload type against a different specific view mode', () => {
    expect(workloadMatchesViewMode('vm', 'system-container')).toBe(false);
    expect(workloadMatchesViewMode('system-container', 'vm')).toBe(false);
    expect(workloadMatchesViewMode('app-container', 'pod')).toBe(false);
    expect(workloadMatchesViewMode('pod', 'app-container')).toBe(false);
    expect(workloadMatchesViewMode('pod', 'system-container')).toBe(false);
  });
});

describe('resolveDiscoveryTargetForWorkload', () => {
  describe('explicit discoveryTarget branch', () => {
    it('returns a fully-resolved target with canonical resource type and preserved hostname', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'vm' as const,
          agentId: 'agent-pve1',
          resourceId: '101',
          hostname: 'pve1.local',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toEqual({
        resourceType: 'vm',
        agentId: 'agent-pve1',
        resourceId: '101',
        hostname: 'pve1.local',
      });
    });

    it('preserves an undefined hostname when not supplied on the explicit target', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'vm' as const,
          agentId: 'agent-pve1',
          resourceId: '101',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toEqual({
        resourceType: 'vm',
        agentId: 'agent-pve1',
        resourceId: '101',
        hostname: undefined,
      });
    });

    it('trims whitespace from explicit agentId and resourceId', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'vm' as const,
          agentId: '  agent-pve1  ',
          resourceId: '\n101\n',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toEqual({
        resourceType: 'vm',
        agentId: 'agent-pve1',
        resourceId: '101',
        hostname: undefined,
      });
    });

    it('canonicalizes an aliased resource type on the explicit target', () => {
      const guest = {
        id: 'app:1',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'docker',
          agentId: 'agent-1',
          resourceId: 'container-1',
        } as unknown as ResourceDiscoveryTarget,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)?.resourceType).toBe('app-container');
    });

    it('falls through when the explicit target has a whitespace-only resource type', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: '   ',
          agentId: 'agent-pve1',
          resourceId: '101',
        } as unknown as ResourceDiscoveryTarget,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('falls through when the explicit target has a whitespace-only agentId', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'vm' as const,
          agentId: '   ',
          resourceId: '101',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('falls through when the explicit target has a whitespace-only resourceId', () => {
      const guest = {
        id: 'vm:pve1:101',
        instance: 'cluster-a',
        workloadType: 'vm' as const,
        type: 'vm',
        node: 'pve1',
        vmid: 101,
        discoveryTarget: {
          resourceType: 'vm' as const,
          agentId: 'agent-pve1',
          resourceId: '',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });
  });

  describe('app-container branch', () => {
    const dockerGuest = {
      id: 'app-container:docker-host-1:grafana',
      workloadType: 'app-container' as const,
      type: 'docker',
      platformType: 'docker',
      dockerHostId: 'docker-host-1',
      containerId: 'container-grafana-abc',
      instance: 'docker-host-1',
      node: 'docker-host-1',
      vmid: 0,
    } as const;

    it('builds an app-container target from the docker host id and container id', () => {
      expect(resolveDiscoveryTargetForWorkload(dockerGuest)).toEqual({
        resourceType: 'app-container',
        agentId: 'docker-host-1',
        resourceId: 'container-grafana-abc',
      });
    });

    it('returns null for a TrueNAS app-container (not docker-managed)', () => {
      const guest = {
        id: 'app-container:truenas-main:nextcloud',
        workloadType: 'app-container' as const,
        type: 'app-container',
        platformType: 'truenas',
        dockerHostId: 'truenas-main',
        containerId: 'nextcloud',
        instance: 'truenas-main',
        node: 'truenas-main',
        vmid: 0,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('returns null when the docker host id is missing', () => {
      expect(
        resolveDiscoveryTargetForWorkload({ ...dockerGuest, dockerHostId: '' }),
      ).toBeNull();
      expect(
        resolveDiscoveryTargetForWorkload({ ...dockerGuest, dockerHostId: '   ' }),
      ).toBeNull();
    });

    it('returns null when the container id is missing', () => {
      expect(
        resolveDiscoveryTargetForWorkload({ ...dockerGuest, containerId: '' }),
      ).toBeNull();
      expect(
        resolveDiscoveryTargetForWorkload({ ...dockerGuest, containerId: '   ' }),
      ).toBeNull();
    });
  });

  describe('pod branch', () => {
    it('builds a pod target from the kubernetes agent id and the pod uid parsed from the id', () => {
      const guest = {
        id: 'k8s:cluster-a:pod:pod-uid-1',
        workloadType: 'pod' as const,
        type: 'k8s',
        kubernetesAgentId: 'k8s-agent-1',
        instance: 'cluster-a',
        node: 'cluster-a',
        vmid: 0,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toEqual({
        resourceType: 'pod',
        agentId: 'k8s-agent-1',
        resourceId: 'pod-uid-1',
      });
    });

    it('falls back to instance then node for the agent id', () => {
      expect(
        resolveDiscoveryTargetForWorkload({
          id: 'k8s:cluster-a:pod:pod-uid-2',
          workloadType: 'pod' as const,
          type: 'pod',
          instance: 'cluster-a',
          node: '',
          vmid: 0,
        })?.agentId,
      ).toBe('cluster-a');

      expect(
        resolveDiscoveryTargetForWorkload({
          id: 'k8s:cluster-a:pod:pod-uid-3',
          workloadType: 'pod' as const,
          type: 'pod',
          instance: '',
          node: 'worker-1',
          vmid: 0,
        })?.agentId,
      ).toBe('worker-1');
    });

    it('uses the raw id when it does not match the k8s pod id shape', () => {
      const guest = {
        id: 'pod-456',
        workloadType: 'pod' as const,
        type: 'pod',
        kubernetesAgentId: 'k8s-agent-1',
        instance: 'cluster-a',
        node: 'cluster-a',
        vmid: 0,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)?.resourceId).toBe('pod-456');
    });

    it('returns null when no agent id source resolves', () => {
      const guest = {
        id: 'k8s:cluster-a:pod:pod-uid-1',
        workloadType: 'pod' as const,
        type: 'pod',
        kubernetesAgentId: '   ',
        instance: '',
        node: '',
        vmid: 0,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('returns null when the id is missing', () => {
      const guest = {
        id: '',
        workloadType: 'pod' as const,
        type: 'pod',
        kubernetesAgentId: 'k8s-agent-1',
        instance: 'cluster-a',
        node: 'cluster-a',
        vmid: 0,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });
  });

  describe('default branch', () => {
    it('returns null for a vm without an explicit discovery target', () => {
      const guest = {
        id: 'vm:pve1:101',
        workloadType: 'vm' as const,
        type: 'vm',
        instance: 'cluster-a',
        node: 'pve1',
        vmid: 101,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('returns null for a system-container without an explicit discovery target', () => {
      const guest = {
        id: 'lxc:pve1:200',
        workloadType: 'system-container' as const,
        type: 'lxc',
        instance: 'cluster-a',
        node: 'pve1',
        vmid: 200,
      };
      expect(resolveDiscoveryTargetForWorkload(guest)).toBeNull();
    });

    it('prefers an explicit discovery target over the resolved workload type', () => {
      // This app-container would otherwise resolve via the type-based branch,
      // but the explicit target takes precedence.
      const guest = {
        id: 'app-container:docker-host-1:grafana',
        workloadType: 'app-container' as const,
        type: 'docker',
        platformType: 'docker',
        dockerHostId: 'docker-host-1',
        containerId: 'container-grafana-abc',
        instance: 'docker-host-1',
        node: 'docker-host-1',
        vmid: 0,
        discoveryTarget: {
          resourceType: 'app-container' as const,
          agentId: 'override-agent',
          resourceId: 'override-id',
        },
      };
      expect(resolveDiscoveryTargetForWorkload(guest)?.agentId).toBe('override-agent');
    });
  });
});

describe('hasDiscoverySupportForWorkload', () => {
  it('returns true when an explicit discovery target resolves', () => {
    const guest = {
      id: 'vm:pve1:101',
      workloadType: 'vm' as const,
      type: 'vm',
      instance: 'cluster-a',
      node: 'pve1',
      vmid: 101,
      discoveryTarget: {
        resourceType: 'vm' as const,
        agentId: 'agent-pve1',
        resourceId: '101',
      },
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(true);
  });

  it('returns true for a docker-managed app-container with host and container ids', () => {
    const guest = {
      id: 'app-container:docker-host-1:grafana',
      workloadType: 'app-container' as const,
      type: 'docker',
      platformType: 'docker',
      dockerHostId: 'docker-host-1',
      containerId: 'container-grafana-abc',
      instance: 'docker-host-1',
      node: 'docker-host-1',
      vmid: 0,
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(true);
  });

  it('returns true for a pod with a kubernetes agent id', () => {
    const guest = {
      id: 'k8s:cluster-a:pod:pod-uid-1',
      workloadType: 'pod' as const,
      type: 'k8s',
      kubernetesAgentId: 'k8s-agent-1',
      instance: 'cluster-a',
      node: 'cluster-a',
      vmid: 0,
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(true);
  });

  it('returns false for a vm without an explicit discovery target', () => {
    const guest = {
      id: 'vm:pve1:101',
      workloadType: 'vm' as const,
      type: 'vm',
      instance: 'cluster-a',
      node: 'pve1',
      vmid: 101,
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(false);
  });

  it('returns false for a TrueNAS app-container', () => {
    const guest = {
      id: 'app-container:truenas-main:nextcloud',
      workloadType: 'app-container' as const,
      type: 'app-container',
      platformType: 'truenas',
      dockerHostId: 'truenas-main',
      containerId: 'nextcloud',
      instance: 'truenas-main',
      node: 'truenas-main',
      vmid: 0,
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(false);
  });

  it('returns false for a pod missing every agent id source', () => {
    const guest = {
      id: 'k8s:cluster-a:pod:pod-uid-1',
      workloadType: 'pod' as const,
      type: 'pod',
      kubernetesAgentId: '',
      instance: '',
      node: '',
      vmid: 0,
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(false);
  });

  it('returns false when an explicit discovery target is missing required parts', () => {
    const guest = {
      id: 'vm:pve1:101',
      workloadType: 'vm' as const,
      type: 'vm',
      instance: 'cluster-a',
      node: 'pve1',
      vmid: 101,
      discoveryTarget: {
        resourceType: 'vm' as const,
        agentId: '',
        resourceId: '101',
      },
    };
    expect(hasDiscoverySupportForWorkload(guest)).toBe(false);
  });
});
