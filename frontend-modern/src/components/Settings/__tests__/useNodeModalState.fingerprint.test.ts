import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import type { NodeConfig } from '@/types/nodes';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';

vi.mock('@/api/nodes', () => ({
  NodesAPI: { testConnection: vi.fn(), testExistingNode: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));
vi.mock('@/stores/notifications', () => ({ notificationStore: { error: vi.fn() } }));

const disposers: Array<() => void> = [];
const mount = (type: 'pve' | 'pbs' | 'pmg' = 'pve') => {
  let state!: NodeModalState;
  const onSave = vi.fn();
  createRoot((dispose) => {
    disposers.push(dispose);
    state = useNodeModalState({
      nodeType: type,
      isOpen: true,
      editingNode: {
        id: 'fixture-source',
        type,
        name: 'Fixture source',
        host: 'https://fixture.example.invalid',
        user: 'fixture@pam',
        tokenName: 'fixture@pam!read',
        hasToken: true,
        verifySSL: true,
        fingerprint: 'ab'.repeat(32),
      } as NodeConfig,
      onSave,
      onClose: vi.fn(),
    });
  });
  return { state, onSave };
};

afterEach(() => {
  disposers.splice(0).forEach((dispose) => dispose());
  vi.resetAllMocks();
});

describe('Proxmox certificate pin draft validation', () => {
  it.each(['pve', 'pbs', 'pmg'] as const)('does not save an impossible %s pin', (type) => {
    const { state, onSave } = mount(type);
    state.updateField('fingerprint', 'not-a-valid-pin');
    state.handleSubmit(new Event('submit'));
    expect(onSave).not.toHaveBeenCalled();
    expect(state.formData().fingerprint).toBe('not-a-valid-pin');
    expect(state.formData().verifySSL).toBe(true);
    expect(NodesAPI.testConnection).not.toHaveBeenCalled();
    expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
  });

  it.each([
    'ab'.repeat(31),
    'ab'.repeat(33),
    'gg'.repeat(32),
    'SHA256:' + 'ab'.repeat(32),
    ' ' + 'ab'.repeat(32),
    'ab'.repeat(32) + ' ',
    ':',
    '   ',
  ])('does not save malformed fingerprint %j', (fingerprint) => {
    const { state, onSave } = mount();
    state.updateField('fingerprint', fingerprint);
    state.handleSubmit(new Event('submit'));
    expect(onSave).not.toHaveBeenCalled();
    expect(state.formData().fingerprint).toBe(fingerprint);
  });

  it.each(['', 'AB'.repeat(32), 'ab:'.repeat(31) + 'ab', ':' + 'a:b:'.repeat(32)])(
    'preserves the runtime-supported pin bytes %j without claiming trust',
    (fingerprint) => {
      const { state, onSave } = mount();
      state.updateField('fingerprint', fingerprint);
      state.handleSubmit(new Event('submit'));
      expect(onSave).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({
          fingerprint,
          verifySSL: true,
        }),
      );
      expect(NodesAPI.testConnection).not.toHaveBeenCalled();
      expect(NodesAPI.testExistingNode).not.toHaveBeenCalled();
      expect(state.testResult()).toBeNull();
    },
  );

  it('does not send entered credentials to test an impossible draft pin', async () => {
    const { state } = mount();
    state.updateField('fingerprint', 'SHA256:' + 'ab'.repeat(32));
    state.updateField('tokenValue', 'synthetic-only-value');
    await state.handleTestConnection();
    expect(NodesAPI.testConnection).not.toHaveBeenCalled();
    expect(state.testResult()).toBeNull();
    expect(state.isTesting()).toBe(false);
  });

  it('still allows a saved-record test while its unsaved pin is malformed', async () => {
    const { state } = mount();
    vi.mocked(NodesAPI.testExistingNode).mockResolvedValue({ status: 'success' });
    state.updateField('fingerprint', 'malformed-draft');
    await state.handleTestConnection();
    expect(NodesAPI.testExistingNode).toHaveBeenCalledExactlyOnceWith('fixture-source');
    expect(NodesAPI.testConnection).not.toHaveBeenCalled();
    expect(state.testResult()).toMatchObject({ scope: 'saved', status: 'success' });
    expect(state.formData().fingerprint).toBe('malformed-draft');
  });

  it('lets an operator correct the field and save without clearing or disabling trust', () => {
    const { state, onSave } = mount();
    state.updateField('fingerprint', 'truncated');
    state.handleSubmit(new Event('submit'));
    expect(onSave).not.toHaveBeenCalled();
    state.updateField('fingerprint', 'CD:'.repeat(31) + 'CD');
    state.handleSubmit(new Event('submit'));
    expect(onSave).toHaveBeenCalledExactlyOnceWith(
      expect.objectContaining({
        fingerprint: 'CD:'.repeat(31) + 'CD',
        verifySSL: true,
      }),
    );
  });
});
