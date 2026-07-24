import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import type { ResourceDiscovery } from '@/types/discovery';
import discoveryTabSource from '@/components/Discovery/DiscoveryTab.tsx?raw';
import discoveryReadinessSource from '@/components/Discovery/discoveryReadiness.ts?raw';
import discoveryTabStateSource from '@/components/Discovery/useDiscoveryTabState.ts?raw';
import { toDiscoveryConfig } from '@/components/Infrastructure/resourceDetailDiscoveryModel';
import { hasMeaningfulDiscoveryContext } from '@/utils/discoveryPresentation';

const baseResource = (): Resource => ({
  id: 'host-abcd',
  type: 'agent',
  name: 'pve1',
  displayName: 'pve1',
  platformId: 'pve1',
  platformType: 'proxmox-pve',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: Date.now(),
  platformData: {
    sources: ['proxmox', 'agent'],
    proxmox: { nodeName: 'pve1' },
    agent: { agentId: 'host-1', hostname: 'pve1.local' },
  },
  identity: {
    hostname: 'stale-hostname',
  },
});

describe('toDiscoveryConfig', () => {
  it('prefers backend discoveryTarget over heuristic IDs', () => {
    const resource: Resource = {
      ...baseResource(),
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'host-1',
        resourceId: 'host-1',
        hostname: 'pve1.local',
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'agent',
      agentId: 'host-1',
      resourceId: 'host-1',
      hostname: 'pve1.local',
      metadataKind: 'agent',
      metadataId: 'host-1',
      targetLabel: 'agent',
    });
  });

  it('falls back to heuristic mapping when discoveryTarget is absent', () => {
    const config = toDiscoveryConfig(baseResource());
    expect(config).toEqual({
      resourceType: 'agent',
      agentId: 'host-1',
      resourceId: 'host-1',
      hostname: 'stale-hostname',
      metadataKind: 'agent',
      metadataId: 'host-1',
      targetLabel: 'agent',
    });
  });

  it('normalizes legacy truenas host types through the canonical agent discovery path', () => {
    const resource: Resource = {
      ...baseResource(),
      type: 'truenas' as unknown as Resource['type'],
      platformType: 'truenas',
      sourceType: 'hybrid',
      platformData: {
        sources: ['agent', 'truenas'],
        truenas: { hostname: 'truenas-main' },
        agent: { hostname: 'truenas-main' },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'agent',
      agentId: 'truenas-main',
      resourceId: 'truenas-main',
      hostname: 'stale-hostname',
      metadataKind: 'agent',
      metadataId: 'truenas-main',
      targetLabel: 'agent',
    });
  });

  it('uses explicit discoveryTarget.agentId when provided', () => {
    const resource: Resource = {
      ...baseResource(),
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'agent-explicit-1',
        resourceId: 'agent-explicit-1',
        hostname: 'pve1.local',
      } as any,
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'agent',
      agentId: 'agent-explicit-1',
      resourceId: 'agent-explicit-1',
      hostname: 'pve1.local',
      metadataKind: 'agent',
      metadataId: 'agent-explicit-1',
      targetLabel: 'agent',
    });
  });

  it('prefers docker hostSourceId for docker-host fallback mapping', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:docker:abc123',
      type: 'docker-host',
      platformType: 'docker',
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'docker-host-1',
          hostname: 'edge-docker',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'agent',
      agentId: 'docker-host-1',
      resourceId: 'docker-host-1',
      hostname: 'stale-hostname',
      metadataKind: 'docker-host',
      metadataId: 'docker-host-1',
      targetLabel: 'host',
    });
  });

  it('prefers proxmox vmid for vm fallback resourceId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:vm:hash-101',
      type: 'vm',
      platformData: {
        sources: ['proxmox'],
        proxmox: {
          nodeName: 'pve1',
          vmid: 101,
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'vm',
      agentId: 'pve1',
      resourceId: '101',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'resource:vm:hash-101',
      targetLabel: 'guest',
    });
  });

  it('does not synthesize service discovery for VMware VMs without a backend target', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'vm-270aaf3d37cc7501',
      type: 'vm',
      name: 'warehouse-api-01',
      platformType: 'vmware-vsphere',
      platformScopes: ['vmware-vsphere'],
      sources: ['vmware'],
      parentName: 'redacted by policy',
      platformData: {
        sources: ['vmware'],
        vmware: {
          connectionName: 'Lab vCenter',
          clusterName: 'redacted by policy',
          runtimeHostName: 'esxi-01.lab.local',
        },
      },
      vmware: {
        connectionName: 'Lab vCenter',
        entityType: 'VirtualMachine',
        managedObjectId: 'vm-201',
      },
    };

    expect(toDiscoveryConfig(resource)).toBeNull();
  });

  it('still uses explicit backend discovery targets for VMware VMs', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'vm-270aaf3d37cc7501',
      type: 'vm',
      name: 'warehouse-api-01',
      platformType: 'vmware-vsphere',
      platformScopes: ['vmware-vsphere'],
      sources: ['vmware'],
      discoveryTarget: {
        resourceType: 'vm',
        agentId: 'guest-agent-01',
        resourceId: 'vm-201',
        hostname: 'warehouse-api-01.internal',
      },
      vmware: {
        connectionName: 'Lab vCenter',
        entityType: 'VirtualMachine',
        managedObjectId: 'vm-201',
      },
    };

    // Guest metadata keys off the canonical workload id (the resource id for
    // non-PVE guests), not the vSphere managed-object id.
    expect(toDiscoveryConfig(resource)).toEqual({
      resourceType: 'vm',
      agentId: 'guest-agent-01',
      resourceId: 'vm-201',
      hostname: 'warehouse-api-01.internal',
      metadataKind: 'guest',
      metadataId: 'vm-270aaf3d37cc7501',
      targetLabel: 'guest',
    });
  });

  it('prefers docker hostSourceId for app-container fallback agentId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:app-container:hash-1',
      type: 'app-container',
      platformType: 'docker',
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'docker-host-1',
          containerId: 'container-abc123',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'app-container',
      agentId: 'docker-host-1',
      resourceId: 'container-abc123',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'app-container:docker-host-1:name:pve1',
      targetLabel: 'container',
    });
  });

  it('keeps backend app-container discoveryTarget identity for Docker workloads', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:app-container:customer-portal',
      type: 'app-container',
      name: 'customer-portal',
      platformType: 'docker',
      discoveryTarget: {
        resourceType: 'app-container',
        agentId: 'agent-edge-01',
        resourceId: 'customer-portal',
        hostname: 'edge-apps-01',
      },
      platformData: {
        sources: ['docker'],
        docker: {
          hostSourceId: 'agent-edge-01',
          containerId: 'abc123def456',
          hostname: 'edge-apps-01',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'app-container',
      agentId: 'agent-edge-01',
      resourceId: 'customer-portal',
      hostname: 'edge-apps-01',
      metadataKind: 'guest',
      metadataId: 'app-container:agent-edge-01:name:customer-portal',
      targetLabel: 'container',
    });
  });

  it('prefers kubernetes cluster/pod IDs for pod fallback mapping', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:pod:hash-1',
      type: 'pod',
      name: 'checkout',
      platformType: 'kubernetes',
      clusterId: 'cluster-a',
      kubernetes: {
        clusterId: 'cluster-a',
        podUid: 'pod-uid-1',
        namespace: 'default',
      },
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          clusterId: 'cluster-a',
          namespace: 'default',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'pod',
      agentId: 'cluster-a',
      resourceId: 'pod-uid-1',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'k8s-workload:cluster-a:pod:default:checkout',
      targetLabel: 'workload',
    });
  });

  it('prefers kubernetes agentId over clusterId for pod fallback agentId', () => {
    const resource: Resource = {
      ...baseResource(),
      id: 'resource:pod:hash-2',
      type: 'pod',
      name: 'checkout',
      platformType: 'kubernetes',
      kubernetes: {
        agentId: 'k8s-agent-1',
        clusterId: 'cluster-a',
        podUid: 'pod-uid-2',
        namespace: 'default',
      },
      platformData: {
        sources: ['kubernetes'],
        kubernetes: {
          agentId: 'k8s-agent-1',
          clusterId: 'cluster-a',
          namespace: 'default',
        },
      },
    };

    const config = toDiscoveryConfig(resource);
    expect(config).toEqual({
      resourceType: 'pod',
      agentId: 'k8s-agent-1',
      resourceId: 'pod-uid-2',
      hostname: 'stale-hostname',
      metadataKind: 'guest',
      metadataId: 'k8s-workload:cluster-a:pod:default:checkout',
      targetLabel: 'workload',
    });
  });
});

