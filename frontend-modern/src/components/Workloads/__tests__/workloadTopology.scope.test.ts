import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { getCanonicalWorkloadId } from '@/utils/workloads';
import {
  buildNodeByInstance,
  buildGuestParentNodeMapFromNodes,
  getWorkloadHistoryNodeId,
  legacyWorkloadNodeScopeId,
  readWorkloadNodeScope,
  workloadNodeScopeId,
} from '../workloadTopology';
import { filterWorkloads, getWorkloadGroupKey, groupWorkloads } from '../workloadSelectors';
import { buildWorkloadNodeOptions, resolveWorkloadNodeHint } from '../workloadRouteModel';

const guest = (instance: string, node: string, extra: Partial<WorkloadGuest> = {}) =>
  ({
    id: 'lab-east-pve1-101',
    instance,
    node,
    vmid: 101,
    name: `${instance}/${node}`,
    type: 'qemu',
    workloadType: 'vm',
    status: 'running',
    platformScopes: ['proxmox-pve'],
    ...extra,
  }) as WorkloadGuest;
const node = (id: string, instance: string, name: string, total: number, status = 'online') =>
  ({
    id,
    instance,
    name,
    status,
    uptime: status === 'online' ? 100 : 0,
    memory: { total, used: 0, free: total, usage: 0 },
  }) as Node;
const guests = [guest('lab-east', 'pve1'), guest('lab', 'east-pve1')];
const nodes = [
  node('native-a', 'lab-east', 'pve1', 8192, 'offline'),
  node('native-b', 'lab', 'east-pve1', 32768),
];
const filtered = (inventory: WorkloadGuest[], selectedNode: string) =>
  filterWorkloads({
    guests: inventory,
    selectedNode,
    viewMode: 'all',
    statusMode: 'all',
    searchTerm: '',
    selectedHostHint: null,
    selectedKubernetesContext: null,
  });

