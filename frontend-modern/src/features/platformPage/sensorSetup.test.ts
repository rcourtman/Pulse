import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { collectOutdatedSensorSetupNodes } from './sensorSetup';

const node = (over: Partial<Resource> & { proxmox?: Resource['proxmox'] }): Resource =>
  ({ id: 'node-1', name: 'pve1', type: 'node', ...over }) as Resource;

const disk = (over: {
  id?: string;
  parentId?: string;
  nodeName?: string;
  diskType?: string;
  temperature?: number;
}): Resource =>
  ({
    id: over.id ?? 'disk-1',
    name: 'disk',
    type: 'physical_disk',
    parentId: over.parentId,
    physicalDisk: {
      diskType: over.diskType ?? 'sata',
      temperature: over.temperature ?? 0,
    },
    platformData: over.nodeName ? { proxmox: { nodeName: over.nodeName } } : {},
  }) as unknown as Resource;

const legacyNode = (over: Partial<Resource> = {}): Resource =>
  node({
    proxmox: {
      node: 'pve1',
      temperatureDetails: { available: true, legacySensorsFormat: true },
    },
    ...over,
  });

describe('collectOutdatedSensorSetupNodes', () => {
  it('flags a legacy-format node with a SATA disk missing a temperature', () => {
    const result = collectOutdatedSensorSetupNodes(
      [legacyNode()],
      [disk({ nodeName: 'pve1', diskType: 'sata' })],
    );
    expect(result).toEqual([{ id: 'node-1', name: 'pve1' }]);
  });

  it('matches disks to nodes by parentId as well as node name', () => {
    const result = collectOutdatedSensorSetupNodes(
      [legacyNode()],
      [disk({ parentId: 'node-1', diskType: 'sas' })],
    );
    expect(result).toEqual([{ id: 'node-1', name: 'pve1' }]);
  });

  it('ignores nodes whose payload is the wrapper format', () => {
    const wrapperNode = node({
      proxmox: { node: 'pve1', temperatureDetails: { available: true } },
    });
    expect(
      collectOutdatedSensorSetupNodes([wrapperNode], [disk({ nodeName: 'pve1' })]),
    ).toEqual([]);
  });

  it('ignores legacy nodes when temperature collection did not succeed', () => {
    const unavailableNode = node({
      proxmox: {
        node: 'pve1',
        temperatureDetails: { available: false, legacySensorsFormat: true },
      },
    });
    expect(
      collectOutdatedSensorSetupNodes([unavailableNode], [disk({ nodeName: 'pve1' })]),
    ).toEqual([]);
  });

  it('ignores legacy nodes whose SATA/SAS disks already have temperatures', () => {
    expect(
      collectOutdatedSensorSetupNodes(
        [legacyNode()],
        [disk({ nodeName: 'pve1', diskType: 'sata', temperature: 34 })],
      ),
    ).toEqual([]);
  });

  it('ignores legacy nodes with only NVMe disks missing temperatures', () => {
    expect(
      collectOutdatedSensorSetupNodes(
        [legacyNode()],
        [disk({ nodeName: 'pve1', diskType: 'nvme' })],
      ),
    ).toEqual([]);
  });

  it('ignores disks that belong to a different node', () => {
    expect(
      collectOutdatedSensorSetupNodes([legacyNode()], [disk({ nodeName: 'pve2' })]),
    ).toEqual([]);
  });

  it('sorts multiple affected nodes by name', () => {
    const nodes = [
      node({
        id: 'node-b',
        name: 'pve-b',
        proxmox: {
          node: 'pve-b',
          temperatureDetails: { available: true, legacySensorsFormat: true },
        },
      }),
      node({
        id: 'node-a',
        name: 'pve-a',
        proxmox: {
          node: 'pve-a',
          temperatureDetails: { available: true, legacySensorsFormat: true },
        },
      }),
    ];
    const disks = [
      disk({ id: 'disk-a', nodeName: 'pve-a' }),
      disk({ id: 'disk-b', nodeName: 'pve-b' }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes, disks).map((n) => n.name)).toEqual([
      'pve-a',
      'pve-b',
    ]);
  });
});
