import { renderHook, waitFor } from '@solidjs/testing-library';
import { beforeEach, describe, expect, it, vi } from 'vitest';
import { TrueNASAPI, type TrueNASConnection } from '@/api/truenas';
import { notificationStore } from '@/stores/notifications';
import { logger } from '@/utils/logger';
import { useTrueNASSettingsPanelState } from '../useTrueNASSettingsPanelState';

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
  isRedactedTrueNASSecret: (value: string | null | undefined) =>
    (value || '').trim() === '********',
}));

vi.mock('@/stores/notifications', () => ({
  notificationStore: {
    success: vi.fn(),
    error: vi.fn(),
  },
}));

vi.mock('@/utils/logger', () => ({
  logger: {
    error: vi.fn(),
  },
}));

const makeConnection = (overrides: Partial<TrueNASConnection> = {}): TrueNASConnection => ({
  id: 'conn-1',
  name: 'tower',
  host: 'truenas.local',
  port: undefined,
  apiKey: undefined,
  username: undefined,
  password: undefined,
  useHttps: true,
  insecureSkipVerify: false,
  fingerprint: undefined,
  enabled: true,
  pollIntervalSeconds: 60,
  monitorDatasets: true,
  monitorPools: true,
  monitorReplication: true,
  ...overrides,
});

