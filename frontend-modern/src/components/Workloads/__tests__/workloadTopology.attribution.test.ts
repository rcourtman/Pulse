import { describe, expect, it } from 'vitest';
import type { Node } from '@/types/api';
import type { WorkloadGuest } from '@/types/workloads';
import { buildGuestParentNodeMapFromNodes } from '../workloadTopology';
import { groupWorkloads } from '../workloadSelectors';
import { buildWorkloadNodeOptions } from '../workloadRouteModel';

const guests = [
  {
    id: 'lab-east-pve1-101',
    vmid: 101,
    node: 'pve1',
    instance: 'lab-east',
    type: 'qemu',
    workloadType: 'vm',
    platformScopes: ['proxmox-pve'],
  },
  {
    id: 'lab-east-pve1-101',
    vmid: 101,
    node: 'east-pve1',
    instance: 'lab',
    type: 'qemu',
    workloadType: 'vm',
    platformScopes: ['proxmox-pve'],
  },
] as WorkloadGuest[];
const nodes = [
  {
    id: 'native-a',
    name: 'pve1',
    instance: 'lab-east',
    status: 'offline',
    memory: { total: 8192 },
  },
  {
    id: 'native-b',
    name: 'east-pve1',
    instance: 'lab',
    status: 'online',
    memory: { total: 32768 },
  },
] as Node[];

describe('Supplied node attribution (existing API regression controls)', () => {
  it('does not combine distinct supplied instance/node pairs into one group', () => {
    expect(
      Object.values(groupWorkloads(guests, 'grouped', null)).map((group) => group.length),
    ).toEqual([1, 1]);
  });
  it('keeps both independently selectable nodes instead of losing an option', () => {
    expect(buildWorkloadNodeOptions(guests, nodes)).toHaveLength(2);
  });
  it.each([nodes, [...nodes].reverse()])(
    'preserves each supplied parent regardless of inventory order',
    (...inventory) => {
      const parents = buildGuestParentNodeMapFromNodes(guests, inventory);
      expect(parents['lab-east:pve1:101']).toBe(nodes[0]);
      expect(parents['lab:east-pve1:101']).toBe(nodes[1]);
    },
  );
  it('does not substitute an inferred guest-ID parent when its explicit source fields disagree', () => {
    const contradictory = { ...guests[1], id: 'native-a-101' };
    const parents = buildGuestParentNodeMapFromNodes([contradictory], nodes);
    expect(parents['lab:east-pve1:101']).toBe(nodes[1]);
  });
  it('cannot give a vSphere VM Proxmox parent state from matching labels', () => {
    const vm = { ...guests[0], platformScopes: ['vmware-vsphere'] };
    expect(buildGuestParentNodeMapFromNodes([vm], nodes)['lab-east:pve1:101']).toBeUndefined();
  });
});
