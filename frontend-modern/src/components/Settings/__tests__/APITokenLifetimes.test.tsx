import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { createSignal, Show } from 'solid-js';
import { cleanup, fireEvent, render, screen, waitFor, within } from '@solidjs/testing-library';
import { SecurityAPI, type APITokenRecord } from '@/api/security';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { dismissTokenReveal, showTokenReveal, useTokenRevealState } from '@/stores/tokenReveal';
import { setOrgID } from '@/utils/apiClient';
import { APITokenManager } from '../APITokenManager';
import { useAPITokenManagerState } from '../useAPITokenManagerState';

vi.mock('@/api/security', () => ({
  SecurityAPI: {
    listTokens: vi.fn(),
    createToken: vi.fn(),
    deleteToken: vi.fn(),
    renameToken: vi.fn(),
    updateTokenScopes: vi.fn(),
  },
}));
vi.mock('@/api/agentCapabilities', () => ({
  fetchAgentCapabilitiesManifest: () => Promise.resolve({ requiredScopes: [] }),
}));
vi.mock('@/contexts/appRuntime', () => ({
  useWebSocket: () => ({
    markDockerRuntimesTokenRevoked: vi.fn(),
    markAgentsTokenRevoked: vi.fn(),
  }),
}));
vi.mock('@/hooks/useResources', () => ({
  useResources: () => ({ byType: () => [], resources: () => [] }),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn(), debug: vi.fn() } }));

const record: APITokenRecord = {
  id: 'token-1',
  name: 'Known token',
  prefix: 'test',
  suffix: '1234',
  createdAt: '2026-11-01T00:00:00Z',
  scopes: ['monitoring:read'],
};
const created = {
  token: 'synthetic-only-secret',
  record: { ...record, id: 'created', name: 'Created token' },
};
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
let state: ReturnType<typeof useAPITokenManagerState>;
let setAllowed!: (allowed: boolean) => void;
let setUser!: (hint: string) => void;
let setMounted!: (mounted: boolean) => void;
const changed = vi.fn();
const Harness = () => {
  const [allowed, allow] = createSignal(true);
  const [user, userTo] = createSignal('initial');
  const [mounted, mount] = createSignal(true);
  setAllowed = allow;
  setUser = userTo;
  setMounted = mount;
  const Panel = () => {
    state = useAPITokenManagerState({
      get canManage() {
        return allowed();
      },
      get currentUser() {
        return user();
      },
      onTokensChanged: changed,
    });
    return null;
  };
  return (
    <Show when={mounted()}>
      <Panel />
    </Show>
  );
};
const ready = async () => {
  render(() => <Harness />);
  await waitFor(() => expect(state.tokensLoaded()).toBe(true));
  state.applyScopePreset(['monitoring:read']);
};
const reveal = useTokenRevealState();
beforeEach(() => {
  vi.resetAllMocks();
  setOrgID('org-a');
  dismissTokenReveal();
  vi.mocked(SecurityAPI.listTokens).mockResolvedValue([record]);
  vi.mocked(SecurityAPI.createToken).mockResolvedValue(created);
  vi.mocked(SecurityAPI.renameToken).mockResolvedValue({ ...record, name: 'Renamed' });
  vi.mocked(SecurityAPI.updateTokenScopes).mockResolvedValue({
    ...record,
    scopes: ['settings:read'],
  });
});
afterEach(() => {
  cleanup();
  dismissTokenReveal();
  setOrgID(null);
});

