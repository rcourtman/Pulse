import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import {
  filterStandaloneVMs,
  getStandaloneVMDisplayStatus,
  getStandaloneVMHypervisorLabel,
  getStandaloneVMSortValue,
  getStandaloneVMStateLabel,
  getStandaloneVMTotalDiskIORate,
  getStandaloneVMTotalNetIORate,
  mapStandaloneVMStatus,
  matchesStandaloneVMHostScope,
  normalizeStandaloneVMStatusFilter,
  resolveStandaloneVMHostName,
} from '../standaloneVmTableModel';

const makeResource = (overrides: Partial<Resource>): Resource =>
  ({
    id: overrides.id ?? 'vm-1',
    name: overrides.name ?? 'pulse-dev',
    displayName: overrides.displayName ?? overrides.name ?? 'pulse-dev',
    type: overrides.type ?? 'vm',
    platformId: overrides.platformId ?? 'agent',
    platformType: overrides.platformType ?? 'agent',
    sourceType: overrides.sourceType ?? 'agent',
    sources: overrides.sources ?? ['agent'],
    status: overrides.status ?? 'online',
    lastSeen: overrides.lastSeen ?? 1_700_000_000_000,
    ...overrides,
  }) as Resource;

describe('standaloneVmTableModel', () => {
  const tower = makeResource({
    id: 'agent:tower',
    name: 'Tower',
    displayName: 'Tower',
    type: 'agent',
  });
  const machinesById = new Map([[tower.id, tower]]);
  const machinesByName = new Map([['tower', tower]]);

  it('normalizes status filter params', () => {
    expect(normalizeStandaloneVMStatusFilter('running')).toBe('running');
    expect(normalizeStandaloneVMStatusFilter('ATTENTION')).toBe('attention');
    expect(normalizeStandaloneVMStatusFilter('stopped')).toBe('stopped');
    expect(normalizeStandaloneVMStatusFilter(['running'])).toBe('running');
    expect(normalizeStandaloneVMStatusFilter('other')).toBe('all');
    expect(normalizeStandaloneVMStatusFilter(undefined)).toBe('all');
  });

  it('maps runtime states and impaired sources into running, stopped, and attention buckets', () => {
    expect(
      mapStandaloneVMStatus(
        makeResource({
          virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 8 },
        }),
      ),
    ).toBe('running');
    expect(
      mapStandaloneVMStatus(
        makeResource({
          virtualMachine: { runtimeState: 'shutoff', hypervisor: 'libvirt', vcpus: 4 },
        }),
      ),
    ).toBe('stopped');
    expect(
      mapStandaloneVMStatus(
        makeResource({
          virtualMachine: { runtimeState: 'paused', hypervisor: 'libvirt', vcpus: 4 },
        }),
      ),
    ).toBe('attention');
    expect(
      mapStandaloneVMStatus(
        makeResource({
          virtualMachine: { runtimeState: 'crashed', hypervisor: 'libvirt', vcpus: 4 },
        }),
      ),
    ).toBe('attention');
    expect(
      getStandaloneVMDisplayStatus(
        makeResource({
          virtualMachine: { runtimeState: 'crashed', hypervisor: 'libvirt', vcpus: 4 },
        }),
      ),
    ).toBe('offline');
    expect(
      mapStandaloneVMStatus(
        makeResource({
          virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 4 },
          sourceStatus: { agent: { status: 'stale' } },
        }),
      ),
    ).toBe('attention');
  });

  it('formats state and hypervisor labels', () => {
    expect(
      getStandaloneVMStateLabel(
        makeResource({ virtualMachine: { runtimeState: 'shutting-down', hypervisor: 'libvirt' } }),
      ),
    ).toBe('Shutting down');
    expect(
      getStandaloneVMHypervisorLabel(
        makeResource({ virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt' } }),
      ),
    ).toBe('Libvirt');
    expect(
      getStandaloneVMHypervisorLabel(
        makeResource({ virtualMachine: { runtimeState: 'running', hypervisor: 'xcp-ng' } }),
      ),
    ).toBe('XCP-ng');
  });

  it('resolves parent host names and scopes by host id or name', () => {
    const vmById = makeResource({ id: 'vm-1', parentId: 'agent:tower' });
    const vmByName = makeResource({ id: 'vm-2', parentName: 'Tower' });
    expect(resolveStandaloneVMHostName(vmById, machinesById, machinesByName)).toBe('Tower');
    expect(resolveStandaloneVMHostName(vmByName, machinesById, machinesByName)).toBe('Tower');
    expect(matchesStandaloneVMHostScope(vmById, 'agent:tower', machinesById, machinesByName)).toBe(
      true,
    );
    expect(
      matchesStandaloneVMHostScope(vmByName, 'agent:tower', machinesById, machinesByName),
    ).toBe(true);
    expect(matchesStandaloneVMHostScope(vmById, 'agent:other', machinesById, machinesByName)).toBe(
      false,
    );
  });

  it('filters by host scope, status, and search query and computes sort values', () => {
    const runningVm = makeResource({
      id: 'vm-pulse-dev',
      name: 'pulse-dev',
      parentId: 'agent:tower',
      virtualMachine: { runtimeState: 'running', hypervisor: 'libvirt', vcpus: 8 },
      cpu: { current: 18.5 },
      memory: { current: 24, used: 5 * 1024 ** 3, total: 20 * 1024 ** 3 },
      diskIO: { readRate: 1024, writeRate: 2048 },
      network: { rxBytes: 500, txBytes: 700 },
    });
    const stoppedVm = makeResource({
      id: 'vm-win11',
      name: 'win11',
      parentId: 'agent:other',
      parentName: 'workstation',
      status: 'offline',
      virtualMachine: { runtimeState: 'shutoff', hypervisor: 'libvirt', vcpus: 4 },
    });

    expect(
      filterStandaloneVMs([runningVm, stoppedVm], 'tower', 'all', machinesById, machinesByName).map(
        (vm) => vm.id,
      ),
    ).toEqual(['vm-pulse-dev']);

    expect(
      filterStandaloneVMs([runningVm, stoppedVm], '', 'stopped', machinesById, machinesByName).map(
        (vm) => vm.id,
      ),
    ).toEqual(['vm-win11']);

    expect(
      filterStandaloneVMs(
        [runningVm, stoppedVm],
        '',
        'all',
        machinesById,
        machinesByName,
        'agent:tower',
      ).map((vm) => vm.id),
    ).toEqual(['vm-pulse-dev']);

    expect(getStandaloneVMTotalDiskIORate(runningVm)).toBe(3072);
    expect(getStandaloneVMTotalNetIORate(runningVm)).toBe(1200);
    expect(getStandaloneVMSortValue(runningVm, 'cpu', machinesById, machinesByName)).toBe(18.5);
    expect(getStandaloneVMSortValue(runningVm, 'host', machinesById, machinesByName)).toBe('Tower');
    expect(getStandaloneVMSortValue(stoppedVm, 'diskio', machinesById, machinesByName)).toBeNull();
  });
});
