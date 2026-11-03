import { renderHook, waitFor } from '@solidjs/testing-library';
import { createSignal } from 'solid-js';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { VMwareAPI, type VMwareConnection } from '@/api/vmware';
import { eventBus } from '@/stores/events';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { useVMwareSettingsPanelState } from '../useVMwareSettingsPanelState';

const scope = vi.hoisted(() => ({ org: 'team-a' }));
vi.mock('@/utils/apiClient', () => ({ getOrgID: () => scope.org }));
vi.mock('@/api/vmware', () => ({
  VMwareAPI: {
    listConnections: vi.fn(),
    createConnection: vi.fn(),
    updateConnection: vi.fn(),
    deleteConnection: vi.fn(),
    previewConnection: vi.fn(),
    previewSavedConnection: vi.fn(),
    testConnection: vi.fn(),
    testSavedConnection: vi.fn(),
  },
  isRedactedVMwareSecret: (value?: string) => value?.trim() === '********',
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn(), warning: vi.fn() },
}));
vi.mock('@/utils/logger', () => ({ logger: { error: vi.fn() } }));

const connection = (): VMwareConnection => ({
  id: 'same-id',
  name: 'Synthetic vCenter',
  host: 'vc.example.invalid',
  username: 'monitor',
  password: '********',
  insecureSkipVerify: false,
  enabled: true,
  monitorVms: true,
  monitorHosts: true,
  monitorDatastores: true,
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
const ready = async (state: ReturnType<typeof useVMwareSettingsPanelState>) =>
  waitFor(() => {
    expect(state.loading()).toBe(false);
    expect(state.connections()).toHaveLength(1);
  });
const openDraft = (state: ReturnType<typeof useVMwareSettingsPanelState>) => {
  state.openEditDialog(state.connections()[0]);
  state.updateForm({ password: 'synthetic-draft', name: 'Unconfirmed draft' });
};

describe('VMware reader lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    scope.org = 'team-a';
    vi.mocked(VMwareAPI.listConnections).mockImplementation(async () => [connection()]);
  });

  it('withdraws accepted inventory, draft secrets and confirmations when access is revoked', async () => {
    const [canLoad, setAllowed] = createSignal(true);
    const { result } = renderHook(() => useVMwareSettingsPanelState({ canLoad }));
    await ready(result);
    openDraft(result);
    const old = result.connections()[0];
    setAllowed(false);
    await waitFor(() => expect(result.connections()).toEqual([]));
    expect(result.form().password).toBe('');
    expect(result.dialogOpen()).toBe(false);
    expect(result.editingConnection()).toBeNull();
    expect(result.pendingDeleteConnection()).toBeNull();
    result.openEditDialog(old);
    result.openDeleteDialog(old);
    result.openCreateDialog();
    result.updateForm({ password: 'must-not-stick' });
    await result.testCurrentForm();
    await result.testSavedConnection(old);
    await result.previewCurrentForm();
    await result.saveCurrentForm();
    await result.deletePendingConnection();
    expect(result.dialogOpen()).toBe(false);
    expect(result.deleteDialogOpen()).toBe(false);
    expect(result.form().password).toBe('');
    for (const [method, mock] of Object.entries(VMwareAPI))
      if (method !== 'listConnections') expect(mock).not.toHaveBeenCalled();
    await result.loadConnections();
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(1);
    setAllowed(true);
    await ready(result);
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(2);
  });

  it('aborts a pending inventory read and rejects its late result after revocation', async () => {
    const pending = deferred<VMwareConnection[]>();
    vi.mocked(VMwareAPI.listConnections).mockReturnValueOnce(pending.promise);
    const [canLoad, setAllowed] = createSignal(true);
    const { result } = renderHook(() => useVMwareSettingsPanelState({ canLoad }));
    const options = vi.mocked(VMwareAPI.listConnections).mock.calls[0][0]!;
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
      useVMwareSettingsPanelState({
        canLoad: () => status().allowed,
        currentUser: () => status().user,
      }),
    );
    await ready(result);
    openDraft(result);
    setStatus({ user: 'reader-a', allowed: true });
    eventBus.emit('org_switched', 'team-a');
    expect(result.form().password).toBe('synthetic-draft');
    expect(result.dialogOpen()).toBe(true);
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(1);
  });

  it('does not accept an A-to-B-to-A read or let its finally clear a later loading state', async () => {
    const a = deferred<VMwareConnection[]>(),
      b = deferred<VMwareConnection[]>(),
      next = deferred<VMwareConnection[]>();
    vi.mocked(VMwareAPI.listConnections)
      .mockReturnValueOnce(a.promise)
      .mockReturnValueOnce(b.promise)
      .mockReturnValueOnce(next.promise);
    const [currentUser, setUser] = createSignal('a');
    const { result } = renderHook(() => useVMwareSettingsPanelState({ currentUser }));
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
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(3);
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
        }[action] as Exclude<keyof typeof VMwareAPI, 'prototype' | 'listConnections'>;
        vi.mocked(VMwareAPI[method]).mockReturnValueOnce(pending.promise);
        const { result } = renderHook(() => useVMwareSettingsPanelState());
        await ready(result);
        const old = result.connections()[0];
        if (['test', 'preview', 'create'].includes(action)) {
          result.openCreateDialog();
          result.updateForm({
            host: 'vc.example.invalid',
            username: 'monitor',
            password: 'synthetic-draft',
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
        expect(VMwareAPI[method]).toHaveBeenCalledTimes(1);
        const call = vi.mocked(VMwareAPI[method]).mock.calls[0];
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
        expect(result.form().password).toBe('');
        expect(result.dialogOpen()).toBe(false);
        openDraft(result); // The late completion must not close/clear this new draft.
        if (outcome === 'success') pending.resolve({ success: true } as never);
        else pending.reject(new Error('obsolete-private-detail'));
        await task;
        expect(result.form().password).toBe('synthetic-draft');
        expect(result.dialogOpen()).toBe(true);
        expect(result.monitoredSystemPreview()).toBeNull();
        expect(result.monitoredSystemPreviewError()).toBeNull();
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(notificationStore.error).not.toHaveBeenCalled();
        expect(logger.error).not.toHaveBeenCalled();
        expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(2);
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
      vi.mocked(VMwareAPI.testSavedConnection).mockReturnValueOnce(pending.promise);
      const { result, cleanup } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      const task = result.testSavedConnection(result.connections()[0]);
      const options = vi.mocked(VMwareAPI.testSavedConnection).mock.calls[0][2]!;
      cleanup();
      expect(options.signal?.aborted).toBe(true);
      if (outcome === 'success') pending.resolve({ success: true } as never);
      else pending.reject(new Error('late'));
      await task;
      eventBus.emit('org_switched', 'team-b');
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(1);
    });

  it('withdraws before a stale action when org storage changed without an event', async () => {
    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await ready(result);
    openDraft(result);
    scope.org = 'team-b';
    await result.saveCurrentForm();
    await ready(result);
    expect(VMwareAPI.updateConnection).not.toHaveBeenCalled();
    expect(result.form().password).toBe('');
    expect(vi.mocked(VMwareAPI.listConnections).mock.calls[1][0]?.expectedOrgID).toBe('team-b');
  });

  it('ignores a preview and Test result for an edited draft, without leaving controls busy', async () => {
    const preview = deferred<never>(),
      test = deferred<never>();
    vi.mocked(VMwareAPI.previewSavedConnection).mockReturnValueOnce(preview.promise);
    vi.mocked(VMwareAPI.testSavedConnection).mockReturnValueOnce(test.promise);
    const { result } = renderHook(() => useVMwareSettingsPanelState());
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

  for (const action of ['Test', 'Preview'] as const)
    for (const status of [401, 403])
      for (const saved of [true, false])
        it(`withdraws edited ${saved ? 'saved' : 'new'} draft on ${action} HTTP ${status}, then recovers only by fresh read`, async () => {
          const method =
            action === 'Test'
              ? saved
                ? 'testSavedConnection'
                : 'testConnection'
              : saved
                ? 'previewSavedConnection'
                : 'previewConnection';
          const pending = deferred<never>();
          vi.mocked(VMwareAPI[method]).mockReturnValueOnce(pending.promise);
          const { result } = renderHook(() => useVMwareSettingsPanelState());
          await ready(result);
          const old = result.connections()[0];
          if (saved) openDraft(result);
          else {
            result.openCreateDialog();
            result.updateForm({
              host: 'vc.example.invalid',
              username: 'monitor',
              password: 'synthetic-draft',
            });
          }
          const task = action === 'Test' ? result.testCurrentForm() : result.previewCurrentForm();
          expect(VMwareAPI[method]).toHaveBeenCalledTimes(1);
          result.updateForm({ name: 'Name edited while request is pending' });
          expect(result.form().name).toBe('Name edited while request is pending');
          pending.reject({ status, message: 'private-provider-detail' });
          await task;
          expect(result.connections()).toEqual([]);
          expect(result.dialogOpen()).toBe(false);
          expect(result.form().password).toBe('');
          expect(result.form().username).toBe('');
          expect(result.monitoredSystemPreview()).toBeNull();
          expect(result.monitoredSystemPreviewError()).toBeNull();
          expect(result.loadingError()).toContain('access could not be verified');
          expect(result.testing() || result.previewing()).toBe(false);
          expect(notificationStore.success).not.toHaveBeenCalled();
          expect(notificationStore.error).toHaveBeenCalledWith(result.loadingError());
          expect(notificationStore.error).not.toHaveBeenCalledWith('private-provider-detail');
          expect(logger.error).not.toHaveBeenCalled();
          result.openEditDialog(old);
          result.openCreateDialog();
          expect(result.dialogOpen()).toBe(false);
          expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(1);
          expect(VMwareAPI[method]).toHaveBeenCalledTimes(1); // No replay on denial.
          await result.loadConnections();
          await ready(result);
          result.openEditDialog(result.connections()[0]);
          expect(result.form().password).toBe('');
          expect(result.loadingError()).toBeNull();
          vi.mocked(VMwareAPI.testSavedConnection).mockResolvedValueOnce({
            success: true,
          } as never);
          expect(await result.testCurrentForm()).toBe(true);
          expect(notificationStore.success).toHaveBeenCalledWith('VMware connection successful');
        });

  for (const action of ['Test', 'Preview'] as const)
    for (const status of [401, 403])
      it(`ignores old-reader ${action} HTTP ${status} after an A-to-B-to-A replacement`, async () => {
        const method = action === 'Test' ? 'testSavedConnection' : 'previewSavedConnection';
        const pending = deferred<never>();
        vi.mocked(VMwareAPI[method]).mockReturnValueOnce(pending.promise);
        const [currentUser, setUser] = createSignal('a');
        const { result } = renderHook(() => useVMwareSettingsPanelState({ currentUser }));
        await ready(result);
        openDraft(result);
        const task = action === 'Test' ? result.testCurrentForm() : result.previewCurrentForm();
        const options = vi.mocked(VMwareAPI[method]).mock.calls[0][2]!;
        result.updateForm({ name: 'Obsolete edited draft' });
        setUser('b');
        setUser('a');
        await ready(result);
        openDraft(result);
        pending.reject({ status, message: 'obsolete-private-detail' });
        await task;
        expect(options.signal?.aborted).toBe(true);
        expect(result.connections()).toHaveLength(1);
        expect(result.dialogOpen()).toBe(true);
        expect(result.form().password).toBe('synthetic-draft');
        expect(result.loadingError()).toBeNull();
        expect(result.testing() || result.previewing()).toBe(false);
        expect(notificationStore.error).not.toHaveBeenCalled();
        expect(notificationStore.success).not.toHaveBeenCalled();
        expect(logger.error).not.toHaveBeenCalled();
        expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(3);
      });

  for (const action of ['Test', 'Preview'] as const)
    it(`ignores a stale-draft ${action} ordinary error without withdrawing the current reader`, async () => {
      const method = action === 'Test' ? 'testSavedConnection' : 'previewSavedConnection';
      const pending = deferred<never>();
      vi.mocked(VMwareAPI[method]).mockReturnValueOnce(pending.promise);
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      openDraft(result);
      const task = action === 'Test' ? result.testCurrentForm() : result.previewCurrentForm();
      result.updateForm({ name: 'Keep this current draft' });
      pending.reject({ status: 500, message: 'obsolete-provider-detail' });
      await task;
      expect(result.connections()).toHaveLength(1);
      expect(result.dialogOpen()).toBe(true);
      expect(result.form().name).toBe('Keep this current draft');
      expect(result.form().password).toBe('synthetic-draft');
      expect(result.loadingError()).toBeNull();
      expect(result.monitoredSystemPreviewError()).toBeNull();
      expect(result.testing() || result.previewing()).toBe(false);
      expect(notificationStore.error).not.toHaveBeenCalled();
      expect(logger.error).not.toHaveBeenCalled();
    });

  it('starts a single mutation and does not retarget it while busy', async () => {
    const pending = deferred<never>();
    vi.mocked(VMwareAPI.updateConnection).mockReturnValueOnce(pending.promise);
    const { result } = renderHook(() => useVMwareSettingsPanelState());
    await ready(result);
    openDraft(result);
    const task = result.saveCurrentForm();
    await result.saveCurrentForm();
    await result.previewCurrentForm();
    result.openCreateDialog();
    result.openDeleteDialog(result.connections()[0]);
    expect(result.dialogOpen()).toBe(true);
    expect(result.deleteDialogOpen()).toBe(false);
    expect(VMwareAPI.updateConnection).toHaveBeenCalledTimes(1);
    expect(VMwareAPI.previewSavedConnection).not.toHaveBeenCalled();
    pending.resolve(connection() as never);
    await task;
    expect(notificationStore.success).toHaveBeenCalledWith('VMware connection updated');
    expect(VMwareAPI.listConnections).toHaveBeenCalledTimes(2);
  });

  for (const status of [401, 403])
    it(`withdraws on HTTP ${status} and requires a fresh read, not a stale confirmation`, async () => {
      vi.mocked(VMwareAPI.updateConnection).mockRejectedValueOnce({
        status,
        message: 'private-provider-detail',
      });
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      openDraft(result);
      const old = result.connections()[0];
      await result.saveCurrentForm();
      expect(result.connections()).toEqual([]);
      expect(result.form().password).toBe('');
      expect(result.loadingError()).toContain('access could not be verified');
      expect(notificationStore.error).not.toHaveBeenCalledWith('private-provider-detail');
      await result.testSavedConnection(old);
      result.openCreateDialog();
      expect(result.dialogOpen()).toBe(false);
      expect(VMwareAPI.testSavedConnection).not.toHaveBeenCalled();
      await result.loadConnections();
      await ready(result);
      result.openCreateDialog();
      expect(result.dialogOpen()).toBe(true);
      expect(result.loadingError()).toBeNull();
    });
  it.each([500, 404, 'empty'] as const)(
    'withdraws a draft on an unread or removed saved source (%s) without writing settings',
    async (outcome) => {
      const { result } = renderHook(() => useVMwareSettingsPanelState());
      await ready(result);
      openDraft(result);
      if (outcome === 'empty') vi.mocked(VMwareAPI.listConnections).mockResolvedValueOnce([]);
      else
        vi.mocked(VMwareAPI.listConnections).mockRejectedValueOnce({
          status: outcome,
          message: 'read failed',
        });
      await result.loadConnections();
      expect(result.connections()).toEqual([]);
      expect(result.dialogOpen()).toBe(false);
      expect(result.form().password).toBe('');
      expect(result.form().hasStoredPassword).toBe(false);
      expect(result.featureDisabled()).toBe(outcome === 404);
      expect(result.loadingError() !== null).toBe(outcome === 500);
      expect(VMwareAPI.updateConnection).not.toHaveBeenCalled();
      expect(VMwareAPI.deleteConnection).not.toHaveBeenCalled();
      await result.loadConnections();
      await ready(result);
      result.openEditDialog(result.connections()[0]);
      expect(result.form()).toMatchObject({
        password: '',
        hasStoredPassword: true,
        insecureSkipVerify: false,
      });
    },
  );
});
