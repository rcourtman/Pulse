import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON, getApiToken, setApiToken } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import { useBackupTransferFlow } from '../useBackupTransferFlow';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
  getApiToken: vi.fn(() => null),
  setApiToken: vi.fn(),
}));
vi.mock('@/stores/notifications', () => ({
  notificationStore: { success: vi.fn(), error: vi.fn() },
}));

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((yes) => {
    resolve = yes;
  });
  return { promise, resolve };
};
const archive = (text: () => Promise<string>) =>
  Object.assign(new File([], 'synthetic-backup.json'), { text });
const mount = (securityStatus: SecurityStatus | null = null) => {
  let dispose!: () => void;
  const state = createRoot((stop) => {
    dispose = stop;
    return useBackupTransferFlow({ securityStatus: () => securityStatus });
  });
  state.setShowImportDialog(true);
  state.setImportPassphrase('original-passphrase');
  return { state, dispose };
};

describe('configuration transfer submission lifetime', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.useFakeTimers();
    vi.mocked(apiFetchJSON).mockResolvedValue({ status: 'success' });
  });
  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it('owns a single import while the selected file is being read', async () => {
    const read = deferred<string>();
    const { state, dispose } = mount();
    const text = vi.fn(() => read.promise);
    state.setImportFile(archive(text));
    try {
      const first = state.handleImport();
      const second = state.handleImport();
      read.resolve('{"data":"synthetic-encrypted-archive"}');
      await Promise.all([first, second]);
      expect(text).toHaveBeenCalledOnce();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
    } finally {
      dispose();
    }
  });

  it('does not combine a submitted file with a later passphrase', async () => {
    const read = deferred<string>();
    const { state, dispose } = mount();
    state.setImportFile(archive(() => read.promise));
    try {
      const pending = state.handleImport();
      state.setImportPassphrase('later-passphrase');
      read.resolve('{"data":"synthetic-encrypted-archive"}');
      await pending;
      const options = vi.mocked(apiFetchJSON).mock.calls[0][1]!;
      expect(JSON.parse(String(options.body))).toEqual({
        passphrase: 'original-passphrase',
        data: 'synthetic-encrypted-archive',
      });
    } finally {
      dispose();
    }
  });

  it('does not send a settings replacement after the view leaves during file reading', async () => {
    const read = deferred<string>();
    const { state, dispose } = mount();
    state.setImportFile(archive(() => read.promise));
    const pending = state.handleImport();
    dispose();
    read.resolve('{"data":"synthetic-encrypted-archive"}');
    await pending;
    expect(apiFetchJSON).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('does not download a late export after the view leaves', async () => {
    const response = deferred<unknown>();
    vi.mocked(apiFetchJSON).mockReturnValueOnce(response.promise);
    const createURL = vi.fn(() => 'blob:synthetic-backup');
    const revokeURL = vi.fn();
    vi.stubGlobal(
      'URL',
      Object.assign(URL, { createObjectURL: createURL, revokeObjectURL: revokeURL }),
    );
    const click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
    const { state, dispose } = mount();
    state.closeImportDialog();
    state.setShowExportDialog(true);
    state.setExportPassphrase('original-passphrase');
    const pending = state.handleExport();
    dispose();
    response.resolve({ status: 'success', data: 'synthetic-encrypted-archive' });
    await pending;
    expect(createURL).not.toHaveBeenCalled();
    expect(click).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it.each(['UI', 'legacy', 'CLI', 'JSON string'] as const)(
    'keeps the %s archive format',
    async (format) => {
      const content = {
        UI: '{"status":"success","data":"synthetic-archive"}',
        legacy: '{"data":"synthetic-archive"}',
        CLI: '  synthetic-archive\n',
        'JSON string': '"synthetic-archive"',
      }[format];
      const { state, dispose } = mount();
      state.setImportFile(archive(async () => content));
      try {
        await state.handleImport();
        expect(apiFetchJSON).toHaveBeenCalledOnce();
        const options = vi.mocked(apiFetchJSON).mock.calls[0][1]!;
        expect(options.retry).toBe(false);
        expect(options.signal).toBeInstanceOf(AbortSignal);
        expect(JSON.parse(String(options.body))).toEqual({
          passphrase: 'original-passphrase',
          data: 'synthetic-archive',
        });
        expect(state.importPassphrase()).toBe('');
        expect(state.importFile()).toBeNull();
        expect(state.transferStatus()?.phase).toBe('reloading');
        expect(vi.getTimerCount()).toBe(1);
      } finally {
        dispose();
      }
      expect(vi.getTimerCount()).toBe(0);
    },
  );

  it.each(['null', '[]', '{}', '{"data":42}', '{"data":""}', '""', '  '])(
    'rejects unusable archive %s without a request',
    async (content) => {
      const { state, dispose } = mount();
      state.setImportFile(archive(async () => content));
      try {
        await state.handleImport();
        expect(apiFetchJSON).not.toHaveBeenCalled();
        expect(state.transferError()).toContain('Invalid backup file');
        expect(state.transferStatus()).toBeNull();
      } finally {
        dispose();
      }
    },
  );

  it('keeps the dialog and its reviewed inputs while a request owns it', async () => {
    const response = deferred<unknown>();
    vi.mocked(apiFetchJSON).mockReturnValue(response.promise);
    const { state, dispose } = mount();
    const file = archive(async () => '{"data":"synthetic-archive"}');
    state.setImportFile(file);
    try {
      const pending = state.handleImport();
      await Promise.resolve();
      state.closeImportDialog();
      state.setShowImportDialog(false);
      state.setImportFile(null);
      state.setShowExportDialog(true);
      state.setExportPassphrase('another-passphrase');
      await state.handleExport();
      await state.handleImport();
      expect(state.showImportDialog()).toBe(true);
      expect(state.showExportDialog()).toBe(false);
      expect(state.importFile()).toBe(file);
      expect(state.importPassphrase()).toBe('original-passphrase');
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      response.resolve({ status: 'success' });
      await pending;
    } finally {
      dispose();
    }
  });

  it('aborts a retired request and suppresses late feedback or reload scheduling', async () => {
    const response = deferred<unknown>();
    vi.mocked(apiFetchJSON).mockReturnValueOnce(response.promise);
    const { state, dispose } = mount();
    state.setImportFile(archive(async () => '{"data":"synthetic-archive"}'));
    const pending = state.handleImport();
    await Promise.resolve();
    const signal = vi.mocked(apiFetchJSON).mock.calls[0][1]!.signal!;
    expect(signal.aborted).toBe(false);
    dispose();
    expect(signal.aborted).toBe(true);
    response.resolve({ status: 'success' });
    await pending;
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(notificationStore.error).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
    await state.handleImport();
    expect(apiFetchJSON).toHaveBeenCalledOnce();
  });

  it('does not turn a private error into a token prompt or automatic replay', async () => {
    vi.mocked(apiFetchJSON).mockRejectedValueOnce(
      new Error('API_TOKEN private-path synthetic-secret'),
    );
    const { state, dispose } = mount();
    state.setImportFile(archive(async () => '{"data":"synthetic-archive"}'));
    try {
      await state.handleImport();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      expect(state.showApiTokenModal()).toBe(false);
      expect(state.transferError()).toContain('Settings may already have changed');
      expect(state.transferError()).not.toMatch(/private-path|synthetic-secret|API_TOKEN/);
      expect(state.transferStatus()).toBeNull();
      expect(notificationStore.success).not.toHaveBeenCalled();
    } finally {
      dispose();
    }
  });

  const tokenSecurity = { hasAuthentication: false, apiTokenConfigured: true } as SecurityStatus;
  it('authenticates at most once for the exact reviewed submission', async () => {
    vi.mocked(getApiToken).mockReturnValue(null);
    const { state, dispose } = mount(tokenSecurity);
    const text = vi.fn(async () => '{"data":"synthetic-archive"}');
    state.setImportFile(archive(text));
    try {
      await state.handleImport();
      expect(state.showApiTokenModal()).toBe(true);
      expect(text).not.toHaveBeenCalled();
      state.setImportPassphrase('unreviewed-passphrase');
      state.setImportFile(archive(async () => '{"data":"other-archive"}'));
      state.setApiTokenInput('synthetic-ui-token');
      state.handleApiTokenAuthenticate();
      state.handleApiTokenAuthenticate();
      await Promise.resolve();
      await Promise.resolve();
      expect(setApiToken).toHaveBeenCalledExactlyOnceWith('synthetic-ui-token');
      expect(text).toHaveBeenCalledOnce();
      expect(state.apiTokenInput()).toBe('');
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      expect(JSON.parse(String(vi.mocked(apiFetchJSON).mock.calls[0][1]!.body))).toEqual({
        passphrase: 'original-passphrase',
        data: 'synthetic-archive',
      });
    } finally {
      dispose();
    }
  });

  it('cancels an authentication intent rather than resuming it in a later draft', async () => {
    const { state, dispose } = mount(tokenSecurity);
    const text = vi.fn(async () => '{"data":"synthetic-archive"}');
    state.setImportFile(archive(text));
    try {
      await state.handleImport();
      state.setApiTokenInput('synthetic-ui-token');
      state.closeApiTokenModal();
      state.handleApiTokenAuthenticate();
      expect(state.showApiTokenModal()).toBe(false);
      expect(state.transferStatus()).toBeNull();
      expect(state.apiTokenInput()).toBe('');
      expect(setApiToken).not.toHaveBeenCalled();
      expect(text).not.toHaveBeenCalled();
      state.closeImportDialog();
      expect(state.importFile()).toBeNull();
      expect(state.importPassphrase()).toBe('');
    } finally {
      dispose();
    }
  });
});
