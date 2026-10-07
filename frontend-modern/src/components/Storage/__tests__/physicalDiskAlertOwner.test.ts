import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineAlertOverrideKeys,
  getPhysicalDiskAlertResourceIds,
  indexPhysicalDiskOwners,
} from '@/components/Storage/physicalDiskAlertOwner';

const resource = (fields: Record<string, unknown>): Resource =>
  ({ name: String(fields.id), status: 'online', lastSeen: 0, ...fields }) as unknown as Resource;

const agentMachine = resource({
  id: 'agent-tower',
  type: 'agent',
  agent: { agentId: 'host-tower' },
  proxmox: { sourceId: 'lab-tower', nodeName: 'tower' },
});
const nodeWithoutAgent = resource({
  id: 'node-bare',
  type: 'agent',
  proxmox: { sourceId: 'lab-bare', nodeName: 'bare' },
});
const unraidArray = resource({ id: 'storage-array', type: 'storage', parentId: 'agent-tower' });
const disk = (parentId?: string) =>
  resource({ id: `disk-${parentId}`, type: 'physical_disk', parentId });

describe('physical disk alert owner', () => {
  const owners = indexPhysicalDiskOwners([
    agentMachine,
    nodeWithoutAgent,
    unraidArray,
    disk('agent-tower'),
  ]);

  it('indexes only agent machines and storage pools', () => {
    expect([...owners.keys()].sort()).toEqual(['agent-tower', 'storage-array']);
  });

  it('reads the reporting machine override keys in the order CheckHost reads them', () => {
    // Agent ID, the canonical ID it resolves to, then the merged node.
    const expected = ['host-tower', 'agent-tower', 'lab-tower'];
    expect(getAgentMachineAlertOverrideKeys(agentMachine)).toEqual(expected);
    expect(getPhysicalDiskAlertResourceIds(disk('agent-tower'), owners)).toEqual(expected);
    // An Unraid array or cache disk hangs off its pool, which hangs off the machine.
    expect(getPhysicalDiskAlertResourceIds(disk('storage-array'), owners)).toEqual(expected);
  });

  it('follows an agent merged into a guest to the guest override keys', () => {
    const guest = resource({
      id: 'vm-unraid',
      type: 'vm',
      agent: { agentId: 'host-unraid' },
      proxmox: { instance: 'lab', node: 'tower', nodeName: 'tower', vmid: 105 },
    });
    const keys = getAgentMachineAlertOverrideKeys(guest);
    expect(keys.slice(0, 2)).toEqual(['host-unraid', 'vm-unraid']);
    expect(keys).toContain('lab:tower:105');
  });

  it('leaves disks no loaded agent reports on the per-type thresholds', () => {
    expect(getPhysicalDiskAlertResourceIds(disk('node-bare'), owners)).toEqual([]);
    expect(getPhysicalDiskAlertResourceIds(disk('vm-not-loaded'), owners)).toEqual([]);
    expect(getPhysicalDiskAlertResourceIds(disk(undefined), owners)).toEqual([]);
  });
});
