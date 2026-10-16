import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import type { NodeConfig } from '@/types/nodes';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';
import type { NodeModalProps } from '../nodeModalModel';

vi.mock('@/api/nodes', () => ({
  NodesAPI: { testExistingNode: vi.fn(), testConnection: vi.fn() },
}));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));

describe('connection tests with retained credentials', () => {
  let dispose: (() => void) | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(NodesAPI.testExistingNode).mockResolvedValue({ status: 'success' });
    vi.mocked(NodesAPI.testConnection).mockResolvedValue({ status: 'success' });
  });

  afterEach(() => dispose?.());

  const mount = (nodeType: NodeModalProps['nodeType']) => {
    let state!: NodeModalState;
    dispose = createRoot((cleanup) => {
      state = useNodeModalState({
        isOpen: true,
        nodeType,
        editingNode: {
          id: `${nodeType}-saved`,
          type: nodeType,
          name: 'saved',
          host: 'https://node.example.test:8006',
          tokenName: 'root@pam!pulse',
          fingerprint: 'saved-pin',
          verifySSL: true,
        } as NodeConfig,
        onClose: vi.fn(),
        onSave: vi.fn(),
      });
      return cleanup;
    });
    return state;
  };

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
});
