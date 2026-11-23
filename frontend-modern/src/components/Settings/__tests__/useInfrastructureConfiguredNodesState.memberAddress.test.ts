import { createRoot, createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { NodesAPI } from '@/api/nodes';
import type { ClusterEndpoint, NodeConfigWithStatus } from '@/types/nodes';
import { useInfrastructureConfiguredNodesState } from '../useInfrastructureConfiguredNodesState';

vi.mock('@/api/nodes', () => ({ NodesAPI: { updateNode: vi.fn(), getNodes: vi.fn() } }));
vi.mock('@/hooks/useUnifiedResources', () => ({
  useUnifiedResources: () => ({ resources: () => [], aggregations: () => null }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const reachedAt = '2026-11-22T09:00:00Z';
const peer: ClusterEndpoint = {
  nodeId: 'node/cedar',
  nodeName: 'cedar',
  host: 'https://cedar.example.invalid:8006',
  ip: '192.0.2.11',
  ipOverride: 'old-cedar.example.invalid',
  tlsEndpoint: 'https://old-cedar.example.invalid:8006',
  online: true,
  lastSeen: reachedAt,
  pulseReachable: false,
  lastPulseCheck: reachedAt,
  pulseError: 'Connection refused',
};
const sibling: ClusterEndpoint = {
  nodeId: 'node/maple',
  nodeName: 'maple',
  host: 'https://maple.example.invalid:8006',
  ip: '192.0.2.12',
  tlsEndpoint: 'https://maple.example.invalid:8006',
  online: true,
  lastSeen: reachedAt,
  pulseReachable: true,
  lastPulseCheck: reachedAt,
};
const snapshot = (cedar = peer): NodeConfigWithStatus => ({
  id: 'pve-0',
  type: 'pve',
  name: 'Fixture cluster',
  host: sibling.host,
  user: 'fixture@pve',
  hasPassword: false,
  hasToken: true,
  verifySSL: true,
  monitorVMs: true,
  monitorContainers: true,
  monitorStorage: true,
  monitorBackups: true,
  monitorPhysicalDisks: false,
  status: 'connected',
  clusterEndpoints: [cedar, sibling],
});

function createNodesState() {
  return createRoot((dispose) => {
    const [saving, setSaving] = createSignal(false);
    const state = useInfrastructureConfiguredNodesState({
      canReadInfrastructure: () => true,
      temperatureMonitoringEnabled: () => false,
      savingTemperatureSetting: saving,
      setSavingTemperatureSetting: setSaving,
    });
    return { state, dispose };
  });
}

describe('saved Proxmox member address readback', () => {
  beforeEach(() => {
    vi.resetAllMocks();
  });

  it('marks the edited member pending on read failure, ignores a pre-save GET and accepts a fresh GET', async () => {
    const { state, dispose } = createNodesState();
    const before = snapshot();
    const newOverride = 'new-cedar.example.invalid:8443';
    const after = snapshot({
      ...peer,
      ipOverride: newOverride,
      tlsEndpoint: `https://${newOverride}`,
      pulseReachable: undefined,
      lastPulseCheck: undefined,
      pulseError: undefined,
    });
    let finishOldRead!: (nodes: NodeConfigWithStatus[]) => void;
    const oldRead = new Promise<NodeConfigWithStatus[]>((resolve) => (finishOldRead = resolve));
    vi.mocked(NodesAPI.getNodes)
      .mockResolvedValueOnce([before])
      .mockReturnValueOnce(oldRead)
      .mockRejectedValueOnce(new Error('Inventory unavailable'))
      .mockResolvedValueOnce([after]);
    vi.mocked(NodesAPI.updateNode).mockResolvedValueOnce({ success: true });
    const endpoints = () => {
      const node = state.nodes()[0];
      return node && 'clusterEndpoints' in node ? node.clusterEndpoints : undefined;
    };
    try {
      await state.loadNodes();
      const prior = state.loadNodes();
      const payload = {
        clusterEndpointOverrides: [{ nodeName: 'cedar', ipOverride: newOverride }],
      };
      expect(await state.saveNode(payload, before)).toBe(true);
      expect(NodesAPI.updateNode).toHaveBeenCalledExactlyOnceWith('pve-0', payload);
      expect(NodesAPI.getNodes).toHaveBeenCalledTimes(3);
      expect(endpoints()).toEqual([
        { ...peer, ipOverride: newOverride, awaitingAddressReadback: true },
        sibling,
      ]);
      expect(state.nodes()[0]).not.toHaveProperty('clusterEndpointOverrides');

      finishOldRead([before]);
      await prior;
      expect(endpoints()?.[0]).toMatchObject({
        ipOverride: newOverride,
        awaitingAddressReadback: true,
      });

      await state.loadNodes();
      expect(endpoints()).toEqual('clusterEndpoints' in after ? after.clusterEndpoints : undefined);
      expect(endpoints()?.[0]).not.toHaveProperty('awaitingAddressReadback');
    } finally {
      dispose();
    }
  });

  it('uses the server result, including a preserved check after equivalent spelling', async () => {
    const { state, dispose } = createNodesState();
    const before = snapshot();
    const equivalent = 'OLD-CEDAR.example.invalid:8006';
    const after = snapshot({
      ...peer,
      ipOverride: equivalent,
      tlsEndpoint: `https://${equivalent}`,
    });
    vi.mocked(NodesAPI.updateNode).mockResolvedValueOnce({ success: true });
    vi.mocked(NodesAPI.getNodes).mockResolvedValueOnce([before]).mockResolvedValueOnce([after]);
    try {
      await state.loadNodes();
      expect(
        await state.saveNode(
          { clusterEndpointOverrides: [{ nodeName: 'cedar', ipOverride: equivalent }] },
          before,
        ),
      ).toBe(true);
      expect(state.nodes()).toEqual([after]);
    } finally {
      dispose();
    }
  });
});
