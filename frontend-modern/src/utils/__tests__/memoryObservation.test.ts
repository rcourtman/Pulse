import { describe, expect, it, vi, afterEach } from 'vitest';
import type { WorkloadGuest } from '@/types/workloads';
import { getWorkloadMemoryObservationPresentation } from '../memoryObservation';

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