describe('useTrueNASSettingsPanelState — error-message + dialog branch coverage', () => {
  beforeEach(() => {
    vi.clearAllMocks();
  });

  // ---------------------------------------------------------------------------
  // getTrueNASErrorMessage — exercised through testCurrentForm / testSavedConnection
  // / previewCurrentForm. Priority: apiErrorDetailField(error, 'error') first,
  // then getErrorMessage(error, fallback).
  // ---------------------------------------------------------------------------

  it('prefers a trimmed details.error cause over the top-level Error.message when testing a connection', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.testConnection).mockRejectedValueOnce(
      Object.assign(new Error('Failed to connect to TrueNAS'), {
        details: { error: '  x509: certificate signed by unknown authority  ' },
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    const ok = await result.testCurrentForm();

    expect(ok).toBe(false);
    expect(notificationStore.error).toHaveBeenCalledWith(
      'x509: certificate signed by unknown authority',
    );
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(logger.error).toHaveBeenCalledWith(
      '[TrueNAS Settings] Connection test failed',
      expect.anything(),
    );
    expect(result.testing()).toBe(false);
  });

  it('prefers details.error even when a sibling message field is also present', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.testConnection).mockRejectedValueOnce({
      message: 'ignored top-level message',
      status: 502,
      details: { error: 'actionable cause from details' },
    });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    await result.testCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('actionable cause from details');
  });

  it('falls back to Error.message (untrimmed) when no details.error is present', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.testConnection).mockRejectedValueOnce(new Error('connection refused'));

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    await result.testCurrentForm();

    expect(notificationStore.error).toHaveBeenCalledWith('connection refused');
  });

  it('extracts and trims a plain-object message through testSavedConnection failure', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.testSavedConnection).mockRejectedValueOnce({ message: ' saved boom ' });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    await result.testSavedConnection(makeConnection({ id: 'conn-saved', name: 'tower' }));

    expect(notificationStore.error).toHaveBeenCalledWith('saved boom');
    expect(logger.error).toHaveBeenCalledWith(
      '[TrueNAS Settings] Saved connection test failed',
      expect.anything(),
    );
    // finally clause reloads the list even on failure
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
    expect(result.testing()).toBe(false);
  });

  const fallbackCases: ReadonlyArray<[string, unknown]> = [
    ['a whitespace-only Error message', new Error('   ')],
    ['an empty Error message', new Error('')],
    ['a plain object with an empty message', { message: '' }],
    ['a plain object with a whitespace message', { message: '   ' }],
    ['a plain object with a non-string message', { message: 42 }],
    ['a plain object without a message field', { status: 500, code: 'request_failed' }],
    ['a null error', null],
    ['an undefined error', undefined],
    ['a string primitive', 'network down'],
    ['a number primitive', 42],
  ];

  it.each(fallbackCases)(
    'delegates to the fallback for %s when no extractable message exists',
    async (_label, error) => {
      vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
      vi.mocked(TrueNASAPI.testConnection).mockRejectedValueOnce(error);

      const { result } = renderHook(() => useTrueNASSettingsPanelState());
      await waitFor(() => expect(result.loading()).toBe(false));

      result.openCreateDialog();
      result.updateForm({ host: 'tower.local', apiKey: 'secret' });
      const ok = await result.testCurrentForm();

      expect(ok).toBe(false);
      expect(notificationStore.error).toHaveBeenCalledWith('TrueNAS connection failed');
    },
  );

  it('routes a generic preview failure through getTrueNASErrorMessage without building an unavailable state', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    // No monitored_system_usage_unavailable code → unavailableState is null → else branch.
    vi.mocked(TrueNASAPI.previewConnection).mockRejectedValueOnce({
      status: 500,
      details: { error: 'preview-specific cause' },
    });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    const preview = await result.previewCurrentForm();

    expect(preview).toBeNull();
    expect(result.monitoredSystemPreview()).toBeNull();
    expect(result.monitoredSystemPreviewErrorTitle()).toBeNull();
    expect(result.monitoredSystemPreviewError()).toBe('preview-specific cause');
    expect(result.previewing()).toBe(false);
  });

  // ---------------------------------------------------------------------------
  // closeDialog — guarded by saving()/testing(); reset path clears all dialog state.
  // ---------------------------------------------------------------------------

  it('resets every piece of dialog state when closeDialog runs while idle', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([
      makeConnection({ id: 'conn-edit', name: 'tower' }),
    ] as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.connections()).toHaveLength(1));

    result.openEditDialog(result.connections()[0]);
    expect(result.dialogOpen()).toBe(true);
    expect(result.form().name).toBe('tower');

    result.closeDialog();

    expect(result.dialogOpen()).toBe(false);
    expect(result.editingConnection()).toBeNull();
    expect(result.form().name).toBe('');
    expect(result.form().host).toBe('');
    expect(result.form().pollIntervalSeconds).toBe('60');
    expect(result.form().authMode).toBe('apiKey');
    expect(result.monitoredSystemPreview()).toBeNull();
  });

  it('keeps the dialog open when closeDialog is invoked while a connection test is in flight', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    let resolveTest!: (value: { success: boolean }) => void;
    vi.mocked(TrueNASAPI.testConnection).mockReturnValueOnce(
      new Promise<{ success: boolean }>((resolve) => {
        resolveTest = resolve;
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    const testPromise = result.testCurrentForm();
    await waitFor(() => expect(result.testing()).toBe(true));

    result.closeDialog();
    expect(result.dialogOpen()).toBe(true);

    resolveTest({ success: true });
    await testPromise;
    expect(result.testing()).toBe(false);
    expect(notificationStore.success).toHaveBeenCalledWith('TrueNAS connection successful');
  });

  it('keeps the dialog open when closeDialog is invoked while a save is in flight', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    let resolveSave!: (value: TrueNASConnection) => void;
    vi.mocked(TrueNASAPI.createConnection).mockReturnValueOnce(
      new Promise<TrueNASConnection>((resolve) => {
        resolveSave = resolve;
      }),
    );

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openCreateDialog();
    result.updateForm({ host: 'tower.local', apiKey: 'secret' });
    const savePromise = result.saveCurrentForm();
    await waitFor(() => expect(result.saving()).toBe(true));

    result.closeDialog();
    expect(result.dialogOpen()).toBe(true);

    resolveSave(makeConnection({ id: 'conn-new' }));
    await savePromise;
    expect(result.saving()).toBe(false);
    // save success path resets the dialog itself
    expect(result.dialogOpen()).toBe(false);
  });

  // ---------------------------------------------------------------------------
  // openDeleteDialog / resetDeleteDialogState / closeDeleteDialog
  // ---------------------------------------------------------------------------

  it('opens the delete confirmation dialog with the pending connection and closes it while idle', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    const target = makeConnection({ id: 'conn-del', name: 'tower' });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openDeleteDialog(target);
    expect(result.deleteDialogOpen()).toBe(true);
    expect(result.pendingDeleteConnection()).toEqual(target);

    result.closeDeleteDialog();
    expect(result.deleteDialogOpen()).toBe(false);
    expect(result.pendingDeleteConnection()).toBeNull();
  });

  it('keeps the delete dialog open when closeDeleteDialog is invoked while a delete is in flight', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    let resolveDelete!: (value: { success: boolean; id: string }) => void;
    vi.mocked(TrueNASAPI.deleteConnection).mockReturnValueOnce(
      new Promise<{ success: boolean; id: string }>((resolve) => {
        resolveDelete = resolve;
      }),
    );
    const target = makeConnection({ id: 'conn-del', name: 'tower' });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openDeleteDialog(target);
    const deletePromise = result.deletePendingConnection();
    await waitFor(() => expect(result.deleting()).toBe(true));

    result.closeDeleteDialog();
    expect(result.deleteDialogOpen()).toBe(true);
    expect(result.pendingDeleteConnection()).toEqual(target);

    resolveDelete({ success: true, id: 'conn-del' });
    await deletePromise;
    expect(result.deleting()).toBe(false);
    expect(result.deleteDialogOpen()).toBe(false);
  });

  // ---------------------------------------------------------------------------
  // deletePendingConnection — success (name + host fallback), failure, null-pending no-op.
  // ---------------------------------------------------------------------------

  it('removes the pending connection, notifies by name, and reloads the list on delete success', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.deleteConnection).mockResolvedValueOnce({ success: true, id: 'conn-1' });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openDeleteDialog(makeConnection({ id: 'conn-1', name: 'tower', host: 'truenas.local' }));
    await result.deletePendingConnection();

    expect(TrueNASAPI.deleteConnection).toHaveBeenCalledWith('conn-1');
    expect(notificationStore.success).toHaveBeenCalledWith('Removed tower');
    expect(result.deleteDialogOpen()).toBe(false);
    expect(result.pendingDeleteConnection()).toBeNull();
    expect(result.deleting()).toBe(false);
    // mount load + post-delete reload
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(2);
  });

  it('uses the connection host in the success notice when the name is empty', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.deleteConnection).mockResolvedValueOnce({ success: true, id: 'conn-2' });

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    result.openDeleteDialog(makeConnection({ id: 'conn-2', name: '', host: 'box.local' }));
    await result.deletePendingConnection();

    expect(notificationStore.success).toHaveBeenCalledWith('Removed box.local');
  });

  it('leaves the delete dialog open, notifies, and skips reload on delete failure', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);
    vi.mocked(TrueNASAPI.deleteConnection).mockRejectedValueOnce(new Error('server is down'));

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    const target = makeConnection({ id: 'conn-3', name: 'tower' });
    result.openDeleteDialog(target);
    await result.deletePendingConnection();

    expect(notificationStore.error).toHaveBeenCalledWith('server is down');
    expect(logger.error).toHaveBeenCalledWith(
      '[TrueNAS Settings] Delete failed',
      expect.anything(),
    );
    // failure path does not reset the dialog or reload
    expect(result.deleteDialogOpen()).toBe(true);
    expect(result.pendingDeleteConnection()).toEqual(target);
    expect(TrueNASAPI.listConnections).toHaveBeenCalledTimes(1);
    expect(result.deleting()).toBe(false);
  });

  it('is a no-op when deletePendingConnection runs with no pending connection', async () => {
    vi.mocked(TrueNASAPI.listConnections).mockResolvedValue([] as never);

    const { result } = renderHook(() => useTrueNASSettingsPanelState());
    await waitFor(() => expect(result.loading()).toBe(false));

    await result.deletePendingConnection();

    expect(TrueNASAPI.deleteConnection).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(result.deleting()).toBe(false);
    expect(result.deleteDialogOpen()).toBe(false);
  });
});
