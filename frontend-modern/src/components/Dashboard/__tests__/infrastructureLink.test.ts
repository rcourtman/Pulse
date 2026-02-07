import { describe, expect, it } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { buildInfrastructureHrefForWorkload } from '@/components/Dashboard/infrastructureLink';

const baseGuest = (overrides: Partial<WorkloadGuest>): WorkloadGuest => ({
  id: 'guest-1',
  vmid: 101,
  name: 'guest-1',
  node: 'node-1',
  instance: 'cluster-a',
  status: 'running',
  type: 'vm',
  cpu: 0,
  cpus: 2,
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
});

describe('buildInfrastructureHrefForWorkload', () => {
  it('maps vm workloads to proxmox infrastructure source with node query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'vm',
        workloadType: 'vm',
        node: 'pve1',
        instance: 'cluster-main',
      }),
    );
    expect(href).toBe('/infrastructure?source=proxmox&q=pve1');
  });

  it('maps docker workloads to docker infrastructure source with context query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'docker',
        workloadType: 'docker',
        contextLabel: 'docker-host-1',
      }),
    );
    expect(href).toBe('/infrastructure?source=docker&q=docker-host-1');
  });

  it('maps kubernetes workloads to kubernetes infrastructure source with cluster query', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'k8s',
        workloadType: 'k8s',
        contextLabel: 'cluster-a',
      }),
    );
    expect(href).toBe('/infrastructure?source=kubernetes&q=cluster-a');
  });

  it('defaults unknown workload types to proxmox mapping (legacy compatibility)', () => {
    const href = buildInfrastructureHrefForWorkload(
      baseGuest({
        type: 'unknown',
        workloadType: undefined,
      }),
    );
    expect(href).toBe('/infrastructure?source=proxmox&q=node-1');
  });
});
