import { afterEach, describe, expect, expectTypeOf, it, vi } from 'vitest';
import { createRoot } from 'solid-js';
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
