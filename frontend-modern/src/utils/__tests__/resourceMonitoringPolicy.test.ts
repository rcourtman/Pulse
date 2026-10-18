import { describe, expect, it } from 'vitest';

import { describeResourceInventoryOwnership } from '@/utils/resourceMonitoringPolicy';

const ownerOf = (resourceType?: string, platformType?: string) =>
  describeResourceInventoryOwnership(resourceType, platformType).ownerLabel;

describe('describeResourceInventoryOwnership', () => {
  it('names vSphere as the owner of a VMware VM instead of guessing Proxmox from the type', () => {
    const ownership = describeResourceInventoryOwnership('vm', 'vmware');

    expect(ownership.ownerLabel).toBe('VMware vSphere');
    expect(ownership.retirementDescription).toContain('vCenter');
    expect(ownership.retirementDescription).not.toContain('Proxmox');
  });

  it('keeps Proxmox as the owner of a VM when no platform is known', () => {
    expect(ownerOf('vm', undefined)).toBe('Proxmox');
    expect(ownerOf('vm', '')).toBe('Proxmox');
  });

  it('lets an explicit platform outrank the resource-type guess', () => {
    expect(ownerOf('vm', 'vmware-vsphere')).toBe('VMware vSphere');
    expect(ownerOf('vm', 'truenas')).toBe('TrueNAS');
    expect(ownerOf('vm', 'kubernetes')).toBe('Kubernetes');
    expect(ownerOf('system-container', 'docker')).toBe('container runtime');
    expect(ownerOf('agent', 'agent')).toBe('Pulse agent');
  });

  it('keeps agent-removal copy to the agent machine, not what the agent reports', () => {
    // libvirt VMs and Unraid arrays arrive through the agent but are not its machine.
    for (const type of ['vm', 'storage']) {
      const ownership = describeResourceInventoryOwnership(type, 'agent');
      expect(ownership.ownerLabel).toBe('source system');
      expect(ownership.retirementDescription).not.toContain('Machines');
    }
    expect(describeResourceInventoryOwnership('', 'agent').retirementDescription).toContain(
      'Agent removal remains available from Machines',
    );
  });

  it('accepts canonical platform keys as well as platform families', () => {
    expect(ownerOf('node', 'proxmox')).toBe('Proxmox');
    expect(ownerOf('vm', 'proxmox-pve')).toBe('Proxmox');
    expect(ownerOf('pbs', 'proxmox-pbs')).toBe('Proxmox');
    expect(ownerOf('pmg', 'proxmox-pmg')).toBe('Proxmox');
    expect(ownerOf('datastore', 'vmware-vsphere')).toBe('VMware vSphere');
  });

  it('falls back to the resource type when the platform is unknown or unset', () => {
    expect(ownerOf('pod', 'generic')).toBe('Kubernetes');
    expect(ownerOf('k8s-node')).toBe('Kubernetes');
    expect(ownerOf('app-container')).toBe('container runtime');
    expect(ownerOf('docker-host')).toBe('container runtime');
    expect(ownerOf('agent')).toBe('Pulse agent');
    expect(ownerOf('storage', 'unknown')).toBe('source system');
  });

  it('reports only the Pulse agent as not provider-owned', () => {
    expect(describeResourceInventoryOwnership('agent').providerOwned).toBe(false);
    expect(describeResourceInventoryOwnership('vm', 'vmware').providerOwned).toBe(true);
  });
});
