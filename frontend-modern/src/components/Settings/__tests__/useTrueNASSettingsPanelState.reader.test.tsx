import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TrueNASAPI, type TrueNASConnection } from '@/api/truenas';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

const scope = vi.hoisted(() => ({ org: 'team-a' }));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => scope.org }));
vi.mock('@/api/truenas', () => ({
  TrueNASAPI: {
    listConnections: vi.fn(),
    createConnection: vi.fn(),
    updateConnection: vi.fn(),
    deleteConnection: vi.fn(),
    previewConnection: vi.fn(),
    previewSavedConnection: vi.fn(),
    testConnection: vi.fn(),
    testSavedConnection: vi.fn(),
  },
  isRedactedTrueNASSecret: (value?: string) => value?.trim() === '********',
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const connection = (): TrueNASConnection => ({
  id: 'same-id',
  name: 'Synthetic NAS',
  host: 'nas.example.invalid',
  username: 'monitor',
  apiKey: '********',
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: 'SHA256:synthetic-pin',
  enabled: true,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
});
const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  let reject!: (error: unknown) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
};
const ready = async (state: ReturnType<typeof useTrueNASSettingsPanelState>) =>
  waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.connections()).toHaveLength(1);
  });
const openDraft = (state: ReturnType<typeof useTrueNASSettingsPanelState>) => {
  state.openEditDialog(state.connections()[0]);
  state.updateForm({ apiKey: 'synthetic-draft', name: 'Unconfirmed draft' });
};

