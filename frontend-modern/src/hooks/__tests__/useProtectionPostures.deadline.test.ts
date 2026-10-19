import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { eventBus } from '@/stores/events';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import {
  fetchAllProtectionPostures,
  PROTECTION_POSTURE_READ_TIMEOUT_MS,
  useProtectionPostures,
} from '@/hooks/useProtectionPostures';

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));

const ids = Array.from({ length: 201 }, (_, i) => `vm:${i}`);
const response = (url: string) => ({
  data: new URL(url, 'https://pulse.invalid').searchParams
    .getAll('resourceId')
    .map((subjectResourceId) => ({
      subjectResourceId,
      state: 'protected',
      freshness: 'current',
      verification: 'verified',
      coverage: 'complete',
      explanation: 'Current verified backup.',
      evaluatedAt: '2026-10-19T09:00:00Z',
      providerStates: [],
      evidenceIds: [],
      repositoryResourceIds: [],
    })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 604800,
    requireVerification: true,
  },
});
const flush = () => vi.advanceTimersByTimeAsync(0);
let dispose = () => {};
const mount = () =>
  createRoot((stop) => {
    dispose = stop;
    return useProtectionPostures(() => ids);
  });

afterEach(() => {
  dispose();
  resetCreateNonSuspendingQueryCacheForTest();
  vi.useRealTimers();
  apiFetchJSON.mockReset();
});

describe('protection read lifetime through the query owner', () => {
  it('settles a superseded manual refresh even when the transport ignores abort', async () => {
    vi.useFakeTimers();
    apiFetchJSON.mockImplementation(async (url: string) => response(url));
    const hook = mount();
    await flush();
    expect(hook.postures()).toHaveLength(201);
    const pending: { signal: AbortSignal; finish: (value: unknown) => void; url: string }[] = [];
    apiFetchJSON.mockImplementation(
      (url: string, { signal }: { signal: AbortSignal }) =>
        new Promise((finish) => pending.push({ signal, finish, url })),
    );
    let settled = false;
    void hook.refetch().then(() => {
      settled = true;
    });
    await flush();
    expect(settled).toBe(false);
    // Existing background polling replaces the foreground refresh once.
    await vi.advanceTimersByTimeAsync(30_000);
    expect(pending).toHaveLength(4);
    expect(pending[0].signal.aborted).toBe(true);
    expect(settled).toBe(true);
    // Cancellation is not a new result. Late obsolete bodies cannot win.
    pending[0].finish(response(pending[0].url));
    pending[1].finish(response(pending[1].url));
    await flush();
    expect(hook.postures()).toHaveLength(201);
    dispose();
    expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
    await flush();
    expect(vi.getTimerCount()).toBe(1); // Retained-query cache expiry only.
  });

  it.each(['denied', 'unreadable'])('cancels every sibling when one batch is %s', async (kind) => {
    vi.useFakeTimers();
    const signals: AbortSignal[] = [];
    apiFetchJSON.mockImplementation((url: string, { signal }: { signal: AbortSignal }) => {
      signals.push(signal);
      if (new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId').length === 1)
        return kind === 'denied'
          ? Promise.reject(Object.assign(new Error('Denied'), { status: 403 }))
          : Promise.resolve({ private: 'PRIVATE_PAYLOAD' });
      return new Promise(() => {});
    });
    const hook = mount();
    await flush();
    expect(hook.response.loading).toBe(false);
    expect(hook.response.error).toBeInstanceOf(Error);
    expect(String(hook.response.error)).not.toContain('PRIVATE_PAYLOAD');
    expect(hook.postures()).toEqual([]);
    expect(signals).toHaveLength(2);
    expect(signals.every((signal) => signal.aborted)).toBe(true);
  });

  it('settles a cancelled refresh on organisation switch and retires every old batch', async () => {
    vi.useFakeTimers();
    apiFetchJSON.mockImplementation(async (url: string) => response(url));
    const hook = mount();
    await flush();
    const signals: AbortSignal[] = [];
    apiFetchJSON.mockImplementation((_url: string, { signal }: { signal: AbortSignal }) => {
      signals.push(signal);
      return new Promise(() => {});
    });
    let settled = false;
    void hook.refetch().then(() => {
      settled = true;
    });
    eventBus.emit('org_switched', 'next-org');
    await flush();
    expect(settled).toBe(true);
    expect(signals.slice(0, 2).every((signal) => signal.aborted)).toBe(true);
    expect(hook.postures()).toEqual([]);
    expect(hook.response.error).toBeNull();
    dispose();
    expect(signals.every((signal) => signal.aborted)).toBe(true);
    await flush();
    expect(vi.getTimerCount()).toBe(0);
  });
});

describe('whole protection evaluation read budget', () => {
  it.each([1, 201])('settles all %i subjects at one deadline, not one per batch', async (count) => {
    vi.useFakeTimers();
    expect(PROTECTION_POSTURE_READ_TIMEOUT_MS).toBe(150_000);
    const pending: { url: string; signal: AbortSignal; finish: (value: unknown) => void }[] = [];
    apiFetchJSON.mockImplementation(
      (url: string, { signal }: { signal: AbortSignal }) =>
        new Promise((finish) => pending.push({ url, signal, finish })),
    );
    let result: unknown;
    let settled = false;
    const read = fetchAllProtectionPostures(ids.slice(0, count), new AbortController().signal)
      .catch((error) => {
        result = error;
      })
      .finally(() => {
        settled = true;
      });
    // One slow body must not restart the whole evaluation's clock after
    // another batch completes (or after the existing Retry-After allowance).
    await vi.advanceTimersByTimeAsync(120_000);
    if (pending.length > 1) pending[0].finish(response(pending[0].url));
    await vi.advanceTimersByTimeAsync(29_999);
    expect(settled).toBe(false);
    await vi.advanceTimersByTimeAsync(1);
    await read;
    expect(String(result)).toBe('Error: The protection posture read timed out.');
    expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
    expect(apiFetchJSON).toHaveBeenCalledTimes(Math.ceil(count / 200));
    pending.forEach(({ finish, url }) => finish(response(url)));
    await flush();
    expect(String(result)).toContain('timed out');
    expect(vi.getTimerCount()).toBe(0);
  });

  it('keeps a successful delayed response inside the existing Retry-After allowance', async () => {
    vi.useFakeTimers();
    apiFetchJSON.mockImplementation(
      (url: string) =>
        new Promise((resolve) => {
          setTimeout(() => resolve(response(url)), 120_000);
        }),
    );
    const controller = new AbortController();
    const remove = vi.spyOn(controller.signal, 'removeEventListener');
    const read = fetchAllProtectionPostures(ids, controller.signal);
    await vi.advanceTimersByTimeAsync(120_000);
    expect((await read).data).toHaveLength(201);
    expect(remove).toHaveBeenCalledWith('abort', expect.any(Function));
    expect(vi.getTimerCount()).toBe(0);
  });

  it('does not start any batch for an already cancelled owner', async () => {
    vi.useFakeTimers();
    const controller = new AbortController();
    controller.abort();
    await expect(fetchAllProtectionPostures(ids, controller.signal)).rejects.toMatchObject({
      name: 'AbortError',
    });
    expect(apiFetchJSON).not.toHaveBeenCalled();
    expect(vi.getTimerCount()).toBe(0);
  });
});
