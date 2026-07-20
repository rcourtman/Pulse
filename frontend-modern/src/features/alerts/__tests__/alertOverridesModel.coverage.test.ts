import { describe, expect, it } from 'vitest';

import type { RawOverrideConfig } from '@/types/alerts';
import type { Resource } from '@/types/resource';

import {
  buildProjectedOverrides,
  normalizeRawOverridesConfig,
  storageOverrideActionId,
  storageOverrideIdCandidates,
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

describe('alertOverridesModel coverage', () => {
  describe('storageOverrideIdCandidates', () => {
    it('prepends metricsTarget resource id when resourceType is storage', () => {
      expect(
        storageOverrideIdCandidates(
          makeResource({
            id: 'storage-hash',
            metricsTarget: { resourceType: 'storage', resourceId: 'canonical-id' },
          }),
        ),
      ).toEqual(['canonical-id', 'storage-hash']);
    });

    it('returns only resource id when metricsTarget resourceType is not storage', () => {
      expect(
        storageOverrideIdCandidates(
          makeResource({
            id: 'storage-hash',
            metricsTarget: { resourceType: 'agent', resourceId: 'other' },
          }),
        ),
      ).toEqual(['storage-hash']);
    });

    it('returns only resource id when metricsTarget is absent', () => {
      expect(storageOverrideIdCandidates(makeResource({ id: 'storage-hash' }))).toEqual([
        'storage-hash',
      ]);
    });
  });

  describe('storageOverrideActionId', () => {
    it('returns the first candidate (metricsTarget resource id)', () => {
      expect(
        storageOverrideActionId(
          makeResource({
            id: 'storage-hash',
            metricsTarget: { resourceType: 'storage', resourceId: 'canonical-id' },
          }),
        ),
      ).toBe('canonical-id');
    });

    it('falls back to resource id when no valid candidates exist', () => {
      expect(storageOverrideActionId(makeResource({ id: '' }))).toBe('');
    });
  });

  describe('buildSharedStorageLegacyKeyMap (via normalizeRawOverridesConfig)', () => {
    it('skips non-shared storage resources', () => {
      const storage = makeResource({
        id: 's1',
        name: 'local-lvm',
        type: 'storage',
        platformId: 'Main',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: false, nodes: ['node-a'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'Main-node-a-local-lvm': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'Main-node-a-local-lvm': cpuThreshold(90, 80) });
    });

    it('skips resources with whitespace-only name', () => {
      const storage = makeResource({
        id: 's1',
        name: '   ',
        type: 'storage',
        platformId: 'Main',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true, nodes: ['node-a'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'Main-node-a-': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'Main-node-a-': cpuThreshold(90, 80) });
    });

    it('skips resources with empty canonical id', () => {
      const storage = makeResource({
        id: '',
        name: 'ceph-pool',
        type: 'storage',
        platformId: 'Main',
        storage: { shared: true, nodes: ['node-a'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'Main-node-a-ceph-pool': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'Main-node-a-ceph-pool': cpuThreshold(90, 80) });
    });

    it('uses platformId as instance when proxmox.instance is absent', () => {
      const storage = makeResource({
        id: 's1',
        name: 'nfs-share',
        type: 'storage',
        platformId: 'DC1',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true, nodes: ['node-a'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'DC1-node-a-nfs-share': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'canonical-1': cpuThreshold(90, 80) });
    });

    it('does not double-prefix when node already starts with instance', () => {
      const storage = makeResource({
        id: 's1',
        name: 'ceph-pool',
        type: 'storage',
        platformId: 'pve1',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true, nodes: ['pve1-node-a'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'pve1-node-a-ceph-pool': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'canonical-1': cpuThreshold(90, 80) });
    });

    it('skips empty and whitespace-only node entries', () => {
      const storage = makeResource({
        id: 's1',
        name: 'ceph-pool',
        type: 'storage',
        platformId: 'Main',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true, nodes: ['valid-node', '', '  '] },
      });

      expect(
        normalizeRawOverridesConfig({ 'Main-valid-node-ceph-pool': cpuThreshold(90, 80) }, [
          storage,
        ]),
      ).toEqual({ 'canonical-1': cpuThreshold(90, 80) });
    });

    it('produces no legacy keys when storage has no nodes array', () => {
      const storage = makeResource({
        id: 's1',
        name: 'ceph-pool',
        type: 'storage',
        platformId: 'Main',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true },
      });

      expect(
        normalizeRawOverridesConfig({ 'Main-node-a-ceph-pool': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'Main-node-a-ceph-pool': cpuThreshold(90, 80) });
    });

    it('uses bare node name when both proxmox.instance and platformId are empty', () => {
      const storage = makeResource({
        id: 's1',
        name: 'ceph-pool',
        type: 'storage',
        platformId: '',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        storage: { shared: true, nodes: ['solo-node'] },
      });

      expect(
        normalizeRawOverridesConfig({ 'solo-node-ceph-pool': cpuThreshold(90, 80) }, [storage]),
      ).toEqual({ 'canonical-1': cpuThreshold(90, 80) });
    });
  });

  describe('normalizeRawOverridesConfig (malformed/empty shapes)', () => {
    it('returns empty object for empty input', () => {
      expect(normalizeRawOverridesConfig({})).toEqual({});
    });

    it('passes through plain keys unchanged', () => {
      expect(normalizeRawOverridesConfig({ 'plain-key': cpuThreshold(90, 80) })).toEqual({
        'plain-key': cpuThreshold(90, 80),
      });
    });

    it('normalizes disk labels that collapse to empty into unknown', () => {
      expect(
        normalizeRawOverridesConfig({
          'agent:agent-1/disk:!!!': cpuThreshold(90, 80),
        }),
      ).toEqual({
        'agent:agent-1/disk:unknown': cpuThreshold(90, 80),
      });
    });

    it('resolves canonical guest key regardless of insertion order', () => {
      expect(
        normalizeRawOverridesConfig({
          'cluster-a:node-1:100': cpuThreshold(95, 90),
          'guest:cluster-a:100': cpuThreshold(70, 65),
        }),
      ).toEqual({
        'guest:cluster-a:100': cpuThreshold(70, 65),
      });
    });
  });

  describe('alertResourceIdCandidates (via buildProjectedOverrides)', () => {
    it('resolves alert platform overrides through metricsTarget resource id candidate', () => {
      const cluster = makeResource({
        id: 'k8s-internal-uid',
        type: 'k8s-cluster',
        name: 'prod-cluster',
        displayName: 'prod-cluster',
        platformId: 'k8s-platform-1',
        metricsTarget: { resourceType: 'k8s-cluster', resourceId: 'k8s-metrics-alias' },
        kubernetes: { clusterName: 'prod-cluster' },
      });

      const result = buildProjectedOverrides({
        rawConfig: { 'k8s-metrics-alias': cpuThreshold(90, 80) },
        nodeResources: [],
        vmResources: [],
        containerResources: [],
        storageResources: [],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
        allResources: [cluster],
      });

      expect(result).toEqual([
        expect.objectContaining({
          id: 'k8s-metrics-alias',
          type: 'kubernetesCluster',
          resourceType: 'Kubernetes Cluster',
          instance: 'prod-cluster',
        }),
      ]);
    });

    it('resolves alert platform overrides through platformId candidate', () => {
      const cluster = makeResource({
        id: 'k8s-internal-uid',
        type: 'k8s-cluster',
        name: 'prod-cluster',
        displayName: 'prod-cluster',
        platformId: 'k8s-platform-1',
        kubernetes: { clusterName: 'prod-cluster' },
      });

      const result = buildProjectedOverrides({
        rawConfig: { 'k8s-platform-1': cpuThreshold(90, 80) },
        nodeResources: [],
        vmResources: [],
        containerResources: [],
        storageResources: [],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
        allResources: [cluster],
      });

      expect(result).toEqual([
        expect.objectContaining({
          id: 'k8s-platform-1',
          type: 'kubernetesCluster',
        }),
      ]);
    });
  });

  describe('upsertProjectedOverride (via buildProjectedOverrides)', () => {
    it('inserts separate overrides for distinct projected ids', () => {
      const guest1 = makeResource({
        id: 'cluster-a:node-1:100',
        name: 'vm-100',
        type: 'vm',
        proxmox: { vmid: 100, node: 'node-1', instance: 'cluster-a' },
        platformData: { proxmox: { vmid: 100, node: 'node-1', instance: 'cluster-a' } },
      });
      const guest2 = makeResource({
        id: 'cluster-a:node-1:101',
        name: 'vm-101',
        type: 'vm',
        proxmox: { vmid: 101, node: 'node-1', instance: 'cluster-a' },
        platformData: { proxmox: { vmid: 101, node: 'node-1', instance: 'cluster-a' } },
      });

      const result = buildProjectedOverrides({
        rawConfig: {
          'guest:cluster-a:100': cpuThreshold(70, 65),
          'guest:cluster-a:101': cpuThreshold(80, 75),
        },
        nodeResources: [],
        vmResources: [guest1, guest2],
        containerResources: [],
        storageResources: [],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
      });

      expect(result).toHaveLength(2);
      expect(result.map((o) => o.id)).toEqual(
        expect.arrayContaining(['guest:cluster-a:100', 'guest:cluster-a:101']),
      );
    });

    it('updates existing override when multiple storage keys resolve to the same canonical id', () => {
      const storage = makeResource({
        id: 'storage-hash',
        name: 'ceph-pool',
        displayName: 'ceph-pool',
        type: 'storage',
        platformId: 'Main',
        metricsTarget: { resourceType: 'storage', resourceId: 'canonical-1' },
        platformData: { node: 'cluster', instance: 'Main' },
      });

      const result = buildProjectedOverrides({
        rawConfig: {
          'canonical-1': cpuThreshold(70, 65),
          'storage-hash': cpuThreshold(95, 90),
        },
        nodeResources: [],
        vmResources: [],
        containerResources: [],
        storageResources: [storage],
        agentResourceList: [],
        containerRuntimeResources: [],
        getChildren: () => [],
        pbsInstanceById: new Map(),
      });

      expect(result).toHaveLength(1);
      expect(result[0].id).toBe('canonical-1');
      expect(result[0].thresholds.cpu).toBe(95);
    });
  });

  describe('buildProjectedOverrides edge cases', () => {
    it('skips raw config keys that match no known resource', () => {
      expect(
        buildProjectedOverrides({
          rawConfig: { 'unknown-key': cpuThreshold(90, 80) },
          nodeResources: [],
          vmResources: [],
          containerResources: [],
          storageResources: [],
          agentResourceList: [],
          containerRuntimeResources: [],
          getChildren: () => [],
          pbsInstanceById: new Map(),
        }),
      ).toEqual([]);
    });
  });
});
