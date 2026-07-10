import { describe, expect, it } from 'vitest';

import type { PBSInstance } from '@/types/api';
import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';

import {
  buildProjectedOverrides,
  dockerHostOverrideIdCandidates,
  hostOverrideIdCandidates,
  nodeOverrideIdCandidates,
} from '../alertOverridesModel';

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: 'resource-1',
    name: 'resource-1',
    type: 'vm',
    ...overrides,
  }) as Resource;

const cpuThreshold = (trigger: number, clear: number): RawOverrideConfig =>
  ({ cpu: { trigger, clear } }) as RawOverrideConfig;

type ProjectArgs = Parameters<typeof buildProjectedOverrides>[0];

const projectOverrides = (
  overrides: Partial<ProjectArgs>,
): ReturnType<typeof buildProjectedOverrides> =>
  buildProjectedOverrides({
    rawConfig: {},
    nodeResources: [],
    vmResources: [],
    containerResources: [],
    storageResources: [],
    agentResourceList: [],
    containerRuntimeResources: [],
    getChildren: () => [],
    pbsInstanceById: new Map(),
    allResources: [],
    ...overrides,
  });

const makePBS = (id: string, name: string): PBSInstance =>
  ({ id, name } as unknown as PBSInstance);

// ---------------------------------------------------------------------------
// hostOverrideIdCandidates
// ---------------------------------------------------------------------------

describe('hostOverrideIdCandidates', () => {
  it('returns only resource.id when no agent identifiers exist', () => {
    expect(hostOverrideIdCandidates(makeResource({ id: 'agent-1' }))).toEqual(['agent-1']);
  });

  it('prepends resource.agent.agentId when present', () => {
    expect(
      hostOverrideIdCandidates(
        makeResource({ id: 'agent-1', agent: { agentId: 'explicit-aid' } }),
      ),
    ).toEqual(['explicit-aid', 'agent-1']);
  });

  it('includes discoveryTarget.agentId as a distinct candidate alongside an explicit agent id', () => {
    expect(
      hostOverrideIdCandidates(
        makeResource({
          id: 'agent-1',
          agent: { agentId: 'explicit-aid' },
          discoveryTarget: { resourceType: 'vm', agentId: 'dt-aid', resourceId: 'dt-rid' },
        }),
      ),
    ).toEqual(['explicit-aid', 'dt-aid', 'agent-1']);
  });

  it('includes platformData.agent.agentId when present', () => {
    expect(
      hostOverrideIdCandidates(
        makeResource({
          id: 'agent-1',
          platformData: { agent: { agentId: 'pd-agent-aid' } },
        }),
      ),
    ).toEqual(['pd-agent-aid', 'agent-1']);
  });

  it('includes platformData.agentId when present', () => {
    expect(
      hostOverrideIdCandidates(
        makeResource({
          id: 'agent-1',
          platformData: { agentId: 'pd-aid' },
        }),
      ),
    ).toEqual(['pd-aid', 'agent-1']);
  });

  it('deduplicates identical IDs across all sources', () => {
    expect(
      hostOverrideIdCandidates(
        makeResource({
          id: 'same',
          agent: { agentId: 'same' },
          platformData: { agentId: 'same' },
        }),
      ),
    ).toEqual(['same']);
  });
});

// ---------------------------------------------------------------------------
// nodeOverrideIdCandidates
// ---------------------------------------------------------------------------

describe('nodeOverrideIdCandidates', () => {
  it('returns empty array when all fields are empty or undefined', () => {
    expect(
      nodeOverrideIdCandidates({
        id: '',
        name: '',
        instance: '',
        host: '',
      }),
    ).toEqual([]);
  });

  it('returns only id when no other fields are populated', () => {
    expect(
      nodeOverrideIdCandidates({
        id: 'node-1',
        name: '',
        instance: '',
        host: '',
      }),
    ).toEqual(['node-1']);
  });

  it('omits composed candidates when instance is missing but name is present', () => {
    expect(
      nodeOverrideIdCandidates({
        id: 'node-1',
        name: 'pve-1',
        instance: '',
        host: '',
      }),
    ).toEqual(['node-1', 'pve-1']);
  });

  it('omits composed candidates when name is missing but instance is present', () => {
    expect(
      nodeOverrideIdCandidates({
        id: 'node-1',
        name: '',
        instance: 'homelab',
        host: '',
      }),
    ).toEqual(['node-1']);
  });

  it('produces instance-name and instance:name composed candidates when both are present', () => {
    expect(
      nodeOverrideIdCandidates({
        id: 'node-1',
        name: 'pve-1',
        instance: 'homelab',
        host: '',
      }),
    ).toEqual(['node-1', 'homelab-pve-1', 'homelab:pve-1', 'pve-1']);
  });

  it('includes linkedAgentId and host when present', () => {
    expect(
      nodeOverrideIdCandidates({
        id: 'node-1',
        name: 'pve-1',
        instance: 'homelab',
        host: 'https://pve-1:8006',
        linkedAgentId: 'agent-linked',
      }),
    ).toEqual([
      'agent-linked',
      'node-1',
      'homelab-pve-1',
      'homelab:pve-1',
      'pve-1',
      'https://pve-1:8006',
    ]);
  });
});

