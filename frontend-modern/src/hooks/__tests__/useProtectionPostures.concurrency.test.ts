import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import {
  MAX_CONCURRENT_PROTECTION_POSTURE_REQUESTS,
  useProtectionPostures,
} from '@/hooks/useProtectionPostures';
import { eventBus } from '@/stores/events';
import type { ProtectionPosturesResponse } from '@/types/recovery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: apiFetchJSONMock }));

const fleet = (count: number) =>
  Array.from({ length: count }, (_, index) => `vm:${String(index).padStart(5, '0')}`);
const disposers: (() => void)[] = [];
interface PendingBatch {
  ids: string[];
  signal: AbortSignal;
  settled: boolean;
  finish: () => void;
  reject: (reason: unknown) => void;
}

function holdBatches(abortRejects = false) {
  const requests: PendingBatch[] = [];
  let active = 0;
  let peak = 0;
  apiFetchJSONMock.mockImplementation(
    (url: string, { signal }: { signal: AbortSignal }) =>
      new Promise<ProtectionPosturesResponse>((resolve, reject) => {
        const ids = new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId');
        active += 1;
        peak = Math.max(peak, active);
        const settle = () => {
          if (batch.settled) return false;
          batch.settled = true;
          active -= 1;
          return true;
        };
        const batch: PendingBatch = {
          ids,
          signal,
          settled: false,
          finish: () => {
            if (settle())
              resolve({ data: ids.map(protectionPosture).reverse(), policy: protectionPolicy });
          },
          reject: (error) => {
            if (settle()) reject(error);
          },
        };
        requests.push(batch);
        if (abortRejects)
          signal.addEventListener(
            'abort',
            () => batch.reject(new DOMException('Cancelled sibling', 'AbortError')),
            { once: true },
          );
      }),
  );
  return { requests, peak: () => peak, active: () => active };
}

function mount(ids: () => readonly string[]) {
  return createRoot((dispose) => {
    disposers.push(dispose);
    return useProtectionPostures(ids);
  });
}

async function flush() {
  // Drain a completed wave without advancing the 30-second polling clock.
  for (let turn = 0; turn < 8; turn++) await Promise.resolve();
}

async function finishRead(requests: PendingBatch[], start = 0) {
  let next = start;
  while (next < requests.length) {
    requests[next++].finish();
    await flush();
  }
}

afterEach(() => {
  disposers.splice(0).forEach((dispose) => dispose());
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('bounded protection posture fan-out', () => {
  it('caps a 10,001-subject read at four active requests and publishes only the complete ordered result', async () => {
    const ids = fleet(10_001);
    const held = holdBatches();
    const hook = mount(() => ids);
    await vi.waitFor(() => expect(held.requests).toHaveLength(4));
    expect(MAX_CONCURRENT_PROTECTION_POSTURE_REQUESTS).toBe(4);
    expect(held.active()).toBe(4);
    expect(hook.postures()).toEqual([]);
    expect(hook.policy().freshnessWindowSeconds).toBe(0);

    // Complete the fourth batch first: exactly one queued batch replaces it.
    held.requests[3].finish();
    await flush();
    expect(held.requests).toHaveLength(5);
    expect(held.active()).toBe(4);
    expect(hook.postures()).toEqual([]);
    expect(hook.resolvedOnce()).toBe(false);
    await finishRead(held.requests);
    await vi.waitFor(() => expect(hook.postures()).toHaveLength(10_001));
    expect(held.peak()).toBe(4);
    expect(held.requests).toHaveLength(51);
    expect(held.requests.flatMap((batch) => batch.ids)).toEqual(ids);
    expect(held.requests.every((batch) => batch.signal === held.requests[0].signal)).toBe(true);
    expect(held.requests[0].signal.aborted).toBe(false);
    expect(hook.postures().map((posture) => posture.subjectResourceId)).toEqual(
      held.requests.flatMap((batch) => batch.ids.slice().reverse()),
    );
    expect(hook.policy()).toEqual(protectionPolicy);
    expect(hook.response.error).toBeNull();
  });

  it('drops queued work and preserves the initiating failure instead of a sibling abort, then retries the whole fleet', async () => {
    const ids = fleet(1_001);
    const held = holdBatches(true);
    const hook = mount(() => ids);
    await vi.waitFor(() => expect(held.requests).toHaveLength(4));
    const failure = new Error('Second batch unavailable');
    held.requests[1].reject(failure);
    await vi.waitFor(() => expect(hook.response.error).toBe(failure));
    expect(held.requests).toHaveLength(4);
    expect(held.requests.every((batch) => batch.signal.aborted)).toBe(true);
    expect(held.active()).toBe(0);
    expect(hook.postures()).toEqual([]);
    expect(hook.policy().freshnessWindowSeconds).toBe(0);

    const retry = hook.refetch();
    expect(held.requests).toHaveLength(8);
    expect(hook.response.error).toBe(failure);
    held.requests[7].finish();
    await flush();
    expect(held.requests).toHaveLength(9);
    expect(hook.postures()).toEqual([]);
    await finishRead(held.requests, 4);
    await retry;
    expect(held.requests.slice(4).flatMap((batch) => batch.ids)).toEqual(ids);
    expect(held.requests).toHaveLength(10);
    expect(held.peak()).toBe(4);
    expect(hook.postures()).toHaveLength(1_001);
    expect(hook.policy()).toEqual(protectionPolicy);
    expect(hook.response.error).toBeNull();
  });

  it.each(['scope', 'organisation', 'replacement', 'empty', 'disposal'] as const)(
    'never dispatches queued work after %s retirement, even if cancelled transports resolve late',
    async (retirement) => {
      const held = holdBatches();
      const [ids, setIDs] = createSignal<readonly string[]>(fleet(1_001));
      const hook = mount(ids);
      await vi.waitFor(() => expect(held.requests).toHaveLength(4));
      const retired = held.requests.slice();
      let retry: Promise<ProtectionPosturesResponse> | undefined;
      if (retirement === 'scope') setIDs(['vm:replacement']);
      if (retirement === 'organisation') eventBus.emit('org_switched', 'fixture-other-org');
      if (retirement === 'replacement') retry = hook.refetch();
      if (retirement === 'empty') setIDs([]);
      if (retirement === 'disposal') disposers.pop()!();
      await flush();
      expect(retired.every((batch) => batch.signal.aborted)).toBe(true);
      const count = held.requests.length;
      expect(count).toBe(
        retirement === 'scope' ? 5 : retirement === 'empty' || retirement === 'disposal' ? 4 : 8,
      );
      for (const batch of retired) batch.finish();
      await flush();
      expect(held.requests).toHaveLength(count);
      expect(hook.postures()).toEqual([]);
      await finishRead(held.requests, 4);
      await retry;
      if (retirement === 'scope') {
        expect(hook.postures().map((posture) => posture.subjectResourceId)).toEqual([
          'vm:replacement',
        ]);
      } else if (retirement === 'organisation' || retirement === 'replacement') {
        expect(hook.postures()).toHaveLength(1_001);
        expect(held.requests.slice(4).flatMap((batch) => batch.ids)).toEqual(fleet(1_001));
      } else {
        expect(hook.postures()).toEqual([]);
      }
      expect(hook.response.error).toBeNull();
    },
  );
});
