import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  VMWARE_TAB_SPECS,
  buildVmwarePageModel,
  filterVmwareDatastores,
  mapVmwareDatastoreStatus,
} from '../vmwarePageModel';

const makeResource = (resource: Partial<Resource> & Pick<Resource, 'id' | 'type'>): Resource => ({
  name: resource.id,
  displayName: resource.id,
  platformId: 'lab',
  platformType: 'vmware-vsphere',
  sourceType: 'api',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...resource,
});

describe('vmwarePageModel', () => {
  it('declares the vSphere section set', () => {
    expect(VMWARE_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'storage']);
    expect(VMWARE_TAB_SPECS.map((tab) => tab.label)).toEqual(['Overview', 'Datastores']);
  });

  it('buckets canonical vSphere hosts, VMs, and datastores', () => {
    const model = buildVmwarePageModel([
      makeResource({ id: 'esxi-host-1', type: 'agent' }),
      makeResource({ id: 'vsphere-vm-1', type: 'vm' }),
      makeResource({
        id: 'datastore-1',
        type: 'storage',
        storage: { topology: 'datastore', platform: 'vmware-vsphere' },
        vmware: { entityType: 'datastore' },
      }),
      makeResource({ id: 'legacy-provider-datastore', type: 'datastore' }),
      makeResource({ id: 'pve-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.hosts.map((r) => r.id)).toEqual(['esxi-host-1']);
    expect(model.vms.map((r) => r.id)).toEqual(['vsphere-vm-1']);
    expect(model.datastores.map((r) => r.id)).toEqual(['datastore-1']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['datastore-1', 'esxi-host-1', 'vsphere-vm-1'].sort(),
    );
  });

  it('filters vSphere datastores using vCenter datastore metadata', () => {
    const accessible = makeResource({
      id: 'ds-accessible',
      type: 'storage',
      name: 'nvme-primary',
      storage: {
        topology: 'datastore',
        platform: 'vmware-vsphere',
        type: 'vmfs',
        nodes: ['esxi-01.lab.local'],
        consumerCount: 2,
        topConsumers: [{ resourceType: 'vm', name: 'warehouse-api-01' }],
      },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        datastoreType: 'VMFS',
        datacenterName: 'Primary DC',
      },
    });
    const inaccessible = makeResource({
      id: 'ds-inaccessible',
      type: 'storage',
      name: 'edge-cold-iscsi',
      status: 'offline',
      storage: { topology: 'datastore', platform: 'vmware-vsphere', nodes: ['esxi-06.lab.local'] },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: false,
        datastoreType: 'VMFS',
        datacenterName: 'Edge DC',
      },
    });
    const maintenance = makeResource({
      id: 'ds-maintenance',
      type: 'storage',
      name: 'maintenance-nfs',
      storage: { topology: 'datastore', platform: 'vmware-vsphere', nodes: ['esxi-07.lab.local'] },
      vmware: {
        entityType: 'datastore',
        datastoreAccessible: true,
        datastoreType: 'NFS41',
        maintenanceMode: 'IN_MAINTENANCE',
      },
    });

    expect(mapVmwareDatastoreStatus(accessible)).toBe('accessible');
    expect(mapVmwareDatastoreStatus(inaccessible)).toBe('inaccessible');
    expect(mapVmwareDatastoreStatus(maintenance)).toBe('maintenance');
    expect(
      filterVmwareDatastores(
        [accessible, inaccessible, maintenance],
        'warehouse',
        'accessible',
      ).map((resource) => resource.id),
    ).toEqual(['ds-accessible']);
    expect(
      filterVmwareDatastores([accessible, inaccessible, maintenance], 'esxi-06', 'all').map(
        (resource) => resource.id,
      ),
    ).toEqual(['ds-inaccessible']);
  });
});
