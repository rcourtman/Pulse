import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { buildNestedWorkloadContextByGuestId } from '../nestedWorkloadContext';

const makeGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest =>
  ({
    id: 'guest-1',
    vmid: 101,
    name: 'guest-1',
    node: 'node-a',
    instance: 'pve-a',
    status: 'running',
    type: 'lxc',
    workloadType: 'system-container',
    cpu: 0,
    cpus: 1,
    memory: { total: 0, used: 0, free: 0, usage: 0 },
    disk: { total: 0, used: 0, free: 0, usage: 0 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 0,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: new Date().toISOString(),
    ...overrides,
  }) as WorkloadGuest;

describe('nested workload context', () => {
  it('maps Proxmox LXC Docker app containers to the owning visible LXC', () => {
    const lxc = makeGuest({
      id: 'lxc-141',
      vmid: 141,
      name: 'media-lxc',
      node: 'node-a',
      instance: 'pve-a',
      platformScopes: ['proxmox-pve'],
    });
    const container = makeGuest({
      id: 'app-container:frigate',
      vmid: 0,
      name: 'frigate',
      type: 'app-container',
      workloadType: 'app-container',
      platformType: 'docker',
      platformScopes: ['proxmox-pve', 'docker'],
      containerRuntime: 'docker',
      dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      dockerHostName: 'media-lxc.mist-stork.ts.net',
      image: 'ghcr.io/blakeblackshear/frigate:stable',
    });

    const result = buildNestedWorkloadContextByGuestId({
      guests: [lxc, container],
      visibleGuests: [lxc],
      excludedWorkloadTypes: ['app-container'],
      platformScope: 'proxmox-pve',
    });

    expect(result['pve-a:node-a:141']).toMatchObject({
      label: 'Docker',
      count: 1,
      href: '/docker/overview?host=media-lxc.mist-stork.ts.net',
      items: [{ name: 'frigate', status: 'running' }],
    });
  });

  it('falls back to the Docker overview when nested containers do not share one host label', () => {
    const lxc = makeGuest({
      id: 'lxc-141',
      vmid: 141,
      name: 'media-lxc',
      node: 'node-a',
      instance: 'pve-a',
      platformScopes: ['proxmox-pve'],
    });
    const firstContainer = makeGuest({
      id: 'app-container:first',
      vmid: 0,
      name: 'first',
      type: 'app-container',
      workloadType: 'app-container',
      platformType: 'docker',
      platformScopes: ['proxmox-pve', 'docker'],
      containerRuntime: 'docker',
      dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      dockerHostName: 'first-host.local',
    });
    const secondContainer = makeGuest({
      id: 'app-container:second',
      vmid: 0,
      name: 'second',
      type: 'app-container',
      workloadType: 'app-container',
      platformType: 'docker',
      platformScopes: ['proxmox-pve', 'docker'],
      containerRuntime: 'docker',
      dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
      dockerHostName: 'second-host.local',
    });

    const result = buildNestedWorkloadContextByGuestId({
      guests: [lxc, firstContainer, secondContainer],
      visibleGuests: [lxc],
      excludedWorkloadTypes: ['app-container'],
      platformScope: 'proxmox-pve',
    });

    expect(result['pve-a:node-a:141']?.href).toBe('/docker/overview');
  });

  it('does not guess when Docker host identity only matches ambiguous VMIDs', () => {
    const firstLxc = makeGuest({
      id: 'lxc-a-141',
      vmid: 141,
      name: 'first-lxc',
      node: 'node-a',
      instance: 'pve-a',
    });
    const secondLxc = makeGuest({
      id: 'lxc-b-141',
      vmid: 141,
      name: 'second-lxc',
      node: 'node-b',
      instance: 'pve-b',
    });
    const container = makeGuest({
      id: 'app-container:ambiguous',
      vmid: 0,
      name: 'ambiguous',
      type: 'app-container',
      workloadType: 'app-container',
      platformScopes: ['proxmox-pve', 'docker'],
      dockerHostId: 'proxmox-lxc-docker:141',
    });

    const result = buildNestedWorkloadContextByGuestId({
      guests: [firstLxc, secondLxc, container],
      visibleGuests: [firstLxc, secondLxc],
      excludedWorkloadTypes: ['app-container'],
      platformScope: 'proxmox-pve',
    });

    expect(result).toEqual({});
  });

  it('only builds summaries for workload types the page deliberately excludes', () => {
    const lxc = makeGuest({ vmid: 141, name: 'media-lxc' });
    const container = makeGuest({
      id: 'app-container:frigate',
      vmid: 0,
      name: 'frigate',
      type: 'app-container',
      workloadType: 'app-container',
      platformScopes: ['proxmox-pve', 'docker'],
      dockerHostId: 'proxmox-lxc-docker:pve-a:node-a:141',
    });

    const result = buildNestedWorkloadContextByGuestId({
      guests: [lxc, container],
      visibleGuests: [lxc],
      excludedWorkloadTypes: [],
      platformScope: 'proxmox-pve',
    });

    expect(result).toEqual({});
  });
});
