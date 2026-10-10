import { describe, expect, it, vi, afterEach, beforeEach } from 'vitest';
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

describe('cache-inclusive Proxmox memory sources', () => {
  const reading = (source: string, state = 'current', changes: Partial<WorkloadGuest> = {}) =>
    guest({
      memory: { ...guest().memory, observation: { state, source, observedAt } },
      ...changes,
    });
  beforeEach(() => {
    vi.spyOn(Date, 'now').mockReturnValue(Date.parse('2026-10-01T12:00:00Z'));
  });

  it.each(['status-mem', 'status-freemem', 'cluster-resources', 'derived-total-minus-used'])(
    'keeps a current %s reading in the row presentation with its caveat',
    (source) => {
      const presentation = getWorkloadMemoryObservationPresentation(reading(source), {
        includeCurrent: false,
      });
      expect(presentation).toMatchObject({ state: 'current', mayIncludeCache: true });
      expect(presentation?.summary).toContain('Proxmox (may include cache)');
      expect(presentation?.message).toContain('may include cached memory');
      expect(presentation?.cacheNote).toBeTruthy();
    },
  );

  it.each([
    'guest-agent-meminfo',
    'guest-agent-meminfo-derived',
    'agent',
    'available-field',
    'derived-free-buffers-cached',
    'previous-snapshot',
    'something-new',
  ])('does not flag %s as cache-inclusive', (source) => {
    const presentation = getWorkloadMemoryObservationPresentation(reading(source));
    expect(presentation?.mayIncludeCache).toBe(false);
    expect(presentation?.cacheNote).toBeUndefined();
    expect(presentation?.message).not.toContain('cached memory');
    expect(presentation?.summary).not.toContain('may include cache');
    expect(
      getWorkloadMemoryObservationPresentation(reading(source), { includeCurrent: false }),
    ).toBeNull();
  });

  it('names the fix that fits the guest: the QEMU agent is a VM-only, Linux-only option', () => {
    const vm = getWorkloadMemoryObservationPresentation(reading('status-mem'))!;
    expect(vm.cacheNote).toContain('memory the VM can reuse');
    expect(vm.cacheNote).toContain('Pulse agent in the VM');
    // The advice is conditional: source selection does not promise a replacement.
    expect(vm.cacheNote).toContain('can let Pulse show its own figure');
    expect(vm.cacheNote).not.toMatch(/\bwill\b/);
    expect(vm.cacheNote).toContain('QEMU guest agent on a Linux VM');
    const canonicalVm = getWorkloadMemoryObservationPresentation(
      reading('status-mem', 'current', { type: 'vm', workloadType: 'vm' }),
    )!;
    expect(canonicalVm.cacheNote).toContain('QEMU guest agent on a Linux VM');
    for (const changes of [
      { type: 'lxc' },
      { type: 'system-container', workloadType: 'system-container' as const },
    ]) {
      const note = getWorkloadMemoryObservationPresentation(
        reading('cluster-resources', 'current', changes),
      )!.cacheNote;
      expect(note).toContain('memory the container can reuse');
      expect(note).toContain('Pulse agent in the container');
      expect(note).not.toContain('QEMU');
    }
    const unknownKind = getWorkloadMemoryObservationPresentation(
      reading('status-mem', 'current', { type: 'docker' }),
    )!.cacheNote;
    expect(unknownKind).toContain('memory the guest can reuse');
    expect(unknownKind).not.toContain('agent');
  });

  it('keeps the caveat on a retained reading without calling it current', () => {
    const presentation = getWorkloadMemoryObservationPresentation(
      reading('status-mem', 'last-known'),
    )!;
    expect(presentation.state).toBe('last-known');
    expect(presentation.mayIncludeCache).toBe(true);
    expect(presentation.message).toContain('Not a current measurement.');
    expect(presentation.message).toContain('may include cached memory');
  });

  it.each([
    ['an unavailable observation', reading('status-mem', 'unavailable')],
    ['a reading with no usable number', reading('status-mem', 'current', { memory: undefined })],
  ])('never qualifies %s', (_label, input) => {
    const presentation = getWorkloadMemoryObservationPresentation(input);
    expect(presentation?.state).toBe('unavailable');
    expect(presentation?.mayIncludeCache).toBe(false);
    expect(presentation?.cacheNote).toBeUndefined();
  });

  it('leaves the selected number unchanged', () => {
    expect(getCurrentWorkloadMemoryUsage(reading('status-mem'))).toBe(25);
    expect(getCurrentWorkloadMemoryUsage(reading('cluster-resources'))).toBe(25);
  });
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
