import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import { VMWARE_TAB_SPECS, buildVmwarePageModel } from '../vmwarePageModel';

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
    expect(VMWARE_TAB_SPECS.map((tab) => tab.id)).toEqual(['overview', 'vms', 'storage']);
  });

  it('buckets vSphere hosts and VMs and ignores non-vSphere resources', () => {
    const model = buildVmwarePageModel([
      makeResource({ id: 'esxi-host-1', type: 'agent' }),
      makeResource({ id: 'vsphere-vm-1', type: 'vm' }),
      makeResource({ id: 'datastore-1', type: 'datastore' }),
      makeResource({ id: 'pve-vm', type: 'vm', platformType: 'proxmox-pve' }),
    ]);

    expect(model.hosts.map((r) => r.id)).toEqual(['esxi-host-1']);
    expect(model.vms.map((r) => r.id)).toEqual(['vsphere-vm-1']);
    expect(model.resources.map((r) => r.id).sort()).toEqual(
      ['datastore-1', 'esxi-host-1', 'vsphere-vm-1'].sort(),
    );
  });
});
