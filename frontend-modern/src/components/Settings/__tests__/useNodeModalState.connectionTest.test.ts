import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import type { ClusterEndpoint, NodeConfig } from '@/types/nodes';
import { useNodeModalState } from '../useNodeModalState';
import type { NodeModalProps } from '../nodeModalModel';

vi.mock('@/api/nodes', () => ({
  NodesAPI: { testExistingNode: vi.fn(), testConnection: vi.fn() },
}));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));

const deferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((done, fail) => {
    resolve = done;
    reject = fail;
  });
  return { promise, resolve, reject };
};

describe('Proxmox connection tests', () => {
  let dispose: (() => void) | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(NodesAPI.testExistingNode).mockResolvedValue({ status: 'success' });
    vi.mocked(NodesAPI.testConnection).mockResolvedValue({ status: 'success' });
  });

  afterEach(() => dispose?.());

  const mountEditor = (
    nodeType: NodeModalProps['nodeType'],
    clusterEndpoints?: ClusterEndpoint[],
  ) =>
    createRoot((cleanup) => {
      dispose = cleanup;
      const [isOpen, setOpen] = createSignal(true);
      const [resetKey, setResetKey] = createSignal<number>();
      const [node, setNode] = createSignal<NodeConfig>({
        id: `${nodeType}-saved`,
        type: nodeType,
        name: 'saved',
        host: 'https://node.example.test:8006',
        tokenName: 'root@pam!pulse',
        fingerprint: 'saved-pin',
        verifySSL: true,
        user: 'fixture@pve',
        monitorVMs: true,
        monitorContainers: true,
        monitorStorage: true,
        monitorBackups: true,
        monitorPhysicalDisks: false,
        monitorDatastores: true,
        monitorSyncJobs: true,
        monitorVerifyJobs: true,
        monitorPruneJobs: true,
        monitorGarbageJobs: true,
        excludeDatastores: [],
        monitorMailStats: true,
        monitorQueues: true,
        monitorQuarantine: true,
        monitorDomainStats: false,
        clusterEndpoints,
      } as NodeConfig);
      const state = useNodeModalState({
        get isOpen() {
          return isOpen();
        },
        get resetKey() {
          return resetKey();
        },
        nodeType,
        get editingNode() {
          return node();
        },
        onClose: vi.fn(),
        onSave: vi.fn(),
      });
      return { state, setNode, setOpen, setResetKey };
    });

  const mount = (nodeType: NodeModalProps['nodeType'], clusterEndpoints?: ClusterEndpoint[]) =>
    mountEditor(nodeType, clusterEndpoints).state;

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'tests edited TLS settings for %s without replacing credentials',
    async (platform) => {
      const state = mount(platform);
      state.updateField('verifySSL', false);
      state.updateField('fingerprint', '');

      await state.handleTestConnection();

      expect(NodesAPI.testExistingNode).toHaveBeenCalledWith(`${platform}-saved`, {
        verifySSL: false,
        fingerprint: '',
      });
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
      expect(state.testResult()?.status).toBe('success');
      expect(state.formData().verifySSL).toBe(false);
      expect(state.formData().fingerprint).toBe('');
    },
  );

  it('tests an edited fingerprint with verification enabled', async () => {
    const state = mount('pve');
    state.updateField('fingerprint', 'edited-pin');

    await state.handleTestConnection();

    expect(NodesAPI.testExistingNode).toHaveBeenCalledWith('pve-saved', {
      verifySSL: true,
      fingerprint: 'edited-pin',
    });
  });

  it('uses the supplied credentials when they are replaced', async () => {
    const state = mount('pve');
    state.updateField('authType', 'token');
    state.updateField('tokenValue', 'replacement-token');
    state.updateField('verifySSL', false);
    state.updateField('fingerprint', '');

    await state.handleTestConnection();

    expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
    expect(NodesAPI.testConnection).toHaveBeenCalledWith(
      expect.objectContaining({
        tokenValue: 'replacement-token',
        verifySSL: false,
        fingerprint: '',
      }),
    );
  });

  it('discards a draft success after editing the endpoint and tests the new draft on request', async () => {
    const state = mount('pve');
    state.updateField('tokenValue', 'draft-token');
    const oldDraft = deferred<Awaited<ReturnType<typeof NodesAPI.testConnection>>>();
    vi.mocked(NodesAPI.testConnection).mockReturnValueOnce(oldDraft.promise);

    const pending = state.handleTestConnection();
    state.updateField('host', 'https://second.example.test:8006');
    state.updateField('fingerprint', 'second-pin');
    expect(state.isTesting()).toBe(true);
    await state.handleTestConnection();
    expect(NodesAPI.testConnection).toHaveBeenCalledTimes(1);
    oldDraft.resolve({ status: 'success', message: 'Old host accepted', isCluster: true });
    await pending;

    expect(state.testResult()).toBeNull();
    expect(state.isTesting()).toBe(false);

    vi.mocked(NodesAPI.testConnection).mockResolvedValueOnce({
      status: 'success',
      message: 'New host answered',
      warnings: ['Inventory scope limited'],
    });
    await state.handleTestConnection();
    expect(NodesAPI.testConnection).toHaveBeenLastCalledWith(
      expect.objectContaining({
        host: 'https://second.example.test:8006',
        fingerprint: 'second-pin',
        verifySSL: true,
      }),
    );
    expect(state.testResult()).toMatchObject({
      status: 'warning',
      message: 'New host answered',
      warnings: ['Inventory scope limited'],
    });
  });

  it('discards a draft failure even when the operator restores the previous pin', async () => {
    const state = mount('pbs');
    state.updateField('tokenValue', 'draft-token');
    const oldDraft = deferred<Awaited<ReturnType<typeof NodesAPI.testConnection>>>();
    vi.mocked(NodesAPI.testConnection).mockReturnValueOnce(oldDraft.promise);

    const pending = state.handleTestConnection();
    state.updateField('fingerprint', 'different-pin');
    state.updateField('fingerprint', 'saved-pin');
    oldDraft.reject(new Error('API request failed: 400 Old failure'));
    await pending;

    expect(state.testResult()).toBeNull();
    expect(state.isTesting()).toBe(false);

    vi.mocked(NodesAPI.testConnection).mockRejectedValueOnce(
      new Error('API request failed: 400 Current failure'),
    );
    await state.handleTestConnection();
    expect(state.testResult()).toMatchObject({ status: 'error', message: 'Current failure' });
  });

  const peer = {
    nodeName: 'peer',
    tlsEndpoint: 'https://peer.test:8006',
    fingerprint: 'peer-pin',
    ipOverride: '',
    host: 'https://peer.test:8006',
    ip: '',
    nodeId: 'peer',
    online: true,
    lastSeen: '',
  } satisfies ClusterEndpoint;

  it('tests an explicit peer CA choice without copying the primary pin or saving', async () => {
    const state = mount('pve', [peer]);
    state.updateClusterEndpointFingerprint(peer.tlsEndpoint, '');
    await state.handleTestClusterEndpoint(peer);
    expect(NodesAPI.testExistingNode).toHaveBeenCalledWith('pve-saved', {
      endpoint: peer.tlsEndpoint,
      fingerprint: '',
      verifySSL: true,
    });
    expect(NodesAPI.testConnection).not.toHaveBeenCalled();
    expect(state.testResult()?.message).toBe(
      'peer: Endpoint connection successful. Background collection was not tested.',
    );
    expect(state.testResult()?.endpoint).toBe(peer.tlsEndpoint);
    state.updateClusterEndpointFingerprint(peer.tlsEndpoint, 'new-pin');
    expect(state.testResult()).toBeNull();
  });

  it('refuses to claim a test of unsaved addresses using stored credentials', async () => {
    const state = mount('pve', [peer]);
    state.updateClusterEndpointOverride('peer', 'new-peer.test');
    await state.handleTestClusterEndpoint(peer);
    expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
    expect(state.testResult()?.status).toBe('error');
    state.updateField('host', 'https://changed-primary.test:8006');
    await state.handleTestConnection();
    expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
  });

  it('does not display an old in-flight success after trust changes', async () => {
    const state = mount('pve', [peer]);
    let complete!: (result: { status: string }) => void;
    vi.mocked(NodesAPI.testExistingNode).mockReturnValueOnce(
      new Promise((resolve) => {
        complete = resolve;
      }),
    );
    const pending = state.handleTestClusterEndpoint(peer);
    state.updateClusterEndpointFingerprint(peer.tlsEndpoint, 'different-pin');
    complete({ status: 'success' });
    await pending;
    expect(state.testResult()).toBeNull();
  });

  it('keeps a valid member result through a health refresh but clears it when saved trust changes', async () => {
    const { state, setNode } = mountEditor('pve', [peer]);
    await state.handleTestClusterEndpoint(peer);
    const result = state.testResult();
    expect(result?.status).toBe('success');

    setNode((node) => ({
      ...node,
      clusterEndpoints: [{ ...peer, online: false, pulseReachable: false, displayName: 'Lab' }],
    }));
    expect(state.testResult()).toBe(result);

    setNode((node) => ({
      ...node,
      clusterEndpoints: [{ ...peer, fingerprint: 'changed-saved-pin' }],
    }));
    expect(state.testResult()).toBeNull();
  });

  it.each(['reset', 'close and reopen', 'different saved node'] as const)(
    'ignores a saved test response after %s',
    async (change) => {
      const { state, setNode, setResetKey, setOpen } = mountEditor('pve');
      const oldTest = deferred<Awaited<ReturnType<typeof NodesAPI.testExistingNode>>>();
      vi.mocked(NodesAPI.testExistingNode).mockReturnValueOnce(oldTest.promise);

      const pending = state.handleTestConnection();
      switch (change) {
        case 'reset':
          setResetKey(1);
          break;
        case 'close and reopen':
          setOpen(false);
          setOpen(true);
          break;
        case 'different saved node':
          setNode((node) => ({ ...node, id: 'pve-different' }));
          break;
      }
      oldTest.resolve({ status: 'success', message: 'Old saved connection answered' });
      await pending;

      expect(state.testResult()).toBeNull();
      expect(state.isTesting()).toBe(false);
    },
  );

  it('tests untouched member trust without implicitly clearing its saved policy', async () => {
    const state = mount('pve', [peer]);
    await state.handleTestClusterEndpoint(peer);
    expect(NodesAPI.testExistingNode).toHaveBeenCalledWith('pve-saved', {
      endpoint: peer.tlsEndpoint,
      verifySSL: true,
    });
  });
});
