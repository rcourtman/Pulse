import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
import { resetCreateNonSuspendingQueryCacheForTest } from '@/hooks/createNonSuspendingQuery';
import {
  MAX_PROTECTION_POSTURE_BATCH_SIZE,
  buildProtectionPostureBatchURL,
  normalizeProtectionPostureResourceIDs,
  useProtectionPostures,
} from '@/hooks/useProtectionPostures';
import type { ProtectionPosturesResponse, ProtectionState } from '@/types/recovery';
import { protectionPolicy, protectionPosture } from '@/utils/__fixtures__/protectionPosture';
import { ProtectionPostureFormatError } from '@/utils/protectionPostureResponse';

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
          .map((subjectResourceId) => protectionPosture(subjectResourceId)),
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

  it.each<ProtectionState>(['protected', 'attention', 'unprotected', 'unknown'])(
    'withdraws a retained %s assessment if any batch fails, until a complete retry succeeds',
    async (state) => {
      const resourceIDs = Array.from({ length: 201 }, (_, index) => `vm:${index}`);
      const response = (url: string): ProtectionPosturesResponse => ({
        data: new URL(url, 'https://pulse.invalid').searchParams
          .getAll('resourceId')
          .map((subjectResourceId) => ({
            subjectResourceId,
            state,
            freshness: 'current',
            verification: 'verified',
            coverage: 'complete',
            providerStates: [],
            repositoryResourceIds: [],
            evidenceIds: [],
            explanation: 'Previous assessment',
            evaluatedAt: '2026-10-24T04:15:12Z',
          })),
        policy: {
          freshnessWindowSeconds: 604800,
          verificationWindowSeconds: 3600,
          requireVerification: true,
        },
      });
      apiFetchJSONMock.mockImplementation(async (url: string) => response(url));
      let dispose = () => {};
      const hook = createRoot((rootDispose) => {
        dispose = rootDispose;
        return useProtectionPostures(() => resourceIDs);
      });
      try {
        await vi.waitFor(() => expect(hook.postures()).toHaveLength(201));
        const failure = new Error('Second batch failed');
        apiFetchJSONMock.mockImplementation(async (url: string) => {
          if (new URL(url, 'https://pulse.invalid').searchParams.getAll('resourceId').length === 1)
            throw failure;
          return response(url);
        });
        const failedRead = hook.refetch();
        expect(hook.postures()).toHaveLength(201);
        await failedRead;
        expect(hook.response.error).toBe(failure);
        expect(hook.resolvedOnce()).toBe(true);
        expect(hook.postures()).toEqual([]);
        expect(hook.postureByResourceID().size).toBe(0);
        expect(hook.policy().freshnessWindowSeconds).toBe(0);

        const pending: (() => void)[] = [];
        apiFetchJSONMock.mockImplementation(
          (url: string) =>
            new Promise<ProtectionPosturesResponse>((resolve) => {
              pending.push(() => resolve(response(url)));
            }),
        );
        const retry = hook.refetch();
        expect(hook.response.loading).toBe(true);
        expect(hook.postures()).toEqual([]);
        expect(pending).toHaveLength(2);
        pending[0]();
        await Promise.resolve();
        expect(hook.postures()).toEqual([]);
        pending[1]();
        await retry;
        expect(hook.response.error).toBeNull();
        expect(hook.postures()).toHaveLength(201);
        expect(hook.policy().freshnessWindowSeconds).toBe(604800);
      } finally {
        dispose();
      }
    },
  );

  it('does not restore a failed cached assessment when Coverage remounts', async () => {
    apiFetchJSONMock.mockResolvedValue({
      data: [protectionPosture('vm:1')],
      policy: protectionPolicy,
    });
    let dispose = () => {};
    const mount = () =>
      createRoot((rootDispose) => {
        dispose = rootDispose;
        return useProtectionPostures(() => ['vm:1']);
      });
    const original = mount();
    await vi.waitFor(() => expect(original.postures()).toHaveLength(1));
    apiFetchJSONMock.mockRejectedValue(new Error('Refresh failed'));
    await original.refetch();
    dispose();

    apiFetchJSONMock.mockImplementation(() => new Promise(() => {}));
    const remounted = mount();
    try {
      expect(remounted.response.error).toBeTruthy();
      expect(remounted.postures()).toEqual([]);
      expect(remounted.postureByResourceID().size).toBe(0);
      expect(remounted.policy().freshnessWindowSeconds).toBe(0);
    } finally {
      dispose();
    }
  });

  it.each(['row', 'policy', 'duplicate', 'mixed policies'])(
    'withdraws the whole 201-subject read after a malformed %s in the last batch',
    async (invalid) => {
      const ids = Array.from({ length: 201 }, (_, index) => `vm:${index}`);
      const validResponse = (url: string) => ({
        data: new URL(url, 'https://pulse.invalid').searchParams
          .getAll('resourceId')
          .map(protectionPosture),
        policy: { ...protectionPolicy },
      });
      apiFetchJSONMock.mockImplementation(async (url: string) => validResponse(url));
      let dispose = () => {};
      const hook = createRoot((rootDispose) => {
        dispose = rootDispose;
        return useProtectionPostures(() => ids);
      });
      try {
        await vi.waitFor(() => expect(hook.postures()).toHaveLength(201));
        apiFetchJSONMock.mockImplementation(async (url: string) => {
          const response = validResponse(url);
          if (response.data.length > 1) return response;
          if (invalid === 'row') return { ...response, data: [null] };
          if (invalid === 'policy')
            return { ...response, policy: { requireVerification: 'false' } };
          if (invalid === 'duplicate') return { ...response, data: [protectionPosture('vm:0')] };
          return { ...response, policy: { ...protectionPolicy, requireVerification: false } };
        });
        await hook.refetch();
        expect(hook.response.error).toBeInstanceOf(ProtectionPostureFormatError);
        expect(hook.postures()).toEqual([]);
        expect(hook.postureByResourceID().size).toBe(0);
        expect(hook.policy().freshnessWindowSeconds).toBe(0);

        apiFetchJSONMock.mockImplementation(async (url: string) => validResponse(url));
        await hook.refetch();
        expect(hook.response.error).toBeNull();
        expect(hook.postures()).toHaveLength(201);
        expect(hook.policy()).toEqual(protectionPolicy);
      } finally {
        dispose();
      }
    },
  );
});
