import { describe, expect, it } from 'vitest';

import type { Node, Temperature } from '@/types/api';

import { getNodeDrawerCurrentMetrics, getNodeDrawerHistoryTarget } from '../nodeDrawerModel';

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
  cpuTemperature: 65,
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
    it.each(['node', 'agent'] as const)(
      'uses canonical %s storage coordinates before display and linked identities',
      (resourceType) => {
        const node = makeNode({
          linkedAgentId: 'agent:legacy-link',
          metricsTarget: { resourceType, resourceId: 'retained-native-id' },
        });
        expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
          resourceType,
          resourceId: 'retained-native-id',
        });
      },
    );

    it.each([
      { resourceType: 'node', resourceId: '   ' },
      { resourceType: 'vm', resourceId: 'guest-id' },
    ] as const)(
      'retains legacy history when canonical coordinates are unusable: %o',
      (metricsTarget) => {
        expect(
          getNodeDrawerHistoryTarget(
            makeNode({ linkedAgentId: 'agent:legacy-link', metricsTarget }),
          ),
        ).toStrictEqual({ resourceType: 'agent', resourceId: 'legacy-link' });
      },
    );

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
        resourceType: 'node',
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

    it('falls through an absent linkedAgentId and empty id to the node name', () => {
      const node = makeNode({
        linkedAgentId: undefined,
        id: '',
        name: 'agent:host-9',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'agent:host-9',
      });
    });
  });

  describe('getNodeDrawerCurrentMetrics', () => {
    it('returns temperature: undefined when only sensor details exist', () => {
      const node = makeNode({
        cpuTemperature: undefined,
        temperature: makeTemp({ cpuPackage: 62, cpuMax: 68 }),
      });
      expect(getNodeDrawerCurrentMetrics(node)).toStrictEqual({ temperature: undefined });
    });

    it('always returns an object with exactly the single key "temperature"', () => {
      // Shape contract: exactly one key regardless of the resolved value.
      const node = makeNode({ cpuTemperature: 42 });
      const result = getNodeDrawerCurrentMetrics(node);
      expect(Object.keys(result)).toEqual(['temperature']);
      expect(result.temperature).toBe(42);
    });
  });
});