describe('Workloads node attribution', () => {
  it.each([guests, [...guests].reverse()])(
    'keeps delimiter-colliding groups and filters separate in either order',
    (...inventory) => {
      const groups = groupWorkloads(inventory, 'grouped', null);
      expect(Object.keys(groups)).toHaveLength(2);
      for (const original of guests) {
        const scope = workloadNodeScopeId(original);
        expect(groups[scope]).toEqual([original]);
        expect(filtered(inventory, scope)).toEqual([original]);
        expect(original.id).toBe('lab-east-pve1-101');
        expect(getCanonicalWorkloadId(original)).toBe(`${original.instance}:${original.node}:101`);
      }
      expect(filtered(inventory, 'lab-east-pve1')).toEqual([]);
      expect(filtered([inventory[0]], 'lab-east-pve1')).toEqual([inventory[0]]);
    },
  );

  it.each([nodes, [...nodes].reverse()])(
    'assigns status and host memory to the exact tuple, not the first legacy alias',
    (...inventory) => {
      const parents = buildGuestParentNodeMapFromNodes(guests, inventory);
      expect(parents['lab-east:pve1:101']).toBe(nodes[0]);
      expect(parents['lab:east-pve1:101']).toBe(nodes[1]);
      expect(parents['lab-east:pve1:101']?.status).toBe('offline');
      expect(parents['lab:east-pve1:101']?.memory.total).toBe(32768);
    },
  );

  it('does not let a contradictory inferred ID override the supplied parent fields', () => {
    const wrongPrefix = guest('lab', 'east-pve1', { id: 'native-a-101' });
    expect(buildGuestParentNodeMapFromNodes([wrongPrefix], nodes)['lab:east-pve1:101']).toBe(
      nodes[1],
    );
    const missing = guest('other', 'absent', { id: 'native-a-101' });
    expect(buildGuestParentNodeMapFromNodes([missing], nodes)['other:absent:101']).toBeUndefined();
  });

  it('retains a unique consistent legacy-ID fallback for missing parent fields', () => {
    const legacy = guest('', '', { id: 'native-a-101' });
    expect(buildGuestParentNodeMapFromNodes([legacy], nodes)[getCanonicalWorkloadId(legacy)]).toBe(
      nodes[0],
    );
    for (const id of ['native-a-102', 'native-a-not-a-vmid']) {
      const unrelatedId = { ...legacy, id };
      expect(
        buildGuestParentNodeMapFromNodes([unrelatedId], nodes)[getCanonicalWorkloadId(unrelatedId)],
      ).toBeUndefined();
    }
  });

  it('leaves duplicated tuples and IDs unknown instead of choosing by inventory order', () => {
    const duplicate = node('native-c', 'lab-east', 'pve1', 65536);
    const map = buildNodeByInstance([...nodes, duplicate]);
    expect(map[workloadNodeScopeId(guests[0])]).toBeUndefined();
    expect(
      buildGuestParentNodeMapFromNodes([guests[0]], [...nodes, duplicate])['lab-east:pve1:101'],
    ).toBeUndefined();
    const shared = nodes.map((n) => ({ ...n, id: 'same-native-id' }));
    expect(buildNodeByInstance(shared)['same-native-id']).toBeUndefined();
    expect(buildGuestParentNodeMapFromNodes(guests, shared)['lab:east-pve1:101']).toBe(shared[1]);
  });

  it('cannot assign a Proxmox node to an unrelated platform or container by matching labels', () => {
    const vsphere = guest('lab-east', 'pve1', { platformScopes: ['vmware-vsphere'] });
    const container = guest('lab-east', 'pve1', {
      type: 'app-container',
      workloadType: 'app-container',
    });
    expect(buildGuestParentNodeMapFromNodes([vsphere, container], nodes)).toEqual({});
    expect(getWorkloadGroupKey(vsphere)).not.toBe(getWorkloadGroupKey(guests[0]));
  });

  it('encodes delimiters, percent escapes and case without normalising independent identities', () => {
    const pairs = [
      { instance: 'a|b', node: 'c' },
      { instance: 'a', node: 'b|c' },
      { instance: 'a%7Cb', node: 'c' },
      { instance: 'A|b', node: 'c' },
      { instance: 'a::b', node: 'c/d' },
      { instance: '', node: 'pve1' },
    ];
    expect(new Set(pairs.map(workloadNodeScopeId)).size).toBe(pairs.length);
    for (const pair of pairs)
      expect(readWorkloadNodeScope(workloadNodeScopeId(pair))).toEqual(pair);
    expect(workloadNodeScopeId({ instance: ' ', node: '' })).toBe('');
    for (const invalid of ['node|a|%XX', 'node|a|b|c', 'node||', 'node|a|b%20']) {
      expect(readWorkloadNodeScope(invalid)).toBeNull();
    }
    expect(legacyWorkloadNodeScopeId(workloadNodeScopeId(guests[0]))).toBe('lab-east-pve1');
  });

  it('keeps filter options distinct and only promotes unambiguous host hints or bookmarks', () => {
    const options = buildWorkloadNodeOptions(guests, nodes);
    expect(options).toHaveLength(2);
    for (const original of guests) {
      expect(resolveWorkloadNodeHint(options, workloadNodeScopeId(original))?.value).toBe(
        workloadNodeScopeId(original),
      );
    }
    expect(resolveWorkloadNodeHint(options, 'lab-east-pve1')).toBeNull();
    expect(resolveWorkloadNodeHint(options, 'pve1')).toEqual(
      options.find((o) => o.label === 'pve1'),
    );
    expect(resolveWorkloadNodeHint(options, 'pve')).toBeNull();
    expect(
      resolveWorkloadNodeHint(buildWorkloadNodeOptions([guests[0]]), 'lab-east-pve1')?.value,
    ).toBe(workloadNodeScopeId(guests[0]));
    const repeated = buildWorkloadNodeOptions([guest('one', 'pve1'), guest('two', 'pve1')]);
    expect(resolveWorkloadNodeHint(repeated, 'pve1')).toBeNull();
    expect(resolveWorkloadNodeHint(repeated, 'pve1 (two)')?.value).toBe(
      workloadNodeScopeId({ instance: 'two', node: 'pve1' }),
    );
  });

  it('maps UI selections to the existing native charts ID without choosing an ambiguous native ID', () => {
    expect(getWorkloadHistoryNodeId(workloadNodeScopeId(guests[1]), nodes)).toBe('native-b');
    expect(
      getWorkloadHistoryNodeId(
        workloadNodeScopeId(guests[1]),
        nodes.map((n) => ({ ...n, id: 'same' })),
      ),
    ).toBeUndefined();
    expect(getWorkloadHistoryNodeId(workloadNodeScopeId(guests[1]), [])).toBeUndefined();
    expect(getWorkloadHistoryNodeId('native-b', nodes)).toBe('native-b');
    expect(getWorkloadHistoryNodeId('')).toBeUndefined();
  });

  it('treats prototype-like native IDs as own entries, not inherited parents', () => {
    const unusual = node('__proto__', 'lab-east', 'pve1', 8192);
    const map = buildNodeByInstance([unusual]);
    expect(Object.getPrototypeOf(map)).toBeNull();
    expect(map.__proto__).toBe(unusual);
    expect(map.toString).toBeUndefined();
  });
});
