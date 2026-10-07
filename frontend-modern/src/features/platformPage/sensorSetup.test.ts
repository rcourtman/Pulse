import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { collectOutdatedSensorSetupNodes } from './sensorSetup';

const node = (over: Partial<Resource> & { proxmox?: Resource['proxmox'] }): Resource =>
  ({ id: 'node-1', name: 'pve1', type: 'agent', ...over }) as Resource;

const flaggedNode = (over: Partial<Resource> = {}): Resource =>
  node({
    proxmox: {
      node: 'pve1',
      temperatureDetails: { available: true, legacySensorsFormat: true },
      sensorSetupOutdated: true,
    },
    ...over,
  });

describe('collectOutdatedSensorSetupNodes', () => {
  it('lists a node carrying the registry verdict without any disk rows', () => {
    expect(collectOutdatedSensorSetupNodes([flaggedNode()])).toEqual([
      { id: 'node-1', name: 'pve1' },
    ]);
  });

  it('leaves a legacy-format node the registry did not flag', () => {
    // The registry clears the verdict when every SATA/SAS disk under the node
    // has a current temperature, for example from a linked host agent.
    const reading = node({
      proxmox: {
        node: 'pve1',
        temperatureDetails: { available: true, legacySensorsFormat: true },
      },
    });
    expect(collectOutdatedSensorSetupNodes([reading])).toEqual([]);
  });

  it('names a node by its Proxmox node, then node name, then resource name, trimmed', () => {
    const nodes = [
      node({ id: 'by-node', proxmox: { node: '  alpha  ', sensorSetupOutdated: true } }),
      node({ id: 'by-node-name', proxmox: { nodeName: 'bravo', sensorSetupOutdated: true } }),
      node({ id: 'by-name', name: 'charlie', proxmox: { sensorSetupOutdated: true } }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes)).toEqual([
      { id: 'by-node', name: 'alpha' },
      { id: 'by-node-name', name: 'bravo' },
      { id: 'by-name', name: 'charlie' },
    ]);
  });

  it('sorts multiple affected nodes by name', () => {
    const nodes = [
      flaggedNode({ id: 'node-b', proxmox: { node: 'pve-b', sensorSetupOutdated: true } }),
      flaggedNode({ id: 'node-a', proxmox: { node: 'pve-a', sensorSetupOutdated: true } }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes).map((n) => n.name)).toEqual(['pve-a', 'pve-b']);
  });
});
