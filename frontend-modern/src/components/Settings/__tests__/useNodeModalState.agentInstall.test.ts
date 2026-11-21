import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import { copyToClipboard } from '@/utils/clipboard';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';

vi.mock('@/api/nodes', () => ({
  NodesAPI: {
    getAgentInstallCommand: vi.fn(),
  },
}));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));
vi.mock('@/stores/tokenReveal', () => ({ showTokenReveal: vi.fn() }));

describe('useNodeModalState host telemetry agent pre-generation controls', () => {
  let dispose: (() => void) | undefined;

  beforeEach(() => {
    vi.clearAllMocks();
    vi.mocked(copyToClipboard).mockResolvedValue(true);
    vi.mocked(NodesAPI.getAgentInstallCommand).mockResolvedValue({
      command: 'curl -fsSL https://pulse.example/install.sh | bash',
      token: 'agent-secret-token',
    });
  });

  afterEach(() => dispose?.());

  const mount = (nodeType: 'pve' | 'pbs' = 'pve') => {
    let state!: NodeModalState;
    dispose = createRoot((cleanup) => {
      state = useNodeModalState({
        isOpen: true,
        nodeType,
        onClose: vi.fn(),
        onSave: vi.fn(),
      });
      return cleanup;
    });
    return state;
  };

  it('defaults to monitoring-only without custom URL and forwards opted-in pre-generation controls', async () => {
    const state = mount('pve');

    expect(state.agentInstallEnableCommands()).toBe(false);
    expect(state.agentInstallInsecure()).toBe(false);
    expect(state.agentInstallUrl()).toBe('');

    await state.copyProxmoxAgentInstallCommand('pve', 'Copied');

    expect(NodesAPI.getAgentInstallCommand).toHaveBeenLastCalledWith({
      type: 'pve',
      enableProxmox: true,
      enableCommands: false,
      insecure: false,
    });
    expect(state.agentInstallCommand()).toBe('curl -fsSL https://pulse.example/install.sh | bash');

    // Changing any pre-generation option clears the stale previously minted command.
    state.setAgentInstallEnableCommands(true);
    expect(state.agentInstallCommand()).toBe('');

    await state.copyProxmoxAgentInstallCommand('pve', 'Copied');
    expect(state.agentInstallCommand()).not.toBe('');

    state.setAgentInstallUrl(' https://10.0.0.5:7655 ');
    expect(state.agentInstallCommand()).toBe('');

    state.setAgentInstallInsecure(true);
    expect(state.agentInstallCommand()).toBe('');

    await state.copyProxmoxAgentInstallCommand('pve', 'Copied');

    expect(NodesAPI.getAgentInstallCommand).toHaveBeenLastCalledWith({
      type: 'pve',
      enableProxmox: true,
      enableCommands: true,
      insecure: true,
      agentUrl: 'https://10.0.0.5:7655',
    });
  });
});
