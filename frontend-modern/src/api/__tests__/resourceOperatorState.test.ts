import { beforeEach, describe, expect, it, vi } from 'vitest';

vi.mock('@/utils/apiClient', () => ({
  apiFetchJSON: vi.fn(),
}));

import {
  clearResourceOperatorState,
  getResourceOperatorState,
  setResourceOperatorState,
  type ResourceOperatorState,
  type ResourceOperatorStateInput,
} from '@/api/resourceOperatorState';
import { apiFetchJSON } from '@/utils/apiClient';

describe('resourceOperatorState api', () => {
  const apiFetchJSONMock = vi.mocked(apiFetchJSON);

  beforeEach(() => {
    apiFetchJSONMock.mockReset();
  });

  it('encodes the resource id segment so colon-bearing canonical ids round-trip safely', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      canonicalId: 'instance:node:101',
      intentionallyOffline: false,
      neverAutoRemediate: false,
      setAt: '2026-05-09T10:00:00Z',
    } satisfies ResourceOperatorState);

    await getResourceOperatorState('instance:node:101');

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      // colons are reserved in URL paths and must be percent-encoded
      // before the canonical id reaches the server router.
      '/api/resources/instance%3Anode%3A101/operator-state',
      { cache: 'no-store' },
    );
  });

  it('returns null when the server reports operator_state_not_set as 404', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(Object.assign(new Error('Not found'), { status: 404 }));

    await expect(getResourceOperatorState('vm:101')).resolves.toBeNull();
  });

  it('rethrows non-404 errors so the caller can surface them to the operator', async () => {
    apiFetchJSONMock.mockRejectedValueOnce(
      Object.assign(new Error('Internal server error'), { status: 500 }),
    );

    await expect(getResourceOperatorState('vm:101')).rejects.toThrow('Internal server error');
  });

  it('PUTs the canonical body shape and returns the read-after-write record', async () => {
    apiFetchJSONMock.mockResolvedValueOnce({
      canonicalId: 'vm:101',
      intentionallyOffline: true,
      neverAutoRemediate: false,
      setAt: '2026-05-09T11:00:00Z',
      setBy: 'operator:richard',
    } satisfies ResourceOperatorState);

    const input: ResourceOperatorStateInput = {
      intentionallyOffline: true,
      neverAutoRemediate: false,
      autoRemediationPolicy: {
        enabled: true,
        capabilityNames: ['restart'],
        window: { timezone: 'Europe/London', startMinute: 60, endMinute: 180 },
      },
    };
    const result = await setResourceOperatorState('vm:101', input);

    expect(apiFetchJSONMock).toHaveBeenCalledWith(
      '/api/resources/vm%3A101/operator-state',
      expect.objectContaining({
        method: 'PUT',
        body: JSON.stringify(input),
      }),
    );
    // The returned record carries server-populated attribution
    // (setAt, setBy) — never echo the input verbatim.
    expect(result.setAt).toBe('2026-05-09T11:00:00Z');
    expect(result.setBy).toBe('operator:richard');
    expect(JSON.parse(apiFetchJSONMock.mock.calls[0][1]?.body as string)).toMatchObject({
      autoRemediationPolicy: {
        enabled: true,
        capabilityNames: ['restart'],
        window: { timezone: 'Europe/London', startMinute: 60, endMinute: 180 },
      },
    });
  });

  it('DELETEs without expecting a body response', async () => {
    apiFetchJSONMock.mockResolvedValueOnce(undefined as never);

    await expect(clearResourceOperatorState('vm:101')).resolves.toBeUndefined();
    expect(apiFetchJSONMock).toHaveBeenCalledWith('/api/resources/vm%3A101/operator-state', {
      method: 'DELETE',
    });
  });
});
