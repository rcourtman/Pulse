import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest';
import { createRoot, createSignal } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import {
  MAX_PROTECTION_POSTURE_BATCH_SIZE,
  buildProtectionPostureBatchURL,
  normalizeProtectionPostureResourceIDs,
  useProtectionPostures,
} from '@/hooks/useProtectionPostures';
import type { ProtectionPosturesResponse } from '@/types/recovery';

const apiFetchJSONMock = vi.hoisted(() => vi.fn());
vi.mock('@/utils/apiClient', () => ({ apiFetchJSON: apiFetchJSONMock }));

afterEach(() => {
  apiFetchJSONMock.mockReset();
  resetCreateNonSuspendingQueryCacheForTest();
});

describe('useProtectionPostures transport', () => {
  it('deduplicates and sorts resource IDs for a stable bounded cache key and request', () => {
    expect(normalizeProtectionPostureResourceIDs(['vm:b', ' vm:a ', '', 'vm:b'])).toEqual([
      'vm:a',
      'vm:b',
    ]);

    const url = new URL(
      buildProtectionPostureBatchURL(['vm:b', 'vm:a', 'vm:b']),
      'https://pulse.invalid',
    );
    expect(url.pathname).toBe('/api/recovery/postures');
    expect(url.searchParams.getAll('resourceId')).toEqual(['vm:a', 'vm:b']);
    expect(url.searchParams.get('limit')).toBe(String(MAX_PROTECTION_POSTURE_BATCH_SIZE));
  });

  it('rejects a batch larger than the server contract', () => {
    const resourceIDs = Array.from(
      { length: MAX_PROTECTION_POSTURE_BATCH_SIZE + 1 },
      (_, index) => `vm:${index}`,
    );
    expect(() => buildProtectionPostureBatchURL(resourceIDs)).toThrow(/limited to 200/);
  });

  it('joins the batches of a large fleet into the rows and the policy the tab reads', async () => {
    const resourceIDs = Array.from(
      { length: MAX_PROTECTION_POSTURE_BATCH_SIZE + 1 },
      (_, index) => `vm:${String(index).padStart(3, '0')}`,
    );
    const policy = {
      freshnessWindowSeconds: 604800,
      verificationWindowSeconds: 3600,
      requireVerification: true,
    };
    // The backend still sends its pagination meta with every batch, and its rows
    // are not in resource ID order (the handler may order by state first).
    apiFetchJSONMock.mockImplementation(async (url: string) => {
      const requested = new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId');
      return {
        data: requested
          .slice()
          .reverse()
          .map((subjectResourceId) => ({ subjectResourceId, state: 'protected' })),
        policy,
        meta: { page: 1, limit: 200, total: requested.length, totalPages: 1 },
      };
    });
    let dispose = () => {};
    const hook = createRoot((rootDispose) => {
      dispose = rootDispose;
      return useProtectionPostures(() => resourceIDs);
    });
    try {
      await vi.waitFor(() => expect(hook.postures()).toHaveLength(resourceIDs.length));
      const batchSizes = apiFetchJSONMock.mock.calls.map(
        ([url]) => new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId').length,
      );
      expect(batchSizes).toEqual([MAX_PROTECTION_POSTURE_BATCH_SIZE, 1]);
      // Each batch keeps the order the backend sent; the batches join in request order.
      expect(hook.postures().map((posture) => posture.subjectResourceId)).toEqual([
        ...resourceIDs.slice(0, MAX_PROTECTION_POSTURE_BATCH_SIZE).reverse(),
        ...resourceIDs.slice(MAX_PROTECTION_POSTURE_BATCH_SIZE),
      ]);
      expect(hook.postureByResourceID().get('vm:200')?.state).toBe('protected');
      expect(hook.policy()).toEqual(policy);
      // The joined response is the rows and the policy at runtime too: the wire meta of the
      // batches is not carried over (the type pin below cannot see a spread of it).
      expect(Object.keys(await hook.refetch()).sort()).toEqual(['data', 'policy']);
    } finally {
      dispose();
    }
  });

  it('models the data and policy of a posture response and nothing else', () => {
    // Enforced by the frontend type check, not at runtime. The endpoint still
    // sends its pagination meta, but the Proxmox Backups tab reads the rows and
    // the policy, and the hook joins 200-ID batches into one response, so a page
    // count or limit would only be invented client-side with no reader.
    expectTypeOf<keyof ProtectionPosturesResponse>().toEqualTypeOf<'data' | 'policy'>();
  });
});

const postureResponse = (resourceIDs: readonly string[], state = 'protected') => ({
  data: resourceIDs.map((subjectResourceId) => ({
    subjectResourceId,
    state,
    freshness: 'current',
    verification: 'verified',
    coverage: 'complete',
    providerStates: [],
    repositoryResourceIds: [],
    evidenceIds: ['point-1'],
    explanation: 'A current verified backup is available.',
    evaluatedAt: '2026-10-15T06:00:00Z',
  })),
  policy: {
    freshnessWindowSeconds: 604800,
    verificationWindowSeconds: 3600,
    requireVerification: true,
  },
});

const requestedIDs = (url: string) =>
  new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId');

const deferred = <T>() => {
  let resolve!: (value: T) => void;
  const promise = new Promise<T>((resolvePromise) => (resolve = resolvePromise));
  return { promise, resolve };
};

