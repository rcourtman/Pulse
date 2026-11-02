import { createSignal } from 'solid-js';
import { renderHook, waitFor } from '@solidjs/testing-library';
import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { apiFetch, setOrgID } from '@/utils/apiClient';
import { eventBus } from '@/stores/events';
import { showSuccess, showWarning } from '@/utils/toast';
import { useAuditLogPanelState, type AuditEvent } from '../useAuditLogPanelState';

const [licensed, setLicensed] = createSignal(true);

vi.mock('@solidjs/router', () => ({
  useLocation: () => ({ pathname: '/settings/security-audit', search: '' }),
  useNavigate: () => vi.fn(),
}));
vi.mock('@/stores/license', () => ({
  getRuntimeCapabilityBlock: () => undefined,
  hasFeature: () => licensed(),
  runtimeCapabilitiesLoaded: () => true,
  loadRuntimeCapabilities: vi.fn().mockResolvedValue(undefined),
}));
vi.mock('@/stores/licenseCommercial', () => ({ getUpgradeActionDestination: () => '/upgrade' }));
vi.mock('@/stores/sessionPresentationPolicy', () => ({
  presentationPolicyHidesUpgradePrompts: () => false,
}));
vi.mock('@/utils/toast', () => ({
  showSuccess: vi.fn(),
  showWarning: vi.fn(),
  showToast: vi.fn(),
}));
vi.mock('@/utils/apiClient', async (importOriginal) => ({
  ...(await importOriginal<typeof import('@/utils/apiClient')>()),
  apiFetch: vi.fn(),
}));

const deferred = <T,>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((done) => {
    resolve = done;
  });
  return { promise, resolve };
};
const event = (id: string): AuditEvent => ({
  id,
  timestamp: '2026-11-02T07:00:00Z',
  event: 'login',
  user: 'fixture-reader',
  ip: '192.0.2.1',
  path: '/api/login',
  success: true,
  details: id,
  signature: 'fixture-signature',
});
const json = (value: unknown, status = 200) =>
  new Response(JSON.stringify(value), {
    status,
    headers: { 'Content-Type': 'application/json' },
  });
const list = (ids: string[]) =>
  json({ events: ids.map(event), total: ids.length, persistentLogging: true });
const verified = () => json({ available: true, verified: true, message: 'matched' });
const settle = async () => {
  await Promise.resolve();
  await Promise.resolve();
};
const mount = () => {
  const [reader, setReader] = createSignal({
    currentUser: 'reader-a',
    accessKey: 'token-a',
    canRead: true,
  });
  const hook = renderHook(() =>
    useAuditLogPanelState({
      get currentUser() {
        return reader().currentUser;
      },
      get accessKey() {
        return reader().accessKey;
      },
      get canRead() {
        return reader().canRead;
      },
    }),
  );
  return { ...hook, reader, setReader };
};

