import { describe, expect, it } from 'vitest';

import type { CoreTemp, Node, Temperature } from '@/types/api';

import { getNodeDrawerHistoryFallbackMetrics } from '../nodeDrawerModel';

const makeNode = (overrides: Partial<Node> = {}): Node => ({
  id: 'agent:pve-node-1',
  name: 'pve-node-1',
  instance: 'homelab',
  host: 'pve-node-1',
  status: 'online',
  type: 'agent',
  cpu: 0.42,
  memory: { total: 8000, used: 3200, free: 4800, usage: 40 },
  disk: { total: 10000, used: 4500, free: 5500, usage: 45 },
  networkIn: 1200,
  networkOut: 800,
  diskRead: 400,
  diskWrite: 300,
  uptime: 3600,
  loadAverage: [0.5],
  kernelVersion: '6.8.0',
  pveVersion: 'pve-manager/9.1.9',
  cpuInfo: { model: 'Ryzen', cores: 8, sockets: 1, mhz: '3200' },
  temperature: {
    cpuPackage: 62.5,
    cpuMax: 65,
    available: true,
    hasCPU: true,
    lastUpdate: new Date().toISOString(),
  },
  temperatureMonitoringEnabled: true,
  lastSeen: new Date().toISOString(),
  connectionHealth: 'online',
  isClusterMember: true,
  clusterName: 'homelab',
  linkedAgentId: '',
  ...overrides,
});

const makeTemp = (overrides: Partial<Temperature> = {}): Temperature => ({
  available: true,
  lastUpdate: new Date().toISOString(),
  ...overrides,
});

describe('getNodeDrawerHistoryFallbackMetrics (branch coverage 0712c)', () => {
  it('returns temperature: undefined when the temperature object is present but available is missing', () => {
    // Optional chain `?.available` with a truthy object whose `available` key is absent:
    // !undefined === true -> early null -> ?? undefined (right arm).
    const node = makeNode({
      temperature: makeTemp({ available: undefined as unknown as boolean }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: undefined,
    });
  });

  it('returns temperature: 0 when cpuPackage is 0 (zero is a valid finite candidate)', () => {
    // isValidTemperature(0): typeof === 'number' && Number.isFinite(0) -> true (left arm of ??).
    // Asserts the boundary that 0 is NOT clobbered by the nullish coalescing operator.
    const node = makeNode({
      temperature: makeTemp({ cpuPackage: 0, cpuMax: undefined }),
    });
    const result = getNodeDrawerHistoryFallbackMetrics(node);
    expect(result.temperature).toBe(0);
    expect(result).toStrictEqual({ temperature: 0 });
  });

  it('preserves a fractional cpuPackage exactly through the ?? left arm', () => {
    // Float candidate survives Math.max single-element spread unchanged.
    const node = makeNode({
      temperature: makeTemp({ cpuPackage: 62.5, cpuMax: undefined }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 62.5,
    });
  });

  it('skips an Infinity cpuPackage (non-finite) and falls back to cpuMax', () => {
    // isValidTemperature(Infinity): Number.isFinite(Infinity) === false -> cpuPackage skipped.
    // Distinct from the NaN case: Infinity is a typeof 'number' that is not finite.
    const node = makeNode({
      temperature: makeTemp({ cpuPackage: Number.POSITIVE_INFINITY, cpuMax: 71 }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 71,
    });
  });

  it('still returns cpuPackage when cores is an empty array (forEach with zero iterations)', () => {
    // Array.isArray([]) === true (cores branch taken) but forEach never executes,
    // so no core candidates are appended; cpuPackage is the sole candidate.
    const node = makeNode({
      temperature: makeTemp({ cpuPackage: 55, cpuMax: undefined, cores: [] }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 55,
    });
  });

  it('builds candidates solely from cores when cpuPackage and cpuMax are absent', () => {
    // cpuPackage/cpuMax undefined -> both isValidTemperature false arms; the entire
    // candidates list is populated by the cores forEach, then Math.max returns 92.
    const node = makeNode({
      temperature: makeTemp({
        cpuPackage: undefined,
        cpuMax: undefined,
        cores: [
          { core: 0, temp: 88 },
          { core: 1, temp: 92 },
          { core: 2, temp: 90 },
        ],
      }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 92,
    });
  });

  it('skips a core whose temp is missing and keeps the valid sibling core', () => {
    // A CoreTemp with `temp` absent (required field omitted via cast): core.temp is
    // undefined -> isValidTemperature false -> skipped; the valid sibling is kept.
    const node = makeNode({
      temperature: makeTemp({
        cpuPackage: undefined,
        cpuMax: undefined,
        cores: [
          { core: 0 } as unknown as CoreTemp,
          { core: 1, temp: 75 },
        ],
      }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 75,
    });
  });

  it('ignores a malformed non-array cores value and yields undefined when no candidates remain', () => {
    // Array.isArray(<object>) === false (distinct from undefined): cores branch skipped.
    // cpuPackage/cpuMax absent -> candidates empty -> length === 0 -> null -> ?? undefined.
    const node = makeNode({
      temperature: makeTemp({
        cpuPackage: undefined,
        cpuMax: undefined,
        cores: { '0': { temp: 99 } } as unknown as CoreTemp[],
      }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: undefined,
    });
  });

  it('returns the maximum (least-negative) value across all-negative finite candidates', () => {
    // All candidates are finite negatives; Math.max picks the least-negative (-3),
    // proving the max aggregation works below zero rather than defaulting to null.
    const node = makeNode({
      temperature: makeTemp({
        cpuPackage: -40,
        cpuMax: -20,
        cores: [
          { core: 0, temp: -10 },
          { core: 1, temp: -3 },
        ],
      }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: -3,
    });
  });

  it('returns temperature: undefined when available is true but every candidate field is invalid', () => {
    // available === true (guard passes), yet cpuPackage/cpuMax are NaN and cores absent:
    // isValidTemperature false for both -> candidates empty -> length === 0 -> null.
    const node = makeNode({
      temperature: makeTemp({ cpuPackage: Number.NaN, cpuMax: Number.NaN }),
    });
    const result = getNodeDrawerHistoryFallbackMetrics(node);
    expect(result).toStrictEqual({ temperature: undefined });
    expect('temperature' in result).toBe(true);
  });

  it('prefers the largest valid core temp over higher-indexed invalid ones', () => {
    // Mixed valid/invalid cores: index 0 valid (95), index 1 NaN (skipped),
    // index 2 valid (60); cpuPackage/cpuMax absent -> Math.max(95, 60) === 95.
    const node = makeNode({
      temperature: makeTemp({
        cpuPackage: undefined,
        cpuMax: undefined,
        cores: [
          { core: 0, temp: 95 },
          { core: 1, temp: Number.NaN },
          { core: 2, temp: 60 },
        ],
      }),
    });
    expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
      temperature: 95,
    });
  });
});