describe('TrueNAS reader lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    scope.org = 'team-a';
    vi.mocked(TrueNASAPI.listConnections).mockImplementation(async () => [connection()]);
  });

  it('withdraws accepted inventory, draft secrets and confirmations when access is revoked', async () => {
    const [canLoad, setAllowed] = createSignal(true);
    const { result } = renderHook(() => useTrueNASSettingsPanelState({ canLoad }));
    await ready(result);
    openDraft(result);
    const old = result.connections()[0];
    setAllowed(false);
    await waitFor(() => expect(result.connections()).toEqual([]));
    expect(result.form().apiKey).toBe('');
    expect(result.dialogOpen()).toBe(false);
    expect(result.editingConnection()).toBeNull();
    expect(result.pendingDeleteConnection()).toBeNull();
    result.openEditDialog(old);
    result.openDeleteDialog(old);
    result.openCreateDialog();
    result.updateForm({ apiKey: 'must-not-stick' });
    await result.testCurrentForm();
    await result.testSavedConnection(old);
    await result.previewCurrentForm();
    await result.saveCurrentForm();
    await result.deletePendingConnection();
    expect(result.dialogOpen()).toBe(false);
    expect(result.deleteDialogOpen()).toBe(false);
    expect(result.form().apiKey).toBe('');
    for (const [method, mock] of Object.entries(TrueNASAPI))
      if (method !== 'listConnections') expect(mock).not.toHaveBeenCalled();
    await result.loadConnections();
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(1);
    setAllowed(true);
    await ready(result);
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
  });

  it('aborts a pending inventory read and rejects its late result after revocation', async () => {
    const pending = deferred<TrueNASConnection[]>();
    vi.mocked(TrueNASAPI.listConnections).mockReturnValueOnce(pending.promise);
    const [canLoad, setAllowed] = createSignal(true);
    const { result } = renderHook(() => useTrueNASSettingsPanelState({ canLoad }));
    const options = vi.mocked(TrueNASAPI.listConnections).mock.calls[0][0]!;
    expect(options).toMatchObject({ expectedOrgID: 'team-a', retry: false });
    setAllowed(false);
    expect(options.signal?.aborted).toBe(true);
    pending.resolve([connection()]);
    await pending.promise;
    await Promise.resolve();
    expect(result.connections()).toEqual([]);
    expect(result.loading()).toBe(false);
    expect(result.loadingError()).toBeNull();
  });

  it('preserves a same-reader draft across copied status and duplicate org events', async () => {
    const [status, setStatus] = createSignal({ user: 'reader-a', allowed: true });
    const { result } = renderHook(() =>
      useTrueNASSettingsPanelState({
        canLoad: () => status().allowed,
        currentUser: () => status().user,
      }),
    );
    await ready(result);
    openDraft(result);
    setStatus({ user: 'reader-a', allowed: true });
    eventBus.emit('org_switched', 'team-a');
    expect(result.form().apiKey).toBe('synthetic-draft');
    expect(result.dialogOpen()).toBe(true);
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(1);
  });

  it('does not accept an A-to-B-to-A read or let its finally clear a later loading state', async () => {
    const a = deferred<TrueNASConnection[]>(),
      b = deferred<TrueNASConnection[]>(),
      next = deferred<TrueNASConnection[]>();
    vi.mocked(TrueNASAPI.listConnections)
      .mockReturnValueOnce(a.promise)
      .mockReturnValueOnce(b.promise)
      .mockReturnValueOnce(next.promise);
    const [currentUser, setUser] = createSignal('a');
    const { result } = renderHook(() => useTrueNASSettingsPanelState({ currentUser }));
    setUser('b');
    setUser('a');
    a.resolve([connection()]);
    b.reject(new Error('obsolete-private-detail'));
    await Promise.allSettled([a.promise, b.promise]);
    await Promise.resolve();
    expect(result.loading()).toBe(true);
    expect(result.connections()).toEqual([]);
    expect(result.loadingError()).toBeNull();
    expect(logger.error).not.toHaveBeenCalled();
    next.resolve([connection()]);
    await ready(result);
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(3);
  });

  const actions = [
    'test',
    'saved-test',
    'preview',
    'saved-preview',
    'create',
    'update',
    'delete',
  ] as const;
  for (const action of actions)
    for (const outcome of ['success', 'error'] as const) {
      it(`ignores ${action} ${outcome} after an org switch, including notifications and follow-up reads`, async () => {
        const pending = deferred<never>();
        const method = {
          test: 'testConnection',
          'saved-test': 'testSavedConnection',
          preview: 'previewConnection',
          'saved-preview': 'previewSavedConnection',
          create: 'createConnection',
          update: 'updateConnection',
          delete: 'deleteConnection',
        }[action] as Exclude<keyof typeof TrueNASAPI, 'prototype' | 'listConnections'>;
        vi.mocked(TrueNASAPI[method]).mockReturnValueOnce(pending.promise);
        const { result } = renderHook(() => useTrueNASSettingsPanelState());
        await ready(result);
        const old = result.connections()[0];
        if (['test', 'preview', 'create'].includes(action)) {
          result.openCreateDialog();
          result.updateForm({
            host: 'nas.example.invalid',
            username: 'monitor',
            apiKey: 'synthetic-draft',
          });
        } else if (action === 'delete') result.openDeleteDialog(old);
        else if (action !== 'saved-test') openDraft(result);
        const task =
          action === 'test'
            ? result.testCurrentForm()
            : action === 'saved-test'
              ? result.testSavedConnection(old)
              : ['preview', 'saved-preview'].includes(action)
                ? result.previewCurrentForm()
                : action === 'delete'
                  ? result.deletePendingConnection()
                  : result.saveCurrentForm();
        expect(TrueNASAPI[method]).toHaveBeenCalledTimes(1);
        const call = vi.mocked(TrueNASAPI[method]).mock.calls[0];
        const options = call[call.length - 1] as {
          signal: AbortSignal;
          expectedOrgID: string;
          retry: boolean;
        };
        expect(options).toMatchObject({ expectedOrgID: 'team-a', retry: false });
        scope.org = 'team-b';
        eventBus.emit('org_switched', 'team-b');
        await ready(result);
        expect(options.signal.aborted).toBe(true);
        expect(result.form().apiKey).toBe('');
        expect(result.dialogOpen()).toBe(false);
        openDraft(result); // The late completion must not close/clear this new draft.
        if (outcome === 'success') pending.resolve({ success: true } as never);
        else pending.reject(new Error('obsolete-private-detail'));
        await task;
        expect(result.form().apiKey).toBe('synthetic-draft');
        expect(result.dialogOpen()).toBe(true);
        expect(result.monitoredSystemPreview()).toBeNull();
        expect(result.monitoredSystemPreviewError()).toBeNull();
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(notificationStore.error).not.toHaveBeenCalled();
        expect(logger.error).not.toHaveBeenCalled();
        expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
        expect(
          result.saving() || result.testing() || result.previewing() || result.deleting(),
        ).toBe(false);
        await result.testSavedConnection(old);
        result.openDeleteDialog(old);
        expect(result.deleteDialogOpen()).toBe(false); // Same ID is not the newly admitted object.
      });
    }

  for (const outcome of ['success', 'error'] as const)
    it(`aborts on disposal and ignores late ${outcome}`, async () => {
      const pending = deferred<never>();
      vi.mocked(TrueNASAPI.testSavedConnection).mockReturnValueOnce(pending.promise);
      const { result, cleanup } = renderHook(() => useTrueNASSettingsPanelState());
      await ready(result);
      const task = result.testSavedConnection(result.connections()[0]);
      const options = vi.mocked(TrueNASAPI.testSavedConnection).mock.calls[0][2]!;
      cleanup();
      expect(options.signal?.aborted).toBe(true);
      if (outcome === 'success') pending.resolve({ success: true } as never);
      else pending.reject(new Error('late'));
      await task;
      eventBus.emit('org_switched', 'team-b');
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(1);
    });

  it('withdraws before a stale action when org storage changed without an event', async () => {
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    openDraft(result);
    scope.org = 'team-b';
    await result.saveCurrentForm();
    await ready(result);
    expect(TrueNASAPI.updateConnection).not.toHaveBeenCalled();
    expect(result.form().apiKey).toBe('');
    expect(vi.mocked(TrueNASAPI.listConnections).mock.calls[1][0]?.expectedOrgID).toBe('team-b');
  });

  it('ignores a preview and Test result for an edited draft, without leaving controls busy', async () => {
    const preview = deferred<never>(),
      test = deferred<never>();
    vi.mocked(TrueNASAPI.previewSavedConnection).mockReturnValueOnce(preview.promise);
    vi.mocked(TrueNASAPI.testSavedConnection).mockReturnValueOnce(test.promise);
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    openDraft(result);
    const a = result.previewCurrentForm();
    result.updateForm({ host: 'changed.example.invalid' });
    preview.resolve({ additional_count: 5 } as never);
    expect(await a).toBeNull();
    expect(result.monitoredSystemPreview()).toBeNull();
    expect(result.previewing()).toBe(false);
    const b = result.testCurrentForm();
    result.updateForm({ host: 'newer.example.invalid' });
    test.resolve({ success: true } as never);
    expect(await b).toBe(false);
    expect(result.testing()).toBe(false);
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('starts a single mutation and does not retarget it while busy', async () => {
    const pending = deferred<never>();
    vi.mocked(TrueNASAPI.updateConnection).mockReturnValueOnce(pending.promise);
    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await ready(result);
    openDraft(result);
    const task = result.saveCurrentForm();
    await result.saveCurrentForm();
    await result.previewCurrentForm();
    result.openCreateDialog();
    result.openDeleteDialog(result.connections()[0]);
    expect(result.dialogOpen()).toBe(true);
    expect(result.deleteDialogOpen()).toBe(false);
    expect(TrueNASAPI.updateConnection).toHaveBeenCalledTimes(1);
    expect(TrueNASAPI.previewSavedConnection).not.toHaveBeenCalled();
    pending.resolve(connection() as never);
    await task;
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection updated');
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
  });

  for (const status of [401, 403])
    it(`withdraws on HTTP ${status} and requires a fresh read, not a stale confirmation`, async () => {
      vi.mocked(TrueNASAPI.updateConnection).mockRejectedValueOnce({
        status,
        message: 'private-provider-detail',
      });
      const { result } = renderHook(() => useTrueNASSettingsPanelState());
      await ready(result);
      openDraft(result);
      const old = result.connections()[0];
      await result.saveCurrentForm();
      expect(result.connections()).toEqual([]);
      expect(result.form().apiKey).toBe('');
      expect(result.loadingError()).toContain('access could not be verified');
      expect(notificationStore.error).not.toHaveBeenCalledWith('private-provider-detail');
      await result.testSavedConnection(old);
      result.openCreateDialog();
      expect(result.dialogOpen()).toBe(false);
      expect(TrueNASAPI.testSavedConnection).not.toHaveBeenCalled();
      await result.loadConnections();
      await ready(result);
      result.openCreateDialog();
      expect(result.dialogOpen()).toBe(true);
      expect(result.loadingError()).toBeNull();
    });
});