describe('Audit log reader lifetime', () => {
  afterEach(() => vi.useRealTimers());
  beforeEach(() => {
    setLicensed(true);
    window.localStorage.clear();
    window.localStorage.setItem('pulse-audit-auto-verify', 'false');
    setOrgID('org-a');
    vi.mocked(apiFetch).mockReset();
    vi.mocked(showSuccess).mockClear();
    vi.mocked(showWarning).mockClear();
  });

  it('withdraws accepted rows and verification immediately on an organisation switch', async () => {
    const next = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['private-a']))
      .mockImplementationOnce(() => next.promise);
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(1));
      setOrgID('org-b');
      eventBus.emit('org_switched', 'org-b');
      expect(result.events()).toEqual([]);
      expect(result.isPersistent()).toBe(false);
      expect(result.loading()).toBe(true);
      next.resolve(list(['private-b']));
      await waitFor(() => expect(result.events()[0]?.id).toBe('private-b'));
      expect(apiFetch).toHaveBeenLastCalledWith(
        expect.any(String),
        expect.objectContaining({ expectedOrgID: 'org-b', retry: false }),
      );
    } finally {
      cleanup();
    }
  });

  it('does not let an old signature completion enter a refreshed page with the same ID', async () => {
    const oldVerify = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['same-id']))
      .mockImplementationOnce(() => oldVerify.promise)
      .mockResolvedValueOnce(list(['same-id']));
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(1));
      const verification = result.verifyEvent(result.events()[0]);
      result.refresh();
      await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(3));
      oldVerify.resolve(verified());
      await verification;
      expect(result.verification()).toEqual({});
      expect(result.verifying()).toEqual({});
    } finally {
      cleanup();
    }
  });
  it.each(['principal', 'credential', 'read access', 'entitlement'] as const)(
    'withdraws reader data on %s loss, then accepts only a fresh read',
    async (change) => {
      const old = deferred<Response>();
      const current = deferred<Response>();
      vi.mocked(apiFetch)
        .mockImplementationOnce(() => old.promise)
        .mockImplementationOnce(() => current.promise);
      const { result, reader, setReader, cleanup } = mount();
      try {
        const firstSignal = vi.mocked(apiFetch).mock.calls[0][1]?.signal;
        if (change === 'principal') setReader({ ...reader(), currentUser: 'reader-b' });
        if (change === 'credential') setReader({ ...reader(), accessKey: 'token-b' });
        if (change === 'read access') setReader({ ...reader(), canRead: false });
        if (change === 'entitlement') setLicensed(false);
        expect(firstSignal?.aborted).toBe(true);
        expect(result.events()).toEqual([]);
        expect(result.verification()).toEqual({});
        if (change === 'read access' || change === 'entitlement') {
          expect(apiFetch).toHaveBeenCalledTimes(1);
          result.refresh();
          result.verifyAll();
          expect(apiFetch).toHaveBeenCalledTimes(1);
          if (change === 'read access') setReader({ ...reader(), canRead: true });
          else setLicensed(true);
        }
        await waitFor(() => expect(apiFetch).toHaveBeenCalledTimes(2));
        old.resolve(list(['withdrawn']));
        await settle();
        expect(result.loading()).toBe(true);
        expect(result.events()).toEqual([]);
        current.resolve(list(['admitted']));
        await waitFor(() => expect(result.events()[0]?.id).toBe('admitted'));
      } finally {
        cleanup();
      }
    },
  );

  it('ignores equal-reader status refreshes without replacing the pending request', async () => {
    const read = deferred<Response>();
    vi.mocked(apiFetch).mockImplementation(() => read.promise);
    const { result, reader, setReader, cleanup } = mount();
    try {
      setReader({ ...reader() });
      expect(apiFetch).toHaveBeenCalledTimes(1);
      expect(vi.mocked(apiFetch).mock.calls[0][1]?.signal?.aborted).toBe(false);
      read.resolve(list(['kept']));
      await waitFor(() => expect(result.events()[0]?.id).toBe('kept'));
    } finally {
      cleanup();
    }
  });

  it('never readmits a response after leaving and returning to the same reader', async () => {
    const first = deferred<Response>();
    const second = deferred<Response>();
    const third = deferred<Response>();
    vi.mocked(apiFetch)
      .mockImplementationOnce(() => first.promise)
      .mockImplementationOnce(() => second.promise)
      .mockImplementationOnce(() => third.promise);
    const { result, reader, setReader, cleanup } = mount();
    try {
      setReader({ ...reader(), currentUser: 'reader-b' });
      setReader({ ...reader(), currentUser: 'reader-a' });
      first.resolve(list(['first-a']));
      second.resolve(list(['b']));
      await settle();
      expect(result.events()).toEqual([]);
      expect(result.loading()).toBe(true);
      third.resolve(list(['current-a']));
      await waitFor(() => expect(result.events()[0]?.id).toBe('current-a'));
    } finally {
      cleanup();
    }
  });

  it('rejects an obsolete response body even after the HTTP response was admitted', async () => {
    const body = deferred<unknown>();
    const oldResponse = new Response();
    vi.spyOn(oldResponse, 'json').mockImplementation(() => body.promise);
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(oldResponse)
      .mockResolvedValueOnce(list(['new-reader']));
    const { result, reader, setReader, cleanup } = mount();
    try {
      await waitFor(() => expect(oldResponse.json).toHaveBeenCalled());
      setReader({ ...reader(), currentUser: 'reader-b' });
      await waitFor(() => expect(result.events()[0]?.id).toBe('new-reader'));
      body.resolve({ events: [event('old-private')], total: 1, persistentLogging: true });
      await settle();
      expect(result.events()[0]?.id).toBe('new-reader');
    } finally {
      cleanup();
    }
  });

  it('clears rows while a refresh is pending, not only after its refusal', async () => {
    const refresh = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['old']))
      .mockImplementationOnce(() => refresh.promise);
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(1));
      result.refresh();
      expect(result.events()).toEqual([]);
      expect(result.totalEvents()).toBe(0);
      expect(result.isPersistent()).toBe(false);
      refresh.resolve(json({ error: 'private body must not become a toast' }, 403));
      await waitFor(() => expect(result.error()).toContain('access could not be confirmed'));
      expect(result.error()).not.toContain('private body');
      expect(showWarning).not.toHaveBeenCalled();
    } finally {
      cleanup();
    }
  });

  it('stops a withdrawn batch and cannot finish a new batch with reused IDs', async () => {
    const old = deferred<Response>();
    const current = deferred<Response>();
    let generation = 0;
    vi.mocked(apiFetch).mockImplementation(async (url) => {
      if (String(url).includes('/verify')) return generation === 0 ? old.promise : current.promise;
      return list(['same-id', 'second']);
    });
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(2));
      result.verifyAll();
      expect(result.verifyingAll()).toBe(true);
      const oldSignal = vi.mocked(apiFetch).mock.calls.at(-1)?.[1]?.signal;
      generation = 1;
      setOrgID('org-b');
      eventBus.emit('org_switched', 'org-b');
      expect(oldSignal?.aborted).toBe(true);
      await waitFor(() => expect(result.events()).toHaveLength(2));
      result.verifyAll();
      old.resolve(verified());
      await settle();
      expect(result.verifyingAll()).toBe(true);
      expect(result.verifyAllDone()).toBe(0);
      expect(result.verification()).toEqual({});
      expect(showSuccess).not.toHaveBeenCalled();
      current.resolve(verified());
      await waitFor(() => expect(result.verifyingAll()).toBe(false));
      expect(result.verifyAllDone()).toBe(2);
      expect(
        vi.mocked(apiFetch).mock.calls.filter(([url]) => String(url).includes('/verify')),
      ).toHaveLength(3);
    } finally {
      cleanup();
    }
  });

  it('cancels verification immediately even if the transport ignores abort, with explicit Resume', async () => {
    const old = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['a', 'b']))
      .mockImplementationOnce(() => old.promise)
      .mockImplementation(async () => verified());
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(2));
      result.verifyAll();
      result.cancelVerification();
      expect(result.verifyingAll()).toBe(false);
      expect(result.verifyCanceled()).toBe(true);
      expect(result.hasResumeEvents()).toBe(true);
      old.resolve(verified());
      await settle();
      expect(result.verification()).toEqual({});
      expect(apiFetch).toHaveBeenCalledTimes(2);
      result.resumeVerification();
      await waitFor(() => expect(result.verification().b?.status).toBe('verified'));
      expect(result.verifyAllDone()).toBe(2);
    } finally {
      cleanup();
    }
  });

  it('does not verify obsolete row objects, duplicates or a row from another organisation', async () => {
    const pending = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['a/b']))
      .mockImplementationOnce(() => pending.promise)
      .mockResolvedValueOnce(list(['new']));
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(1));
      const row = result.events()[0];
      await result.verifyEvent({ ...row });
      expect(apiFetch).toHaveBeenCalledTimes(1);
      const work = result.verifyEvent(row);
      await result.verifyEvent(row);
      expect(apiFetch).toHaveBeenCalledTimes(2);
      expect(apiFetch).toHaveBeenLastCalledWith(
        '/api/audit/a%2Fb/verify',
        expect.objectContaining({ expectedOrgID: 'org-a', retry: false }),
      );
      setOrgID('org-b'); // Intentionally no event: action must notice storage drift.
      await result.verifyEvent(row);
      expect(
        vi.mocked(apiFetch).mock.calls.filter(([url]) => String(url).includes('/verify')),
      ).toHaveLength(1);
      pending.resolve(verified());
      await work;
      await waitFor(() => expect(result.events()[0]?.id).toBe('new'));
      expect(result.verification()).toEqual({});
    } finally {
      cleanup();
    }
  });

  it.each([401, 402, 403])(
    'withdraws the whole page on a fresh verification refusal (%s)',
    async (status) => {
      vi.mocked(apiFetch)
        .mockResolvedValueOnce(list(['a', 'b']))
        .mockResolvedValueOnce(json({ error: 'private-refusal' }, status))
        .mockResolvedValueOnce(list(['recovered']));
      const { result, cleanup } = mount();
      try {
        await waitFor(() => expect(result.events()).toHaveLength(2));
        result.verifyAll();
        await waitFor(() => expect(result.error()).toContain('access could not be confirmed'));
        expect(result.events()).toEqual([]);
        expect(result.verification()).toEqual({});
        expect(result.verifyingAll()).toBe(false);
        expect(apiFetch).toHaveBeenCalledTimes(2);
        expect(showWarning).not.toHaveBeenCalled();
        result.refresh();
        await waitFor(() => expect(result.events()[0]?.id).toBe('recovered'));
      } finally {
        cleanup();
      }
    },
  );

  it.each([
    { events: null, total: 0, persistentLogging: true },
    { events: [event('a'), event('a')], total: 2, persistentLogging: true },
    { events: [event('a')], total: -1, persistentLogging: true },
    { events: [event('a')], total: 1.5, persistentLogging: true },
    { events: [null], total: 1, persistentLogging: true },
    { events: [{ ...event('a'), success: 'true' }], total: 1, persistentLogging: true },
  ])('keeps malformed inventory unavailable until an explicit read succeeds: %#', async (body) => {
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(json(body))
      .mockResolvedValueOnce(list(['recovered']));
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.error()).toBe('Audit log returned an invalid response'));
      expect(result.isPersistent()).toBe(false);
      expect(result.events()).toEqual([]);
      result.verifyAll();
      expect(apiFetch).toHaveBeenCalledTimes(1);
      result.refresh();
      await waitFor(() => expect(result.events()[0]?.id).toBe('recovered'));
    } finally {
      cleanup();
    }
  });

  it('accepts omitted optional strings from the actual audit Event JSON contract', async () => {
    vi.mocked(apiFetch).mockResolvedValue(
      json({
        events: [
          {
            id: 'startup',
            timestamp: '2026-11-02T07:00:00Z',
            event: 'startup',
            ip: '',
            success: true,
          },
        ],
        total: 1,
        persistentLogging: true,
      }),
    );
    const { result, cleanup } = mount();
    try {
      await waitFor(() => expect(result.events()).toHaveLength(1));
      expect(result.error()).toBeNull();
    } finally {
      cleanup();
    }
  });

  it('aborts pending verification at disposal and suppresses its ignored-abort result', async () => {
    const pending = deferred<Response>();
    vi.mocked(apiFetch)
      .mockResolvedValueOnce(list(['a']))
      .mockImplementationOnce(() => pending.promise);
    const { result, cleanup } = mount();
    await waitFor(() => expect(result.events()).toHaveLength(1));
    result.verifyAll();
    const signal = vi.mocked(apiFetch).mock.calls.at(-1)?.[1]?.signal;
    cleanup();
    expect(signal?.aborted).toBe(true);
    pending.resolve(verified());
    await settle();
    expect(result.events()).toEqual([]);
    expect(result.verification()).toEqual({});
    expect(showSuccess).not.toHaveBeenCalled();
    expect(showWarning).not.toHaveBeenCalled();
  });
});
