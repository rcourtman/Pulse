import { describe, expect, it } from 'vitest';
import type { Alert } from '@/types/api';
import type { Resource } from '@/types/resource';
import {
  getAgentMachineAlertOverrideKeys,
  getPhysicalDiskAlertResourceIds,
  getPhysicalDiskAlerts,
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

  describe('the alerts a disk drawer lists', () => {
    const alertOn = (id: string, resourceId: string, level: Alert['level'] = 'warning'): Alert =>
      ({
        id,
        type: 'disk-temperature',
        level,
        resourceId,
        resourceName: resourceId,
        node: 'tower',
        instance: 'lab',
        message: id,
        value: 0,
        threshold: 0,
        startTime: '2026-10-20T00:00:00Z',
        acknowledged: false,
      }) as Alert;
    const agentDisk = resource({
      id: 'disk-agent-sda',
      type: 'physical_disk',
      parentId: 'agent-tower',
      physicalDisk: { devPath: '/dev/sda' },
      canonicalIdentity: { aliases: ['lab:tower:disk:dev-sda', 'tower'] },
    });
    const active = {
      own: alertOn('own', 'disk-agent-sda'),
      pve: alertOn('pve', 'lab:tower:disk:dev-sda', 'critical'),
      agent: alertOn('agent', 'agent:host-tower/disk_temp:sda'),
      sibling: alertOn('sibling', 'agent:host-tower/disk_temp:sdb'),
      machine: alertOn('machine', 'agent:host-tower'),
      node: alertOn('node', 'tower'),
    };

    it('lists its own, its Proxmox disk key and its agent temperature key, most severe first', () => {
      expect(
        getPhysicalDiskAlerts(agentDisk, owners, active, true).map((alert) => alert.id),
      ).toEqual(['pve', 'own', 'agent']);
    });

    it("never lists a sibling disk's, its machine's or its node's alerts", () => {
      const ids = getPhysicalDiskAlerts(agentDisk, owners, active, true).map((alert) => alert.id);
      expect(ids).not.toContain('sibling');
      expect(ids).not.toContain('machine');
      expect(ids).not.toContain('node');
    });

    it('lists nothing while alert detection is off', () => {
      expect(getPhysicalDiskAlerts(agentDisk, owners, active, false)).toEqual([]);
    });

    it("takes a known reporting agent's temperature key without a loaded machine", () => {
      // A guest drawer lists the guest's own disks: no agent machine is loaded
      // there, but the guest names the agent that reports their temperatures.
      const ids = getPhysicalDiskAlerts(agentDisk, new Map(), active, true, 'host-tower').map(
        (alert) => alert.id,
      );
      expect(ids).toEqual(['pve', 'own', 'agent']);
    });
  });
});
