import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { NodesAPI } from '@/api/nodes';
import { copyToClipboard } from '@/utils/clipboard';
import { notificationStore } from '@/stores/notifications';
import { showTokenReveal } from '@/stores/tokenReveal';
import type { NodeConfig } from '@/types/nodes';
import { useNodeModalState, type NodeModalState } from '../useNodeModalState';

vi.mock('@/api/nodes', () => ({
  NodesAPI: {
    getProxmoxSetupCommand: vi.fn(),
    downloadProxmoxSetupScript: vi.fn(),
    getAgentInstallCommand: vi.fn(),
  },
}));
vi.mock('@/utils/clipboard', () => ({ copyToClipboard: vi.fn() }));
vi.mock('@/stores/tokenReveal', () => ({ showTokenReveal: vi.fn() }));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), info: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));

type Platform = 'pve' | 'pbs';
const setup = (type: Platform) => ({
  type,
  host: `https://${type}.example.invalid`,
  command: 'fixture-setup-command',
  commandWithEnv: 'fixture-setup-command',
  url: '/api/setup-script',
  downloadURL: '/api/setup-script',
  scriptFileName: `pulse-setup-${type}.sh`,
  setupToken: 'synthetic-setup-token',
  tokenHint: 'synthetic-hint',
  expires: Date.now() / 1000 + 300,
});
const agent = { command: 'fixture-agent-command', token: 'synthetic-agent-token' };
const script = {
  content: '#!/bin/bash\n# synthetic only\n',
  contentType: 'text/x-shellscript',
  fileName: 'pulse-setup.sh',
};
const disposers: Array<() => void> = [];
const mount = (type: Platform) => {
  let state!: NodeModalState;
  const original = {
    id: `${type}-alpha`,
    type,
    host: `https://${type}.example.invalid`,
    name: 'alpha',
    verifySSL: true,
  } as NodeConfig;
  const [node, setNode] = createSignal(original);
  const [open, setOpen] = createSignal(true);
  const [reset, setReset] = createSignal(0);
  const [blocked, setBlocked] = createSignal(false);
  createRoot((dispose) => {
    disposers.push(dispose);
    state = useNodeModalState({
      get isOpen() {
        return open();
      },
      get nodeType() {
        return node().type;
      },
      get editingNode() {
        return node();
      },
      get resetKey() {
        return reset();
      },
      setupHandoffDisabled: blocked,
      onSave: vi.fn(),
      onClose: vi.fn(),
    });
  });
  return { state, original, setNode, setOpen, setReset, setBlocked, dispose: disposers.at(-1)! };
};
const deferred = <T>() => {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const flush = async () => {
  for (let i = 0; i < 5; i++) await Promise.resolve();
};
const copy = (state: NodeModalState, type: Platform) =>
  state.copyQuickSetupCommand(type, type === 'pve', 'Fixture copied');
const download = (state: NodeModalState, type: Platform) =>
  state.downloadProxmoxSetupScript(type, type === 'pve');
const copyAgent = (state: NodeModalState, type: Platform) =>
  state.copyProxmoxAgentInstallCommand(type, 'Fixture copied');
const noOutput = () => {
  expect(showTokenReveal).not.toHaveBeenCalled();
  expect(copyToClipboard).not.toHaveBeenCalled();
  expect(notificationStore.success).not.toHaveBeenCalled();
  expect(notificationStore.error).not.toHaveBeenCalled();
  expect(URL.createObjectURL).not.toHaveBeenCalled();
};
beforeEach(() => {
  vi.mocked(copyToClipboard).mockResolvedValue(true);
  vi.stubGlobal(
    'URL',
    Object.assign(class extends URL {}, {
      createObjectURL: vi.fn(() => 'blob:fixture'),
      revokeObjectURL: vi.fn(),
    }),
  );
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
});
afterEach(() => {
  disposers.splice(0).forEach((dispose) => dispose());
  vi.restoreAllMocks();
  vi.resetAllMocks();
  vi.unstubAllGlobals();
});

describe.each(['pve', 'pbs'] as const)(
  '%s setup output belongs to its current consumer',
  (type) => {
    beforeEach(() => {
      vi.mocked(NodesAPI.getProxmoxSetupCommand).mockResolvedValue(setup(type));
      vi.mocked(NodesAPI.getAgentInstallCommand).mockResolvedValue(agent);
      vi.mocked(NodesAPI.downloadProxmoxSetupScript).mockResolvedValue(script);
    });
    it('rejects obsolete mint output after endpoint A → B → A', async () => {
      const { state, original } = mount(type);
      const pending = deferred<ReturnType<typeof setup>>();
      vi.mocked(NodesAPI.getProxmoxSetupCommand).mockReturnValueOnce(pending.promise);
      const running = copy(state, type);
      state.updateField('host', 'https://other.example.invalid');
      state.updateField('host', original.host);
      pending.resolve(setup(type));
      await running;
      noOutput();
      expect(state.quickSetupCommandReady()).toBe(false);
    });
    it('rejects obsolete download output after the connection name changes', async () => {
      const { state } = mount(type);
      const pending = deferred<typeof script>();
      vi.mocked(NodesAPI.downloadProxmoxSetupScript).mockReturnValueOnce(pending.promise);
      const running = download(state, type);
      await flush();
      expect(NodesAPI.downloadProxmoxSetupScript).toHaveBeenCalledTimes(1);
      state.updateField('name', 'new draft');
      pending.resolve(script);
      await running;
      noOutput();
      expect(state.quickSetupCommandReady()).toBe(false);
    });
    it('rejects obsolete agent output after TLS preference A → B → A', async () => {
      const { state } = mount(type);
      state.updateField('setupMode', 'agent');
      const pending = deferred<typeof agent>();
      vi.mocked(NodesAPI.getAgentInstallCommand).mockReturnValueOnce(pending.promise);
      const running = copyAgent(state, type);
      state.setAgentInstallInsecure(true);
      state.setAgentInstallInsecure(false);
      pending.resolve(agent);
      await running;
      noOutput();
      expect(state.agentInstallCommand()).toBe('');
    });
    it('keeps issuance single across copy, download and mode changes', async () => {
      const { state } = mount(type);
      const pending = deferred<ReturnType<typeof setup>>();
      vi.mocked(NodesAPI.getProxmoxSetupCommand).mockReturnValueOnce(pending.promise);
      const running = copy(state, type);
      await copy(state, type);
      await download(state, type);
      state.updateField('setupMode', 'agent');
      await copyAgent(state, type);
      expect(NodesAPI.getProxmoxSetupCommand).toHaveBeenCalledTimes(1);
      expect(NodesAPI.getAgentInstallCommand).not.toHaveBeenCalled();
      expect(state.isPreparingSetup()).toBe(true);
      pending.resolve(setup(type));
      await running;
      noOutput();
      expect(state.isPreparingSetup()).toBe(false);
      await copyAgent(state, type);
      expect(NodesAPI.getAgentInstallCommand).toHaveBeenCalledTimes(1);
    });
    it.each([
      'host',
      'name',
      'mode',
      'auth',
      'close-reopen',
      'reset',
      'target',
      'approval',
      'dispose',
    ])('discards a late script failure after %s without false feedback', async (change) => {
      const hook = mount(type),
        { state } = hook;
      const pending = deferred<typeof script>();
      vi.mocked(NodesAPI.downloadProxmoxSetupScript).mockReturnValueOnce(pending.promise);
      const running = download(state, type);
      await flush();
      if (change === 'host') state.updateField('host', 'https://other.example.invalid');
      if (change === 'name') state.updateField('name', 'other');
      if (change === 'mode') {
        state.updateField('setupMode', 'agent');
        state.updateField('setupMode', 'auto');
      }
      if (change === 'auth') state.updateField('authType', 'password');
      if (change === 'close-reopen') {
        hook.setOpen(false);
        hook.setOpen(true);
      }
      if (change === 'reset') hook.setReset(1);
      if (change === 'target') hook.setNode({ ...hook.original, id: 'another-node' });
      if (change === 'approval') {
        hook.setBlocked(true);
        hook.setBlocked(false);
      }
      if (change === 'dispose') hook.dispose();
      expect(state.isPreparingSetup()).toBe(true);
      pending.reject(new Error('Obsolete fixture error'));
      await running;
      noOutput();
      expect(state.isPreparingSetup()).toBe(false);
    });
    it.each(['setup', 'agent'] as const)(
      'does not revive %s output during a late clipboard completion',
      async (kind) => {
        const { state } = mount(type);
        if (kind === 'agent') state.updateField('setupMode', 'agent');
        const pending = deferred<boolean>();
        vi.mocked(copyToClipboard).mockReturnValueOnce(pending.promise);
        const running = kind === 'agent' ? copyAgent(state, type) : copy(state, type);
        await flush();
        expect(copyToClipboard).toHaveBeenCalledTimes(1);
        expect(showTokenReveal).not.toHaveBeenCalled();
        state.updateField('host', 'https://new.example.invalid');
        pending.resolve(true);
        await running;
        expect(showTokenReveal).not.toHaveBeenCalled();
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(state.agentInstallCommand()).toBe('');
        expect(state.quickSetupCommandReady()).toBe(false);
        expect(state.isPreparingSetup()).toBe(false);
      },
    );
    it('reuses a current artifact through ordinary same-target health polling', async () => {
      const hook = mount(type),
        { state } = hook;
      await copy(state, type);
      hook.setNode({ ...hook.original, status: 'online' } as NodeConfig);
      state.updateField('guestURL', 'https://guest.example.invalid');
      await download(state, type);
      expect(NodesAPI.getProxmoxSetupCommand).toHaveBeenCalledExactlyOnceWith({
        type,
        host: hook.original.host,
        name: 'alpha',
        backupPerms: type === 'pve',
      });
      expect(NodesAPI.downloadProxmoxSetupScript).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({
          type,
          host: hook.original.host,
          setupToken: 'synthetic-setup-token',
        }),
      );
      expect(showTokenReveal).toHaveBeenCalledTimes(2);
      expect(copyToClipboard).toHaveBeenCalledExactlyOnceWith('fixture-setup-command');
      expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
      expect(URL.revokeObjectURL).toHaveBeenCalledExactlyOnceWith('blob:fixture');
      expect(notificationStore.error).not.toHaveBeenCalled();
    });
    it('clears a completed cache on a name change and renews an expired artifact', async () => {
      const { state } = mount(type);
      await copy(state, type);
      state.updateField('name', 'renamed');
      expect(state.quickSetupCommandReady()).toBe(false);
      vi.mocked(NodesAPI.getProxmoxSetupCommand).mockResolvedValueOnce({
        ...setup(type),
        expires: 1,
      });
      await copy(state, type);
      await copy(state, type);
      expect(NodesAPI.getProxmoxSetupCommand).toHaveBeenCalledTimes(3);
      expect(NodesAPI.getProxmoxSetupCommand).toHaveBeenLastCalledWith(
        expect.objectContaining({ name: 'renamed' }),
      );
    });
    it('keeps current agent scope, TLS defaults and separate-token handoff', async () => {
      const { state } = mount(type);
      state.updateField('setupMode', 'agent');
      await copyAgent(state, type);
      expect(NodesAPI.getAgentInstallCommand).toHaveBeenCalledExactlyOnceWith({
        type,
        enableProxmox: true,
        enableCommands: false,
        insecure: false,
      });
      expect(showTokenReveal).toHaveBeenCalledExactlyOnceWith(
        expect.objectContaining({ token: agent.token, source: type }),
      );
      expect(copyToClipboard).toHaveBeenCalledExactlyOnceWith(agent.command);
      expect(state.agentInstallCommand()).toBe(agent.command);
      state.setAgentInstallInsecure(true);
      expect(state.agentInstallCommand()).toBe('');
    });
    it.each(['setup', 'agent'] as const)(
      'keeps current %s copy failures actionable without revealing a token',
      async (kind) => {
        const { state } = mount(type);
        if (kind === 'agent') state.updateField('setupMode', 'agent');
        vi.mocked(copyToClipboard).mockResolvedValueOnce(false);
        await (kind === 'agent' ? copyAgent(state, type) : copy(state, type));
        expect(showTokenReveal).not.toHaveBeenCalled();
        expect(notificationStore.error).toHaveBeenCalledTimes(1);
        expect(state.isPreparingSetup()).toBe(false);
        await (kind === 'agent' ? copyAgent(state, type) : copy(state, type));
        expect(showTokenReveal).toHaveBeenCalledTimes(1);
      },
    );
    it('keeps current download errors actionable without revealing a token', async () => {
      const { state } = mount(type);
      vi.mocked(NodesAPI.downloadProxmoxSetupScript).mockRejectedValueOnce(
        new Error('Synthetic transport error'),
      );
      await download(state, type);
      expect(notificationStore.error).toHaveBeenCalledExactlyOnceWith(
        'Failed to download script. Please check your connection.',
      );
      expect(showTokenReveal).not.toHaveBeenCalled();
      expect(state.isPreparingSetup()).toBe(false);
      await download(state, type);
      expect(showTokenReveal).toHaveBeenCalledTimes(1);
      expect(NodesAPI.getProxmoxSetupCommand).toHaveBeenCalledTimes(1);
    });
    it('reclaims the blob and withholds its token when the download handoff fails', async () => {
      const { state } = mount(type);
      vi.mocked(HTMLAnchorElement.prototype.click).mockImplementationOnce(() => {
        throw new Error('Synthetic browser download refusal');
      });
      await download(state, type);
      expect(URL.revokeObjectURL).toHaveBeenCalledExactlyOnceWith('blob:fixture');
      expect(document.querySelector('a[download]')).toBeNull();
      expect(showTokenReveal).not.toHaveBeenCalled();
      expect(notificationStore.error).toHaveBeenCalledTimes(1);
      expect(state.isPreparingSetup()).toBe(false);
    });
    it('keeps the original busy owner through departure and drops late agent errors', async () => {
      const hook = mount(type),
        { state } = hook;
      state.updateField('setupMode', 'agent');
      const pending = deferred<typeof agent>();
      vi.mocked(NodesAPI.getAgentInstallCommand).mockReturnValueOnce(pending.promise);
      const running = copyAgent(state, type);
      hook.setOpen(false);
      hook.setOpen(true);
      expect(state.loadingAgentCommand()).toBe(true);
      expect(state.isPreparingSetup()).toBe(true);
      await copyAgent(state, type);
      expect(NodesAPI.getAgentInstallCommand).toHaveBeenCalledTimes(1);
      pending.reject(new Error('Obsolete agent error'));
      await running;
      noOutput();
      expect(state.loadingAgentCommand()).toBe(false);
      expect(state.isPreparingSetup()).toBe(false);
    });
    it('enforces the existing setup review boundary in the handlers', async () => {
      const hook = mount(type);
      hook.setBlocked(true);
      await copy(hook.state, type);
      await copyAgent(hook.state, type);
      await download(hook.state, type);
      expect(NodesAPI.getAgentInstallCommand).not.toHaveBeenCalled();
      expect(NodesAPI.getProxmoxSetupCommand).not.toHaveBeenCalled();
      noOutput();
    });
  },
);
