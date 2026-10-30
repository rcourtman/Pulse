import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi, type MockInstance } from 'vitest';
import { apiClient, clearApiToken, getApiToken, getOrgID, setOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import {
  MAX_CONFIG_IMPORT_FILE_BYTES,
  MAX_CONFIG_IMPORT_REQUEST_BYTES,
  useBackupTransferFlow,
} from '../useBackupTransferFlow';

vi.mock('@/stores/notifications', () => ({
  notificationStore: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

const fetchMock = vi.fn();
const disposers: (() => void)[] = [];
const passphrase = 'synthetic-passphrase';
let timeoutSpy: MockInstance<typeof globalThis.setTimeout>;
const reloadTimers = () =>
  timeoutSpy.mock.calls.flatMap((args, index) =>
    args[1] === 2000 ? [timeoutSpy.mock.results[index].value] : [],
  );

function deferred<T>() {
  let resolve!: (value: T) => void;
  let reject!: (reason: Error) => void;
  const promise = new Promise<T>((yes, no) => {
    resolve = yes;
    reject = no;
  });
  return { promise, resolve, reject };
}

function setup(kind: 'export' | 'import' = 'import', tokenOnly = false) {
  let dispose!: () => void;
  const flow = createRoot((cleanup) => {
    dispose = cleanup;
    return useBackupTransferFlow({
      securityStatus: () =>
        ({ hasAuthentication: !tokenOnly, apiTokenConfigured: tokenOnly }) as SecurityStatus,
    });
  });
  disposers.push(dispose);
  if (kind === 'import') flow.setShowImportDialog(true);
  else flow.setShowExportDialog(true);
  flow.setImportPassphrase(passphrase);
  flow.setExportPassphrase(passphrase);
  flow.setExportPassphraseConfirmation(passphrase);
  const read = vi.fn().mockResolvedValue('{"data":"synthetic-archive"}');
  flow.setImportFile({ size: 28, text: read } as unknown as File);
  return { flow, read, dispose };
}

const response = (status = 200, body: unknown = { status: 'success', data: 'synthetic-archive' }) =>
  new Response(JSON.stringify(body), {
    status,
    headers: {
      'Content-Type': 'application/json',
      'X-CSRF-Token': 'replacement-csrf',
      'Retry-After': '1',
    },
  });

beforeEach(() => {
  vi.useFakeTimers();
  vi.clearAllMocks();
  timeoutSpy = vi.spyOn(globalThis, 'setTimeout');
  fetchMock.mockReset().mockResolvedValue(response());
  vi.stubGlobal('fetch', fetchMock);
  clearApiToken();
  setOrgID('tenant-a');
  document.cookie = 'pulse_csrf=synthetic-csrf; Path=/';
  vi.stubGlobal(
    'URL',
    class extends URL {
      static createObjectURL = vi.fn(() => 'blob:synthetic');
      static revokeObjectURL = vi.fn();
    },
  );
  vi.spyOn(HTMLAnchorElement.prototype, 'click').mockImplementation(() => {});
});

afterEach(() => {
  for (const dispose of disposers.splice(0)) dispose();
  vi.restoreAllMocks();
  vi.unstubAllGlobals();
  vi.useRealTimers();
  clearApiToken();
  setOrgID(null);
});

describe('configuration transfers keep their confirmed workspace and one dispatch', () => {
  it.each([
    ['', passphrase, 'Please enter a backup passphrase'],
    ['short', 'short', 'at least 12 characters'],
    [passphrase, '', 'Please confirm the backup passphrase'],
    [passphrase, `${passphrase}-typo`, 'Passphrases do not match'],
  ])('rejects an unconfirmed export before asking for a token', async (value, confirm, message) => {
    const { flow } = setup('export', true);
    flow.setExportPassphrase(value);
    flow.setExportPassphraseConfirmation(confirm);
    await flow.handleExport();
    expect(flow.transferError()).toContain(message);
    expect(flow.showApiTokenModal()).toBe(false);
    expect(flow.isTransferring()).toBe(false);
    expect(flow.showExportDialog()).toBe(true);
    expect(flow.exportPassphrase()).toBe(value);
    expect(flow.exportPassphraseConfirmation()).toBe(confirm);
    expect(fetchMock).not.toHaveBeenCalled();
    expect(URL.createObjectURL).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('requires exact confirmation, without trimming or changing the encryption passphrase', async () => {
    const { flow } = setup('export');
    const value = ` ${passphrase} `;
    flow.setExportPassphrase(value);
    await flow.handleExport();
    expect(flow.transferError()).toContain('Passphrases do not match');
    expect(fetchMock).not.toHaveBeenCalled();
    flow.setExportPassphraseConfirmation(value);
    await flow.handleExport();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ passphrase: value });
    expect(flow.transferError()).toBe('');
    expect(flow.exportPassphraseConfirmation()).toBe('');
  });

  it('clears both export secrets on close and when switching to import', () => {
    const { flow } = setup('export');
    flow.closeExportDialog();
    expect(flow.exportPassphrase()).toBe('');
    expect(flow.exportPassphraseConfirmation()).toBe('');
    flow.setShowExportDialog(true);
    flow.setExportPassphrase(passphrase);
    flow.setExportPassphraseConfirmation(passphrase);
    flow.setShowImportDialog(true);
    expect(flow.exportPassphrase()).toBe('');
    expect(flow.exportPassphraseConfirmation()).toBe('');
    expect(flow.showExportDialog()).toBe(false);
    expect(flow.showImportDialog()).toBe(true);
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('freezes the confirmed export passphrase and never dispatches its confirmation', async () => {
    const { flow } = setup('export');
    const result = deferred<Response>();
    fetchMock.mockReturnValue(result.promise);
    const pending = flow.handleExport();
    flow.setExportPassphrase('edited-after-submit');
    flow.setExportPassphraseConfirmation('edited-confirmation');
    await flow.handleExport();
    flow.closeExportDialog();
    expect(flow.showExportDialog()).toBe(true);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({ passphrase });
    result.resolve(response());
    await pending;
    expect(flow.exportPassphrase()).toBe('');
    expect(flow.exportPassphraseConfirmation()).toBe('');
  });

  it('rejects an oversized file before reading it or asking for a token', async () => {
    const { flow, read } = setup('import', true);
    flow.setImportFile({ size: MAX_CONFIG_IMPORT_FILE_BYTES + 1, text: read } as unknown as File);
    await flow.handleImport();
    expect(read).not.toHaveBeenCalled();
    expect(fetchMock).not.toHaveBeenCalled();
    expect(flow.showApiTokenModal()).toBe(false);
    expect(flow.isTransferring()).toBe(false);
    expect(flow.transferError()).toContain('2 MiB browser import limit. No import was sent.');
    expect(flow.transferError()).not.toContain('may have applied');
    expect(flow.importFile()).not.toBeNull();
    expect(flow.importPassphrase()).toBe(passphrase);
    expect(reloadTimers()).toHaveLength(0);
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('accepts file formatting at the local read limit when the actual request fits', async () => {
    const { flow, read } = setup();
    const envelope = '{"data":"synthetic-archive"}';
    read.mockResolvedValue(envelope.padEnd(MAX_CONFIG_IMPORT_FILE_BYTES));
    flow.setImportFile({ size: MAX_CONFIG_IMPORT_FILE_BYTES, text: read } as unknown as File);
    await flow.handleImport();
    expect(read).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      passphrase,
      data: 'synthetic-archive',
    });
    expect(flow.transferError()).toBe('');
  });

  it.each([-1, 0, 1])('checks the complete request at the byte boundary (%i)', async (delta) => {
    const { flow, read } = setup();
    const overhead = new Blob([JSON.stringify({ passphrase, data: '' })]).size;
    const data = 'A'.repeat(MAX_CONFIG_IMPORT_REQUEST_BYTES - overhead + delta);
    read.mockResolvedValue(JSON.stringify({ data }));
    await flow.handleImport();
    expect(read).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(delta > 0 ? 0 : 1);
    if (delta > 0) {
      expect(flow.transferError()).toContain('server’s 1 MiB request limit');
      expect(flow.transferError()).toContain('No import was sent');
      expect(flow.transferError()).not.toContain('may have applied');
      expect(flow.showImportDialog()).toBe(true);
      expect(reloadTimers()).toHaveLength(0);
      expect(notificationStore.success).not.toHaveBeenCalled();
    } else {
      expect(new Blob([fetchMock.mock.calls[0][1].body]).size).toBe(
        MAX_CONFIG_IMPORT_REQUEST_BYTES + delta,
      );
      expect(flow.transferError()).toBe('');
    }
    expect(flow.isTransferring()).toBe(false);
  });

  it.each(['🔒'.repeat(8), '"\\\n'.repeat(8)])(
    'includes UTF-8 and JSON escaping in the password budget',
    async (password) => {
      const { flow, read } = setup();
      // The ASCII request would fit exactly, but this password costs more bytes.
      const overhead = new Blob([JSON.stringify({ passphrase, data: '' })]).size;
      const data = 'A'.repeat(MAX_CONFIG_IMPORT_REQUEST_BYTES - overhead);
      read.mockResolvedValue(JSON.stringify({ data }));
      flow.setImportPassphrase(password);
      await flow.handleImport();
      expect(fetchMock).not.toHaveBeenCalled();
      expect(flow.transferError()).toContain('including the password. No import was sent.');
      expect(flow.importPassphrase()).toBe(password);
      expect(flow.isTransferring()).toBe(false);
    },
  );

  it('allows a deliberate corrected submission after a local size rejection', async () => {
    const { flow, read } = setup();
    read.mockResolvedValue(JSON.stringify({ data: 'A'.repeat(MAX_CONFIG_IMPORT_REQUEST_BYTES) }));
    await flow.handleImport();
    expect(fetchMock).not.toHaveBeenCalled();
    read.mockResolvedValue('{"data":"synthetic-archive"}');
    await flow.handleImport();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(flow.transferError()).toBe('');
    expect(flow.showImportDialog()).toBe(false);
  });

  it.each(['export', 'import'] as const)(
    'does not send %s after the workspace changes before confirmation',
    async (kind) => {
      const { flow } = setup(kind);
      setOrgID('tenant-b');
      await (kind === 'import' ? flow.handleImport() : flow.handleExport());
      expect(fetchMock).not.toHaveBeenCalled();
      expect(flow.transferOrgID()).toBe('tenant-a');
      expect(flow.transferError()).toContain('Organisation changed');
      expect(notificationStore.error).not.toHaveBeenCalled();
    },
  );

  it('does not retarget a slow file read to the newly selected workspace', async () => {
    const { flow, read } = setup();
    const file = deferred<string>();
    read.mockReturnValue(file.promise);
    const pending = flow.handleImport();
    setOrgID('tenant-b');
    file.resolve('{"data":"synthetic-archive"}');
    await pending;
    expect(fetchMock).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('rechecks the captured target after asynchronous CSRF preparation', async () => {
    const { flow } = setup();
    const csrf = deferred<string>();
    vi.spyOn(apiClient as any, 'loadCSRFToken').mockReturnValue(null);
    const prepare = vi.spyOn(apiClient as any, 'ensureCSRFToken').mockReturnValue(csrf.promise);
    const pending = flow.handleImport();
    await vi.waitFor(() => expect(prepare).toHaveBeenCalled());
    setOrgID('tenant-b');
    csrf.resolve('synthetic-csrf');
    await pending;
    expect(fetchMock).not.toHaveBeenCalled();
    expect(flow.transferError()).toContain('Organisation changed');
  });

  it.each(['export', 'import'] as const)(
    'does not replay %s on invalid_org, CSRF or rate-limit responses',
    async (kind) => {
      for (const status of [400, 403, 429]) {
        fetchMock.mockReset().mockResolvedValue(response(status, { error: 'invalid_org' }));
        const { flow } = setup(kind);
        await (kind === 'import' ? flow.handleImport() : flow.handleExport());
        expect(fetchMock).toHaveBeenCalledTimes(1);
        expect(fetchMock.mock.calls[0][1].headers['X-Pulse-Org-ID']).toBe('tenant-a');
        expect(getOrgID()).toBe('tenant-a');
        expect(notificationStore.success).not.toHaveBeenCalled();
      }
    },
  );

  it('takes one immutable file/passphrase snapshot and blocks double submission', async () => {
    const { flow, read } = setup();
    const file = deferred<string>();
    read.mockReturnValue(file.promise);
    const pending = flow.handleImport();
    flow.setImportPassphrase('edited-after-submit');
    flow.setImportFile({ text: vi.fn() } as unknown as File);
    await flow.handleImport();
    flow.closeImportDialog();
    expect(flow.showImportDialog()).toBe(true);
    expect(flow.isTransferring()).toBe(true);
    file.resolve('{"data":"synthetic-archive"}');
    await pending;
    expect(read).toHaveBeenCalledTimes(1);
    expect(fetchMock).toHaveBeenCalledTimes(1);
    const [, options] = fetchMock.mock.calls[0];
    expect(JSON.parse(options.body)).toEqual({ passphrase, data: 'synthetic-archive' });
    expect(options.headers['X-Pulse-Org-ID']).toBe('tenant-a');
    expect(options.headers['X-CSRF-Token']).toBe('synthetic-csrf');
    expect(options.credentials).toBe('include');
    expect(flow.isTransferring()).toBe(false);
  });

  it.each(['export', 'import'] as const)(
    'does not replay %s when an API token is entered',
    async (kind) => {
      const { flow } = setup(kind, true);
      await (kind === 'import' ? flow.handleImport() : flow.handleExport());
      expect(flow.showApiTokenModal()).toBe(true);
      expect(fetchMock).not.toHaveBeenCalled();
      flow.setApiTokenInput('synthetic-token');
      flow.handleApiTokenAuthenticate();
      expect(getApiToken()).toBe('synthetic-token');
      expect(fetchMock).not.toHaveBeenCalled();
      await (kind === 'import' ? flow.handleImport() : flow.handleExport());
      expect(fetchMock).toHaveBeenCalledTimes(1);
      expect(fetchMock.mock.calls[0][1].headers['X-API-Token']).toBe('synthetic-token');
    },
  );

  it('keeps a token error visible and does not replay an uncertain import after token entry', async () => {
    const { flow } = setup();
    fetchMock.mockResolvedValue(response(403, { error: 'API token required' }));
    await flow.handleImport();
    expect(flow.showApiTokenModal()).toBe(true);
    expect(flow.transferError()).toContain('Check the destination before trying again');
    flow.setApiTokenInput('synthetic-token');
    flow.handleApiTokenAuthenticate();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(flow.transferError()).toContain('Import was not confirmed');
  });

  it('does not save a prompted token in a different workspace', async () => {
    const { flow } = setup('import', true);
    await flow.handleImport();
    flow.setApiTokenInput('synthetic-token');
    setOrgID('tenant-b');
    flow.handleApiTokenAuthenticate();
    expect(getApiToken()).toBeNull();
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it('does not download an export response into a different workspace', async () => {
    const { flow } = setup('export');
    const result = deferred<Response>();
    fetchMock.mockReturnValue(result.promise);
    const pending = flow.handleExport();
    setOrgID('tenant-b');
    result.resolve(response());
    await pending;
    expect(URL.createObjectURL).not.toHaveBeenCalled();
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(flow.exportPassphrase()).toBe(passphrase);
  });

  it('reports the original destination and never reloads a newly selected workspace after success', async () => {
    const { flow } = setup();
    const result = deferred<Response>();
    fetchMock.mockReturnValue(result.promise);
    const pending = flow.handleImport();
    await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
    setOrgID('tenant-b');
    result.resolve(response());
    await pending;
    expect(notificationStore.success).not.toHaveBeenCalled();
    expect(flow.transferError()).toBe(
      'Import completed for workspace tenant-a. The selected workspace changed. No reload was performed.',
    );
    expect(reloadTimers()).toHaveLength(0);
    expect(flow.importPassphrase()).toBe('');
    expect(flow.importFile()).toBeNull();
    expect(flow.showImportDialog()).toBe(true);
    expect(notificationStore.error).not.toHaveBeenCalled();
  });

  it.each(['export', 'import'] as const)(
    'aborts %s and ignores a late response after unmount',
    async (kind) => {
      const { flow, dispose } = setup(kind);
      const result = deferred<Response>();
      fetchMock.mockReturnValue(result.promise);
      const pending = kind === 'import' ? flow.handleImport() : flow.handleExport();
      await vi.waitFor(() => expect(fetchMock).toHaveBeenCalledTimes(1));
      dispose();
      expect(fetchMock.mock.calls[0][1].signal.aborted).toBe(true);
      result.resolve(response());
      await pending;
      expect(notificationStore.success).not.toHaveBeenCalled();
      expect(URL.createObjectURL).not.toHaveBeenCalled();
      expect(reloadTimers()).toHaveLength(0);
    },
  );

  it('does not dispatch a file read that finishes after unmount', async () => {
    const { flow, read, dispose } = setup();
    const file = deferred<string>();
    read.mockReturnValue(file.promise);
    const pending = flow.handleImport();
    dispose();
    file.resolve('{"data":"synthetic-archive"}');
    await pending;
    expect(fetchMock).not.toHaveBeenCalled();
  });

  it.each([
    '{"status":"success","data":"synthetic-archive"}',
    '{"data":"synthetic-archive"}',
    '"synthetic-archive"',
    'synthetic-archive\n',
  ])('preserves supported archive envelope %s and explicit default scope', async (contents) => {
    setOrgID(null);
    const { flow, read } = setup();
    read.mockResolvedValue(contents);
    await flow.handleImport();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(fetchMock.mock.calls[0][1].headers['X-Pulse-Org-ID']).toBe('default');
    expect(JSON.parse(fetchMock.mock.calls[0][1].body)).toEqual({
      passphrase,
      data: 'synthetic-archive',
    });
    expect(flow.showImportDialog()).toBe(false);
    expect(flow.importFile()).toBeNull();
    expect(flow.importPassphrase()).toBe('');
  });

  it.each(['null', '{}', '{"data":5}', '{"data":""}', '  '])(
    'rejects invalid archive %s without network access',
    async (contents) => {
      const { flow, read } = setup();
      read.mockResolvedValue(contents);
      await flow.handleImport();
      expect(fetchMock).not.toHaveBeenCalled();
      expect(flow.transferError()).toContain('Invalid backup file format');
      expect(flow.isTransferring()).toBe(false);
    },
  );

  it('retains an uncertain response without success, reloading or a second request', async () => {
    const { flow } = setup();
    fetchMock.mockRejectedValue(new TypeError('Failed to fetch'));
    await flow.handleImport();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(flow.transferError()).toContain('Import was not confirmed');
    expect(flow.showImportDialog()).toBe(true);
    expect(reloadTimers()).toHaveLength(0);
    expect(notificationStore.success).not.toHaveBeenCalled();
  });

  it('downloads the successful export once, releases the URL and clears the secret', async () => {
    const { flow } = setup('export');
    await flow.handleExport();
    expect(fetchMock).toHaveBeenCalledTimes(1);
    expect(URL.createObjectURL).toHaveBeenCalledTimes(1);
    expect(HTMLAnchorElement.prototype.click).toHaveBeenCalledTimes(1);
    expect(URL.revokeObjectURL).toHaveBeenCalledWith('blob:synthetic');
    expect(flow.exportPassphrase()).toBe('');
    expect(flow.exportPassphraseConfirmation()).toBe('');
    expect(flow.showExportDialog()).toBe(false);
  });

  it('captures the new target only after the operator closes and reopens the dialog', () => {
    const { flow } = setup();
    flow.closeImportDialog();
    setOrgID('tenant-b');
    flow.setShowImportDialog(true);
    expect(flow.transferOrgID()).toBe('tenant-b');
    expect(flow.importFile()).toBeNull();
    expect(flow.importPassphrase()).toBe('');
  });

  it.each(['leave', 'reopen'] as const)(
    'cancels the actual successful-import reload timer on %s',
    async (action) => {
      const { flow, dispose } = setup();
      await flow.handleImport();
      expect(reloadTimers()).toHaveLength(1);
      const cancel = vi.spyOn(globalThis, 'clearTimeout');
      if (action === 'leave') dispose();
      else flow.setShowExportDialog(true);
      expect(cancel).toHaveBeenCalledWith(reloadTimers()[0]);
    },
  );
});