describe('resource drawer discovery promotion', () => {
  it('points disabled discovery readiness at Assistant-owned service identification settings', () => {
    expect(discoveryTabSource).toContain('Service identification is off');
    expect(discoveryTabSource).toContain('getDiscoveryServiceContextSettingsTarget');
    expect(discoveryTabSource).toContain('serviceContextSettingsTarget.label');
    expect(discoveryTabStateSource).toContain(
      'Service context is disabled in Settings -> Pulse Intelligence -> Assistant.',
    );
    expect(discoveryReadinessSource).toContain('Settings -> Pulse Intelligence -> Assistant');
  });

  it('does not treat command-only diagnostic records as meaningful resource context', () => {
    const discovery: ResourceDiscovery = {
      id: 'system-container:pve4:152',
      resource_type: 'system-container',
      resource_id: '152',
      target_id: 'pve4',
      hostname: 'smtp-relay-32',
      service_type: 'unknown',
      service_name: 'Unknown Container',
      service_version: 'unknown',
      category: 'unknown',
      cli_access: 'pct exec 152 -- /bin/bash',
      facts: [
        {
          category: 'service',
          key: 'status',
          value: 'online',
          source: 'metadata',
          confidence: 1,
          discovered_at: '2026-05-19T00:00:00Z',
        },
        {
          category: 'config',
          key: 'config_availability',
          value: 'missing_node_config',
          source: 'all_commands',
          confidence: 1,
          discovered_at: '2026-05-19T00:00:00Z',
        },
      ],
      config_paths: [],
      data_paths: [],
      log_paths: [],
      ports: [],
      user_notes: '',
      user_secrets: {},
      confidence: 0,
      ai_reasoning: 'Discovery commands could not inspect the workload.',
      discovered_at: '2026-05-19T00:00:00Z',
      updated_at: '2026-05-19T00:00:00Z',
      scan_duration: 2816,
      suggested_url_diagnostic: 'no host or IP candidate available',
    };

    expect(hasMeaningfulDiscoveryContext(discovery)).toBe(false);
  });
});
