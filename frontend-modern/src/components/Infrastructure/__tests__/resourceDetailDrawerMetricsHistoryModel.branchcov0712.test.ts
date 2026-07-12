import { describe, expect, it } from 'vitest';

import {
  getResourceMetricsHistoryFallbackMetrics,
  getResourceMetricsHistoryTarget,
} from '@/components/Infrastructure/resourceDetailDrawerMetricsHistoryModel';
import type {
  MetricsHistoryTargetResourceType,
  Resource,
} from '@/types/resource';

// Minimal valid Resource fixture; mirrors the baseResource() convention used by
// sibling branchcov tests in this directory.
const baseResource = (overrides: Partial<Resource>): Resource => ({
  id: 'agent-1',
  type: 'agent',
  name: 'host-1',
  displayName: 'Host 1',
  platformId: 'host-1',
  platformType: 'agent',
  sourceType: 'hybrid',
  status: 'online',
  lastSeen: 1_700_000_000_000,
  ...overrides,
});

describe('getResourceMetricsHistoryTarget branch coverage', () => {
  it('returns the metricsTarget pair when both resourceType and resourceId resolve', () => {
    // TRUE arm of `if (metricsType && metricsId)`.
    const resource = baseResource({
      metricsTarget: { resourceType: 'pod', resourceId: 'pod-7' },
    });
    expect(getResourceMetricsHistoryTarget(resource)).toStrictEqual({
      resourceType: 'pod',
      resourceId: 'pod-7',
    });
  });

  it('trims whitespace around metricsTarget fields before returning them', () => {
    // Drives asTrimmedString's "non-empty after trim -> return trimmed" arm for
    // both fields, then the TRUE arm of the `metricsType && metricsId` guard.
    const resource = baseResource({
      metricsTarget: {
        resourceType: '  vm  ' as MetricsHistoryTargetResourceType,
        resourceId: '  vm-9  ',
      },
    });
    expect(getResourceMetricsHistoryTarget(resource)).toStrictEqual({
      resourceType: 'vm',
      resourceId: 'vm-9',
    });
  });

  it('falls through the metricsTarget branch when resourceId is empty/whitespace', () => {
    // asTrimmedString('   ') -> undefined, so metricsId is falsy ->
    // FALSE arm of `metricsType && metricsId`. Non-agent type then hits the
    // final `return null`.
    const resource = baseResource({
      type: 'vm',
      metricsTarget: { resourceType: 'vm', resourceId: '   ' },
    });
    expect(getResourceMetricsHistoryTarget(resource)).toBeNull();
  });

  it('falls through the metricsTarget branch when resourceType is empty string', () => {
    // metricsType falsy -> FALSE arm of the guard even though resourceId is set.
    const resource = baseResource({
      type: 'pod',
      metricsTarget: {
        resourceType: '' as MetricsHistoryTargetResourceType,
        resourceId: 'pod-1',
      },
    });
    expect(getResourceMetricsHistoryTarget(resource)).toBeNull();
  });

  it('treats a non-string metricsTarget.resourceType as undefined', () => {
    // typeof !== 'string' -> asTrimmedString returns undefined -> guard FALSE.
    // The cast is required because resourceType is typed as a string union.
    const resource = baseResource({
      type: 'pod',
      metricsTarget: {
        resourceType: 42 as unknown as MetricsHistoryTargetResourceType,
        resourceId: 'pod-1',
      },
    });
    expect(getResourceMetricsHistoryTarget(resource)).toBeNull();
  });

  it('synthesizes an agent target from resource.id when metricsTarget is absent', () => {
    // TRUE arm of `resource.type === 'agent'` AND TRUE arm of the
    // `resourceId ? {...} : null` ternary.
    const resource = baseResource({ type: 'agent', id: 'host-42' });
    expect(getResourceMetricsHistoryTarget(resource)).toStrictEqual({
      resourceType: 'agent',
      resourceId: 'host-42',
    });
  });

  it('returns null for an agent whose id trims to empty', () => {
    // TRUE arm of `resource.type === 'agent'` but asTrimmedString('   ') is
    // undefined -> FALSE arm of the ternary.
    const resource = baseResource({ type: 'agent', id: '   ' });
    expect(getResourceMetricsHistoryTarget(resource)).toBeNull();
  });

  it('returns null for a non-agent resource with no metricsTarget', () => {
    // FALSE arm of `resource.type === 'agent'` -> final `return null`.
    const resource = baseResource({ type: 'vm' });
    expect(getResourceMetricsHistoryTarget(resource)).toBeNull();
  });
});

