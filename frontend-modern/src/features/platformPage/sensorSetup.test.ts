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

  it('names a node by its table label and adds the Proxmox node name when they differ', () => {
    // The nodes table labels the row by the operator-set display name, while
    // the setup script is re-run on the Proxmox node itself.
    const renamed = node({
      id: 'node-3',
      name: 'West Production C',
      proxmox: { node: 'pve3', nodeName: 'pve3', sensorSetupOutdated: true },
    });
    expect(collectOutdatedSensorSetupNodes([renamed])).toEqual([
      { id: 'node-3', name: 'West Production C', nodeName: 'pve3' },
    ]);
  });

  it('omits the Proxmox node name when the table label contains it as a word', () => {
    const flagged = (id: string, name: string, nodeName?: string) =>
      node({ id, name, proxmox: { nodeName, sensorSetupOutdated: true } });
    expect(
      collectOutdatedSensorSetupNodes([
        flagged('same', ' PVE1 ', 'pve1'),
        flagged('fqdn', 'pve2.lan', 'pve2'),
        flagged('embedded', 'Rack 4 (pve3)', 'pve3'),
        flagged('punctuated', '"pve5": primary', 'pve5'),
        flagged('unknown', 'pve4'),
      ]),
    ).toEqual([
      { id: 'punctuated', name: '"pve5": primary' },
      { id: 'same', name: 'PVE1' },
      { id: 'fqdn', name: 'pve2.lan' },
      { id: 'unknown', name: 'pve4' },
      { id: 'embedded', name: 'Rack 4 (pve3)' },
    ]);
  });

  it('keeps the exact Proxmox node name when the label only resembles it', () => {
    // The setup script is re-run on this host, so a label that differs inside
    // the name (punctuation, spacing, a suffix or a non-ASCII lookalike
    // letter) must not stand in for it.
    const flagged = (id: string, name: string, nodeName: string) =>
      node({ id, name, proxmox: { nodeName, sensorSetupOutdated: true } });
    expect(
      collectOutdatedSensorSetupNodes([
        flagged('punctuation', 'PVE3', 'pve-3'),
        flagged('spacing', 'PVE 4', 'pve-4'),
        flagged('suffix', 'pve5-backup', 'pve5'),
        flagged('prefix', 'pve10', 'pve1'),
        flagged('lookalike', 'PVE-\u212A', 'pve-k'),
      ]),
    ).toEqual([
      { id: 'spacing', name: 'PVE 4', nodeName: 'pve-4' },
      { id: 'lookalike', name: 'PVE-\u212A', nodeName: 'pve-k' },
      { id: 'prefix', name: 'pve10', nodeName: 'pve1' },
      { id: 'punctuation', name: 'PVE3', nodeName: 'pve-3' },
      { id: 'suffix', name: 'pve5-backup', nodeName: 'pve5' },
    ]);
  });

  it('labels a node without a resource name by its id, as the table does', () => {
    const nodes = [
      node({ id: 'by-id', name: '  ', proxmox: { node: '  alpha  ', sensorSetupOutdated: true } }),
      node({ id: 'bare', name: '', proxmox: { sensorSetupOutdated: true } }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes)).toEqual([
      { id: 'bare', name: 'bare' },
      { id: 'by-id', name: 'by-id', nodeName: 'alpha' },
    ]);
  });

  it('reads the Proxmox node name from nodeName before node', () => {
    const nodes = [
      node({
        id: 'both',
        name: 'West Production C',
        proxmox: { node: 'stale', nodeName: ' pve3 ', sensorSetupOutdated: true },
      }),
      node({
        id: 'blank',
        name: 'West Production D',
        proxmox: { node: 'pve4', nodeName: '  ', sensorSetupOutdated: true },
      }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes)).toEqual([
      { id: 'both', name: 'West Production C', nodeName: 'pve3' },
      { id: 'blank', name: 'West Production D', nodeName: 'pve4' },
    ]);
  });

  it('sorts multiple affected nodes by their table label', () => {
    const nodes = [
      flaggedNode({
        id: 'node-a',
        name: 'West Production C',
        proxmox: { nodeName: 'pve-a', sensorSetupOutdated: true },
      }),
      flaggedNode({
        id: 'node-b',
        name: 'East Backup',
        proxmox: { nodeName: 'pve-b', sensorSetupOutdated: true },
      }),
    ];
    expect(collectOutdatedSensorSetupNodes(nodes).map((n) => n.name)).toEqual([
      'East Backup',
      'West Production C',
    ]);
  });
});
