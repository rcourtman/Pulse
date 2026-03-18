import { describe, expect, it } from 'vitest';
import {
  buildProxmoxDiscoveryPrefillNode,
  getProxmoxVariantPresentation,
} from '@/utils/proxmoxSettingsPresentation';

describe('proxmoxSettingsPresentation', () => {
  it('returns canonical variant presentation for proxmox node types', () => {
    expect(getProxmoxVariantPresentation('pve')).toMatchObject({
      title: 'Proxmox VE nodes',
      addLabel: 'Add Proxmox VE Connection',
    });
    expect(getProxmoxVariantPresentation('pbs')).toMatchObject({
      title: 'Proxmox Backup Server nodes',
      addLabel: 'Add Backup Server Connection',
    });
    expect(getProxmoxVariantPresentation('pmg')).toMatchObject({
      title: 'Proxmox Mail Gateway nodes',
      addLabel: 'Add Mail Gateway Connection',
    });
  });

  it('builds canonical discovery prefills for each proxmox node type', () => {
    expect(
      buildProxmoxDiscoveryPrefillNode({
        type: 'pve',
        ip: '10.0.0.2',
        port: 8006,
        hostname: 'pve1',
      } as never),
    ).toMatchObject({
      type: 'pve',
      name: 'pve1',
      host: 'https://10.0.0.2:8006',
      monitorVMs: true,
      monitorContainers: true,
      monitorStorage: true,
    });

    expect(
      buildProxmoxDiscoveryPrefillNode({
        type: 'pbs',
        ip: '10.0.0.3',
        port: 8007,
        hostname: 'pbs1',
      } as never),
    ).toMatchObject({
      type: 'pbs',
      name: 'pbs1',
      monitorDatastores: true,
      monitorPruneJobs: true,
    });

    expect(
      buildProxmoxDiscoveryPrefillNode({
        type: 'pmg',
        ip: '10.0.0.4',
        port: 8006,
        hostname: 'pmg1',
      } as never),
    ).toMatchObject({
      type: 'pmg',
      name: 'pmg1',
      monitorMailStats: true,
      monitorQueues: true,
    });
  });
});
