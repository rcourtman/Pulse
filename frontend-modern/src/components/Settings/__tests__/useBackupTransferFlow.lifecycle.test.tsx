import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetchJSON } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
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
  const promise = new Promise<T>((yes) => { resolve = yes; });
  return { promise, resolve };
};
const archive = (text: () => Promise<string>) =>
  Object.assign(new File([], 'synthetic-backup.json'), { text });
const mount = () => {
  let dispose!: () => void;
  const state = createRoot((stop) => {
    dispose = stop;
    return useBackupTransferFlow({ securityStatus: () => null });
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
  afterEach(() => { vi.clearAllTimers(); vi.useRealTimers(); vi.restoreAllMocks(); });

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
    } finally { dispose(); }
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
        passphrase: 'original-passphrase', data: 'synthetic-encrypted-archive',
      });
    } finally { dispose(); }
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
    vi.stubGlobal('URL', Object.assign(URL, { createObjectURL: createURL, revokeObjectURL: revokeURL }));
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
});