// ---------------------------------------------------------------------------
// dockerHostOverrideIdCandidates
// ---------------------------------------------------------------------------

describe('dockerHostOverrideIdCandidates', () => {
  it('includes discoveryTarget.resourceId when resourceType is app-container', () => {
    expect(
      dockerHostOverrideIdCandidates(
        makeResource({
          id: 'docker-host-1',
          discoveryTarget: {
            resourceType: 'app-container',
            agentId: 'dt-agent',
            resourceId: 'dt-resource-id',
          },
        }),
      ),
    ).toEqual(['dt-resource-id', 'dt-agent', 'docker-host-1']);
  });

  it('excludes discoveryTarget.resourceId when resourceType is not app-container', () => {
    expect(
      dockerHostOverrideIdCandidates(
        makeResource({
          id: 'docker-host-1',
          discoveryTarget: {
            resourceType: 'agent',
            agentId: 'dt-agent',
            resourceId: 'dt-resource-id',
          },
        }),
      ),
    ).toEqual(['dt-agent', 'docker-host-1']);
  });

  it('returns only resource.id when discoveryTarget and platformData are absent', () => {
    expect(dockerHostOverrideIdCandidates(makeResource({ id: 'dh-1' }))).toEqual(['dh-1']);
  });

  it('includes platformData.docker.hostSourceId when present', () => {
    expect(
      dockerHostOverrideIdCandidates(
        makeResource({
          id: 'dh-1',
          platformData: { docker: { hostSourceId: 'docker-source' } },
        }),
      ),
    ).toEqual(['docker-source', 'dh-1']);
  });

  it('includes platformData.hostSourceId when present', () => {
    expect(
      dockerHostOverrideIdCandidates(
        makeResource({
          id: 'dh-1',
          platformData: { hostSourceId: 'pd-source' },
        }),
      ),
    ).toEqual(['pd-source', 'dh-1']);
  });

  it('deduplicates overlapping IDs', () => {
    expect(
      dockerHostOverrideIdCandidates(
        makeResource({
          id: 'same',
          platformData: { docker: { hostSourceId: 'same' } },
        }),
      ),
    ).toEqual(['same']);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — isTrueNASResource branches (via allResources)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — isTrueNASResource', () => {
  it('classifies agent with platformType truenas as truenasSystem', () => {
    const agent = makeResource({
      id: 'truenas-agent',
      type: 'agent',
      name: 'truenas-main',
      displayName: 'truenas-main',
      platformType: 'truenas',
      truenas: {},
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-agent': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'truenas-agent',
        type: 'truenasSystem',
        resourceType: 'TrueNAS System',
        instance: 'TrueNAS',
      }),
    ]);
  });

  it('classifies agent with sources including truenas as truenasSystem', () => {
    const agent = makeResource({
      id: 'truenas-src-agent',
      type: 'agent',
      name: 'truenas-src',
      displayName: 'truenas-src',
      platformType: 'agent',
      sources: ['truenas'],
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-src-agent': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasSystem', resourceType: 'TrueNAS System' }),
    ]);
  });

  it('classifies storage with storage.platform truenas as truenasPool', () => {
    const storage = makeResource({
      id: 'truenas-pool-1',
      type: 'storage',
      name: 'tank',
      displayName: 'tank',
      platformType: 'generic',
      storage: { platform: 'truenas' },
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-pool-1': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasPool', resourceType: 'TrueNAS Pool' }),
    ]);
  });

  it('classifies storage with sources including truenas as truenasPool', () => {
    const storage = makeResource({
      id: 'truenas-pool-2',
      type: 'storage',
      name: 'pool2',
      displayName: 'pool2',
      sources: ['truenas'],
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-pool-2': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasPool', resourceType: 'TrueNAS Pool' }),
    ]);
  });

  it('classifies storage with topology dataset as truenasDataset', () => {
    const storage = makeResource({
      id: 'truenas-dataset-1',
      type: 'storage',
      name: 'ds1',
      displayName: 'ds1',
      platformType: 'truenas',
      storage: { platform: 'truenas', topology: 'dataset' },
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-dataset-1': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasDataset', resourceType: 'TrueNAS Dataset' }),
    ]);
  });

  it('classifies pool type as truenasPool when isTrueNASResource is true', () => {
    const pool = makeResource({
      id: 'truenas-zfs-pool',
      type: 'pool',
      name: 'zfspool',
      displayName: 'zfspool',
      platformType: 'truenas',
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-zfs-pool': cpuThreshold(90, 80) },
      allResources: [pool],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasPool', resourceType: 'TrueNAS Pool' }),
    ]);
  });

  it('classifies dataset type with topology dataset as truenasDataset', () => {
    const dataset = makeResource({
      id: 'truenas-ds-type',
      type: 'dataset',
      name: 'myds',
      displayName: 'myds',
      platformType: 'truenas',
      storage: { platform: 'truenas', topology: 'dataset' },
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-ds-type': cpuThreshold(90, 80) },
      allResources: [dataset],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasDataset', resourceType: 'TrueNAS Dataset' }),
    ]);
  });

  it('classifies dataset type with topology pool as truenasPool', () => {
    const dataset = makeResource({
      id: 'truenas-ds-pool',
      type: 'dataset',
      name: 'ds-as-pool',
      displayName: 'ds-as-pool',
      platformType: 'truenas',
      storage: { platform: 'truenas', topology: 'pool' },
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-ds-pool': cpuThreshold(90, 80) },
      allResources: [dataset],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasPool', resourceType: 'TrueNAS Pool' }),
    ]);
  });

  it('classifies physical_disk with truenas platform as truenasDisk', () => {
    const disk = makeResource({
      id: 'truenas-disk-1',
      type: 'physical_disk',
      name: 'da0',
      displayName: 'da0',
      platformType: 'truenas',
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-disk-1': cpuThreshold(90, 80) },
      allResources: [disk],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'truenasDisk', resourceType: 'TrueNAS Disk' }),
    ]);
  });

  it('does not classify agent without truenas or vmware markers into alertPlatformMap', () => {
    const agent = makeResource({
      id: 'plain-agent',
      type: 'agent',
      name: 'plain-agent',
      displayName: 'plain-agent',
      platformType: 'agent',
    });

    const result = projectOverrides({
      rawConfig: { 'plain-agent': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toEqual([]);
  });

  it('does not classify physical_disk without truenas markers into alertPlatformMap', () => {
    const disk = makeResource({
      id: 'plain-disk',
      type: 'physical_disk',
      name: 'sda',
      displayName: 'sda',
      platformType: 'generic',
    });

    const result = projectOverrides({
      rawConfig: { 'plain-disk': cpuThreshold(90, 80) },
      allResources: [disk],
    });

    expect(result).toEqual([]);
  });

  it('does not classify storage without truenas or vmware markers into alertPlatformMap', () => {
    const storage = makeResource({
      id: 'plain-storage',
      type: 'storage',
      name: 'lvm',
      displayName: 'lvm',
      platformType: 'generic',
    });

    const result = projectOverrides({
      rawConfig: { 'plain-storage': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — isVMwareResource branches (via allResources)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — isVMwareResource', () => {
  it('classifies agent with platformType vmware-vsphere as vmwareHost', () => {
    const agent = makeResource({
      id: 'vmware-host-1',
      type: 'agent',
      name: 'esxi-01',
      displayName: 'esxi-01',
      platformType: 'vmware-vsphere',
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-host-1': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareHost', resourceType: 'vSphere Host' }),
    ]);
  });

  it('classifies agent with sources including vmware as vmwareHost', () => {
    const agent = makeResource({
      id: 'vmware-src-host',
      type: 'agent',
      name: 'esxi-src',
      displayName: 'esxi-src',
      sources: ['vmware'],
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-src-host': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareHost', resourceType: 'vSphere Host' }),
    ]);
  });

  it('classifies vm with platformType vmware-vsphere as vmwareVm', () => {
    const vm = makeResource({
      id: 'vmware-vm-1',
      type: 'vm',
      name: 'web-01',
      displayName: 'web-01',
      platformType: 'vmware-vsphere',
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-vm-1': cpuThreshold(90, 80) },
      allResources: [vm],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareVm', resourceType: 'vSphere VM' }),
    ]);
  });

  it('classifies vm with vmware object as vmwareVm', () => {
    const vm = makeResource({
      id: 'vmware-vm-2',
      type: 'vm',
      name: 'web-02',
      displayName: 'web-02',
      vmware: { connectionName: 'vc-01' },
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-vm-2': cpuThreshold(90, 80) },
      allResources: [vm],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'vmwareVm',
        resourceType: 'vSphere VM',
        instance: 'vc-01',
      }),
    ]);
  });

  it('classifies storage with storage.platform vmware-vsphere as vmwareDatastore', () => {
    const storage = makeResource({
      id: 'vmware-ds-1',
      type: 'storage',
      name: 'datastore1',
      displayName: 'datastore1',
      storage: { platform: 'vmware-vsphere' },
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-ds-1': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareDatastore', resourceType: 'vSphere Datastore' }),
    ]);
  });

  it('classifies storage with vmware object as vmwareDatastore', () => {
    const storage = makeResource({
      id: 'vmware-ds-2',
      type: 'storage',
      name: 'datastore2',
      displayName: 'datastore2',
      vmware: { connectionName: 'vc-02' },
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-ds-2': cpuThreshold(90, 80) },
      allResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareDatastore', resourceType: 'vSphere Datastore' }),
    ]);
  });

  it('classifies network with platformType vmware-vsphere as vmwareNetwork', () => {
    const network = makeResource({
      id: 'vmware-net-1',
      type: 'network',
      name: 'VM Network',
      displayName: 'VM Network',
      platformType: 'vmware-vsphere',
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-net-1': cpuThreshold(90, 80) },
      allResources: [network],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'vmwareNetwork', resourceType: 'vSphere Network' }),
    ]);
  });

  it('does not classify vm without vmware markers into alertPlatformMap', () => {
    const vm = makeResource({
      id: 'plain-vm',
      type: 'vm',
      name: 'plain-vm',
      displayName: 'plain-vm',
      platformType: 'generic',
    });

    const result = projectOverrides({
      rawConfig: { 'plain-vm': cpuThreshold(90, 80) },
      allResources: [vm],
    });

    expect(result).toEqual([]);
  });

  it('does not classify network without vmware markers into alertPlatformMap', () => {
    const network = makeResource({
      id: 'plain-network',
      type: 'network',
      name: 'lan',
      displayName: 'lan',
      platformType: 'generic',
    });

    const result = projectOverrides({
      rawConfig: { 'plain-network': cpuThreshold(90, 80) },
      allResources: [network],
    });

    expect(result).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — Kubernetes resource types (via allResources)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — Kubernetes types', () => {
  it('classifies k8s-node as kubernetesNode', () => {
    const node = makeResource({
      id: 'k8s-node-1',
      type: 'k8s-node',
      name: 'worker-1',
      displayName: 'worker-1',
      kubernetes: { nodeName: 'worker-1', clusterName: 'prod' },
    });

    const result = projectOverrides({
      rawConfig: { 'k8s-node-1': cpuThreshold(90, 80) },
      allResources: [node],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'kubernetesNode',
        resourceType: 'Kubernetes Node',
        node: 'worker-1',
        instance: 'prod',
      }),
    ]);
  });

  it('classifies k8s-namespace as kubernetesNamespace', () => {
    const ns = makeResource({
      id: 'k8s-ns-1',
      type: 'k8s-namespace',
      name: 'default',
      displayName: 'default',
      kubernetes: { namespace: 'default', clusterName: 'prod' },
    });

    const result = projectOverrides({
      rawConfig: { 'k8s-ns-1': cpuThreshold(90, 80) },
      allResources: [ns],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'kubernetesNamespace',
        resourceType: 'Kubernetes Namespace',
      }),
    ]);
  });

  it('classifies k8s-deployment as kubernetesDeployment', () => {
    const deploy = makeResource({
      id: 'k8s-deploy-1',
      type: 'k8s-deployment',
      name: 'nginx',
      displayName: 'nginx',
      kubernetes: { namespace: 'default' },
    });

    const result = projectOverrides({
      rawConfig: { 'k8s-deploy-1': cpuThreshold(90, 80) },
      allResources: [deploy],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'kubernetesDeployment',
        resourceType: 'Kubernetes Deployment',
      }),
    ]);
  });

  it('classifies pod as kubernetesPod', () => {
    const pod = makeResource({
      id: 'k8s-pod-1',
      type: 'pod',
      name: 'nginx-abc',
      displayName: 'nginx-abc',
      kubernetes: { namespace: 'default' },
    });

    const result = projectOverrides({
      rawConfig: { 'k8s-pod-1': cpuThreshold(90, 80) },
      allResources: [pod],
    });

    expect(result).toEqual([
      expect.objectContaining({ type: 'kubernetesPod', resourceType: 'Kubernetes Pod' }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — alertResourceIdCandidates (via allResources)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — alertResourceIdCandidates', () => {
  it('resolves overrides through discoveryTarget.resourceId candidate', () => {
    const cluster = makeResource({
      id: 'k8s-internal-uid',
      type: 'k8s-cluster',
      name: 'prod',
      displayName: 'prod',
      discoveryTarget: {
        resourceType: 'agent',
        agentId: 'agent-1',
        resourceId: 'dt-canonical-id',
      },
    });

    const result = projectOverrides({
      rawConfig: { 'dt-canonical-id': cpuThreshold(90, 80) },
      allResources: [cluster],
    });

    expect(result).toEqual([
      expect.objectContaining({ id: 'dt-canonical-id', type: 'kubernetesCluster' }),
    ]);
  });

  it('resolves overrides through the resource.id candidate', () => {
    const cluster = makeResource({
      id: 'k8s-uid-2',
      type: 'k8s-cluster',
      name: 'dev',
      displayName: 'dev',
    });

    const result = projectOverrides({
      rawConfig: { 'k8s-uid-2': cpuThreshold(90, 80) },
      allResources: [cluster],
    });

    expect(result).toEqual([
      expect.objectContaining({ id: 'k8s-uid-2', type: 'kubernetesCluster' }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — storageCoords branches
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — storageCoords', () => {
  it('derives datastore instance from platformData.pbsInstanceId', () => {
    const datastore = makeResource({
      id: 'ds-backup-1',
      type: 'datastore',
      name: 'backup-ds',
      displayName: 'backup-ds',
      platformData: { pbsInstanceId: 'pbs-inst-1', pbsInstanceName: 'pbs-node-a' },
    });

    const result = projectOverrides({
      rawConfig: { 'ds-backup-1': cpuThreshold(90, 80) },
      storageResources: [datastore],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'pbs-node-a', instance: 'pbs-inst-1' }),
    ]);
  });

  it('falls back to parentId when pbsInstanceId is absent for datastore', () => {
    const datastore = makeResource({
      id: 'ds-backup-2',
      type: 'datastore',
      name: 'backup-ds-2',
      displayName: 'backup-ds-2',
      parentId: 'pbs-parent-1',
    });

    const result = projectOverrides({
      rawConfig: { 'ds-backup-2': cpuThreshold(90, 80) },
      storageResources: [datastore],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'pbs-parent-1', instance: 'pbs-parent-1' }),
    ]);
  });

  it('falls back to platformId when neither pbsInstanceId nor parentId exist', () => {
    const datastore = makeResource({
      id: 'ds-backup-3',
      type: 'datastore',
      name: 'backup-ds-3',
      displayName: 'backup-ds-3',
      platformId: 'pbs-platform-1',
    });

    const result = projectOverrides({
      rawConfig: { 'ds-backup-3': cpuThreshold(90, 80) },
      storageResources: [datastore],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'pbs-platform-1', instance: 'pbs-platform-1' }),
    ]);
  });

  it('falls back to literal pbs when no instance identifiers exist for datastore', () => {
    const datastore = makeResource({
      id: 'ds-backup-4',
      type: 'datastore',
      name: 'backup-ds-4',
      displayName: 'backup-ds-4',
    });

    const result = projectOverrides({
      rawConfig: { 'ds-backup-4': cpuThreshold(90, 80) },
      storageResources: [datastore],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'pbs', instance: 'pbs' }),
    ]);
  });

  it('uses pbsInstanceName as node when present', () => {
    const datastore = makeResource({
      id: 'ds-backup-5',
      type: 'datastore',
      name: 'backup-ds-5',
      displayName: 'backup-ds-5',
      platformData: { pbsInstanceId: 'inst-5', pbsInstanceName: 'friendly-pbs' },
    });

    const result = projectOverrides({
      rawConfig: { 'ds-backup-5': cpuThreshold(90, 80) },
      storageResources: [datastore],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'friendly-pbs', instance: 'inst-5' }),
    ]);
  });

  it('uses platformData.node and platformData.instance for non-datastore storage', () => {
    const storage = makeResource({
      id: 'storage-regular',
      type: 'storage',
      name: 'local',
      displayName: 'local',
      platformData: { node: 'pve-1', instance: 'Main' },
    });

    const result = projectOverrides({
      rawConfig: { 'storage-regular': cpuThreshold(90, 80) },
      storageResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: 'pve-1', instance: 'Main' }),
    ]);
  });

  it('returns empty node when platformData.node is absent for non-datastore', () => {
    const storage = makeResource({
      id: 'storage-no-node',
      type: 'storage',
      name: 'local2',
      displayName: 'local2',
      platformId: 'Main',
      platformData: { instance: 'Main' },
    });

    const result = projectOverrides({
      rawConfig: { 'storage-no-node': cpuThreshold(90, 80) },
      storageResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: '', instance: 'Main' }),
    ]);
  });

  it('falls back to platformId for instance when platformData.instance is absent', () => {
    const storage = makeResource({
      id: 'storage-no-instance',
      type: 'storage',
      name: 'local3',
      displayName: 'local3',
      platformId: 'fallback-pid',
    });

    const result = projectOverrides({
      rawConfig: { 'storage-no-instance': cpuThreshold(90, 80) },
      storageResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: '', instance: 'fallback-pid' }),
    ]);
  });

  it('returns empty node and instance when no coords are available', () => {
    const storage = makeResource({
      id: 'storage-bare',
      type: 'storage',
      name: 'bare',
      displayName: 'bare',
    });

    const result = projectOverrides({
      rawConfig: { 'storage-bare': cpuThreshold(90, 80) },
      storageResources: [storage],
    });

    expect(result).toEqual([
      expect.objectContaining({ node: '', instance: '' }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — Docker host and container matching
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — Docker matching', () => {
  it('creates dockerHost override when key matches dockerHostMap', () => {
    const host = makeResource({
      id: 'docker-host-1',
      type: 'docker-host',
      name: 'docker-host-1',
      displayName: 'Docker Host 1',
    });

    const result = projectOverrides({
      rawConfig: {
        'docker-host-1': {
          disableConnectivity: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      containerRuntimeResources: [host],
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'docker-host-1',
        type: 'dockerHost',
        resourceType: 'Container Runtime',
        disableConnectivity: true,
      }),
    ]);
  });

  it('creates dockerContainer override via dockerContainerMap with container id containing slash', () => {
    const host = makeResource({
      id: 'docker-host-2',
      type: 'docker-host',
      name: 'dh-2',
      displayName: 'Docker Host 2',
    });
    const container = makeResource({
      id: 'docker-host-2/abc123',
      type: 'app-container',
      name: 'nginx',
      displayName: 'nginx',
      parentId: 'docker-host-2',
    });

    const result = projectOverrides({
      rawConfig: {
        'docker:docker-host-2/abc123': {
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      containerRuntimeResources: [host],
      getChildren: (rid) => (rid === 'docker-host-2' ? [container] : []),
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'dockerContainer',
        name: 'nginx',
        node: 'Docker Host 2',
      }),
    ]);
  });

  it('creates dockerContainer override with container id without slash', () => {
    const host = makeResource({
      id: 'docker-host-3',
      type: 'docker-host',
      name: 'dh-3',
      displayName: 'Docker Host 3',
    });
    const container = makeResource({
      id: 'shortid456',
      type: 'app-container',
      name: 'redis',
      displayName: 'redis',
      parentId: 'docker-host-3',
    });

    const result = projectOverrides({
      rawConfig: {
        'docker:docker-host-3/shortid456': {
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      containerRuntimeResources: [host],
      getChildren: (rid) => (rid === 'docker-host-3' ? [container] : []),
    });

    expect(result).toHaveLength(1);
    expect(result[0].type).toBe('dockerContainer');
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — Docker key fallback (no map match)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — Docker key fallback', () => {
  it('creates dockerContainer when docker: key has host/containerId but no matching host', () => {
    const result = projectOverrides({
      rawConfig: {
        'docker:unknown-host/c-123': {
          disabled: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'docker:unknown-host/c-123',
        type: 'dockerContainer',
        name: 'c-123',
        node: 'unknown-host',
        disabled: true,
      }),
    ]);
  });

  it('creates dockerHost when docker: key has no containerId', () => {
    const result = projectOverrides({
      rawConfig: {
        'docker:lone-host': {
          disableConnectivity: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'docker:lone-host',
        type: 'dockerHost',
        name: 'lone-host',
        resourceType: 'Container Runtime',
        disableConnectivity: true,
      }),
    ]);
  });

  it('creates dockerHost with key as name when rest is empty', () => {
    const result = projectOverrides({
      rawConfig: {
        'docker:': {
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'docker:',
        type: 'dockerHost',
        name: 'docker:',
      }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — agent disk override
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — agent disk override', () => {
  it('creates agentDisk override with display name from matched agent', () => {
    const agent = makeResource({
      id: 'agent-disk-host',
      type: 'agent',
      name: 'node-1',
      displayName: 'Node One',
    });

    const result = projectOverrides({
      rawConfig: {
        'agent:agent-disk-host/disk:nvme-0n1': {
          disabled: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      agentResourceList: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'agentDisk',
        resourceType: 'Agent Disk',
        name: 'nvme/0n1',
        node: 'Node One',
        disabled: true,
      }),
    ]);
  });

  it('falls back to raw agentId for node when agent is not in agentMap', () => {
    const result = projectOverrides({
      rawConfig: {
        'agent:ghost-agent/disk:sda-1': {
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'agentDisk',
        name: 'sda/1',
        node: 'ghost-agent',
      }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — agent resource override
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — agent resource override', () => {
  it('creates agent override via agentMap with instance from platformData.agent.platform', () => {
    const agent = makeResource({
      id: 'agent-res-1',
      type: 'agent',
      name: 'node-a',
      displayName: 'Node A',
      platformData: { agent: { platform: 'linux' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'agent-res-1': {
          disabled: true,
          disableConnectivity: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      agentResourceList: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'agent-res-1',
        type: 'agent',
        resourceType: 'Agent',
        node: 'Node A',
        instance: 'linux',
        disabled: true,
        disableConnectivity: true,
      }),
    ]);
  });

  it('falls back to platformData.agent.osName for instance', () => {
    const agent = makeResource({
      id: 'agent-res-2',
      type: 'agent',
      name: 'node-b',
      displayName: 'Node B',
      platformData: { agent: { osName: 'Ubuntu 22.04' } },
    });

    const result = projectOverrides({
      rawConfig: { 'agent-res-2': cpuThreshold(90, 80) },
      agentResourceList: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ instance: 'Ubuntu 22.04' }),
    ]);
  });

  it('falls back to platformData.platform for instance', () => {
    const agent = makeResource({
      id: 'agent-res-3',
      type: 'agent',
      name: 'node-c',
      displayName: 'Node C',
      platformData: { platform: 'debian' },
    });

    const result = projectOverrides({
      rawConfig: { 'agent-res-3': cpuThreshold(90, 80) },
      agentResourceList: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ instance: 'debian' }),
    ]);
  });

  it('returns empty string for instance when no platform/os data exists', () => {
    const agent = makeResource({
      id: 'agent-res-4',
      type: 'agent',
      name: 'node-d',
      displayName: 'Node D',
    });

    const result = projectOverrides({
      rawConfig: { 'agent-res-4': cpuThreshold(90, 80) },
      agentResourceList: [agent],
    });

    expect(result).toEqual([
      expect.objectContaining({ instance: '' }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — PBS override
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — PBS override', () => {
  it('creates pbs override when pbs- key matches pbsInstanceById', () => {
    const result = projectOverrides({
      rawConfig: {
        'pbs-backup-1': {
          disableConnectivity: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      pbsInstanceById: new Map([['pbs-backup-1', makePBS('pbs-backup-1', 'Backup Server 1')]]),
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'pbs-backup-1',
        type: 'pbs',
        resourceType: 'PBS',
        name: 'Backup Server 1',
        disableConnectivity: true,
      }),
    ]);
  });

  it('skips override creation when pbs- key has no matching instance', () => {
    const result = projectOverrides({
      rawConfig: {
        'pbs-unknown': cpuThreshold(90, 80),
      },
      pbsInstanceById: new Map(),
    });

    expect(result).toEqual([]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — node resource override
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — node resource override', () => {
  it('creates agent override when key matches a node resource id', () => {
    const node = makeResource({
      id: 'node-resource-1',
      type: 'agent',
      name: 'pve-node-1',
      displayName: 'PVE Node 1',
    });

    const result = projectOverrides({
      rawConfig: {
        'node-resource-1': {
          disableConnectivity: true,
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      nodeResources: [node],
    });

    expect(result).toEqual([
      expect.objectContaining({
        id: 'node-resource-1',
        type: 'agent',
        resourceType: 'Agent',
        disableConnectivity: true,
      }),
    ]);
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — poweredOffSeverity branches
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — poweredOffSeverity', () => {
  it('passes through poweredOffSeverity critical for guest overrides', () => {
    const guest = makeResource({
      id: 'cluster-a:node-1:100',
      name: 'vm-100',
      displayName: 'vm-100',
      type: 'vm',
      proxmox: { vmid: 100, node: 'node-1', instance: 'cluster-a' },
      platformData: { proxmox: { vmid: 100, node: 'node-1', instance: 'cluster-a' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'guest:cluster-a:100': {
          poweredOffSeverity: 'critical',
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      vmResources: [guest],
    });

    expect(result).toHaveLength(1);
    expect(result[0].poweredOffSeverity).toBe('critical');
  });

  it('passes through poweredOffSeverity warning for guest overrides', () => {
    const guest = makeResource({
      id: 'cluster-a:node-1:101',
      name: 'vm-101',
      displayName: 'vm-101',
      type: 'vm',
      proxmox: { vmid: 101, node: 'node-1', instance: 'cluster-a' },
      platformData: { proxmox: { vmid: 101, node: 'node-1', instance: 'cluster-a' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'guest:cluster-a:101': {
          poweredOffSeverity: 'warning',
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      vmResources: [guest],
    });

    expect(result).toHaveLength(1);
    expect(result[0].poweredOffSeverity).toBe('warning');
  });

  it('sets poweredOffSeverity to undefined when value is not critical or warning', () => {
    const guest = makeResource({
      id: 'cluster-a:node-1:102',
      name: 'vm-102',
      displayName: 'vm-102',
      type: 'vm',
      proxmox: { vmid: 102, node: 'node-1', instance: 'cluster-a' },
      platformData: { proxmox: { vmid: 102, node: 'node-1', instance: 'cluster-a' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'guest:cluster-a:102': {
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      vmResources: [guest],
    });

    expect(result).toHaveLength(1);
    expect(result[0].poweredOffSeverity).toBeUndefined();
  });

  it('passes through poweredOffSeverity critical for docker container fallback', () => {
    const result = projectOverrides({
      rawConfig: {
        'docker:fallback-host/c-456': {
          poweredOffSeverity: 'critical',
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
    });

    expect(result).toHaveLength(1);
    expect(result[0].poweredOffSeverity).toBe('critical');
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — guest matching via guestMap (container)
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — guest override via guestMap', () => {
  it('projects container guest with resourceType Container', () => {
    const container = makeResource({
      id: 'cluster-b:node-2:200',
      name: 'lxc-200',
      displayName: 'lxc-200',
      type: 'system-container',
      proxmox: { vmid: 200, node: 'node-2', instance: 'cluster-b' },
      platformData: { proxmox: { vmid: 200, node: 'node-2', instance: 'cluster-b' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'guest:cluster-b:200': {
          disabled: true,
          backup: { enabled: true, warningDays: 1, criticalDays: 3 },
          cpu: { trigger: 85, clear: 75 },
        } as RawOverrideConfig,
      },
      containerResources: [container],
    });

    expect(result).toEqual([
      expect.objectContaining({
        type: 'guest',
        resourceType: 'Container',
        vmid: 200,
        node: 'node-2',
        instance: 'cluster-b',
        disabled: true,
      }),
    ]);
  });

  it('passes backup and snapshot config through for guest overrides', () => {
    const guest = makeResource({
      id: 'cluster-c:node-3:300',
      name: 'vm-300',
      displayName: 'vm-300',
      type: 'vm',
      proxmox: { vmid: 300, node: 'node-3', instance: 'cluster-c' },
      platformData: { proxmox: { vmid: 300, node: 'node-3', instance: 'cluster-c' } },
    });

    const result = projectOverrides({
      rawConfig: {
        'guest:cluster-c:300': {
          backup: { enabled: true, warningDays: 2, criticalDays: 7 },
          snapshot: { enabled: true, warningDays: 1, criticalDays: 5 },
          cpu: { trigger: 90, clear: 80 },
        } as RawOverrideConfig,
      },
      vmResources: [guest],
    });

    expect(result).toHaveLength(1);
    expect(result[0].backup).toEqual({ enabled: true, warningDays: 2, criticalDays: 7 });
    expect(result[0].snapshot).toEqual({ enabled: true, warningDays: 1, criticalDays: 5 });
  });

  it('matches guest via direct vmResources lookup when not in guestMap', () => {
    const guest = makeResource({
      id: 'direct-vm-id',
      name: 'direct-vm',
      displayName: 'direct-vm',
      type: 'vm',
    });

    const result = projectOverrides({
      rawConfig: {
        'direct-vm-id': cpuThreshold(90, 80),
      },
      vmResources: [guest],
    });

    expect(result).toHaveLength(1);
    expect(result[0].type).toBe('guest');
  });
});

// ---------------------------------------------------------------------------
// buildProjectedOverrides — alert platform node/instance resolution
// ---------------------------------------------------------------------------

describe('buildProjectedOverrides — alert platform node/instance resolution', () => {
  it('uses truenas.hostname for node on TrueNAS system resources', () => {
    const agent = makeResource({
      id: 'truenas-with-hostname',
      type: 'agent',
      name: 'tn',
      displayName: 'tn',
      platformType: 'truenas',
      truenas: { hostname: 'truenas.local' },
    });

    const result = projectOverrides({
      rawConfig: { 'truenas-with-hostname': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toHaveLength(1);
    expect(result[0].node).toBe('truenas.local');
  });

  it('uses vmware.runtimeHostName for node on VMware host resources', () => {
    const agent = makeResource({
      id: 'vmware-with-hostname',
      type: 'agent',
      name: 'esxi',
      displayName: 'esxi',
      platformType: 'vmware-vsphere',
      vmware: { runtimeHostName: 'esxi-01.local', connectionName: 'vc-01' },
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-with-hostname': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toHaveLength(1);
    expect(result[0].node).toBe('esxi-01.local');
    expect(result[0].instance).toBe('vc-01');
  });

  it('uses vmware.vcenterHost for node and instance when higher-precedence fields are absent', () => {
    const agent = makeResource({
      id: 'vmware-vcenter',
      type: 'agent',
      name: 'esxi-vc',
      displayName: 'esxi-vc',
      platformType: 'vmware-vsphere',
      vmware: { vcenterHost: 'vcenter.local' },
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-vcenter': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toHaveLength(1);
    expect(result[0].node).toBe('vcenter.local');
    expect(result[0].instance).toBe('vcenter.local');
  });

  it('uses literal vSphere for instance when vmware object exists but no connection fields', () => {
    const agent = makeResource({
      id: 'vmware-bare',
      type: 'agent',
      name: 'esxi-bare',
      displayName: 'esxi-bare',
      vmware: {},
    });

    const result = projectOverrides({
      rawConfig: { 'vmware-bare': cpuThreshold(90, 80) },
      allResources: [agent],
    });

    expect(result).toHaveLength(1);
    expect(result[0].instance).toBe('vSphere');
  });

  it('uses parentName for node when present', () => {
    const pod = makeResource({
      id: 'pod-with-parent',
      type: 'pod',
      name: 'pod-1',
      displayName: 'pod-1',
      parentName: 'worker-node-1',
      kubernetes: { namespace: 'default' },
    });

    const result = projectOverrides({
      rawConfig: { 'pod-with-parent': cpuThreshold(90, 80) },
      allResources: [pod],
    });

    expect(result).toHaveLength(1);
    expect(result[0].node).toBe('worker-node-1');
  });
});
