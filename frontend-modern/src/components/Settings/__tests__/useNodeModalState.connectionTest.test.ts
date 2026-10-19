import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import type { NodeConfig } from '@/types/nodes';
import type { NodeModalProps } from '../nodeModalModel';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';

vi.mock('@/api/nodes', () => ({
  NodesAPI: { testConnection: vi.fn(), testExistingNode: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn(), info: vi.fn() } }));

type NodeType = NodeModalProps['nodeType'];
type TestResponse = Awaited<ReturnType<typeof NodesAPI.testConnection>>;
const success: TestResponse = { status: 'success', message: 'Fixture connected' };
const savedNode = (type: NodeType = 'pve') =>
  ({
    id: `${type}-fixture`,
    type,
    name: 'Fixture',
    host: 'https://saved.example.invalid:8006',
    user: 'fixture@pam',
    tokenName: 'fixture@pam!read',
    hasToken: true,
    hasPassword: type === 'pmg',
    verifySSL: true,
    fingerprint: 'ab'.repeat(32),
  }) as unknown as NodeConfig;
const disposers: Array<() => void> = [];
const mount = (type: NodeType = 'pve', existing = true) => {
  let state!: NodeModalState;
  const [node, setNode] = createSignal<NodeConfig | undefined>(
    existing ? savedNode(type) : undefined,
  );
  const [open, setOpen] = createSignal(true);
  const [resetKey, setResetKey] = createSignal(0);
  createRoot((dispose) => {
    disposers.push(dispose);
    state = useNodeModalState({
      get nodeType() {
        return node()?.type ?? type;
      },
      get editingNode() {
        return node();
      },
      get isOpen() {
        return open();
      },
      get resetKey() {
        return resetKey();
      },
      onSave: vi.fn(),
      onClose: vi.fn(),
    });
  });
  return { state, setNode, setOpen, setResetKey, dispose: disposers.at(-1)! };
};
const deferred = () => {
  let resolve!: (value: TestResponse) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<TestResponse>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};

afterEach(() => {
  disposers.splice(0).forEach((dispose) => dispose());
  vi.resetAllMocks();
});

