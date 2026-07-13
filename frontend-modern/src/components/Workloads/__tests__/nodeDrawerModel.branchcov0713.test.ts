import { describe, expect, it } from 'vitest';

import type { Node, Temperature } from '@/types/api';

import { getNodeDrawerHistoryFallbackMetrics, getNodeDrawerHistoryTarget } from '../nodeDrawerModel';

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

describe('nodeDrawerModel (branch coverage 0713)', () => {
  describe('getNodeDrawerHistoryTarget', () => {
    it('strips only the first agent: prefix when the value is double-prefixed', () => {
      // stripAgentPrefix runs a single startsWith + slice, so 'agent:agent:node-9'
      // collapses to 'agent:node-9' (one prefix removed), NOT 'node-9'.
      const node = makeNode({
        linkedAgentId: 'agent:agent:node-9',
        id: 'agent:pve-node-1',
        name: 'pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'agent:node-9',
      });
    });

    it('preserves a realistic agent UUID after stripping the prefix', () => {
      // Happy path: long UUID round-trips through stripAgentPrefix without truncation.
      const uuid = '550e8400-e29b-41d4-a716-446655440000';
      const node = makeNode({
        linkedAgentId: `agent:${uuid}`,
        id: 'agent:pve-node-1',
        name: 'pve-node-1',
      });
      const result = getNodeDrawerHistoryTarget(node);
      expect(result).toStrictEqual({ resourceType: 'agent', resourceId: uuid });
      expect(result?.resourceId).toHaveLength(uuid.length);
    });

    it('is case-sensitive: a capitalised "Agent:" prefix is not stripped', () => {
      // String.prototype.startsWith is case-sensitive, so 'Agent:foo' survives intact.
      const node = makeNode({
        linkedAgentId: 'Agent:foo',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'Agent:foo',
      });
    });

    it('treats the truthy string "0" as a valid resourceId', () => {
      // '0' is a truthy string (unlike the number 0): the || chain selects it and
      // !'0' === false, so the success branch runs.
      const node = makeNode({
        linkedAgentId: '0',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: '0',
      });
    });

    it('trims whitespace on the id fallback arm when id has no agent: prefix', () => {
      // || arm 2 (id) + stripAgentPrefix false arm (no prefix) + .trim() -> 'node-7'.
      const node = makeNode({
        linkedAgentId: '',
        id: '  node-7  ',
        name: 'whatever',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'node-7',
      });
    });

    it('returns null when the name fallback is whitespace-only', () => {
      // || arm 3 (name): '   ' is truthy so it is chosen, but .trim() -> '' -> !resourceId null.
      const node = makeNode({
        linkedAgentId: '',
        id: '',
        name: '   ',
      });
      expect(getNodeDrawerHistoryTarget(node)).toBeNull();
    });

    it('accepts a single-character unprefixed linkedAgentId', () => {
      // Minimal truthy input: one character, no prefix, no surrounding whitespace.
      const node = makeNode({
        linkedAgentId: 'a',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'a',
      });
    });

    it('falls through an absent linkedAgentId and empty id to a prefixed name', () => {
      // linkedAgentId is undefined (optional field) -> || arm 2 (id '') is falsy
      // -> || arm 3 (name 'agent:host-9') -> stripAgentPrefix true arm -> 'host-9'.
      const node = makeNode({
        linkedAgentId: undefined,
        id: '',
        name: 'agent:host-9',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'host-9',
      });
    });
  });

  describe('getNodeDrawerHistoryFallbackMetrics', () => {
    it('returns temperature: undefined for an empty temperature object', () => {
      // temperature?.available on {} -> undefined -> !undefined === true -> null -> ?? undefined.
      const node = makeNode({ temperature: {} as unknown as Temperature });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: undefined,
      });
    });

    it('treats a truthy non-boolean available value (1) as available', () => {
      // !1 === false so the guard passes; cpuPackage 50 is the sole candidate.
      const node = makeNode({
        temperature: makeTemp({
          available: 1 as unknown as boolean,
          cpuPackage: 50,
          cpuMax: undefined,
        }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: 50,
      });
    });

    it('returns cpuPackage when it is the largest of all valid sources', () => {
      // cpuPackage 95 > cpuMax 70 and core temps 80/60 -> Math.max selects cpuPackage.
      const node = makeNode({
        temperature: makeTemp({
          cpuPackage: 95,
          cpuMax: 70,
          cores: [
            { core: 0, temp: 80 },
            { core: 1, temp: 60 },
          ],
        }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: 95,
      });
    });

    it('skips an Infinity core temp and returns the max of the finite cores', () => {
      // core temp Infinity -> Number.isFinite false -> skipped; Math.max(88, 72) === 88.
      const node = makeNode({
        temperature: makeTemp({
          cpuPackage: undefined,
          cpuMax: undefined,
          cores: [
            { core: 0, temp: 88 },
            { core: 1, temp: Number.POSITIVE_INFINITY },
            { core: 2, temp: 72 },
          ],
        }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: 88,
      });
    });

    it('returns undefined when cores is an empty array and cpu fields are absent', () => {
      // Array.isArray([]) === true -> forEach runs zero times; candidates empty -> null.
      const node = makeNode({
        temperature: makeTemp({ cpuPackage: undefined, cpuMax: undefined, cores: [] }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: undefined,
      });
    });

    it('preserves Number.MAX_VALUE as a finite boundary candidate', () => {
      // Number.isFinite(MAX_VALUE) === true -> valid; sole candidate -> returned unchanged.
      const node = makeNode({
        temperature: makeTemp({ cpuPackage: Number.MAX_VALUE, cpuMax: undefined }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: Number.MAX_VALUE,
      });
    });

    it('returns cpuMax when cpuPackage is absent and cores are absent', () => {
      // cpuPackage undefined -> isValidTemperature false -> skipped; cpuMax 68 is the sole candidate.
      const node = makeNode({
        temperature: makeTemp({ cpuPackage: undefined, cpuMax: 68 }),
      });
      expect(getNodeDrawerHistoryFallbackMetrics(node)).toStrictEqual({
        temperature: 68,
      });
    });

    it('always returns an object with exactly the single key "temperature"', () => {
      // Shape contract: exactly one key regardless of the resolved value.
      const node = makeNode({
        temperature: makeTemp({ cpuPackage: 42, cpuMax: undefined }),
      });
      const result = getNodeDrawerHistoryFallbackMetrics(node);
      expect(Object.keys(result)).toEqual(['temperature']);
      expect(result.temperature).toBe(42);
    });
  });
});