describe('getResourceMetricsHistoryFallbackMetrics branch coverage', () => {
  it('returns every metric as a finite number when the resource is fully populated', () => {
    // All finiteMetric calls take the "finite number -> return value" arm;
    // memory/disk ternaries take the TRUE arm and getMemoryPercent/getDiskPercent
    // take their `(used/total)*100` arms.
    const resource = baseResource({
      cpu: { current: 12.5 },
      memory: { current: 0, total: 100, used: 30 },
      disk: { current: 0, total: 200, used: 80 },
      network: { rxBytes: 1000, txBytes: 2000 },
      diskIO: { readRate: 3000, writeRate: 4000 },
    });
    expect(getResourceMetricsHistoryFallbackMetrics(resource)).toStrictEqual({
      cpu: 12.5,
      memory: 30, // (30 / 100) * 100
      disk: 40, // (80 / 200) * 100
      netin: 1000,
      netout: 2000,
      diskread: 3000,
      diskwrite: 4000,
    });
  });

  it('returns undefined for every field when no metric objects are present', () => {
    // Every optional chain short-circuits to undefined; memory/disk ternaries
    // take the FALSE arm because `resource.memory`/`resource.disk` are falsy.
    const resource = baseResource({});
    expect(getResourceMetricsHistoryFallbackMetrics(resource)).toStrictEqual({
      cpu: undefined,
      memory: undefined,
      disk: undefined,
      netin: undefined,
      netout: undefined,
      diskread: undefined,
      diskwrite: undefined,
    });
  });

  it('uses memory.current when total/used are absent', () => {
    // getMemoryPercent: `memory.total && memory.used` is falsy -> returns current.
    const resource = baseResource({ memory: { current: 45.5 } });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).memory).toBe(45.5);
  });

  it('uses disk.current when total/used are absent', () => {
    // getDiskPercent: `disk.total && disk.used` is falsy -> returns current.
    const resource = baseResource({ disk: { current: 67 } });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).disk).toBe(67);
  });

  it('coerces a NaN memory.current to undefined via finiteMetric', () => {
    // getMemoryPercent returns NaN; finiteMetric: typeof === 'number' but
    // Number.isFinite is FALSE -> undefined.
    const resource = baseResource({ memory: { current: Number.NaN } });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).memory).toBeUndefined();
  });

  it('coerces a NaN disk.current to undefined via finiteMetric', () => {
    const resource = baseResource({ disk: { current: Number.NaN } });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).disk).toBeUndefined();
  });

  it('coerces an Infinity cpu.current to undefined via finiteMetric', () => {
    const resource = baseResource({
      cpu: { current: Number.POSITIVE_INFINITY },
    });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).cpu).toBeUndefined();
  });

  it('coerces a NaN cpu.current to undefined via finiteMetric', () => {
    const resource = baseResource({ cpu: { current: Number.NaN } });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).cpu).toBeUndefined();
  });

  it('coerces a non-number cpu.current (malformed payload) to undefined', () => {
    // finiteMetric: `typeof value === 'number'` is FALSE -> undefined.
    // Cast required because ResourceMetric.current is typed as number.
    const resource = baseResource({
      cpu: { current: 'busy' as unknown as number },
    });
    expect(getResourceMetricsHistoryFallbackMetrics(resource).cpu).toBeUndefined();
  });

  it('coerces non-finite network and diskIO rates to undefined', () => {
    const resource = baseResource({
      network: { rxBytes: Number.NaN, txBytes: Number.NEGATIVE_INFINITY },
      diskIO: {
        readRate: Number.POSITIVE_INFINITY,
        writeRate: Number.NaN,
      },
    });
    expect(getResourceMetricsHistoryFallbackMetrics(resource)).toStrictEqual({
      cpu: undefined,
      memory: undefined,
      disk: undefined,
      netin: undefined,
      netout: undefined,
      diskread: undefined,
      diskwrite: undefined,
    });
  });

  it('emits the memory ternary FALSE arm as undefined when memory is explicitly absent but disk is present', () => {
    // Mixes a present disk (TRUE arm) with an absent memory (FALSE arm) to prove
    // the two ternaries are evaluated independently.
    const resource = baseResource({ disk: { current: 18 } });
    const result = getResourceMetricsHistoryFallbackMetrics(resource);
    expect(result.memory).toBeUndefined();
    expect(result.disk).toBe(18);
  });
});