describe('node connection-test evidence', () => {
  it.each(['pve', 'pbs', 'pmg'] as const)(
    'blocks an impossible new %s pin locally and sends only an explicit corrected test',
    async (type) => {
      const { state } = mount(type, false);
      state.updateField('host', 'https://fixture.example.invalid');
      state.updateField('authType', 'password');
      state.updateField('user', 'fixture@pam');
      state.updateField('password', 'synthetic-only-password');
      state.updateField('fingerprint', 'SHA256:' + 'ab'.repeat(32));
      await state.handleTestConnection();
      expect(state.fingerprintError()).toContain('exactly 64 hexadecimal characters');
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
      expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
      expect(state.testResult()).toBeNull();

      const corrected = 'CD:'.repeat(31) + 'CD';
      state.updateField('fingerprint', corrected);
      expect(state.fingerprintError()).toBeNull();
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
      vi.mocked(NodesAPI.testConnection).mockResolvedValue(success);
      await state.handleTestConnection();
      expect(NodesAPI.testConnection).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({ fingerprint: corrected, verifySSL: true }),
      );
      expect(state.testResult()).toMatchObject({ scope: 'form', status: 'success' });
    },
  );

  it.each(['pve', 'pbs', 'pmg'] as const)(
    'tests only the saved %s record without a new secret',
    async (type) => {
      const { state } = mount(type);
      vi.mocked(NodesAPI.testExistingNode).mockResolvedValue(success);
      state.updateField('host', 'https://edited.example.invalid');
      state.updateField('verifySSL', false);
      state.updateField('fingerprint', 'cd'.repeat(32));
      state.updateField('user', 'different@pam');
      state.updateField('tokenName', 'different@pam!read');
      expect(state.connectionTestScope()).toBe('saved');
      await state.handleTestConnection();
      expect(NodesAPI.testExistingNode).toHaveBeenCalledExactlyOnceWith(`${type}-fixture`);
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
      expect(state.testResult()).toMatchObject({ scope: 'saved', status: 'success' });
    },
  );

  it.each(['token', 'password'])(
    'tests entered endpoint and TLS with a newly entered %s',
    async (auth) => {
      const { state } = mount();
      vi.mocked(NodesAPI.testConnection).mockResolvedValue(success);
      state.updateField('authType', auth);
      state.updateField(auth === 'token' ? 'tokenValue' : 'password', 'fixture-only-secret');
      state.updateField('host', 'https://entered.example.invalid');
      state.updateField('fingerprint', 'cd'.repeat(32));
      expect(state.connectionTestScope()).toBe('form');
      await state.handleTestConnection();
      expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
      expect(NodesAPI.testConnection).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({
          host: 'https://entered.example.invalid',
          verifySSL: true,
          fingerprint: 'cd'.repeat(32),
          [auth === 'token' ? 'tokenValue' : 'password']: 'fixture-only-secret',
        }),
      );
      expect(state.testResult()).toMatchObject({ scope: 'form', status: 'success' });
    },
  );

  it.each([
    ['host', 'https://other.example.invalid'],
    ['verifySSL', false],
    ['fingerprint', ''],
    ['authType', 'password'],
    ['user', 'other@pam'],
    ['password', 'new-fixture'],
    ['tokenName', 'other@pam!read'],
    ['tokenValue', 'new-fixture'],
  ] as const)(
    'retires completed evidence when %s changes, without another test',
    async (field, value) => {
      const { state } = mount();
      vi.mocked(NodesAPI.testExistingNode).mockResolvedValue(success);
      await state.handleTestConnection();
      expect(state.testResult()).not.toBeNull();
      state.updateField(field, value);
      expect(state.testResult()).toBeNull();
      expect(NodesAPI.testExistingNode).toHaveBeenCalledTimes(1);
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
    },
  );

  it.each(['success', 'error'])(
    'discards an in-flight %s after A → B → A without overlapping tests',
    async (outcome) => {
      const { state } = mount();
      const pending = deferred();
      vi.mocked(NodesAPI.testExistingNode).mockReturnValueOnce(pending.promise);
      const running = state.handleTestConnection();
      state.updateField('host', 'https://other.example.invalid');
      state.updateField('host', savedNode().host);
      await state.handleTestConnection();
      expect(state.isTesting()).toBe(true);
      expect(NodesAPI.testExistingNode).toHaveBeenCalledTimes(1);
      if (outcome === 'success') pending.resolve(success);
      else pending.reject(new Error('Obsolete failure'));
      await running;
      expect(state.testResult()).toBeNull();
      expect(state.isTesting()).toBe(false);
      vi.mocked(NodesAPI.testExistingNode).mockResolvedValueOnce(success);
      await state.handleTestConnection();
      expect(state.testResult()?.scope).toBe('saved');
      expect(NodesAPI.testExistingNode).toHaveBeenCalledTimes(2);
    },
  );

  it.each(['target', 'saved-trust', 'close-reopen', 'reset', 'dispose'])(
    'drops late results after %s',
    async (change) => {
      const hook = mount();
      const pending = deferred();
      vi.mocked(NodesAPI.testExistingNode).mockReturnValueOnce(pending.promise);
      const running = hook.state.handleTestConnection();
      hook.state.updateField('name', 'Keep my edit');
      if (change === 'target') hook.setNode({ ...savedNode(), id: 'pve-other' });
      if (change === 'saved-trust') hook.setNode({ ...savedNode(), verifySSL: false });
      if (change === 'close-reopen') {
        hook.setOpen(false);
        hook.setOpen(true);
      }
      if (change === 'reset') hook.setResetKey(1);
      if (change === 'dispose') hook.dispose();
      pending.resolve(success);
      await running;
      expect(hook.state.testResult()).toBeNull();
      if (change === 'saved-trust') expect(hook.state.formData().name).toBe('Keep my edit');
      expect(NodesAPI.testExistingNode).toHaveBeenCalledTimes(1);
    },
  );

  it('keeps a result across health-only polls and unrelated coverage/name edits', async () => {
    const { state, setNode } = mount();
    vi.mocked(NodesAPI.testExistingNode).mockResolvedValue(success);
    await state.handleTestConnection();
    const result = state.testResult();
    state.updateField('name', 'Renamed');
    state.updateField('monitorVMs', false);
    state.updateClusterEndpointOverride('member', '192.0.2.4');
    state.updateClusterNodeDisplayName('member', 'Readable name');
    setNode({ ...savedNode(), status: 'disconnected' });
    expect(state.testResult()).toBe(result);
    expect(state.formData().name).toBe('Renamed');
    expect(NodesAPI.testExistingNode).toHaveBeenCalledTimes(1);
  });

  it('preserves saved-test warnings instead of presenting an unqualified success', async () => {
    const { state } = mount('pbs');
    vi.mocked(NodesAPI.testExistingNode).mockResolvedValue({
      ...success,
      warnings: ['Fixture limited coverage'],
    });
    await state.handleTestConnection();
    expect(state.testResult()).toMatchObject({
      scope: 'saved',
      status: 'warning',
      warnings: ['Fixture limited coverage'],
    });
  });

  it.each(['error', 'future-status'])(
    'does not turn a returned %s into success',
    async (status) => {
      const { state } = mount();
      vi.mocked(NodesAPI.testExistingNode).mockResolvedValue({ status });
      await state.handleTestConnection();
      expect(state.testResult()).toMatchObject({
        status: 'error',
        message: 'Connection failed',
        scope: 'saved',
      });
    },
  );

  it('keeps a failed current request scoped and retires it on a trust edit', async () => {
    const { state } = mount();
    vi.mocked(NodesAPI.testExistingNode).mockRejectedValue(
      new Error('API request failed: 400 Fixture pin mismatch'),
    );
    await state.handleTestConnection();
    expect(state.testResult()).toMatchObject({
      status: 'error',
      message: 'Fixture pin mismatch',
      scope: 'saved',
    });
    state.updateField('fingerprint', 'cd'.repeat(32));
    expect(state.testResult()).toBeNull();
  });

  it('validates a new form locally rather than falling back to saved credentials', async () => {
    const { state } = mount('pbs', false);
    expect(state.connectionTestScope()).toBe('form');
    await state.handleTestConnection();
    expect(state.testResult()).toMatchObject({
      scope: 'form',
      status: 'error',
      message: 'Endpoint URL is required',
    });
    state.updateField('host', 'https://new.example.invalid');
    await state.handleTestConnection();
    expect(state.testResult()?.message).toBe('Token ID and token value are required');
    expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
    expect(NodesAPI.testConnection).not.toHaveBeenCalled();
  });
});
