import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  computeWorkloadStats,
  filterWorkloads,
  getWorkloadSearchCandidates,
} from '../workloadSelectors';

const makeGuest = (overrides: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  ({
    id: 'pve-pve1-101',
    vmid: 101,
    name: 'prod-db-01',
    node: 'pve1',
    instance: 'pve',
    status: 'running',
    type: 'qemu',
    platformType: 'proxmox-pve',
    platformScopes: ['proxmox-pve'],
    cpu: 0.25,
    cpus: 4,
    memory: { total: 4096, used: 2048, free: 2048, usage: 50 },
    disk: { total: 10000, used: 5000, free: 5000, usage: 50 },
    networkIn: 0,
    networkOut: 0,
    diskRead: 0,
    diskWrite: 0,
    uptime: 3600,
    template: false,
    lastBackup: 0,
    tags: [],
    lock: '',
    lastSeen: '2026-11-27T23:40:00Z',
    ...overrides,
  }) as WorkloadGuest;

const baseFilter = {
  viewMode: 'all' as const,
  selectedNode: null,
  selectedHostHint: null,
  selectedKubernetesContext: null,
};

describe('workloadSelectors staleness classification and search', () => {
  it('moves stale running guests into Attention while keeping stopped+stale in Stopped', () => {
    const staleRunning = makeGuest({
      id: 'stale-running',
      name: 'prod-db-01',
      status: 'running',
      sourceStatus: {
        proxmox: { status: 'error', error: 'PVE API timeout (504)' },
      },
    });
    const staleStopped = makeGuest({
      id: 'stale-stopped',
      name: 'archive-worker',
      status: 'stopped',
      sourceStatus: {
        proxmox: { status: 'stale' },
      },
    });
    const freshRunning = makeGuest({
      id: 'fresh-running',
      name: 'web-frontend',
      status: 'running',
      sourceStatus: {
        proxmox: { status: 'online' },
      },
    });
    const guests = [staleRunning, staleStopped, freshRunning];

    const stats = computeWorkloadStats(guests);
    expect(stats.running).toBe(1);
    expect(stats.degraded).toBe(1);
    expect(stats.stopped).toBe(1);

    const attention = filterWorkloads({
      ...baseFilter,
      guests,
      statusMode: 'degraded',
      searchTerm: '',
    });
    expect(attention.map((g) => g.id)).toEqual(['stale-running']);

    const running = filterWorkloads({
      ...baseFilter,
      guests,
      statusMode: 'running',
      searchTerm: '',
    });
    expect(running.map((g) => g.id)).toEqual(['fresh-running']);
  });

  it('indexes stale badge and collection error for text search', () => {
    const staleVmware = makeGuest({
      id: 'vsphere-101',
      name: 'billing-api-vm',
      platformType: 'vmware-vsphere',
      platformScopes: ['vmware-vsphere'],
      sourceStatus: {
        vmware: { status: 'error', error: 'vCenter session expired (401)' },
      },
    });
    const freshVmware = makeGuest({
      id: 'vsphere-201',
      name: 'auth-gateway-vm',
      platformType: 'vmware-vsphere',
      platformScopes: ['vmware-vsphere'],
      sourceStatus: { vmware: { status: 'online' } },
    });

    expect(getWorkloadSearchCandidates(staleVmware)).toContain('Stale data');
    expect(getWorkloadSearchCandidates(staleVmware)).toContain('vCenter session expired (401)');

    const byStale = filterWorkloads({
      ...baseFilter,
      guests: [staleVmware, freshVmware],
      statusMode: 'all',
      searchTerm: 'stale',
    });
    expect(byStale.map((g) => g.id)).toEqual(['vsphere-101']);

    const byError = filterWorkloads({
      ...baseFilter,
      guests: [staleVmware, freshVmware],
      statusMode: 'all',
      searchTerm: 'session expired',
    });
    expect(byError.map((g) => g.id)).toEqual(['vsphere-101']);
  });
});
