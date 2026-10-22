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

// Matches handleImportConfig's existing http.MaxBytesReader budget.
const limit = 1024 * 1024;
const passphrase = 'synthetic-private-passphrase';
const bytes = (value: string) => new TextEncoder().encode(value).byteLength;
const body = (data: string, password = passphrase) =>
  JSON.stringify({ passphrase: password, data });
const archive = (content: string) =>
  Object.assign(new File([content], 'private-backup-name.json'), {
    text: vi.fn(async () => content),
  });
const mount = (content: string, password = passphrase, needsToken = false) => {
  let dispose!: () => void;
  const flow = createRoot((stop) => {
    dispose = stop;
    return useBackupTransferFlow({
      securityStatus: () =>
        needsToken
          ? ({ hasAuthentication: false, apiTokenConfigured: true } as SecurityStatus)
          : null,
    });
  });
  const file = archive(content);
  flow.setShowImportDialog(true);
  flow.setImportFile(file);
  flow.setImportPassphrase(password);
  return { flow, file, dispose };
};

const expectNotSent = (
  flow: ReturnType<typeof useBackupTransferFlow>,
  file: File,
  password = passphrase,
) => {
  expect(apiFetchJSON).not.toHaveBeenCalled();
  expect(notificationStore.success).not.toHaveBeenCalled();
  expect(flow.transferStatus()).toBeNull();
  expect(flow.showImportDialog()).toBe(true);
  expect(flow.importFile()).toBe(file);
  expect(flow.importPassphrase()).toBe(password);
  expect(flow.transferError()).toContain('No import request was sent');
  expect(flow.transferError()).not.toContain('may already have changed');
  expect(flow.transferError()).not.toContain('private-backup-name');
  expect(flow.transferError()).not.toContain(password);
  expect(vi.getTimerCount()).toBe(0);
};

describe('configuration import byte budgets', () => {
  beforeEach(() => {
    vi.resetAllMocks();
    vi.useFakeTimers();
    vi.mocked(getApiToken).mockReturnValue(null);
    vi.mocked(apiFetchJSON).mockResolvedValue({ status: 'success' });
  });
  afterEach(() => {
    vi.clearAllTimers();
    vi.useRealTimers();
  });

  it('rejects an oversized file before reading or asking for a token', async () => {
    const { flow, file, dispose } = mount('A'.repeat(limit + 1), passphrase, true);
    try {
      await flow.handleImport();
      expect(file.text).not.toHaveBeenCalled();
      expect(flow.showApiTokenModal()).toBe(false);
      expect(setApiToken).not.toHaveBeenCalled();
      expectNotSent(flow, file);
      expect(flow.transferError()).toContain('file exceeds');
    } finally {
      dispose();
    }
  });

  it('bounds file reading even when an oversized file contains a small usable archive', async () => {
    const content = JSON.stringify({ data: 'small-archive', extra: 'A'.repeat(limit) });
    const { flow, file, dispose } = mount(content);
    try {
      await flow.handleImport();
      expect(file.text).not.toHaveBeenCalled();
      expectNotSent(flow, file);
    } finally {
      dispose();
    }
  });

  it('reads a file exactly at the file budget, then rejects its oversized request', async () => {
    const { flow, file, dispose } = mount('A'.repeat(limit));
    try {
      await flow.handleImport();
      expect(file.size).toBe(limit);
      expect(file.text).toHaveBeenCalledOnce();
      expectNotSent(flow, file);
      expect(flow.transferError()).toContain('backup and password exceed');
    } finally {
      dispose();
    }
  });

  it.each([limit - 1, limit])('sends one request at %i UTF-8 bytes', async (size) => {
    const data = 'A'.repeat(size - bytes(body('')));
    const { flow, file, dispose } = mount(JSON.stringify({ status: 'success', data }));
    try {
      expect(file.size).toBeLessThan(limit);
      await flow.handleImport();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      const options = vi.mocked(apiFetchJSON).mock.calls[0][1]!;
      expect(bytes(String(options.body))).toBe(size);
      expect(String(options.body)).toBe(body(data));
      expect(options.retry).toBe(false);
      expect(options.signal).toBeInstanceOf(AbortSignal);
      expect(flow.transferError()).toBeNull();
      expect(flow.transferStatus()?.phase).toBe('reloading');
    } finally {
      dispose();
    }
    expect(vi.getTimerCount()).toBe(0);
  });

  it.each(['ASCII', 'multibyte', 'escaped'] as const)(
    'does not send an oversized %s password/request',
    async (kind) => {
      const password =
        kind === 'multibyte'
          ? '🔒'.repeat(12)
          : kind === 'escaped'
            ? '"\\\n'.repeat(12)
            : passphrase;
      const data = 'A'.repeat(limit - bytes(body('', password)) + 1);
      expect(bytes(body(data, password))).toBe(limit + 1);
      if (kind === 'multibyte') expect(body(data, password).length).toBeLessThan(limit);
      const { flow, file, dispose } = mount(JSON.stringify({ data }), password);
      try {
        await flow.handleImport();
        expect(file.text).toHaveBeenCalledOnce();
        expectNotSent(flow, file, password);
      } finally {
        dispose();
      }
    },
  );

  it('counts JSON escaping in the raw archive, not just the data string length', async () => {
    const data = 'A\\'.repeat(Math.floor(limit / 3));
    expect(bytes(data)).toBeLessThan(limit);
    expect(bytes(body(data))).toBeGreaterThan(limit);
    const { flow, file, dispose } = mount(data);
    try {
      await flow.handleImport();
      expectNotSent(flow, file);
    } finally {
      dispose();
    }
  });

  it('allows an explicit corrected submission without replaying the rejected file', async () => {
    const { flow, file, dispose } = mount('A'.repeat(limit + 1));
    try {
      await flow.handleImport();
      expectNotSent(flow, file);
      await vi.advanceTimersByTimeAsync(5000);
      expect(apiFetchJSON).not.toHaveBeenCalled();
      const replacement = archive('{"data":"synthetic-archive"}');
      flow.setImportFile(replacement);
      expect(apiFetchJSON).not.toHaveBeenCalled();
      await flow.handleImport();
      expect(apiFetchJSON).toHaveBeenCalledOnce();
      expect(JSON.parse(String(vi.mocked(apiFetchJSON).mock.calls[0][1]!.body))).toEqual({
        passphrase,
        data: 'synthetic-archive',
      });
      expect(flow.transferError()).toBeNull();
    } finally {
      dispose();
    }
  });
});