describe('API token request lifetimes', () => {
  it.each(['success', 'failure'])(
    'ignores a late create %s after unmount instead of revealing its credential globally',
    async (result) => {
      await ready();
      const pending = deferred<typeof created>();
      vi.mocked(SecurityAPI.createToken).mockReturnValueOnce(pending.promise);
      const writing = state.handleGenerate();
      const options = vi.mocked(SecurityAPI.createToken).mock.calls[0][2]!;
      setMounted(false);
      if (result === 'success') pending.resolve(created);
      else pending.reject(new Error('synthetic failure'));
      await writing;
      expect(reveal()).toBeNull();
      expect(state.newTokenValue()).toBeNull();
      expect(options?.signal?.aborted).toBe(true);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
    },
  );

  it.each(['permission', 'organisation', 'user identity'])(
    'withdraws a create across a %s change and never replays it on return',
    async (change) => {
      await ready();
      const pending = deferred<typeof created>();
      vi.mocked(SecurityAPI.createToken).mockReturnValueOnce(pending.promise);
      const writing = state.handleGenerate();
      const signal = vi.mocked(SecurityAPI.createToken).mock.calls[0][2]!.signal!;
      if (change === 'permission') {
        setAllowed(false);
        setAllowed(true);
      }
      if (change === 'organisation') {
        setOrgID('org-b');
        eventBus.emit('org_switched', 'org-b');
        setOrgID('org-a');
        eventBus.emit('org_switched', 'org-a');
      }
      if (change === 'user identity') {
        setUser('other');
        setUser('initial');
      }
      expect(signal.aborted).toBe(true);
      pending.resolve(created);
      await writing;
      expect(reveal()).toBeNull();
      expect(changed).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(SecurityAPI.createToken).toHaveBeenCalledTimes(1);
      await waitFor(() => expect(state.tokensLoaded()).toBe(true));
      expect(state.sortedTokens()).toEqual([record]);
    },
  );

  it.each(['success', 'failure'])(
    'ignores a disposed inventory %s without claiming absence or notifying',
    async (result) => {
      const pending = deferred<APITokenRecord[]>();
      vi.mocked(SecurityAPI.listTokens).mockReturnValueOnce(pending.promise);
      render(() => <Harness />);
      const signal = vi.mocked(SecurityAPI.listTokens).mock.calls[0][0]!.signal!;
      setMounted(false);
      expect(signal.aborted).toBe(true);
      if (result === 'success') pending.resolve([]);
      else pending.reject(new Error('synthetic failure'));
      await pending.promise.catch(() => {});
      await Promise.resolve();
      expect(state.tokensLoaded()).toBe(false);
      expect(notificationStore.error).not.toHaveBeenCalled();
    },
  );

  it('does not let an old organisation inventory replace the new accepted read', async () => {
    const oldRead = deferred<APITokenRecord[]>();
    vi.mocked(SecurityAPI.listTokens).mockReturnValueOnce(oldRead.promise);
    render(() => <Harness />);
    const oldSignal = vi.mocked(SecurityAPI.listTokens).mock.calls[0][0]!.signal!;
    setOrgID('org-b');
    eventBus.emit('org_switched', 'org-b');
    await waitFor(() => expect(state.tokensLoaded()).toBe(true));
    oldRead.resolve([]);
    await oldRead.promise;
    await Promise.resolve();
    expect(oldSignal.aborted).toBe(true);
    expect(state.sortedTokens()).toEqual([record]);
    expect(SecurityAPI.listTokens).toHaveBeenLastCalledWith(
      expect.objectContaining({ expectedOrgID: 'org-b' }),
    );
  });

  it.each(['rename', 'scope', 'revoke'] as const)(
    'ignores a late %s success after withdrawal',
    async (operation) => {
      await ready();
      const pending = deferred<APITokenRecord>();
      vi.mocked(SecurityAPI.renameToken).mockReturnValueOnce(pending.promise);
      vi.mocked(SecurityAPI.updateTokenScopes).mockReturnValueOnce(pending.promise);
      vi.mocked(SecurityAPI.deleteToken).mockReturnValueOnce(pending.promise.then(() => undefined));
      const writing =
        operation === 'rename'
          ? state.handleRename(record, 'Renamed')
          : operation === 'scope'
            ? state.handleUpdateScopes(record, ['settings:read'])
            : state.handleDelete(record);
      setAllowed(false);
      pending.resolve({ ...record, name: 'Obsolete completion' });
      await writing;
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(changed).not.toHaveBeenCalled();
      expect(state.sortedTokens()).not.toContainEqual(
        expect.objectContaining({ name: 'Obsolete completion' }),
      );
    },
  );

  it('serialises all writes and resumes an interrupted inventory read without accepting its old snapshot', async () => {
    const read = deferred<APITokenRecord[]>();
    vi.mocked(SecurityAPI.listTokens)
      .mockReturnValueOnce(read.promise)
      .mockResolvedValueOnce([record, created.record]);
    render(() => <Harness />);
    state.applyScopePreset(['monitoring:read']);
    const pending = deferred<typeof created>();
    vi.mocked(SecurityAPI.createToken).mockReturnValueOnce(pending.promise);
    const writing = state.handleGenerate();
    await state.handleGenerate();
    await state.loadTokens();
    expect(SecurityAPI.createToken).toHaveBeenCalledTimes(1);
    expect(SecurityAPI.listTokens).toHaveBeenCalledTimes(1);
    pending.resolve(created);
    await writing;
    await waitFor(() => expect(state.tokensLoaded()).toBe(true));
    read.resolve([]);
    await read.promise;
    await Promise.resolve();
    expect(state.sortedTokens()).toHaveLength(2);
    expect(reveal()?.token).toBe(created.token);
    expect(SecurityAPI.listTokens).toHaveBeenCalledTimes(2);
    expect(changed).toHaveBeenCalledTimes(1);
  });

  it('clears only its owned reveal and local credential when its authority is withdrawn', async () => {
    await ready();
    await state.handleGenerate();
    expect(reveal()?.token).toBe(created.token);
    setAllowed(false);
    expect(reveal()).toBeNull();
    expect(state.newTokenValue()).toBeNull();
    state.reopenTokenDialog();
    await state.copyNewMonitoringKioskLink();
    expect(reveal()).toBeNull();
    expect(state.newMonitoringKioskLink()).toBeNull();
  });

  it('does not dismiss a replacement token reveal owned by another issuer', async () => {
    await ready();
    await state.handleGenerate();
    showTokenReveal({ token: 'other-synthetic-secret', source: 'agent-setup' });
    const other = reveal();
    setMounted(false);
    expect(reveal()).toBe(other);
    expect(state.newTokenValue()).toBeNull();
  });

  it('rejects a row retained from an earlier accepted inventory', async () => {
    await ready();
    await state.loadTokens();
    // Same ID, different accepted object is not the earlier confirmation.
    const obsolete = { ...record };
    expect(await state.handleRename(obsolete, 'Wrong')).toBe(false);
    expect(await state.handleUpdateScopes(obsolete, ['*'])).toBe(false);
    await state.handleDelete(obsolete);
    expect(SecurityAPI.renameToken).not.toHaveBeenCalled();
    expect(SecurityAPI.updateTokenScopes).not.toHaveBeenCalled();
    expect(SecurityAPI.deleteToken).not.toHaveBeenCalled();
  });

  it('closes an open action dialog on withdrawal while keeping read-only inventory usable', async () => {
    const [allowed, allow] = createSignal(true);
    render(() => <APITokenManager canManage={allowed()} />);
    await screen.findAllByText('Known token');
    const row = screen.getByRole('article', { name: 'API token Known token' });
    fireEvent.click(within(row).getByRole('button', { name: 'Rename' }));
    await screen.findByRole('dialog', { name: 'Rename API token' });
    allow(false);
    await waitFor(() => expect(screen.queryByRole('dialog')).not.toBeInTheDocument());
    await screen.findAllByText('Known token');
    expect(
      within(screen.getByRole('article', { name: 'API token Known token' })).getByRole('button', {
        name: 'Rename',
      }),
    ).toBeDisabled();
    expect(SecurityAPI.renameToken).not.toHaveBeenCalled();
  });
});
