import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';

import {
  buildProxmoxPageModel,
  getProxmoxProviderScope,
  isProxmoxChildOfNode,
} from '../proxmoxPageModel';

// ---------------------------------------------------------------------------
// Fixture builder — mirrors the sibling proxmoxPageModel.test.ts factory so
// import style and default platform posture stay aligned.
// ---------------------------------------------------------------------------

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'proxmox-pve',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

// ===========================================================================
// getProxmoxProviderScope — the `||` fallback chain. The happy-path specs
// only ever exercise resources whose `proxmox.instance` (or, transitively,
// `platformId`) short-circuits the chain. The cold arms are the later
// operands: clusterId, parentId, and the final id fall-through.
// ===========================================================================

describe('getProxmoxProviderScope fallback chain', () => {
  it('falls back to clusterId when proxmox.instance and platformId are absent', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformId: '',
      clusterId: 'cluster-9',
    });
    expect(getProxmoxProviderScope(resource)).toBe('cluster-9');
  });

  it('falls back to parentId when instance, platformId, and clusterId are absent', () => {
    const resource = makeResource({
      id: 'r',
      type: 'agent',
      platformId: '',
      parentId: 'host-node',
    });
    expect(getProxmoxProviderScope(resource)).toBe('host-node');
  });

  it('returns resource.id as the final fallback when every scope field is absent', () => {
    const resource = makeResource({ id: 'last-resort', type: 'agent', platformId: '' });
    expect(getProxmoxProviderScope(resource)).toBe('last-resort');
  });
});

// ===========================================================================
// isProxmoxChildOfNode — the parentId early-return guard. Existing specs
// relate guests to nodes via nodeName/scope only, so the
// `child.parentId && child.parentId === node.id` arm is never taken.
// ===========================================================================

describe('isProxmoxChildOfNode parentId early-return', () => {
  it('returns true when child.parentId matches node.id without consulting node names', () => {
    const node = makeResource({
      id: 'node-1',
      type: 'agent',
      proxmox: { nodeName: 'node-1' },
    });
    // Deliberately mismatched nodeName to prove the parentId arm wins outright.
    const child = makeResource({
      id: 'vm-1',
      type: 'vm',
      parentId: 'node-1',
      proxmox: { nodeName: 'unrelated-host', vmid: 101 },
    });
    expect(isProxmoxChildOfNode(child, node)).toBe(true);
  });

  it('does not early-return when child.parentId is set but differs from node.id', () => {
    const node = makeResource({
      id: 'node-1',
      type: 'agent',
      proxmox: { nodeName: 'node-1' },
    });
    const child = makeResource({
      id: 'vm-2',
      type: 'vm',
      parentId: 'other-node',
      proxmox: { nodeName: 'different-host', vmid: 102 },
    });
    expect(isProxmoxChildOfNode(child, node)).toBe(false);
  });
});

// ===========================================================================
// buildProxmoxPageModel — the cluster-group sort comparator. The happy-path
// specs always insert named clusters before the standalone bucket, so V8's
// sort only ever calls the comparator with (standalone, non-standalone) pairs
// and only the `1` (consequent) arm fires. Inserting a standalone node FIRST
// makes the comparator receive (non-standalone, standalone) pairs and return
// the `-1` (alternate) arm.
// ===========================================================================

describe('buildProxmoxPageModel standalone-first sort comparator', () => {
  it('exercises the -1 arm of the sort comparator when the standalone group is inserted first', () => {
    // A node carrying no cluster hint resolves to 'Standalone'. Placed before a
    // named-cluster node, the standalone bucket lands first in the insertion
    // order, flipping the comparator argument direction.
    const standaloneNode = makeResource({
      id: 'solo-node',
      type: 'agent',
      proxmox: { nodeName: 'solo-node' },
    });
    const namedNode = makeResource({
      id: 'named-node',
      type: 'agent',
      proxmox: { nodeName: 'named-node', clusterName: 'named' },
    });

    const model = buildProxmoxPageModel([standaloneNode, namedNode]);

    // Standalone was inserted first; the sort still places named before standalone.
    expect(model.clusterGroups.map((group) => group.label)).toEqual(['named', 'Standalone']);
    expect(model.clusterGroups.map((group) => group.id)).toEqual([
      'lab::named',
      'lab::__standalone__',
    ]);
    // The standalone bucket never counts as a real cluster.
    expect(model.summary.clusterCount).toBe(1);
  });
});
