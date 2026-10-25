import { createRoot } from 'solid-js';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiClient, clearApiToken, getApiToken, getOrgID, setOrgID } from '@/utils/apiClient';
import { notificationStore } from '@/stores/notifications';
import type { SecurityStatus } from '@/types/config';
import { useBackupTransferFlow } from '../useBackupTransferFlow';

vi.mock('@/stores/notifications', () => ({
  notificationStore: { error: vi.fn(), success: vi.fn(), info: vi.fn() },
}));

const fetchMock = vi.fn();
const disposers: (() => void)[] = [];
const passphrase = 'synthetic-passphrase';

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
  const read = vi.fn().mockResolvedValue('{"data":"synthetic-archive"}');
  flow.setImportFile({ text: read } as unknown as File);
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
  it.each(['export', 'import'] as const)(
    'does not send %s after the workspace changes before confirmation',
    async (kind) => {
      const { flow } = setup(kind);
      setOrgID('tenant-b');
      await (kind === 'import' ? flow.handleImport() : flow.handleExport());
      expect(fetchMock).not.toHaveBeenCalled();
      expect(flow.transferOrgID()).toBe('tenant-a');
      expect(notificationStore.error).toHaveBeenCalledWith(
        expect.stringContaining('Organisation changed'),
      );
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
    expect(flow.transferError()).toContain('Import completed for workspace tenant-a');
    expect(vi.getTimerCount()).toBe(0);
    expect(flow.importPassphrase()).toBe('');
    expect(flow.importFile()).toBeNull();
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
      expect(vi.getTimerCount()).toBe(0);
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
    expect(vi.getTimerCount()).toBe(0);
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

  it('cancels the successful-import reload timer when Settings is left or a new transfer is opened', async () => {
    const { flow, dispose } = setup();
    await flow.handleImport();
    expect(vi.getTimerCount()).toBe(1);
    dispose();
    expect(vi.getTimerCount()).toBe(0);
    const next = setup();
    await next.flow.handleImport();
    expect(vi.getTimerCount()).toBe(1);
    next.flow.setShowExportDialog(true);
    expect(vi.getTimerCount()).toBe(0);
  });
});
