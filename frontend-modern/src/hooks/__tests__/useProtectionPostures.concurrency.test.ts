import { afterEach, describe, expect, it, vi } from 'vitest';
import {
  fetchAllProtectionPostures,
  PROTECTION_POSTURE_READ_TIMEOUT_MS,
} from '@/hooks/useProtectionPostures';

const apiFetchJSON = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON }));

const ids = Array.from({ length: 2001 }, (_, index) => `vm:${String(index).padStart(4, '0')}`);
const requestedIDs = (url: string) =>
  new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId');
const policy = {
  freshnessWindowSeconds: 604800,
  verificationWindowSeconds: 604800,
  requireVerification: true,
};
const response = (url: string) => ({
  data: requestedIDs(url).map((subjectResourceId) => ({
    subjectResourceId,
    state: 'protected',
    freshness: 'current',
    verification: 'verified',
    coverage: 'complete',
    explanation: 'Synthetic current verified backup.',
    evaluatedAt: '2026-10-22T00:00:00Z',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: [],
  })),
  policy,
});
const flush = () => vi.advanceTimersByTimeAsync(0);

type Pending = {
  url: string;
  signal: AbortSignal;
  finish: (value: unknown) => void;
  fail: (error: unknown) => void;
};
const holdReads = () => {
  const pending: Pending[] = [];
  apiFetchJSON.mockImplementation(
    (url: string, { signal }: { signal: AbortSignal }) =>
      new Promise((finish, fail) => pending.push({ url, signal, finish, fail })),
  );
  return pending;
};

afterEach(() => {
  vi.useRealTimers();
  apiFetchJSON.mockReset();
});

describe('bounded whole-fleet protection reads', () => {
  it('admits four batches, refills a freed slot and preserves request-order results', async () => {
    vi.useFakeTimers();
    const pending = holdReads();
    const owner = new AbortController();
    const read = fetchAllProtectionPostures(ids, owner.signal);
    // Catch cancellation even when a regression assertion exits early.
    void read.catch(() => {});
    try {
      await flush();
      expect(pending).toHaveLength(4);
      expect(pending.every(({ signal }) => signal === pending[0].signal)).toBe(true);
      pending[3].finish(response(pending[3].url));
      await flush();
      expect(pending).toHaveLength(5);
      // Complete the rest in reverse waves to detect completion-order joins.
      const done = new Set([3]);
      while (done.size < 11) {
        const wave = pending.map((entry, index) => ({ entry, index })).reverse();
        for (const { entry, index } of wave) {
          if (done.has(index)) continue;
          done.add(index);
          entry.finish(response(entry.url));
        }
        await flush();
        expect(pending.length - done.size).toBeLessThanOrEqual(4);
      }
      const result = await read;
      expect(pending.map(({ url }) => requestedIDs(url).length)).toEqual([
        200, 200, 200, 200, 200, 200, 200, 200, 200, 200, 1,
      ]);
      expect(result.data.map((posture) => posture.subjectResourceId)).toEqual(ids);
      expect(result.policy).toEqual(policy);
      expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      owner.abort();
      pending.forEach(({ finish, url }) => finish(response(url)));
      await read.catch(() => {});
    }
  });

  it.each(['denied', 'malformed'])(
    'stops queued work on a %s batch, including late siblings',
    async (kind) => {
      vi.useFakeTimers();
      const pending = holdReads();
      const owner = new AbortController();
      const read = fetchAllProtectionPostures(ids, owner.signal).catch((error) => error);
      try {
        await flush();
        expect(pending).toHaveLength(4);
        if (kind === 'denied') pending[1].fail(Object.assign(new Error('Denied'), { status: 403 }));
        else pending[1].finish({ private: 'SYNTHETIC_PRIVATE_BODY' });
        const result = await read;
        expect(result).toBeInstanceOf(Error);
        expect(String(result)).not.toContain('SYNTHETIC_PRIVATE_BODY');
        expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
        pending.forEach(({ finish, url }) => finish(response(url)));
        await flush();
        expect(pending).toHaveLength(4);
        expect(vi.getTimerCount()).toBe(0);
      } finally {
        owner.abort();
        await read;
      }
    },
  );

  it('uses the original whole-evaluation deadline while batches are queued', async () => {
    vi.useFakeTimers();
    const pending = holdReads();
    const owner = new AbortController();
    const read = fetchAllProtectionPostures(ids, owner.signal).catch((error) => error);
    try {
      await flush();
      expect(pending).toHaveLength(4);
      await vi.advanceTimersByTimeAsync(120_000);
      pending[0].finish(response(pending[0].url));
      await flush();
      expect(pending).toHaveLength(5);
      let settled = false;
      void read.then(() => (settled = true));
      await vi.advanceTimersByTimeAsync(PROTECTION_POSTURE_READ_TIMEOUT_MS - 120_001);
      expect(settled).toBe(false);
      await vi.advanceTimersByTimeAsync(1);
      expect(String(await read)).toBe('Error: The protection posture read timed out.');
      expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
      pending.forEach(({ finish, url }) => finish(response(url)));
      await flush();
      expect(pending).toHaveLength(5);
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      owner.abort();
      await read;
    }
  });

  it('settles cancellation without starting queued subjects after late transport success', async () => {
    vi.useFakeTimers();
    const pending = holdReads();
    const owner = new AbortController();
    const read = fetchAllProtectionPostures(ids, owner.signal).catch((error) => error);
    try {
      await flush();
      expect(pending).toHaveLength(4);
      owner.abort();
      expect(await read).toMatchObject({ name: 'AbortError' });
      expect(pending.every(({ signal }) => signal.aborted)).toBe(true);
      pending.forEach(({ finish, url }) => finish(response(url)));
      await flush();
      expect(pending).toHaveLength(4);
      expect(vi.getTimerCount()).toBe(0);
    } finally {
      owner.abort();
      await read;
    }
  });

  it.each([0, 1, 201])(
    'keeps the %i-subject request count and policy compatibility',
    async (count) => {
      vi.useFakeTimers();
      apiFetchJSON.mockImplementation(async (url: string) => response(url));
      const result = await fetchAllProtectionPostures(
        ids.slice(0, count),
        new AbortController().signal,
      );
      expect(apiFetchJSON).toHaveBeenCalledTimes(Math.ceil(count / 200));
      expect(result.data).toHaveLength(count);
      expect(result.policy).toEqual(
        count === 0
          ? {
              freshnessWindowSeconds: 0,
              verificationWindowSeconds: 0,
              requireVerification: false,
            }
          : policy,
      );
      expect(vi.getTimerCount()).toBe(0);
    },
  );
});
