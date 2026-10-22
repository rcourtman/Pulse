import { describe, expect, it } from 'vitest';

import type { Resource } from '@/types/resource';
import { buildResourcePagePath, buildResourceThresholdsPath } from '../resourcePagePath';

const resource = (overrides: Partial<Resource>): Resource =>
  ({ id: 'res-1', name: 'res-1', status: 'online', ...overrides }) as Resource;

describe('buildResourcePagePath', () => {
  // Patrol's "Open resource" sent every subject to Machines, which lists only
  // standalone agent hosts; 19 of 20 decisions on a mock estate dead-ended.
  it('opens a registry resource on the page of the platform that reports it', () => {
    expect(
      buildResourcePagePath({
        id: 'mock-cluster-1-pve1-105',
        resource: resource({ id: 'vm-2e08', type: 'vm', platformScopes: ['proxmox-pve'] }),
      }),
    ).toBe('/proxmox/overview?resource=vm-2e08');
    expect(
      buildResourcePagePath({
        id: 'vm-9',
        resource: resource({ id: 'vm-9', type: 'vm', platformScopes: ['vmware-vsphere'] }),
      }),
    ).toBe('/vmware/overview?resource=vm-9');
    expect(
      buildResourcePagePath({
        id: 'storage-1',
        resource: resource({ id: 'storage-1', type: 'storage', platformScopes: ['proxmox-pve'] }),
      }),
    ).toBe('/proxmox/storage?resource=storage-1');
  });

  it('opens the owning tab where the page cannot select one resource', () => {
    // An ESXi host reports as an agent-type resource from vSphere.
    expect(
      buildResourcePagePath({
        id: 'agent-b602',
        type: 'agent',
        resource: resource({ id: 'agent-b602', type: 'agent', sources: ['vmware'] }),
      }),
    ).toBe('/vmware/overview');
    expect(
      buildResourcePagePath({
        id: 'network-a6af',
        resource: resource({
          id: 'network-a6af',
          type: 'network' as Resource['type'],
          sources: ['vmware'],
        }),
      }),
    ).toBe('/vmware/networks');
    expect(
      buildResourcePagePath({
        id: 'storage-23a2',
        type: 'truenas-dataset',
        resource: resource({ id: 'storage-23a2', type: 'storage', sources: ['truenas'] }),
      }),
    ).toBe('/truenas/storage?q=res-1');
  });

  it('filters a physical disk by the registry name and a machine by its own name', () => {
    expect(
      buildResourcePagePath({
        id: 'Production West:pve5:disk:dev-nvme0',
        name: 'Samsung SSD 970 EVO Plus 1TB (/dev/nvme0)',
        resource: resource({
          id: 'physical_disk-0dfd',
          type: 'physical_disk' as Resource['type'],
          name: 'Samsung SSD 970 EVO Plus 1TB',
          sources: ['proxmox'],
        }),
      }),
    ).toBe('/proxmox/storage?tab=disks&q=Samsung+SSD+970+EVO+Plus+1TB');
    expect(
      buildResourcePagePath({
        id: 'agent-1',
        resource: resource({
          id: 'agent-1',
          type: 'agent',
          name: 'Apollo-114',
          platformScopes: ['agent'],
        }),
      }),
    ).toBe('/standalone/machines?q=Apollo-114');
  });

  it('keeps a Proxmox node that also runs the Pulse agent on the Proxmox page', () => {
    expect(
      buildResourcePagePath({
        id: 'agent-pve1',
        resource: resource({
          id: 'agent-pve1',
          type: 'agent',
          platformScopes: ['agent', 'proxmox-pve'],
        }),
      }),
    ).toBe('/proxmox/overview');
  });

  it('falls back to the record type when the registry cannot resolve the id', () => {
    expect(
      buildResourcePagePath({ id: 'docker:host/abc', name: 'nginx', type: 'app-container' }),
    ).toBe('/docker/overview?q=nginx');
    expect(buildResourcePagePath({ id: 'pve:vm:101', type: 'vm' })).toBe(
      '/proxmox/overview?resource=pve%3Avm%3A101',
    );
    expect(buildResourcePagePath({ id: 'pod-1', name: 'api-7f', type: 'pod' })).toBe(
      '/kubernetes/workloads?q=api-7f',
    );
    expect(buildResourcePagePath({ id: 'ds-1', type: 'truenas-dataset' })).toBe('/truenas/storage');
    // TrueNAS tabs filter to a named pool, dataset, app, VM or share.
    expect(buildResourcePagePath({ id: 'ds-1', name: 'tank/apps', type: 'truenas-dataset' })).toBe(
      '/truenas/storage?q=tank%2Fapps',
    );
    // A disk name from an alert is not known to match the disk table.
    expect(buildResourcePagePath({ id: 'sdc', name: 'sdc', type: 'truenas-disk' })).toBe(
      '/truenas/storage',
    );
    expect(
      buildResourcePagePath({
        id: 'pve5:disk:nvme0',
        name: 'SSD (/dev/nvme0)',
        type: 'proxmox-disk',
      }),
    ).toBe('/proxmox/storage?tab=disks');
    expect(buildResourcePagePath({ id: 'x', type: 'something-new' })).toBeNull();
  });

  it('honours the reporting platform over the resource kind', () => {
    // TrueNAS apps are app containers reported by TrueNAS, not Docker.
    expect(
      buildResourcePagePath({
        id: 'app-1',
        resource: resource({ id: 'app-1', type: 'app-container', sources: ['truenas'] }),
      }),
    ).toBe('/truenas/apps?q=res-1');
    expect(
      buildResourcePagePath({
        id: 'disk-1',
        type: 'truenas-disk',
        resource: resource({
          id: 'disk-1',
          type: 'physical_disk' as Resource['type'],
          sources: ['truenas'],
        }),
      }),
    ).toBe('/truenas/storage?q=res-1');
  });

  it('opens the Docker or Kubernetes tab that lists the resource kind', () => {
    expect(
      buildResourcePagePath({
        id: 'svc-1',
        name: 'web',
        resource: resource({ id: 'svc-1', type: 'docker-service', name: 'web' }),
      }),
    ).toBe('/docker/swarm');
    expect(
      buildResourcePagePath({
        id: 'ns-1',
        resource: resource({ id: 'ns-1', type: 'k8s-namespace', name: 'payments' }),
      }),
    ).toBe('/kubernetes/configuration?q=payments');
    expect(
      buildResourcePagePath({
        id: 'node-1',
        resource: resource({ id: 'node-1', type: 'k8s-node', name: 'worker-2' }),
      }),
    ).toBe('/kubernetes/nodes?q=worker-2');
  });

  // Alerts name their type in display words, which Patrol items carry over.
  it('reads the alert display vocabulary as type hints', () => {
    expect(buildResourcePagePath({ id: 'vm-9', type: 'vSphere VM' })).toBe(
      '/vmware/overview?resource=vm-9',
    );
    expect(buildResourcePagePath({ id: 'ds-1', type: 'TrueNAS Dataset' })).toBe('/truenas/storage');
    expect(buildResourcePagePath({ id: 'pod-1', name: 'api-7f', type: 'Kubernetes Pod' })).toBe(
      '/kubernetes/workloads?q=api-7f',
    );
    expect(buildResourcePagePath({ id: 'c-1', name: 'nginx', type: 'Docker' })).toBe(
      '/docker/overview?q=nginx',
    );
    expect(buildResourcePagePath({ id: 'h-1', type: 'vSphere Host' })).toBe('/vmware/overview');
    expect(buildResourcePagePath({ id: 'ct-1', type: 'Container' })).toBe(
      '/proxmox/overview?resource=ct-1',
    );
  });
  it('opens Kubernetes events and policy objects and TrueNAS shares on their tabs', () => {
    expect(
      buildResourcePagePath({
        id: 'e1',
        resource: resource({ id: 'e1', type: 'k8s-event' as Resource['type'], name: 'evt' }),
      }),
    ).toBe('/kubernetes/events?q=evt');
    expect(
      buildResourcePagePath({
        id: 'pdb1',
        resource: resource({
          id: 'pdb1',
          type: 'k8s-pod-disruption-budget' as Resource['type'],
          name: 'api',
        }),
      }),
    ).toBe('/kubernetes/configuration?q=api');
    expect(
      buildResourcePagePath({
        id: 'share-1',
        resource: resource({
          id: 'share-1',
          type: 'network-share' as Resource['type'],
          sources: ['truenas'],
        }),
      }),
    ).toBe('/truenas/shares?q=res-1');
  });
  it('opens the storage a ZFS pool or device record belongs to', () => {
    expect(
      buildResourcePagePath({ id: 'pve5-local-zfs/zfs-pool:rpool/device:sda', type: 'storage' }),
    ).toBe('/proxmox/storage?resource=pve5-local-zfs');
  });
  it('opens an availability check on the availability checks tab', () => {
    expect(
      buildResourcePagePath({
        id: 'endpoint-1',
        resource: resource({
          id: 'endpoint-1',
          type: 'network-endpoint' as Resource['type'],
          sources: ['availability' as never],
        }),
      }),
    ).toBe('/standalone/availability');
  });
  it('opens the thresholds tab that holds a resource, searched to its name', () => {
    expect(buildResourceThresholdsPath({ id: 'c-1', name: 'nginx', type: 'app-container' })).toBe(
      '/alerts/thresholds/docker?q=nginx',
    );
    expect(
      buildResourceThresholdsPath({
        id: 'mock-1-105',
        name: 'postgres-replica-01',
        resource: resource({
          id: 'vm-2e08',
          type: 'vm',
          name: 'postgres-replica-01',
          platformScopes: ['proxmox-pve'],
        }),
      }),
    ).toBe('/alerts/thresholds/proxmox?q=postgres-replica-01');
    expect(
      buildResourceThresholdsPath({
        id: 'agent-1',
        resource: resource({
          id: 'agent-1',
          type: 'agent',
          name: 'Apollo-114',
          platformScopes: ['agent'],
        }),
      }),
    ).toBe('/alerts/thresholds/systems?q=Apollo-114');
    // Threshold rows are built from the display label, not the registry name.
    expect(
      buildResourceThresholdsPath({
        id: 'node-1',
        resource: resource({
          id: 'node-1',
          type: 'agent',
          name: 'titan.example.com',
          displayName: 'Primary',
          platformScopes: ['proxmox-pve'],
        }),
      }),
    ).toBe('/alerts/thresholds/proxmox?q=Primary');
    // A composed record label would match no row and hide them all.
    expect(
      buildResourceThresholdsPath({
        id: 'pve1-local-zfs/zfs-pool:rpool',
        name: 'west-a-service-pool (local-zfs, sdb2)',
        type: 'storage',
      }),
    ).toBe('/alerts/thresholds/proxmox');
    // Physical disks have no threshold rows of their own to search for.
    expect(
      buildResourceThresholdsPath({
        id: 'disk-1',
        type: 'proxmox-disk',
        resource: resource({
          id: 'physical_disk-1',
          type: 'physical_disk' as Resource['type'],
          name: 'Samsung SSD',
          sources: ['proxmox'],
        }),
      }),
    ).toBe('/alerts/thresholds/proxmox');
    expect(buildResourceThresholdsPath({ id: 'x', type: 'something-new' })).toBe(
      '/alerts/thresholds',
    );
  });
});
