import { describe, expect, it } from 'vitest';
import type { Resource } from '@/types/resource';
import type { WorkloadGuest } from '@/types/workloads';
import {
  getCurrentWorkloadMemoryUsage,
  getWorkloadMemoryObservationPresentation,
} from '@/utils/memoryObservation';
import { evaluateFilterStack, parseFilterStack } from '@/utils/searchQuery';
import { createWorkloadSortComparator } from '@/components/Workloads/workloadSelectors';
import {
  getGuestDrawerCurrentMetrics,
  getGuestDrawerDeferredMetrics,
} from '@/components/Workloads/guestDrawerModel';
import {
  getResourceMetricsHistoryCurrentMetrics,
  getResourceMetricsHistoryDeferredMetrics,
} from '@/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel';

const observedAt = '2026-10-10T12:00:00Z';
const guest = (source = 'agent', state = 'current', usage = 95): WorkloadGuest =>
  ({
    id: 'fixture:pve:101',
    name: 'linked-guest',
    type: 'qemu',
    status: 'running',
    vmid: 101,
    node: 'pve',
    instance: 'fixture',
    platformScopes: ['proxmox-pve'],
    agentStale: true,
    // Another producer can renew this without a new agent report.
    lastSeen: '2026-10-11T12:00:00Z',
    memory: {
      total: 100,
      used: usage,
      free: 100 - usage,
      usage,
      observation: { state, source, observedAt },
    },
  }) as WorkloadGuest;

const resource = (input: WorkloadGuest): Resource => ({
  id: input.id,
  name: input.name,
  displayName: input.name,
  platformId: 'fixture',
  sourceType: 'hybrid',
  type: 'vm',
  status: 'online',
  platformType: 'proxmox-pve',
  lastSeen: Date.parse(input.lastSeen!),
  agent: { stale: input.agentStale },
  proxmox: { runtimeStatus: input.status },
  memory: { current: input.memory.usage, observation: input.memory.observation },
});

describe('source-owned stale agent memory', () => {
  it.each(['paused', 'suspended', 'stopped'])(
    'uses native %s for shared memory selection, search, sorting and both History consumers',
    (status) => {
      const input = { ...guest(), status, agentStale: false };
      const original = structuredClone(input);
      const independent = {
        ...guest('guest-agent-meminfo', 'current', 10),
        id: 'independent',
        status: 'running',
        agentStale: false,
      };
      expect(evaluateFilterStack(input, parseFilterStack('memory>90'))).toBe(false);
      expect(evaluateFilterStack(input, parseFilterStack('memory<100'))).toBe(false);
      expect(
        [input, independent].sort(createWorkloadSortComparator('memory', 'desc')!).map((g) => g.id),
      ).toEqual(['independent', input.id]);
      for (const source of ['agent', 'guest-agent-meminfo']) {
        input.memory.observation!.source = source;
        expect(getGuestDrawerCurrentMetrics(input).memory).toBeUndefined();
        expect(getResourceMetricsHistoryCurrentMetrics(resource(input)).memory).toBeUndefined();
        const retained = status === 'stopped' ? undefined : 95;
        expect(getGuestDrawerDeferredMetrics(input).memory.lastKnownValue).toBe(retained);
        expect(
          getResourceMetricsHistoryDeferredMetrics(resource(input)).memory.lastKnownValue,
        ).toBe(retained);
      }
      input.memory.observation!.source = 'agent';
      expect(input).toEqual(original);
    },
  );

  it.each([0, 95])(
    'retains a value (%s) without asserting a current agent measurement',
    (usage) => {
      const input = guest('agent', 'current', usage);
      const original = structuredClone(input);
      const reading = getWorkloadMemoryObservationPresentation(input, { includeCurrent: false });
      expect(reading?.state).toBe('last-known');
      expect(reading?.summary).toContain('Last known · Pulse Agent (stopped reporting)');
      expect(reading?.summary).toContain('2026-10-10 12:00:00 UTC');
      expect(reading?.message).toContain('stopped reporting');
      expect(reading?.message).not.toContain('2026-10-11');
      expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
      expect(input).toEqual(original);
    },
  );

  it.each(['guest-agent-meminfo', 'guest-agent-meminfo-derived', 'status-mem', 'available-field'])(
    'does not downgrade independent current %s memory when the linked agent is stale',
    (source) => {
      const input = guest(source);
      expect(getCurrentWorkloadMemoryUsage(input)).toBe(95);
      expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe('current');
      expect(getWorkloadMemoryObservationPresentation(input)?.message).not.toContain(
        'stopped reporting',
      );
      expect(getResourceMetricsHistoryCurrentMetrics(resource(input)).memory).toBe(95);
    },
  );

  it('never supplies missing provenance or revives unavailable telemetry', () => {
    for (const state of ['unavailable', 'future-state']) {
      const input = guest('agent', state);
      expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
      expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe(
        state === 'unavailable' ? 'unavailable' : 'unknown',
      );
    }
    for (const time of [undefined, 'invalid', '2099-01-01T00:00:00Z']) {
      const input = guest();
      input.memory.observation!.observedAt = time;
      expect(getCurrentWorkloadMemoryUsage(input)).toBeNull();
      expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe('unknown');
    }
    const input = guest();
    input.memory.usageUnavailable = true;
    expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe('unavailable');
  });

  it('does not rank or filter a retained high reading as current pressure', () => {
    const stale = guest();
    const current = { ...guest('guest-agent-meminfo', 'current', 10), id: 'current' };
    for (const direction of ['asc', 'desc'] as const) {
      expect([stale, current].sort(createWorkloadSortComparator('memory', direction)!)).toEqual([
        current,
        stale,
      ]);
    }
    for (const query of ['memory>90', 'memory<100', 'memory=95']) {
      expect(evaluateFilterStack(stale, parseFilterStack(query))).toBe(false);
    }
  });

  it.each([0, 95])(
    'keeps the value (%s) as original last-known evidence in both History consumers',
    (usage) => {
      const input = guest('agent', 'current', usage);
      const canonical = resource(input);
      expect(getGuestDrawerCurrentMetrics(input).memory).toBeUndefined();
      expect(getResourceMetricsHistoryCurrentMetrics(canonical).memory).toBeUndefined();
      for (const deferred of [
        getGuestDrawerDeferredMetrics(input),
        getResourceMetricsHistoryDeferredMetrics(canonical),
      ]) {
        expect(deferred.memory.lastKnownValue).toBe(usage);
        expect(deferred.memory.message).toContain('2026-10-10 12:00:00 UTC');
        expect(deferred.memory.message).toContain('stopped reporting');
      }
    },
  );

  it('restores current measured zero only when its source reports again', () => {
    const input = guest('agent', 'current', 0);
    input.agentStale = false;
    input.memory.observation!.observedAt = '2026-10-11T12:00:00Z';
    expect(getCurrentWorkloadMemoryUsage(input)).toBe(0);
    expect(getWorkloadMemoryObservationPresentation(input)?.state).toBe('current');
    expect(getWorkloadMemoryObservationPresentation(input)?.message).not.toContain(
      'stopped reporting',
    );
    expect(getGuestDrawerCurrentMetrics(input).memory).toBe(0);
    expect(getResourceMetricsHistoryCurrentMetrics(resource(input)).memory).toBe(0);
    expect(getGuestDrawerDeferredMetrics(input)).not.toHaveProperty('memory');
    expect(getResourceMetricsHistoryDeferredMetrics(resource(input))).not.toHaveProperty('memory');
  });
});