describe('protection verdict ownership', () => {
  it.each([1, MAX_PROTECTION_POSTURE_BATCH_SIZE + 1])(
    'withdraws every current verdict and its rule when a refresh of %i workloads fails',
    async (count) => {
      const ids = Array.from({ length: count }, (_, index) => `vm:${index}`);
      apiFetchJSONMock.mockImplementation(async (url: string) =>
        postureResponse(requestedIDs(url)),
      );
      let dispose = () => {};
      const hook = createRoot((stop) => {
        dispose = stop;
        return useProtectionPostures(() => ids);
      });
      try {
        await vi.waitFor(() => expect(hook.postures()).toHaveLength(count));
        apiFetchJSONMock.mockImplementation(async (url: string) => {
          if (requestedIDs(url).includes(ids.at(-1)!)) throw new Error('Unavailable');
          return postureResponse(requestedIDs(url), 'attention');
        });
        await hook.refetch();
        expect(hook.response.error).toBeInstanceOf(Error);
        expect(hook.resolvedOnce()).toBe(true);
        expect(hook.postures()).toEqual([]);
        expect(hook.postureByResourceID().size).toBe(0);
        expect(hook.policy()).toEqual({
          freshnessWindowSeconds: 0,
          verificationWindowSeconds: 0,
          requireVerification: false,
        });
      } finally {
        dispose();
      }
    },
  );

  it('keeps a failed verdict unavailable through a pending retry, then accepts the new result', async () => {
    apiFetchJSONMock.mockResolvedValue(postureResponse(['vm:1']));
    let dispose = () => {};
    const hook = createRoot((stop) => {
      dispose = stop;
      return useProtectionPostures(() => ['vm:1']);
    });
    try {
      await vi.waitFor(() => expect(hook.postures()).toHaveLength(1));
      apiFetchJSONMock.mockRejectedValue(new Error('Unavailable'));
      await hook.refetch();
      const pending = deferred<ReturnType<typeof postureResponse>>();
      apiFetchJSONMock.mockReturnValue(pending.promise);
      const retry = hook.refetch();
      expect(hook.response.loading).toBe(true);
      expect(hook.postures()).toEqual([]);
      pending.resolve(postureResponse(['vm:1'], 'unprotected'));
      await retry;
      expect(hook.response.error).toBeNull();
      expect(hook.postureByResourceID().get('vm:1')?.state).toBe('unprotected');
      expect(hook.policy().requireVerification).toBe(true);
    } finally {
      dispose();
    }
  });

  it('does not revive cached verdicts from a failed read when the tab remounts', async () => {
    apiFetchJSONMock.mockResolvedValue(postureResponse(['vm:1']));
    let dispose = () => {};
    const mount = () =>
      createRoot((stop) => {
        dispose = stop;
        return useProtectionPostures(() => ['vm:1']);
      });
    const first = mount();
    try {
      await vi.waitFor(() => expect(first.postures()).toHaveLength(1));
      apiFetchJSONMock.mockRejectedValue(new Error('Unavailable'));
      await first.refetch();
    } finally {
      dispose();
    }
    apiFetchJSONMock.mockReturnValue(new Promise(() => {}));
    const remounted = mount();
    try {
      expect(remounted.response.error).toBeInstanceOf(Error);
      expect(remounted.postures()).toEqual([]);
      expect(remounted.policy().freshnessWindowSeconds).toBe(0);
    } finally {
      dispose();
    }
  });

  it('does not lend a previous fleet its verdicts or rule while the replacement is pending', async () => {
    apiFetchJSONMock.mockResolvedValue(postureResponse(['vm:1']));
    const [ids, setIDs] = createSignal(['vm:1']);
    let dispose = () => {};
    const hook = createRoot((stop) => {
      dispose = stop;
      return useProtectionPostures(ids);
    });
    try {
      await vi.waitFor(() => expect(hook.postures()).toHaveLength(1));
      const pending = deferred<ReturnType<typeof postureResponse>>();
      apiFetchJSONMock.mockReturnValue(pending.promise);
      setIDs(['vm:1', 'vm:2']);
      await vi.waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalledTimes(2));
      expect(hook.resolvedOnce()).toBe(false);
      expect(hook.postures()).toEqual([]);
      expect(hook.policy().requireVerification).toBe(false);
      pending.resolve(postureResponse(['vm:1', 'vm:2'], 'attention'));
      await vi.waitFor(() => expect(hook.postures()).toHaveLength(2));
      expect(hook.postureByResourceID().get('vm:1')?.state).toBe('attention');
    } finally {
      dispose();
    }
  });

  it('passes one abort signal to every batch and aborts disposed reads', async () => {
    apiFetchJSONMock.mockReturnValue(new Promise(() => {}));
    const ids = Array.from({ length: MAX_PROTECTION_POSTURE_BATCH_SIZE + 1 }, (_, i) => `vm:${i}`);
    let dispose = () => {};
    createRoot((stop) => {
      dispose = stop;
      return useProtectionPostures(() => ids);
    });
    try {
      await vi.waitFor(() => expect(apiFetchJSONMock).toHaveBeenCalledTimes(2));
      const signals = apiFetchJSONMock.mock.calls.map(([, options]) => options?.signal);
      expect(signals[0]).toBeInstanceOf(AbortSignal);
      expect(signals[1]).toBe(signals[0]);
      expect(signals[0].aborted).toBe(false);
      dispose();
      expect(signals[0].aborted).toBe(true);
    } finally {
      dispose();
    }
  });
});
