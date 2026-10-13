import { describe, expect, it } from 'vitest';

import type { Node, Temperature } from '@/types/api';

import {
  getNodeDrawerCurrentMetrics,
  getNodeDrawerHistoryGroups,
  getNodeDrawerHistoryTarget,
} from '../nodeDrawerModel';

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

describe('nodeDrawerModel (branch coverage 2)', () => {
  describe('getNodeDrawerHistoryTarget', () => {
    it('prefers linkedAgentId and returns it unchanged when it has no agent: prefix', () => {
      // || chain arm 1 (linkedAgentId truthy) + stripAgentPrefix false arm + success return.
      const node = makeNode({
        linkedAgentId: 'agent-uuid-9',
        id: 'agent:pve-node-1',
        name: 'pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'agent-uuid-9',
      });
    });

    it('strips the agent: prefix from linkedAgentId', () => {
      // stripAgentPrefix true arm (startsWith) on the linkedAgentId arm.
      const node = makeNode({
        linkedAgentId: 'agent:agent-uuid-9',
        id: 'agent:pve-node-1',
        name: 'pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'agent-uuid-9',
      });
    });

    it('uses the node identity directly when no agent is linked', () => {
      const node = makeNode({
        linkedAgentId: '',
        id: 'homelab-pve-node-1',
        name: 'pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'homelab-pve-node-1',
      });
    });

    it('falls back to name when linkedAgentId and id are both empty', () => {
      // || chain arm 3 (name) + stripAgentPrefix false arm (no prefix on name).
      const node = makeNode({
        linkedAgentId: '',
        id: '',
        name: 'pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'pve-node-1',
      });
    });

    it('preserves the node name fallback verbatim', () => {
      const node = makeNode({
        linkedAgentId: '',
        id: '',
        name: 'agent:pve-node-1',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'agent:pve-node-1',
      });
    });

    it('returns null when linkedAgentId, id, and name are all empty', () => {
      // || chain arm 4 ('') + !resourceId true arm.
      const node = makeNode({
        linkedAgentId: '',
        id: '',
        name: '',
      });
      expect(getNodeDrawerHistoryTarget(node)).toBeNull();
    });

    it('ignores a whitespace-only linked agent id and uses the node id', () => {
      const node = makeNode({
        linkedAgentId: '   ',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'agent:ignored',
      });
    });

    it('returns null when the chosen value is exactly "agent:" (strip -> empty)', () => {
      // stripAgentPrefix('agent:') -> '' -> !resourceId null.
      const node = makeNode({
        linkedAgentId: 'agent:',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toBeNull();
    });

    it('trims surrounding whitespace before stripping the agent: prefix', () => {
      // '  agent:foo  '.trim() -> 'agent:foo' -> strip -> 'foo'.
      const node = makeNode({
        linkedAgentId: '  agent:foo  ',
        id: 'agent:ignored',
        name: 'ignored',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'agent',
        resourceId: 'foo',
      });
    });

    it('treats an absent (undefined) linkedAgentId as falsy and falls through to id', () => {
      // Optional linkedAgentId absent -> || falls to id arm.
      const node = makeNode({
        linkedAgentId: undefined,
        id: 'agent:node-7',
        name: 'node-7',
      });
      expect(getNodeDrawerHistoryTarget(node)).toStrictEqual({
        resourceType: 'node',
        resourceId: 'agent:node-7',
      });
    });
  });

  describe('getNodeDrawerHistoryGroups', () => {
    it('keeps disk throughput for agent-backed nodes', () => {
      const groups = getNodeDrawerHistoryGroups(makeNode({ linkedAgentId: 'host-1' }));
      expect(groups.map((group) => group.id)).toContain('disk-io');
    });

    it('omits unavailable host disk throughput for API-only nodes', () => {
      const groups = getNodeDrawerHistoryGroups(makeNode({ linkedAgentId: '' }));
      expect(groups.map((group) => group.id)).toStrictEqual(['utilization', 'network', 'thermals']);
    });

    it('adds GPU history only when the linked agent reports typed GPU telemetry', () => {
      const groups = getNodeDrawerHistoryGroups(
        makeNode({
          linkedAgentId: 'host-1',
          sensors: { gpu: [{ id: '0', utilizationPercent: 42 }] },
        }),
      );
      expect(groups.map((group) => group.id)).toStrictEqual([
        'utilization',
        'network',
        'disk-io',
        'thermals',
        'gpu-utilization',
        'gpu-thermal',
      ]);
    });
  });

  describe('getNodeDrawerCurrentMetrics', () => {
    it('returns the backend primary CPU temperature unchanged', () => {
      const node = makeNode({ cpuTemperature: 71.5 });
      expect(getNodeDrawerCurrentMetrics(node)).toStrictEqual({ temperature: 71.5 });
    });

    it('returns temperature: undefined when the backend sent no primary reading', () => {
      const node = makeNode({ cpuTemperature: undefined });
      expect(getNodeDrawerCurrentMetrics(node)).toStrictEqual({ temperature: undefined });
    });

    it('never recomputes the reading from the sensor details', () => {
      // A hotter core in the details must not override the backend's rule,
      // because the history line beneath the headline was recorded by it.
      const node = makeNode({
        cpuTemperature: 44,
        temperature: makeTemp({ cpuPackage: 44, cpuMax: 51, cores: [{ core: 0, temp: 51 }] }),
      });
      expect(getNodeDrawerCurrentMetrics(node)).toStrictEqual({ temperature: 44 });
    });

    it('projects bounded aggregate GPU values into current history metrics', () => {
      const node = makeNode({
        sensors: {
          gpu: [
            {
              id: '0',
              utilizationPercent: 42,
              memoryUsedBytes: 8 * 1024,
              memoryTotalBytes: 32 * 1024,
              temperatureCelsius: 63,
            },
            {
              id: '1',
              utilizationPercent: 75,
              memoryUsedBytes: 24 * 1024,
              memoryTotalBytes: 32 * 1024,
              temperatureCelsius: 70,
            },
          ],
        },
      });

      expect(getNodeDrawerCurrentMetrics(node)).toStrictEqual({
        temperature: 65,
        gpu: 75,
        gpu_memory: 75,
        gpu_temperature: 70,
      });
    });
  });
});
