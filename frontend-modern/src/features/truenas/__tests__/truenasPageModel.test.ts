import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  TRUENAS_TAB_SPECS,
  buildTrueNASPageModel,
  filterTrueNASApps,
  filterTrueNASVMs,
  mapTrueNASAppStatus,
  mapTrueNASVMStatus,
} from '../truenasPageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'truenas',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('truenasPageModel', () => {
  it('declares the TrueNAS section set as Overview + Storage', () => {
    expect(TRUENAS_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'storage']);
  });

  it('buckets systems and apps while keeping storage inventory in scope for shared surfaces', () => {
    const model = buildTrueNASPageModel([
      makeResource({ id: 'truenas-system', type: 'agent' }),
      makeResource({ id: 'truenas-vm', type: 'vm' }),
      makeResource({ id: 'truenas-app', type: 'app-container' }),
      makeResource({ id: 'truenas-pool', type: 'pool' }),
      makeResource({ id: 'truenas-disk', type: 'physical_disk' }),
      makeResource({ id: 'docker-host', type: 'agent', platformType: 'docker' }),
      makeResource({ id: 'pve-node', type: 'agent', platformType: 'proxmox-pve' }),
    ]);

    expect(model.systems.map((r) => r.id)).toEqual(['truenas-system']);
    expect(model.vms.map((r) => r.id)).toEqual(['truenas-vm']);
    expect(model.apps.map((r) => r.id)).toEqual(['truenas-app']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['truenas-app', 'truenas-disk', 'truenas-pool', 'truenas-system', 'truenas-vm'].sort(),
    );
  });

  it('filters apps using native TrueNAS app.query metadata', () => {
    const nextcloud = makeResource({
      id: 'app-nextcloud',
      type: 'app-container',
      parentName: 'truenas-main',
      truenas: {
        hostname: 'truenas-main',
        app: {
          id: 'nextcloud',
          name: 'Nextcloud',
          state: 'RUNNING',
          humanVersion: '29.0.7',
          images: ['docker.io/library/nextcloud:29.0.7'],
          usedPorts: [
            {
              containerPort: 443,
              protocol: 'tcp',
              hostPorts: [{ hostPort: 30443, hostIp: '0.0.0.0' }],
            },
          ],
          volumes: [{ source: '/mnt/tank/apps/nextcloud', destination: '/var/www/html' }],
        },
      },
    });
    const adguard = makeResource({
      id: 'app-adguard',
      type: 'app-container',
      status: 'offline',
      truenas: {
        app: {
          id: 'adguard',
          name: 'AdGuard Home',
          state: 'STOPPED',
          images: ['docker.io/adguard/adguardhome:v0.107'],
        },
      },
    });

    expect(mapTrueNASAppStatus(nextcloud)).toBe('running');
    expect(mapTrueNASAppStatus(adguard)).toBe('stopped');
    expect(filterTrueNASApps([nextcloud, adguard], '30443', 'all').map((r) => r.id)).toEqual([
      'app-nextcloud',
    ]);
    expect(
      filterTrueNASApps([nextcloud, adguard], 'adguardhome', 'stopped').map((r) => r.id),
    ).toEqual(['app-adguard']);
  });

  it('filters VMs using native TrueNAS vm.query metadata', () => {
    const windows = makeResource({
      id: 'vm-windows',
      type: 'vm',
      truenas: {
        hostname: 'truenas-main',
        vm: {
          id: '42',
          name: 'windows-lab',
          description: 'Build validation workstation',
          state: 'RUNNING',
          bootloader: 'UEFI',
          cpuMode: 'HOST-PASSTHROUGH',
          uuid: 'vm-uuid-1',
        },
      },
    });
    const ubuntu = makeResource({
      id: 'vm-ubuntu',
      type: 'vm',
      status: 'offline',
      truenas: {
        vm: {
          id: '43',
          name: 'ubuntu-build',
          state: 'STOPPED',
          machineType: 'q35',
        },
      },
    });

    expect(mapTrueNASVMStatus(windows)).toBe('running');
    expect(mapTrueNASVMStatus(ubuntu)).toBe('stopped');
    expect(filterTrueNASVMs([windows, ubuntu], 'passthrough', 'running').map((r) => r.id)).toEqual([
      'vm-windows',
    ]);
    expect(filterTrueNASVMs([windows, ubuntu], 'q35', 'stopped').map((r) => r.id)).toEqual([
      'vm-ubuntu',
    ]);
  });
});
