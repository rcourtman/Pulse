import { afterEach, describe, expect, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import { useProtectionPostures } from '@/hooks/useProtectionPostures';
import { eventBus } from '@/stores/events';
import type { ProtectionPosturesResponse } from '@/types/recovery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: apiFetchJSONMock }));

const fleet = Array.from({ length: 401 }, (_, index) => `vm:${index}`);
const disposers: (() => void)[] = [];

interface PendingBatch {
  ids: string[];
  signal?: AbortSignal;
  resolve: (response: ProtectionPosturesResponse) => void;
  reject: (reason: unknown) => void;
}

// Deliberately allow a cancelled request to resolve late as well: the query's
// existing stale-result guard must still hold if a transport ignores abort.
function holdBatches() {
  const requests: PendingBatch[] = [];
  apiFetchJSONMock.mockImplementation(
    (url: string, options?: { signal?: AbortSignal }) =>
      new Promise<ProtectionPosturesResponse>((resolve, reject) => {
        requests.push({
          ids: new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId'),
          signal: options?.signal,
          resolve,
          reject,
        });
      }),
  );
  return requests;
}

function mount(ids: () => readonly string[] = () => fleet) {
  return createRoot((dispose) => {
    disposers.push(dispose);
    return useProtectionPostures(ids);
  });
}

function finish(batches: PendingBatch[]) {
  for (const batch of batches) {
    batch.resolve({ data: batch.ids.map(protectionPosture), policy: protectionPolicy });
  }
}

function expectCancelled(batches: PendingBatch[]) {
  expect(batches).toHaveLength(3);
  for (const batch of batches) {
    expect(batch.signal).toBeInstanceOf(AbortSignal);
    expect(batch.signal?.aborted).toBe(true);
  }
}

afterEach(() => {
  disposers.splice(0).forEach((dispose) => dispose());
  vi.useRealTimers();
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('protection posture batch lifetime', () => {
  it('cancels every pending batch when the consuming surface unmounts', async () => {
    const requests = holdBatches();
    mount();
    await vi.waitFor(() => expect(requests).toHaveLength(3));
    disposers.pop()!();
    expectCancelled(requests);
  });

  it('cancels the previous resource scope without publishing its late assessments', async () => {
    const requests = holdBatches();
    const [ids, setIDs] = createSignal<readonly string[]>(fleet);
    const hook = mount(ids);
    await vi.waitFor(() => expect(requests).toHaveLength(3));
    setIDs(['vm:replacement']);
    await vi.waitFor(() => expect(requests).toHaveLength(4));
    expectCancelled(requests.slice(0, 3));
    expect(requests[3].signal?.aborted).toBe(false);
    finish(requests.slice(3));
    await vi.waitFor(() => expect(hook.postures()).toHaveLength(1));
    finish(requests.slice(0, 3));
    await Promise.resolve();
    expect(hook.postures().map((posture) => posture.subjectResourceId)).toEqual(['vm:replacement']);
    expect(hook.response.error).toBeNull();
  });

  it('cancels the entire pending read on an organisation switch', async () => {
    const requests = holdBatches();
    const hook = mount();
    await vi.waitFor(() => expect(requests).toHaveLength(3));
    eventBus.emit('org_switched', 'replacement-org');
    expectCancelled(requests.slice(0, 3));
    expect(requests).toHaveLength(6);
    expect(hook.postures()).toEqual([]);
    expect(hook.policy().freshnessWindowSeconds).toBe(0);
    finish(requests.slice(3));
    await vi.waitFor(() => expect(hook.postures()).toHaveLength(401));
    expect(hook.response.error).toBeNull();
  });

  it('cancels a replaced refresh and an emptied scope', async () => {
    const requests = holdBatches();
    const [ids, setIDs] = createSignal<readonly string[]>(fleet);
    const hook = mount(ids);
    await vi.waitFor(() => expect(requests).toHaveLength(3));
    const retry = hook.refetch();
    expectCancelled(requests.slice(0, 3));
    expect(requests).toHaveLength(6);
    setIDs([]);
    await vi.waitFor(() => expect(hook.response.loading).toBe(false));
    expectCancelled(requests.slice(3));
    finish(requests);
    await retry;
    expect(hook.postures()).toEqual([]);
    expect(hook.response.error).toBeNull();
    expect(hook.resolvedOnce()).toBe(false);
  });

  it('cancels sibling batches on failure and keeps the original error until a full retry', async () => {
    const requests = holdBatches();
    const hook = mount();
    await vi.waitFor(() => expect(requests).toHaveLength(3));
    const failure = new Error('One batch is unavailable');
    requests[1].reject(failure);
    await vi.waitFor(() => expect(hook.response.error).toBe(failure));
    expectCancelled(requests);
    expect(hook.postures()).toEqual([]);
    const retry = hook.refetch();
    expect(requests).toHaveLength(6);
    expect(hook.response.error).toBe(failure);
    finish(requests.slice(3));
    await retry;
    expect(hook.postures()).toHaveLength(401);
    expect(hook.response.error).toBeNull();
    expect(requests.slice(3).every((batch) => batch.signal?.aborted === false)).toBe(true);
  });

  it('lets the poll replace a pending foreground read without leaving its batches alive', async () => {
    vi.useFakeTimers();
    const requests = holdBatches();
    const hook = mount();
    await vi.advanceTimersByTimeAsync(0);
    expect(requests).toHaveLength(3);
    await vi.advanceTimersByTimeAsync(30_000);
    expectCancelled(requests.slice(0, 3));
    expect(requests).toHaveLength(6);
    finish(requests.slice(3));
    await vi.advanceTimersByTimeAsync(0);
    expect(hook.postures()).toHaveLength(401);
    expect(hook.response.loading).toBe(false);
  });
});
