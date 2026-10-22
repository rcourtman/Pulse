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

const mount = (kind: 'export' | 'import') => {
  let dispose!: () => void;
  const flow = createRoot((stop) => {
    dispose = stop;
    return useBackupTransferFlow({ securityStatus: () => null });
  });
  const file = Object.assign(new File([], 'synthetic-backup.json'), {
    text: async () => '{"data":"synthetic-encrypted-archive"}',
  });
  if (kind === 'export') {
    flow.setShowExportDialog(true);
    flow.setExportPassphrase('synthetic-passphrase');
  } else {
    flow.setShowImportDialog(true);
    flow.setImportPassphrase('synthetic-passphrase');
    flow.setImportFile(file);
  }
  return { flow, file, dispose };
};

const invalidAcknowledgements = [
  { name: 'without acknowledgement', response: {} },
  { name: 'null', response: null },
  { name: 'an array', response: [] },
  { name: 'a success string', response: 'success' },
  { name: 'a boolean', response: true },
  { name: 'a number', response: 1 },
  { name: 'missing status with data', response: { data: 'synthetic-archive' } },
  { name: 'an error status', response: { status: 'error', message: 'private-response-sentinel' } },
  { name: 'an uncompleted status', response: { status: 'pending' } },
  { name: 'wrong status case', response: { status: 'SUCCESS' } },
  { name: 'a non-string status', response: { status: true } },
  {
    name: 'a contradictory error',
    response: { status: 'success', data: 'synthetic-archive', error: 'private-response-sentinel' },
  },
  {
    name: 'a structured error',
    response: {
      status: 'success',
      data: 'synthetic-archive',
      error: { detail: 'private-response-sentinel' },
    },
  },
];

describe('configuration transfer completion contract', () => {
  let downloaded: BlobPart[][];
  let createURL: ReturnType<typeof vi.fn<typeof URL.createObjectURL>>;
  let revokeURL: ReturnType<typeof vi.fn<typeof URL.revokeObjectURL>>;
  let click: ReturnType<typeof vi.spyOn>;
  beforeEach(() => {
    vi.resetAllMocks();
    vi.useFakeTimers();
    downloaded = [];
    const NativeBlob = Blob;
    vi.stubGlobal(
      'Blob',
      class extends NativeBlob {
        constructor(parts: BlobPart[] = [], options?: BlobPropertyBag) {
          super(parts, options);
          downloaded.push(parts);
        }
      },
    );
    createURL = vi.fn<typeof URL.createObjectURL>(() => 'blob:synthetic-backup');
    revokeURL = vi.fn<typeof URL.revokeObjectURL>();
    class DownloadURL extends URL {
      static createObjectURL = createURL;
      static revokeObjectURL = revokeURL;
    }
    vi.stubGlobal('URL', DownloadURL);
    click = vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
  });
  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
    vi.restoreAllMocks();
    vi.unstubAllGlobals();
  });

  it.each(invalidAcknowledgements)('does not confirm an import $name', async ({ response }) => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce(response);
    const { flow, file, dispose } = mount('import');
    try {
      await flow.handleImport();
      expect(flow.transferError()).toContain('Settings may already have changed');
      expect(flow.transferError()).not.toContain('private-response-sentinel');
      expect(flow.transferStatus()).toBeNull();
      expect(flow.showImportDialog()).toBe(true);
      expect(flow.importFile()).toBe(file);
      expect(flow.importPassphrase()).toBe('synthetic-passphrase');
      expect(flow.showApiTokenModal()).toBe(false);
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      expect(vi.mocked(apiFetchJSON).mock.calls[0][1]?.retry).toBe(false);
      expect(vi.getTimerCount()).toBe(0);
      await vi.advanceTimersByTimeAsync(5000);
      expect(apiFetchJSON).toHaveBeenCalledOnce();
    } finally {
      dispose();
    }
  });

  it.each([
    ...invalidAcknowledgements,
    { name: 'without encrypted data', response: { status: 'success' } },
    { name: 'with null data', response: { status: 'success', data: null } },
    {
      name: 'with structured data',
      response: { status: 'success', data: { secret: 'private-response-sentinel' } },
    },
    { name: 'with empty data', response: { status: 'success', data: '' } },
    { name: 'with blank data', response: { status: 'success', data: ' \n\t' } },
  ])('does not download an export $name', async ({ response }) => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce(response);
    const { flow, dispose } = mount('export');
    try {
      await flow.handleExport();
      expect(flow.transferError()).toContain('Could not export');
      expect(flow.transferError()).not.toContain('private-response-sentinel');
      expect(flow.transferStatus()).toBeNull();
      expect(flow.showExportDialog()).toBe(true);
      expect(flow.exportPassphrase()).toBe('synthetic-passphrase');
      expect(downloaded).toEqual([]);
      expect(createURL).not.toHaveBeenCalled();
      expect(click).not.toHaveBeenCalled();
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
    } finally {
      dispose();
    }
  });

  it('accepts the actual import acknowledgement and cancels its owned reload on disposal', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      status: 'success',
      message: 'Configuration imported successfully',
      remoteAgentEndpoints: { reconfigured: false, actionRequiredIfServerAddressChanged: true },
    });
    const { flow, dispose } = mount('import');
    try {
      await flow.handleImport();
      expect(flow.transferError()).toBeNull();
      expect(flow.importFile()).toBeNull();
      expect(flow.importPassphrase()).toBe('');
      expect(flow.transferStatus()?.phase).toBe('reloading');
      expect(notificationStore.success).toHaveBeenCalledExactlyOnceWith(
        'Configuration imported successfully. Reloading...',
      );
      expect(vi.getTimerCount()).toBe(1);
    } finally {
      dispose();
    }
    expect(vi.getTimerCount()).toBe(0);
  });

  it('downloads only the acknowledged archive, not unrelated response fields', async () => {
    vi.mocked(apiFetchJSON).mockResolvedValueOnce({
      status: 'success',
      data: 'synthetic-encrypted-archive',
      privateExtra: 'private-response-sentinel',
    });
    const { flow, dispose } = mount('export');
    try {
      await flow.handleExport();
      expect(flow.transferError()).toBeNull();
      expect(JSON.parse(String(downloaded[0][0]))).toEqual({
        status: 'success',
        data: 'synthetic-encrypted-archive',
      });
      expect(createURL).toHaveBeenCalledOnce();
      expect(click).toHaveBeenCalledOnce();
      expect(revokeURL).toHaveBeenCalledExactlyOnceWith('blob:synthetic-backup');
      expect(flow.showExportDialog()).toBe(false);
      expect(flow.exportPassphrase()).toBe('');
      expect(flow.transferStatus()).toBeNull();
      expect(notificationStore.success).toHaveBeenCalledExactlyOnceWith(
        'Configuration exported successfully',
      );
    } finally {
      dispose();
    }
  });
});
