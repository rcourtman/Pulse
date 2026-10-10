import { describe, expect, it, vi, afterEach } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import {
  getCurrentWorkloadMemoryUsage,
  getWorkloadMemoryObservationPresentation,
} from '../memoryObservation';

const guest = (changes: Partial<WorkloadGuest> = {}): WorkloadGuest =>
  ({
    id: 'fixture-pve1-101',
    vmid: 101,
    node: 'pve1',
    instance: 'fixture',
    type: 'qemu',
    memory: { total: 100, used: 25, free: 75, usage: 25 },
    ...changes,
  }) as WorkloadGuest;
const observedAt = '2026-09-30T11:00:00Z';
afterEach(() => vi.restoreAllMocks());

describe('selected workload memory observation', () => {
  it.each(['qemu', 'lxc'])('requires original provenance for a legacy %s reading', (type) => {
    expect(getWorkloadMemoryObservationPresentation(guest({ type }))?.state).toBe('unknown');
  });
  it('requires provenance for canonical Proxmox guests without legacy IDs', () => {
    expect(
      getWorkloadMemoryObservationPresentation(
        guest({ vmid: 0, node: '', instance: '', type: 'vm', platformScopes: ['proxmox-pve'] }),
      )?.state,
    ).toBe('unknown');
  });
  it.each([
    { type: 'vm', platformScopes: ['vmware-vsphere' as const] },
    { type: 'docker', vmid: 0, node: '', instance: '' },
  ])('preserves unannotated unrelated workloads ($type)', (changes) => {
    expect(getWorkloadMemoryObservationPresentation(guest(changes))).toBeNull();
  });
  it.each([undefined, -1, 101, NaN, Infinity])('keeps invalid usage unavailable (%s)', (usage) => {
    const memory = { ...guest().memory, usage } as WorkloadGuest['memory'];
    expect(getWorkloadMemoryObservationPresentation(guest({ memory }))?.state).toBe('unavailable');
  });
  it.each(['last-known', 'current'])('preserves measured zero and its %s state', (state) => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-01T12:00:00Z'));
    expect(
      getWorkloadMemoryObservationPresentation(
        guest({
          memory: {
            total: 100,
            used: 0,
            free: 100,
            usage: 0,
            observation: { state, source: 'agent', observedAt },
          },
        }),
      )?.state,
    ).toBe(state);
  });
  it('qualifies current row memory without allocating an unused date label', () => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-01T12:00:00Z'));
    const iso = vi.spyOn(Date.prototype, 'toISOString');
    const input = guest({
      memory: { ...guest().memory, observation: { state: 'current', source: 'agent', observedAt } },
    });
    expect(getWorkloadMemoryObservationPresentation(input, { includeCurrent: false })).toBeNull();
    expect(iso).not.toHaveBeenCalled();
    input.memory.observation!.state = 'last-known';
    expect(getWorkloadMemoryObservationPresentation(input, { includeCurrent: false })?.state).toBe(
      'last-known',
    );
    expect(iso).toHaveBeenCalledOnce();
  });

  it.each(['usageUnavailable', 'telemetryAvailability'])(
    'cannot revive a current numeric carrier under %s',
    (flag) => {
      const input = guest({
        memory: {
          ...guest().memory,
          observation: { state: 'current', source: 'agent', observedAt },
        },
      });
      if (flag === 'usageUnavailable') input.memory.usageUnavailable = true;
      else
        input.telemetryAvailability = {
          cpu: true,
          memory: false,
          disk: true,
          networkIO: true,
          diskIO: true,
          uptime: true,
        };
      expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe('unavailable');
    },
  );
});

describe('current workload memory selection', () => {
  const current = (changes: Partial<WorkloadGuest> = {}) =>
    guest({
      memory: {
        ...guest().memory,
        observation: { state: 'current', source: 'guest-agent-meminfo', observedAt },
      },
      ...changes,
    });

  it.each([
    undefined,
    { state: 'last-known', source: 'guest-agent-meminfo', observedAt },
    { state: 'unavailable', source: 'agent', observedAt },
    { state: 'future-state', source: 'agent', observedAt },
    { state: 'current', source: 'agent' },
    { state: 'current', source: 'agent', observedAt: 'invalid' },
    { state: 'current', source: 'agent', observedAt: '1970-01-01T00:00:00Z' },
    { state: 'current', source: 'agent', observedAt: '2099-01-01T00:00:00Z' },
  ])('does not select unqualified memory evidence (%j)', (observation) => {
    const input = current({ memory: { ...guest().memory, observation } });
    expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
    expect(getWorkloadMemoryObservationPresentation(input)?.state).not.toBe('current');
  });

  it.each([undefined, -1, 101, NaN, Infinity])('does not select invalid usage (%s)', (usage) => {
    const input = current();
    input.memory = { ...input.memory, usage } as WorkloadGuest['memory'];
    expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
  });

  it('keeps zero current and does not format a label for numeric selection', () => {
    const input = current();
    input.memory = { ...input.memory, used: 0, free: 100, usage: 0 };
    const iso = vi.spyOn(Date.prototype, 'toISOString');
    expect(getCurrentWorkloadMemoryUsage(input)).toBe(0);
    expect(iso).not.toHaveBeenCalled();
  });

  it.each(['guest-agent-meminfo', 'agent', 'status-mem'])(
    'selects current %s memory independently of a QEMU disk deferral',
    (source) => {
      const input = current({
        diskStatusReason: 'prev-vm-locked',
        lock: 'backup',
        agentStale: true,
      });
      input.memory.observation!.source = source;
      expect(getCurrentWorkloadMemoryUsage(input)).toBe(25);
    },
  );

  it('cannot use current metadata to revive unavailable telemetry', () => {
    const input = current();
    input.memory.usageUnavailable = true;
    expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
    input.memory.usageUnavailable = false;
    input.telemetryAvailability = {
      cpu: true,
      memory: false,
      disk: true,
      networkIO: true,
      diskIO: true,
      uptime: true,
    };
    expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
  });

  it.each([
    { type: 'vm', platformScopes: ['vmware-vsphere'] },
    { type: 'docker', vmid: 0, node: '', instance: '' },
  ])('preserves valid unannotated unrelated workloads (%j)', (changes) => {
    expect(getCurrentWorkloadMemoryUsage(guest(changes))).toBe(25);
    const input = current(changes);
    input.memory.observation!.state = 'last-known';
    expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
  });
});
